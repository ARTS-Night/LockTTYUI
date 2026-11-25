//go:build linux
package main
import ("strings";"testing";"time")
func TestFrameDelta(t *testing.T){
 old:="\x1b[Halpha\x1b[K\r\nbeta\x1b[K\x1b[J"
 next:="\x1b[Halpha\x1b[K\r\ngamma\x1b[K\x1b[J"
 got:=frameDelta(old,next,false)
 if strings.Contains(got,"alpha")||!strings.Contains(got,"\x1b[2;1H")||!strings.Contains(got,"gamma"){t.Fatal(got)}
 if frameDelta(next,next,false)!=""||frameDelta(old,next,true)!=next||frameDelta("",next,false)!=next{t.Fatal("incorrect repaint")}
}

func TestScreenRepairCadence(t *testing.T){
 now:=time.Unix(1000,0)
 if !screenRepairDue(now,time.Time{}){t.Fatal("initial render must restore screen")}
 if screenRepairDue(now.Add(screenRepairInterval-time.Nanosecond),now){t.Fatal("repaired too early")}
 if !screenRepairDue(now.Add(screenRepairInterval),now){t.Fatal("screen damage must be repaired within the interval")}
 if !screenRepairDue(now.Add(-time.Second),now){t.Fatal("monotonic clock reset should force repair")}
}
func TestFullRepairRestoresExternalConsoleWrites(t *testing.T){
 // Kernel printk may change physical terminal cells without changing our frame.
 // The normal delta must skip identical content, but the repair forces a full
 // in-place repaint of the frame, including every previously unchanged row.
 s:="\\x1b[HLOCKTTYUI\\x1b[K\\r\\nCPU\\x1b[K\\r\\nMEMORY\\x1b[K\\x1b[J"
 var w frameWriter
 if got:=w.delta(s,s,false);len(got)!=0{t.Fatalf("unchanged frame emitted delta: %q",got)}
 if got:=string(w.delta(s,s,true));got!=s{t.Fatalf("repair did not restore entire frame: %q",got)}
 if strings.Contains(s,"\\x1b[2J"){t.Fatal("repair should not flash screen clear")}
}
