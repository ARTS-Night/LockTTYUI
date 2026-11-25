//go:build linux

package main

import (
 "strconv"
 "strings"
)

// One reusable scratch buffer owned by the terminal loop. No history of frames.
type frameWriter struct{scratch []byte}
func (w *frameWriter) delta(previous,current string,resize bool)[]byte{
 w.scratch=w.scratch[:0]
 if resize||previous==""{return w.full(current)}
 if previous==current{return w.scratch}
 old:=strings.TrimSuffix(strings.TrimPrefix(previous,"\x1b[H"),"\x1b[J")
 now:=strings.TrimSuffix(strings.TrimPrefix(current,"\x1b[H"),"\x1b[J")
 for row:=1;;row++{
  a,restA,moreA:=strings.Cut(old,"\r\n")
  b,restB,moreB:=strings.Cut(now,"\r\n")
  if moreA!=moreB{return w.full(current)}
  if a!=b{
   w.scratch=append(w.scratch,"\x1b["...)
   w.scratch=strconv.AppendInt(w.scratch,int64(row),10)
   w.scratch=append(w.scratch,";1H\x1b[0m"...)
   w.scratch=append(w.scratch,b...)
   if len(w.scratch)>=len(current){return w.full(current)}
  }
  if !moreB{break}
  old,now=restA,restB
 }
 return w.scratch
}
func (w *frameWriter) full(s string)[]byte{
 w.scratch=append(w.scratch[:0],s...)
 return w.scratch
}
func frameDelta(previous,current string,resize bool)string{
 var w frameWriter
 return string(w.delta(previous,current,resize))
}
