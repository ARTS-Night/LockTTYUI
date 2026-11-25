//go:build linux

package main

import (
 "bufio"
 "fmt"
 "io"
 "math"
 "os"
 "strconv"
 "strings"
 "time"
)

// Linux /proc/vmstat: pgpgin/pgpgout are KiB; pswpin/pswpout are pages.
// These are storage-backed paging counters, NOT physical DRAM bandwidth.
type vmCounters struct {
 pageIn, pageOut, swapIn, swapOut uint64
 known [4]bool
}
func parseVMCounters(r io.Reader) (vmCounters,error) {
 var v vmCounters
 scanner:=bufio.NewScanner(r)
 for scanner.Scan() {
  key,value,ok:=strings.Cut(scanner.Text()," ")
  if !ok {continue}
  idx:=-1
  switch key {
  case "pgpgin":idx=0
  case "pgpgout":idx=1
  case "pswpin":idx=2
  case "pswpout":idx=3
  default:continue
  }
  n,err:=strconv.ParseUint(strings.TrimSpace(value),10,64)
  if err!=nil {continue}
  if idx<2 {
   if n>math.MaxUint64/1024 {continue}
   n*=1024
  }else{
   pageSize:=uint64(os.Getpagesize())
   if n>math.MaxUint64/pageSize {continue}
   n*=pageSize
  }
  switch idx {
  case 0:v.pageIn=n
  case 1:v.pageOut=n
  case 2:v.swapIn=n
  case 3:v.swapOut=n
  }
  v.known[idx]=true
 }
 return v,scanner.Err()
}
func readVMCounters()(vmCounters,error){
 f,err:=os.Open("/proc/vmstat")
 if err!=nil{return vmCounters{},err}
 defer f.Close()
 return parseVMCounters(f)
}
func vmRates(prev,next vmCounters,elapsed time.Duration)[4]float64 {
 rates:=[4]float64{-1,-1,-1,-1}
 if elapsed<=0{return rates}
 a:=[4]uint64{prev.pageIn,prev.pageOut,prev.swapIn,prev.swapOut}
 b:=[4]uint64{next.pageIn,next.pageOut,next.swapIn,next.swapOut}
 for i:=range rates {
  if prev.known[i]&&next.known[i]&&b[i]>=a[i]{
   rates[i]=float64(b[i]-a[i])/elapsed.Seconds()
  }
 }
 return rates
}
const (
 ioNetRX=iota
 ioNetTX
 ioDiskRead
 ioDiskWrite
 ioPageIn
 ioPageOut
 ioSwapIn
 ioSwapOut
 ioCount
)
type activityPulse struct{
 known bool
 lastActive time.Time
}
func (p *activityPulse) update(rate float64,now time.Time){
 p.known=rate>=0&&!math.IsNaN(rate)&&!math.IsInf(rate,0)
 if p.known&&rate>=1 {p.lastActive=now}
}
func (p activityPulse) active(now time.Time)bool{
 d:=now.Sub(p.lastActive)
 return p.known&&!p.lastActive.IsZero()&&d>=0&&d<900*time.Millisecond
}
// Avoid blink toggling on every fast metric sample. A single icon stays lit for
// 900ms after traffic and remains dim during idle periods.
type ioTelemetry struct {
 rates [ioCount]float64
 pulses [ioCount]activityPulse
}
func (s *ioTelemetry) update(now time.Time,rates [ioCount]float64){
 s.rates=rates
 for i,rate:=range rates{s.pulses[i].update(rate,now)}
}
func (s ioTelemetry) indicator(id int,now time.Time,ascii bool,upstream bool)string{
 p:=s.pulses[id]
 if !p.known{return "\x1b[90m?\x1b[0m"}
 if !p.active(now){
  if ascii{return "\x1b[90m.\x1b[0m"}
  return "\x1b[90m○\x1b[0m"
 }
 mark:="●";if ascii{mark="*"}
 // Green indicates incoming/reads, red indicates outgoing/writes.
 if upstream{return "\x1b[31m"+mark+"\x1b[0m"}
 return "\x1b[32m"+mark+"\x1b[0m"
}
func (s ioTelemetry) ioLabel(id int,now time.Time,ascii bool,upstream bool,name string)string {
 return fmt.Sprintf("%s %-8s %s",s.indicator(id,now,ascii,upstream),name,rateText(s.rateAt(id)))
}
func (s ioTelemetry) rateAt(id int)float64{
 if !s.pulses[id].known{return -1}
 return s.rates[id]
}
func memoryIOLines(s ioTelemetry,now time.Time,ascii bool,detail int,width int) []string{
 if detail<1||width<28{return nil}
 page:=fmt.Sprintf("PAGE IN %s  OUT %s",s.ioLabel(ioPageIn,now,ascii,false,""),s.ioLabel(ioPageOut,now,ascii,true,""))
 if width<56{
  page=fmt.Sprintf("PAGE %s IN  %s OUT",rateText(s.rateAt(ioPageIn)),rateText(s.rateAt(ioPageOut)))
 }
 lines:=[]string{"\x1b[90m// MEMORY I/O (PAGING)\x1b[0m",page}
 if detail>=2{
  swap:=fmt.Sprintf("SWAP IN %s  OUT %s",s.ioLabel(ioSwapIn,now,ascii,false,""),s.ioLabel(ioSwapOut,now,ascii,true,""))
  if width<56{swap=fmt.Sprintf("SWAP %s IN  %s OUT",rateText(s.rateAt(ioSwapIn)),rateText(s.rateAt(ioSwapOut)))}
  lines=append(lines,swap)
 }
 return lines
}
