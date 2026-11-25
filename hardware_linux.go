//go:build linux

package main

import (
 "fmt"
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

type hardwareInfo struct{swap,cpu,gpu string}
func readHardware() hardwareInfo{
 h:=hardwareInfo{}
 if b,e:=os.ReadFile("/proc/meminfo");e==nil{
  var total,free uint64
  for _,line:=range strings.Split(string(b),"\n"){
   f:=strings.Fields(line);if len(f)<2{continue}
   n,e:=strconv.ParseUint(f[1],10,64);if e!=nil{continue}
   switch f[0]{case "SwapTotal:":total=n;case "SwapFree:":free=n}
  }
  if total>0&&free<=total{h.swap=fmt.Sprintf("SWAP %s / %s",sizeText((total-free)*1024),sizeText(total*1024))}
 }
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
 return h
}
