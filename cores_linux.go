//go:build linux

package main

import (
 "bufio"
 "io"
 "os"
 "strconv"
 "strings"
)

type coreSeries struct {
 id int
 live float64
}

func parseCores(r io.Reader) map[int]cpu {
 result:=map[int]cpu{}
 s:=bufio.NewScanner(r)
 for s.Scan(){
  f:=strings.Fields(s.Text())
  if len(f)<9 || !strings.HasPrefix(f[0],"cpu") || f[0]=="cpu"{continue}
  id,e:=strconv.Atoi(f[0][3:])
  if e!=nil||id<0{continue}
  var c cpu
  valid:=true
  for i:=1;i<=8;i++ {
   v,e:=strconv.ParseUint(f[i],10,64)
   if e!=nil{valid=false;break}
   c.total+=v
   if i==4||i==5{c.idle+=v}
  }
  if valid{result[id]=c}
 }
 return result
}
func readCores() map[int]cpu {
 f,e:=os.Open("/proc/stat");if e!=nil{return nil};defer f.Close()
 return parseCores(f)
}
func updateCores(series map[int]*coreSeries,prev,next map[int]cpu){
 for id:=range series {if _,ok:=next[id];!ok{delete(series,id)}}
 for id,c:=range next{
  s:=series[id]
  if s==nil{
   s=&coreSeries{id:id,live:-1}
   series[id]=s
  }
  s.live=-1
  if p,ok:=prev[id];ok{s.live=cpuUsage(p,c)}
 }
}
