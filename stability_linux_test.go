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
 for _,w:=range []int{20,40,80,120,200}{
  for _,height:=range []int{10,24,30,48,80}{
   for _,ascii:=range []bool{true,false}{
    frame:=dashboard(now,w,height,ascii,true,time.Second,h,h,h,h,[]volume{{"ROOT",capacity{100,50,40}}},systemInfo{},now,[4]float64{50,50,0,0},cores,hardwareInfo{cpu:"AVG 3 GHz",swap:"SWAP 1/2",gpu:"GPU01 50%",docker:"DOCKER 1 populated scopes / 1 visible"},0,0,1024)
    lines:=strings.Split(frame,"\r\n")
    if len(lines)>height{t.Fatalf("%dx%d: %d lines",w,height,len(lines))}
    for _,line:=range lines{if visibleWidth(line)>min(140,w-1){t.Fatalf("%dx%d: line too wide",w,height)}}
   }
  }
 }
}
