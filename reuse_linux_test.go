//go:build linux
package main

import (
 "strings"
 "testing"
 "time"
)

func TestMetricMapReuse(t *testing.T){
 cpuDst:=map[int]cpu{999:{total:1}}
 cpuData:="cpu 10 0 0 10 0 0 0 0\ncpu0 5 0 0 5 0 0 0 0\n"
 _,got,ok:=parseCPUSnapshotBuffer(strings.NewReader(cpuData),make([]byte,65536),cpuDst)
 if !ok||len(got)!=1||got[0].total!=10{t.Fatalf("cpu reuse: %v %v",got,ok)}
 if _,exists:=got[999];exists{t.Fatal("stale CPU entry retained")}
 netDst:=map[string]traffic{"stale":{rx:1}}
 netData:="Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n eth0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\n"
 net,err:=parseTrafficBuffer(strings.NewReader(netData),make([]byte,65536),netDst)
 if err!=nil||len(net)!=1||net["eth0"].rx!=100{t.Fatalf("net reuse: %v %v",net,err)}
 if _,exists:=net["stale"];exists{t.Fatal("stale network entry retained")}
}
func TestFrameWriterReusesBuffer(t *testing.T){
 w:=frameWriter{}
 first:="\x1b[Hhello\x1b[K\x1b[J"
 second:="\x1b[Hworld\x1b[K\x1b[J"
 if got:=string(w.delta("",first,true));got!=first{t.Fatal("full redraw changed")}
 if got:=string(w.delta(first,second,false));!strings.Contains(got,"world"){t.Fatalf("delta missing update: %q",got)}
 if got:=w.delta(second,second,false);len(got)!=0{t.Fatal("unchanged frame generated output")}
}
func TestActivityIndicator(t *testing.T){
 now:=time.Now()
 if s:=activityMark(10,now,now,false,"6");!strings.Contains(s,"●"){t.Fatalf("active: %q",s)}
 if s:=activityMark(0,now,now,false,"6");!strings.Contains(s,"○"){t.Fatalf("idle: %q",s)}
 if s:=activityMark(-1,now,now,false,"6");!strings.Contains(s,"?"){t.Fatalf("unknown: %q",s)}
}
