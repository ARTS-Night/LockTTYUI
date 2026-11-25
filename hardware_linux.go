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
func discoverDisks() []string {
 var disks []string
 paths,_:=filepath.Glob("/sys/block/*")
 for _,p:=range paths{
  name:=filepath.Base(p)
  if strings.HasPrefix(name,"loop")||strings.HasPrefix(name,"ram")||strings.HasPrefix(name,"zram")||strings.HasPrefix(name,"sr"){continue}
  if _,e:=os.Stat(filepath.Join(p,"device"));e!=nil{continue}
  slaves,e:=os.ReadDir(filepath.Join(p,"slaves"));if e!=nil||len(slaves)>0{continue}
  disks=append(disks,p)
 }
 return disks
}
func readDiskTraffic(disks []string,reuse ...map[string]traffic) map[string]traffic {
 var out map[string]traffic
 if len(reuse)>0{out=reuse[0]}
 if out==nil{out=make(map[string]traffic,len(disks))}else{clear(out)}
 for _,p:=range disks{
  name:=filepath.Base(p)
  b,e:=os.ReadFile(filepath.Join(p,"stat"));if e!=nil{continue}
  f:=strings.Fields(string(b));if len(f)<7{continue}
  a,e:=strconv.ParseUint(f[2],10,64);z,e2:=strconv.ParseUint(f[6],10,64)
  if e==nil&&e2==nil&&a<=^uint64(0)/512&&z<=^uint64(0)/512{out[name]=traffic{a*512,z*512}}
 }
 return out
}

type hardwareInfo struct{cpu,gpu,docker string; ram,swapCapacity capacity; swapKnown bool; statuses []statusItem; checking bool; events *eventStream; failedUnits uint64; failedKnown bool; volumes []volume; disks []string; io ioTelemetry}
type hardwareCollector struct{
 discovered time.Time
 failedChecked time.Time
 failedUnits uint64
 failedKnown bool
 mounts,disks,freq,temps,cards []string
 nvidia bool
}
func (c *hardwareCollector) discover(now time.Time){
 c.mounts=discoverMountPaths()
 c.disks=discoverDisks()
 c.freq,_=filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq")
 c.temps=nil
 sensors,_:=filepath.Glob("/sys/class/hwmon/hwmon*/name")
 for _,p:=range sensors{
  b,e:=os.ReadFile(p);if e!=nil{continue}
  switch strings.TrimSpace(string(b)){case "coretemp","k10temp","zenpower":default:continue}
  inputs,_:=filepath.Glob(filepath.Join(filepath.Dir(p),"temp*_input"))
  c.temps=append(c.temps,inputs...)
 }
 c.cards,_=filepath.Glob("/sys/class/drm/card[0-9]*")
 _,err:=os.Stat("/usr/bin/nvidia-smi");c.nvidia=err==nil
 c.discovered=now
}
func readHardware() hardwareInfo{var c hardwareCollector;return c.read()}
func (c *hardwareCollector) read() hardwareInfo{
 now:=time.Now()
 if c.discovered.IsZero()||now.Sub(c.discovered)>=30*time.Second{c.discover(now)}
 h:=hardwareInfo{volumes:readVolumePaths(c.mounts),disks:c.disks}
 paths:=c.freq
 var sum float64;var count int
 for _,p:=range paths{if n,ok:=readNumber(p);ok&&n>0{sum+=float64(n);count++}}
 if count>0{h.cpu=fmt.Sprintf("AVG %.2f GHz",sum/float64(count)/1e6)}
 var maxTemp uint64
 for _,input:=range c.temps{if n,ok:=readNumber(input);ok&&n<=150000&&n>maxTemp{maxTemp=n}}
 if maxTemp>0{h.cpu=strings.TrimSpace(fmt.Sprintf("%s  TEMP %.0fC",h.cpu,float64(maxTemp)/1000))}
 cards:=c.cards
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
 if h.gpu==""&&c.nvidia{h.gpu=readNVIDIA()}
 h.docker=readDockerSummary()
 h.statuses=readServiceStatus()
 if c.failedChecked.IsZero()||now.Sub(c.failedChecked)>=15*time.Second{
  c.failedUnits,c.failedKnown=readFailedCount()
  c.failedChecked=now
 }
 h.failedUnits,h.failedKnown=c.failedUnits,c.failedKnown
 return h
}

// Query numeric GPU fields only; no shell, device identities or process information.
// This optional driver tool is bounded and may be absent or blocked by confinement.
func readNVIDIA() string{
 if _,e:=os.Stat("/usr/bin/nvidia-smi");e!=nil{return ""}
 ctx,cancel:=context.WithTimeout(context.Background(),350*time.Millisecond);defer cancel()
 cmd:=exec.CommandContext(ctx,"/usr/bin/nvidia-smi","--query-gpu=utilization.gpu,memory.used,memory.total,temperature.gpu","--format=csv,noheader,nounits")
 cmd.WaitDelay=100*time.Millisecond
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
 var values [4]uint64
 var known [4]bool
 for _,line:=range strings.Split(data,"\n"){
  key,rest,ok:=strings.Cut(line,":");if !ok{continue}
  idx:=0
  switch key{case "MemTotal":idx=0;case "MemAvailable":idx=1;case "SwapTotal":idx=2;case "SwapFree":idx=3;default:continue}
  f:=strings.Fields(rest);if len(f)<2||f[1]!="kB"{continue}
  n,e:=strconv.ParseUint(f[0],10,64);if e==nil&&n<=^uint64(0)/1024{values[idx]=n*1024;known[idx]=true}
 }
 t,tok:=values[0],known[0];a,aok:=values[1],known[1]
 if tok&&aok&&t>0&&a<=t{ram=capacity{t,t-a,a}}
 t,tok=values[2],known[2];a,aok=values[3],known[3]
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
