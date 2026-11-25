//go:build linux

package main

import (
 "context"
 "fmt"
 "os"
 "os/exec"
 "strings"
 "strconv"
 "time"
)

type statusItem struct{label,state,color string}
type watchedUnit struct{unit,label string}
var watchedUnits=[]watchedUnit{
 {"docker.service","DOCKER"},{"containerd.service","CONTAINERD"},{"sshd.service","SSH"},
 {"firewalld.service","FIREWALL"},{"NetworkManager.service","NETWORK"},
 {"chronyd.service","TIME SYNC"},{"nginx.service","NGINX"},{"httpd.service","HTTPD"},
 {"podman.service","PODMAN"},
}
func unitStatus(label,load,active string)(statusItem,bool){
 if load=="not-found"||load==""{return statusItem{},false}
 item:=statusItem{label,"UNKNOWN","3"}
 if load=="masked"{item.state="MASKED";return item,true}
 if load!="loaded"{return item,true}
 switch active{
 case "active":item.state="ACTIVE";item.color="2"
 case "inactive":item.state="INACTIVE";item.color="1"
 case "failed":item.state="FAILED";item.color="1"
 case "activating","deactivating","reloading":item.state="TRANSITION"
 }
 return item,true
}
func parseServiceStatus(data string)[]statusItem{
 var out []statusItem
 for _,block:=range strings.Split(strings.TrimSpace(data),"\n\n"){
  props:=map[string]string{}
  for _,line:=range strings.Split(block,"\n"){
   k,v,ok:=strings.Cut(line,"=");if ok{props[k]=v}
  }
  for _,u:=range watchedUnits{
   if props["Id"]!=u.unit{continue}
   if item,ok:=unitStatus(u.label,props["LoadState"],props["ActiveState"]);ok{out=append(out,item)}
   break
  }
 }
 return out
}
func readServiceStatus()[]statusItem{
 var out []statusItem
 if _,err:=os.Stat("/run/systemd/system");err==nil{
  args:=[]string{"--system","--no-pager","show","--property=Id,LoadState,ActiveState"}
  for _,u:=range watchedUnits{args=append(args,u.unit)}
  ctx,cancel:=context.WithTimeout(context.Background(),time.Second)
  cmd:=exec.CommandContext(ctx,"/usr/bin/systemctl",args...)
  cmd.Env=[]string{"PATH=/usr/bin:/bin","LC_ALL=C","SYSTEMD_COLORS=0"}
  cmd.WaitDelay=100*time.Millisecond
  data,err:=cmd.Output()
  // systemctl may return nonzero for absent units while still emitting valid blocks.
  if err==nil||ctx.Err()==nil{out=parseServiceStatus(string(data))}
  cancel()
 }
 if data,err:=os.ReadFile("/sys/fs/selinux/enforce");err==nil{
  switch strings.TrimSpace(string(data)){
  case "1":out=append(out,statusItem{"SELINUX","ENFORCING","2"})
  case "0":out=append(out,statusItem{"SELINUX","PERMISSIVE","3"})
  }
 }
 return out
}
func statusLines(items []statusItem,docker string,width int,ascii bool,checking ...bool)[]string{
 if len(items)==0&&docker==""{return nil}
 scanning:=len(checking)>0&&checking[0]
 title:="\x1b[1;36m// SYSTEM STATUS\x1b[0m"
 if scanning{title+="  \x1b[1;33mSCANNING...\x1b[0m"}
 lines:=[]string{title}
 mark:="●";if ascii{mark="*"}
 cols:=max(1,width/28)
 cell:=width/cols
 for i:=0;i<len(items);i+=cols{
  var b strings.Builder
  for j:=i;j<min(i+cols,len(items));j++{
   v:=items[j]
   if scanning{v.color="3";v.state="CHECKING"}
   b.WriteString(fitLine(fmt.Sprintf("\x1b[3%sm%s\x1b[0m %-12s \x1b[1;3%sm%s\x1b[0m",v.color,mark,v.label,v.color,v.state),cell))
  }
  lines=append(lines,b.String())
 }
 if docker!=""{if scanning{lines=append(lines,"\x1b[33mCONTAINERS CHECKING\x1b[0m")}else{lines=append(lines,docker)}}
 return lines
}

 // Read the system manager's aggregate count; never expose unit names.
func parseFailedCount(data string)(uint64,bool){
 n,err:=strconv.ParseUint(strings.TrimSpace(data),10,64)
 return n,err==nil
}
func readFailedCount()(uint64,bool){
 if _,err:=os.Stat("/run/systemd/system");err!=nil{return 0,false}
 ctx,cancel:=context.WithTimeout(context.Background(),time.Second)
 defer cancel()
 cmd:=exec.CommandContext(ctx,"/usr/bin/systemctl","--system","--no-pager","show","--property=NFailedUnits","--value")
 cmd.Env=[]string{"PATH=/usr/bin:/bin","LC_ALL=C","SYSTEMD_COLORS=0"}
 cmd.WaitDelay=100*time.Millisecond
 data,err:=cmd.Output()
 if err!=nil{return 0,false}
 return parseFailedCount(string(data))
}
func systemIndicator(hw hardwareInfo,ascii bool)string{
 mark:="●";if ascii{mark="*"}
 color,state:="3","N/A"
 if hw.checking{state="CHECKING"}else if hw.failedKnown{
  color="2";if hw.failedUnits>0{color="1"}
  state=fmt.Sprintf("%02d ERR",hw.failedUnits)
 }
 return fmt.Sprintf("\x1b[1;3%sm%s SYSTEM | %s\x1b[0m",color,mark,state)
}
