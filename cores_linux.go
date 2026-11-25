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

func parseCPUSnapshot(r io.Reader,reuse ...map[int]cpu) (cpu,map[int]cpu,bool) {
 return parseCPUSnapshotBuffer(r,nil,reuse...)
}
func parseCPUSnapshotBuffer(r io.Reader,buffer []byte,reuse ...map[int]cpu) (cpu,map[int]cpu,bool) {
 var total cpu
 found:=false
 var result map[int]cpu
 if len(reuse)>0{result=reuse[0]}
 if result==nil{result=make(map[int]cpu)}else{clear(result)}
 s:=bufio.NewScanner(r)
 if buffer!=nil{s.Buffer(buffer,65536)}
 for s.Scan(){
  line:=s.Text()
  if !strings.HasPrefix(line,"cpu"){continue}
  f:=strings.Fields(line)
  if len(f)<9 {continue}
  id:=-1
  if f[0]!="cpu"{var e error;id,e=strconv.Atoi(f[0][3:]);if e!=nil||id<0{continue}}
  var c cpu
  valid:=true
  for i:=1;i<=8;i++ {
   v,e:=strconv.ParseUint(f[i],10,64)
   if e!=nil{valid=false;break}
   c.total+=v
   if i==4||i==5{c.idle+=v}
  }
  if valid{if id<0{total=c;found=true}else{result[id]=c}}
 }
 return total,result,found&&s.Err()==nil
}
func parseCores(r io.Reader)map[int]cpu{_,cores,_:=parseCPUSnapshot(r);return cores}
func readCPUSnapshot(reuse ...map[int]cpu)(cpu,map[int]cpu,bool){return readCPUBuffer(nil,reuse...)}
func readCPUBuffer(buffer []byte,reuse ...map[int]cpu)(cpu,map[int]cpu,bool){
 f,e:=os.Open("/proc/stat");if e!=nil{if len(reuse)>0&&reuse[0]!=nil{clear(reuse[0]);return cpu{},reuse[0],false};return cpu{},nil,false};defer f.Close()
 return parseCPUSnapshotBuffer(f,buffer,reuse...)
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
