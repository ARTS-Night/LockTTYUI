//go:build linux

package main

import (
 "strings"
 "testing"
 "time"
)

func TestBalancedCPUAndMemoryHistoryHeight(t *testing.T) {
 cases:=[]struct{
  name string
  side bool
  cpu,mem,rows int
  wantCPU,wantMem int
 }{
  {"screenshot-memory-has-extra-detail",true,3,7,2,6,2},
  {"cpu-more-detail",true,7,3,2,2,6},
  {"equal-detail",true,4,4,3,3,3},
  {"single-column-keeps-independent-panels",false,3,7,2,2,2},
 }
 for _,c:=range cases{
  t.Run(c.name,func(t *testing.T){
   cpuRows,memRows:=balancedGraphRows(c.rows,c.cpu,c.mem,c.side)
   if cpuRows!=c.wantCPU||memRows!=c.wantMem{
    t.Fatalf("graph rows CPU=%d MEM=%d; expected %d/%d",cpuRows,memRows,c.wantCPU,c.wantMem)
   }
   if c.side&&cpuRows+c.cpu!=memRows+c.mem{
    t.Fatalf("side-by-side panels must have equal heights: CPU %d MEM %d",cpuRows+c.cpu,memRows+c.mem)
   }
  })
 }
}

func TestIdleCPUHasSubtleReferenceAndNoFakeBar(t *testing.T){
 var b strings.Builder
 zeros:=make([]float64,60)
 barHistory(&b,"",zeros,48,6,false,time.Second,0,"7")
 out:=b.String()
 if !strings.Contains(out," 50 |\x1b[90m"){
  t.Fatalf("idle graph should expose a dim 50%% reference, got %q",out)
 }
 if strings.Contains(out,"\x1b[47m"){
  t.Fatal("an idle CPU must not show white fill")
 }
 for _,line:=range splitLines(out){
  if strings.Contains(line," 50 |")&&visibleWidth(line)!=53{
   t.Fatalf("reference line width does not match graph width: %d",visibleWidth(line))
  }
 }
 if visibleWidth(cpuReferenceLine(37,true))!=37{
  t.Fatal("ASCII guide must stay terminal-cell accurate")
 }
}

func TestRAMAndSwapHaveDistinctMonochromePalette(t *testing.T){
 hw:=hardwareInfo{
  ram:capacity{total:100,used:50,available:50},
  swapCapacity:capacity{total:100,used:25,available:75},
  swapKnown:true,
 }
 s:=strings.Join(memoryMeterLines(hw,36,true),"\n")
 if !strings.Contains(s,"\x1b[47m")||!strings.Contains(s,"\x1b[100m"){
  t.Fatalf("expected white RAM and grey SWAP bars: %q",s)
 }
}
