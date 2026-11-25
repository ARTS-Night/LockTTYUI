//go:build linux

package main

import (
 "fmt"
 "strings"
)

// Frames contain only trusted dashboard text and ANSI formatting.
// A resize always uses the original full repaint to clear stale cells.
func frameDelta(previous,current string,resize bool)string{
 if resize||previous==""{return current}
 if previous==current{return ""}
 split:=func(s string)[]string{
  return strings.Split(strings.TrimSuffix(strings.TrimPrefix(s,"\x1b[H"),"\x1b[J"),"\r\n")
 }
 old,now:=split(previous),split(current)
 if len(old)!=len(now){return current}
 var b strings.Builder
 for i,line:=range now{
  if line==old[i]{continue}
  fmt.Fprintf(&b,"\x1b[%d;1H\x1b[0m%s",i+1,line)
 }
 if b.Len()>=len(current){return current}
 return b.String()
}
