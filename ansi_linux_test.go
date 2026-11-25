//go:build linux
package main
import ("strings";"testing")
func TestANSICompressionPreservesCells(t *testing.T){
 for _,input:=range []string{
  "\x1b[44m \x1b[0m\x1b[44m \x1b[0m",
  "\x1b[34m▄\x1b[0m\x1b[35m▄\x1b[0m",
  "   ",
 }{
  got:=compressGraphLine(input)
  if visibleWidth(got)!=visibleWidth(input){t.Fatalf("visible width changed: %q -> %q",input,got)}
  if strings.Count(got,"\x1b[")>strings.Count(input,"\x1b["){t.Fatalf("ANSI controls increased: %q",got)}
 }
}
func BenchmarkANSICompression(b *testing.B){
 input:=strings.Repeat("\x1b[44m \x1b[0m",160)
 b.ReportAllocs()
 for i:=0;i<b.N;i++{compressGraphLine(input)}
}
