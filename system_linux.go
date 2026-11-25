//go:build linux

package main

import (
 "fmt"
 "os"
 "strconv"
 "strings"
 "time"
)

type systemInfo struct {
 uptime string
 load string
 tasks string
}

func readSystemInfo() systemInfo {
 info:=systemInfo{"N/A","N/A","N/A"}
 if b,e:=os.ReadFile("/proc/uptime");e==nil {
  f:=strings.Fields(string(b))
  if len(f)>0 {
   if v,e:=strconv.ParseFloat(f[0],64);e==nil&&v>=0&&v<1e12 {
    n:=int64(v)
    info.uptime=fmt.Sprintf("%dd %02dh %02dm",n/86400,n%86400/3600,n%3600/60)
   }
  }
 }
 if b,e:=os.ReadFile("/proc/loadavg");e==nil {
  f:=strings.Fields(string(b))
  if len(f)>=4 {
   valid:=true
   for _,s:=range f[:3]{if _,e:=strconv.ParseFloat(s,64);e!=nil{valid=false}}
   if valid{info.load=strings.Join(f[:3],"/")}
   p:=strings.Split(f[3],"/")
   if len(p)==2 {
    a,e:=strconv.ParseUint(p[0],10,64);z,e2:=strconv.ParseUint(p[1],10,64)
    if e==nil&&e2==nil{info.tasks=fmt.Sprintf("%d/%d",a,z)}
   }
  }
 }
 return info
}

func telemetryLine(info systemInfo,w int,now time.Time,lastSample time.Time) string {
 state:="WAIT"
 if !lastSample.IsZero(){state="POLL";if now.Sub(lastSample)<150*time.Millisecond{state="READ"}}
 line:=fmt.Sprintf("UP %s  |  LOAD %s  |  TASKS %s  |  [%s]",info.uptime,info.load,info.tasks,state)
 if visibleWidth(line)>w {line=fmt.Sprintf("UP %s  |  LOAD %s",info.uptime,info.load)}
 return "\x1b[0;37m"+centerLine(line,w)+"\x1b[0m"
}
