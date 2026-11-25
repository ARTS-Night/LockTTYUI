//go:build linux

package main

import (
 "strings"
 "testing"
 "time"
)

func TestRateScaleHysteresis(t *testing.T){
 var s rateScale
 s.update(8192)
 for i:=0;i<14;i++{s.update(1024)}
 if s.value!=8192{t.Fatal(s)}
 s.update(1024)
 if s.value!=1024{t.Fatal(s)}
 s.update(16384)
 if s.value!=16384{t.Fatal(s)}
}
func TestNVIDIAFields(t *testing.T){
 got:=parseNVIDIA("50, 4096, 10240, 60\ninvalid, 1, 2, 3")
 if !strings.Contains(got,"50%")||!strings.Contains(got,"TEMP 60C"){t.Fatal(got)}
 if parseNVIDIA("101, 1, 2, 3")!=""{t.Fatal("invalid utilization")}
}
func TestDashboardBounds(t *testing.T){
 h:=make([]float64,60)
 cores:=map[int]*coreSeries{}
 for i:=0;i<64;i++{cores[i]=&coreSeries{id:i,live:50}}
 now:=time.Unix(100,0)
 for _,w:=range []int{20,40,80,120,200,320}{
  for _,height:=range []int{10,24,30,48,80,120}{
   for _,ascii:=range []bool{true,false}{
    frame:=dashboard(now,w,height,ascii,true,time.Second,h,h,h,h,h,[]volume{{"ROOT",capacity{100,50,40}}},systemInfo{},now,[4]float64{50,50,0,0},cores,hardwareInfo{cpu:"AVG 3 GHz",ram:capacity{100,50,50},swapCapacity:capacity{50,10,40},swapKnown:true,gpu:"GPU01 50%",docker:"DOCKER 1 populated scopes / 1 visible"},0,0,1024)
    lines:=strings.Split(frame,"\r\n")
    if len(lines)>height{t.Fatalf("%dx%d: %d lines",w,height,len(lines))}
    for _,line:=range lines{if visibleWidth(line)>min(512,w-1){t.Fatalf("%dx%d: line too wide",w,height)}}
   }
  }
 }
}

func TestStackedMemoryRows(t *testing.T){
 rows:=stackedMemoryRows([]float64{25,50},[]float64{25,50},2,2,false)
 if len(rows)!=2||visibleWidth(rows[0])!=2||visibleWidth(rows[1])!=2{t.Fatal(rows)}
 if !strings.Contains(rows[1],"\x1b[37;100m▄"){t.Fatal("missing mixed cell",rows)}
 if !strings.Contains(rows[0],"\x1b[100m "){t.Fatal("missing swap",rows)}
}

func TestCombinedMemoryPercent(t *testing.T){
 hw:=hardwareInfo{ram:capacity{800,600,200},swapCapacity:capacity{400,100,300},swapKnown:true}
 if p:=combinedMemoryPercent(hw);p<58.33||p>58.34{t.Fatal(p)}
 hw.swapKnown=false
 if combinedMemoryPercent(hw)!=-1{t.Fatal("unknown swap must not be zero")}
}
func TestSmallSwapVisible(t *testing.T){
 rows:=stackedMemoryRows([]float64{50},[]float64{0.01},1,4,false)
 if !strings.Contains(strings.Join(rows,""),"\x1b[90m▄"){t.Fatal(rows)}
}

func TestResponsiveGraphGrowth(t *testing.T){
 h:=make([]float64,60)
 for i:=range h{h[i]=50}
 hw:=hardwareInfo{ram:capacity{100,50,50},swapKnown:true}
 render:=func(w,height int)string{return dashboard(time.Unix(100,0),w,height,true,true,time.Second,h,h,h,h,h,nil,systemInfo{},time.Time{},[4]float64{50,50,0,0},nil,hw,0,0,1024)}
 small,tall,wide:=render(120,48),render(120,90),render(240,48)
 if coloredCells(tall,"\x1b[47m")<=coloredCells(small,"\x1b[47m"){t.Fatal("graph did not grow vertically")}
 if coloredCells(wide,"\x1b[47m")<=coloredCells(small,"\x1b[47m"){t.Fatal("graph did not grow horizontally")}
 if !strings.Contains(wide,"-60s"){t.Fatal("history span changed")}
 if len(strings.Split(tall,"\r\n"))!=90{t.Fatal("footer not anchored")}
}

func coloredCells(frame,code string)int{
 n:=0
 for _,part:=range strings.Split(frame,code)[1:]{
  end:=strings.IndexByte(part,27)
  if end>=0{n+=visibleWidth(part[:end])}
 }
 return n
}
