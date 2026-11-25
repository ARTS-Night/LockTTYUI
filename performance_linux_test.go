//go:build linux

package main

import (
 "strings"
 "testing"
 "time"
)

func BenchmarkFrameDelta(b *testing.B){
 var w frameWriter
 previous:="\x1b[H"+strings.Repeat("unchanged\x1b[K\r\n",100)+"old\x1b[K\x1b[J"
 current:="\x1b[H"+strings.Repeat("unchanged\x1b[K\r\n",100)+"new\x1b[K\x1b[J"
 b.ReportAllocs()
 for i:=0;i<b.N;i++{w.delta(previous,current,false)}
}
func BenchmarkGraphCacheHit(b *testing.B){
 c:=dashboardCache{}
 key:=graphKey{width:120,rows:24,period:time.Second}
 draw:=func()[]string{return []string{"test"}}
 c.graph(0,key,draw)
 b.ReportAllocs()
 for i:=0;i<b.N;i++{c.graph(0,key,draw)}
}
func TestBoundedCache(t *testing.T){
 c:=dashboardCache{}
 for i:=0;i<10000;i++{
  key:=graphKey{width:80+i%5,rows:10+i%3}
  c.graph(0,key,func()[]string{return []string{"a"}})
  c.invalidate()
  if c.graphs[0].valid{t.Fatal("cache survived invalidation")}
 }
}
