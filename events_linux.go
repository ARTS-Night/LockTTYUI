//go:build linux

package main

import (
 "fmt"
 "time"
)

type telemetryEvent struct{at time.Time; level,message string}
type eventStream struct{items [6]telemetryEvent; next,count int}
func (e *eventStream) add(at time.Time,level,message string){
 e.items[e.next]=telemetryEvent{at,level,message}
 e.next=(e.next+1)%len(e.items)
 e.count=min(e.count+1,len(e.items))
}
func (e *eventStream) lines(limit int)[]string{
 if e==nil||e.count==0||limit<=0{return nil}
 lines:=[]string{"\x1b[1;36m// EVENT STREAM\x1b[0m"}
 for i:=min(limit,e.count);i>0;i--{
  v:=e.items[(e.next-i+len(e.items))%len(e.items)]
  color:="6";if v.level=="OK"{color="2"}else if v.level=="WARN"{color="3"}else if v.level=="FAIL"{color="1"}
  lines=append(lines,fmt.Sprintf("%s \x1b[3%sm[%-4s]\x1b[0m %s",v.at.Format("15:04:05"),color,v.level,v.message))
 }
 return lines
}
// Only fixed labels and validated states enter the public console.
func (e *eventStream) recordStatus(at time.Time,old,next []statusItem){
 for _,v:=range next{
  changed:=true
  for _,p:=range old{if p.label==v.label&&p.state==v.state{changed=false;break}}
  if !changed{continue}
  level:="INFO"
  if v.color=="2"{level="OK"}else if v.state=="FAILED"{level="FAIL"}else if v.color=="3"{level="WARN"}
  e.add(at,level,v.label+" "+v.state)
 }
}
