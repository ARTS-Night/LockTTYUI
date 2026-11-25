//go:build linux

package main

import (
 "fmt"
 "context"
 "os/exec"
 "time"
 "os"
 "path/filepath"
 "strconv"
 "strings"
)

func readNumber(path string)(uint64,bool){
 b,e:=os.ReadFile(path);if e!=nil{return 0,false}
 n,e:=strconv.ParseUint(strings.TrimSpace(string(b)),10,64);return n,e==nil
}

// Physical/virtual whole leaf disks only; exclude partitions and stacked devices.
func readDiskTraffic() map[string]traffic {
 out:=map[string]traffic{}
 paths,_:=filepath.Glob("/sys/block/*")
 for _,p:=range paths{
  name:=filepath.Base(p)
  if strings.HasPrefix(name,"loop")||strings.HasPrefix(name,"ram")||strings.HasPrefix(name,"zram")||strings.HasPrefix(name,"sr"){continue}
  if _,e:=os.Stat(filepath.Join(p,"device"));e!=nil{continue}
  slaves,e:=os.ReadDir(filepath.Join(p,"slaves"));if e!=nil||len(slaves)>0{continue}
  b,e:=os.ReadFile(filepath.Join(p,"stat"));if e!=nil{continue}
  f:=strings.Fields(string(b));if len(f)<7{continue}
  a,e:=strconv.ParseUint(f[2],10,64);z,e2:=strconv.ParseUint(f[6],10,64)
  if e==nil&&e2==nil&&a<=^uint64(0)/512&&z<=^uint64(0)/512{out[name]=traffic{a*512,z*512}}
 }
 return out
}

type hardwareInfo struct{cpu,gpu,docker string; ram,swapCapacity capacity; swapKnown bool}
func readHardware() hardwareInfo{
 h:=hardwareInfo{}
 paths,_:=filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq")
 var sum float64;var count int
 for _,p:=range paths{if n,ok:=readNumber(p);ok&&n>0{sum+=float64(n);count++}}
 if count>0{h.cpu=fmt.Sprintf("AVG %.2f GHz",sum/float64(count)/1e6)}
 sensors,_:=filepath.Glob("/sys/class/hwmon/hwmon*/name")
 var maxTemp uint64
 for _,p:=range sensors{
  b,e:=os.ReadFile(p);if e!=nil{continue}
  switch strings.TrimSpace(string(b)){case "coretemp","k10temp","zenpower":default:continue}
  inputs,_:=filepath.Glob(filepath.Join(filepath.Dir(p),"temp*_input"))
  for _,input:=range inputs{if n,ok:=readNumber(input);ok&&n<=150000&&n>maxTemp{maxTemp=n}}
 }
 if maxTemp>0{h.cpu=strings.TrimSpace(fmt.Sprintf("%s  TEMP %.0fC",h.cpu,float64(maxTemp)/1000))}
 cards,_:=filepath.Glob("/sys/class/drm/card[0-9]*")
 idx:=0
 for _,p:=range cards{
  if strings.Contains(filepath.Base(p),"-"){continue}
  dev:=filepath.Join(p,"device")
  busy,ok:=readNumber(filepath.Join(dev,"gpu_busy_percent"))
  used,u:=readNumber(filepath.Join(dev,"mem_info_vram_used"))
  total,t:=readNumber(filepath.Join(dev,"mem_info_vram_total"))
  if !ok&&!(u&&t&&total>0){continue}
  idx++
  if idx>1{h.gpu+="  ";};h.gpu+=fmt.Sprintf("GPU%02d",idx)
  if ok&&busy<=100{h.gpu+=fmt.Sprintf(" %d%%",busy)}
  if u&&t&&total>0&&used<=total{h.gpu+=fmt.Sprintf(" VRAM %s/%s",sizeText(used),sizeText(total))}
  if idx>=2{break}
 }
 if h.gpu==""{h.gpu=readNVIDIA()}
 h.docker=readDockerSummary()
 return h
}

// Query numeric GPU fields only; no shell, device identities or process information.
// This optional driver tool is bounded and may be absent or blocked by confinement.
func readNVIDIA() string{
 if _,e:=os.Stat("/usr/bin/nvidia-smi");e!=nil{return ""}
 ctx,cancel:=context.WithTimeout(context.Background(),350*time.Millisecond);defer cancel()
 cmd:=exec.CommandContext(ctx,"/usr/bin/nvidia-smi","--query-gpu=utilization.gpu,memory.used,memory.total,temperature.gpu","--format=csv,noheader,nounits")
 cmd.Env=[]string{"PATH=/usr/bin:/bin","LC_ALL=C"}
 b,e:=cmd.Output();if e!=nil{return ""}
 return parseNVIDIA(string(b))
}
func parseNVIDIA(data string)string{
 var lines []string
 for _,line:=range strings.Split(data,"\n"){
  f:=strings.Split(line,",");if len(f)!=4{continue}
  var v [4]uint64;ok:=true
  for i:=range f{n,e:=strconv.ParseUint(strings.TrimSpace(f[i]),10,64);if e!=nil{ok=false;break};v[i]=n}
  if !ok||v[0]>100||v[1]>v[2]||v[2]==0||v[2]>1048576||v[3]>150{continue}
  lines=append(lines,fmt.Sprintf("GPU%02d %d%% VRAM %s/%s TEMP %dC",len(lines)+1,v[0],sizeText(v[1]*1048576),sizeText(v[2]*1048576),v[3]))
  if len(lines)==2{break}
 }
 return strings.Join(lines,"  ")
}

// Best-effort visible cgroup count; never connect to the Docker socket.
// A missing or inaccessible Docker hierarchy is not evidence of zero containers.
func readDockerSummary()string{
 paths,_:=filepath.Glob("/sys/fs/cgroup/system.slice/docker-*.scope")
 if len(paths)==0{return ""}
 active:=0;known:=0
 for _,p:=range paths{
  b,e:=os.ReadFile(filepath.Join(p,"cgroup.events"));if e!=nil{continue}
  for _,line:=range strings.Split(string(b),"\n"){
   if line=="populated 1"{active++;known++;break}
   if line=="populated 0"{known++;break}
  }
 }
 if known==0{return ""}
 return fmt.Sprintf("DOCKER %d populated scopes / %d visible",active,known)
}


func memoryCapacity(data string)(ram,swap capacity,swapKnown bool){
 values:=map[string]uint64{}
 for _,line:=range strings.Split(data,"\n"){
  f:=strings.Fields(line);if len(f)<3||f[2]!="kB"{continue}
  switch f[0]{case "MemTotal:","MemAvailable:","SwapTotal:","SwapFree:":default:continue}
  n,e:=strconv.ParseUint(f[1],10,64);if e==nil&&n<=^uint64(0)/1024{values[f[0]]=n*1024}
 }
 t,tok:=values["MemTotal:"];a,aok:=values["MemAvailable:"]
 if tok&&aok&&t>0&&a<=t{ram=capacity{t,t-a,a}}
 t,tok=values["SwapTotal:"];a,aok=values["SwapFree:"]
 if tok&&aok&&a<=t{swap=capacity{t,t-a,a};swapKnown=true}
 return
}
func readMemoryCapacity()(capacity,capacity,bool){
 b,e:=os.ReadFile("/proc/meminfo");if e!=nil{return capacity{},capacity{},false}
 return memoryCapacity(string(b))
}

func combinedMemoryPercent(hw hardwareInfo)float64{
 if hw.ram.total==0||!hw.swapKnown{return -1}
 total:=float64(hw.ram.total)+float64(hw.swapCapacity.total)
 return 100*(float64(hw.ram.used)+float64(hw.swapCapacity.used))/total
}
