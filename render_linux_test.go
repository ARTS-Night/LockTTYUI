//go:build linux
package main
import ("strings";"testing")
func TestFrameDelta(t *testing.T){
 old:="\x1b[Halpha\x1b[K\r\nbeta\x1b[K\x1b[J"
 next:="\x1b[Halpha\x1b[K\r\ngamma\x1b[K\x1b[J"
 got:=frameDelta(old,next,false)
 if strings.Contains(got,"alpha")||!strings.Contains(got,"\x1b[2;1H")||!strings.Contains(got,"gamma"){t.Fatal(got)}
 if frameDelta(next,next,false)!=""||frameDelta(old,next,true)!=next||frameDelta("",next,false)!=next{t.Fatal("incorrect repaint")}
}
