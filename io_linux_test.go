//go:build linux

package main

import (
 "os"
 "strings"
 "testing"
 "time"
)

func TestVMStatPagingUnitConversions(t *testing.T) {
 v,err:=parseVMCounters(strings.NewReader(
  "nr_free_pages 5\npgpgin 12\npgpgout 7\npswpin 3\npswpout 2\n"))
 if err!=nil{t.Fatal(err)}
 for _,ok:=range v.known{if !ok{t.Fatalf("missing vmstat field: %+v",v)}}
 if v.pageIn!=12*1024||v.pageOut!=7*1024||v.swapIn!=3*uint64(os.Getpagesize())||v.swapOut!=2*uint64(os.Getpagesize()){
  t.Fatalf("wrong unit conversion: %+v",v)
 }
}

func TestVMStatDeltaAndUnavailable(t *testing.T){
 a,_:=parseVMCounters(strings.NewReader("pgpgin 5\npgpgout 10\npswpin 4\npswpout 4\n"))
 b,_:=parseVMCounters(strings.NewReader("pgpgin 7\npgpgout 9\npswpin 6\n"))
 v:=vmRates(a,b,2*time.Second)
 if v[0]!=1024||v[1]!=-1||v[2]!=float64(os.Getpagesize())||v[3]!=-1{t.Fatal(v)}
 if v:=vmRates(a,b,0);v!=([4]float64{-1,-1,-1,-1}){t.Fatal(v)}
}

func TestPulseHoldsActivityAndSettles(t *testing.T){
 now:=time.Unix(1000,0)
 var pulse activityPulse
 pulse.update(2048,now)
 if !pulse.active(now.Add(500*time.Millisecond)){t.Fatal("lost burst too soon")}
 pulse.update(0,now.Add(500*time.Millisecond))
 if !pulse.active(now.Add(800*time.Millisecond)){t.Fatal("pulse must not flicker between samples")}
 if pulse.active(now.Add(time.Second)){t.Fatal("pulse did not settle")}
 pulse.update(-1,now.Add(1100*time.Millisecond))
 if pulse.known{t.Fatal("unavailable rate must not look idle")}
 var io ioTelemetry
 var rates [ioCount]float64
 for i:=range rates{rates[i]=-1}
 rates[ioNetRX]=1024;rates[ioDiskWrite]=2048
 io.update(now,rates)
 if !strings.Contains(io.indicator(ioNetRX,now,false,false),"\x1b[32m") {t.Fatal("receive must be green")}
 if !strings.Contains(io.indicator(ioDiskWrite,now,false,true),"\x1b[31m") {t.Fatal("write must be red")}
 if !strings.Contains(io.indicator(ioPageIn,now,false,false),"?"){t.Fatal("unknown must be explicit")}
 if strings.Contains(io.indicator(ioNetRX,now,true,false),"●"){t.Fatal("ascii mode emitted Unicode")}
}

func TestNoBlueCyanMagentaYellowInDashboard(t *testing.T){
 now:=time.Unix(1000,0)
 history:=make([]float64,60)
 for i:=range history{history[i]=35}
 hw:=hardwareInfo{ram:capacity{100,35,65},swapCapacity:capacity{50,10,40},swapKnown:true,
  statuses:[]statusItem{{label:"SSH",state:"ACTIVE",color:"2"},{label:"FIREWALL",state:"FAILED",color:"1"}},
  failedKnown:true,failedUnits:1}
 rates:=[ioCount]float64{30000,4000,5000,8000,4000,5000,0,0}
 hw.io.update(now,rates)
 frame:=dashboard(now,160,100,false,true,time.Second,history,history,history,history,history,
  []volume{{label:"ROOT",capacity:capacity{100,50,50}}},systemInfo{},now,
  [4]float64{35,45,30000,4000},nil,hw,5000,8000,32768)
 for _,want:=range []string{"// NETWORK","DOWNLOAD","UPLOAD","DISK I/O","READ","WRITE","MEMORY I/O (PAGING)","PAGE","SWAP"}{
  if !strings.Contains(frame,want){t.Fatalf("missing dashboard feature %q",want)}
 }
 if !strings.Contains(frame,"\x1b[90m")||!strings.Contains(frame,"\x1b[32m")||!strings.Contains(frame,"\x1b[31m"){
  t.Fatal("missing grey/green/red theme colors")
 }
 for _,bad:=range []string{"\x1b[34m","\x1b[35m","\x1b[36m","\x1b[33m","\x1b[44m","\x1b[45m","\x1b[46m"}{
  if strings.Contains(frame,bad){t.Fatalf("deprecated palette escape %q remains",bad)}
 }
}
