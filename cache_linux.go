//go:build linux

package main

import (
 "strings"
 "time"
)

// Each cache has one entry per graph, bounded by current terminal geometry.
// History generation changes only when data is committed or reset.
type graphKey struct{generation uint64; width,rows int; ascii bool; scale float64; period time.Duration}
type graphEntry struct{key graphKey; valid bool; lines []string}
type dashboardCache struct{generation uint64; graphs [3]graphEntry; ids []int}
func (c *dashboardCache) invalidate(){c.generation++;c.graphs=[3]graphEntry{}}
func (c *dashboardCache) graph(slot int,key graphKey,draw func()[]string)[]string{
 if c==nil{return draw()}
 key.generation=c.generation
 e:=&c.graphs[slot]
 if !e.valid||e.key!=key{e.key=key;e.lines=draw();e.valid=true}
 return e.lines
}

// Compress adjacent equal-color full/half cells without changing their appearance.
func compressGraphLine(s string)string{
 var b strings.Builder
 b.Grow(len(s))
 current:=""
 for len(s)>0{
  if strings.HasPrefix(s,"\x1b["){
   end:=strings.IndexByte(s,'m')
   if end>=0{
    code:=s[:end+1]
    if code=="\x1b[0m"{
     s=s[end+1:]
     if strings.HasPrefix(s,current)&&current!=""{s=s[len(current):];continue}
     b.WriteString(code);current="";continue
    }
    b.WriteString(code);current=code;s=s[end+1:];continue
   }
  }
  b.WriteByte(s[0]);s=s[1:]
 }
 return b.String()
}
