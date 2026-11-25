//go:build linux

package main

import (
 "bufio"
 "fmt"
 "io"
 "os"
 "strconv"
 "strings"
 "syscall"
 "time"
)

type capacity struct { total, used, available uint64 }
type volume struct { label string; capacity }

func filesystemCapacity(path string) (capacity, error) {
 var s syscall.Statfs_t
 if err:=syscall.Statfs(path,&s); err!=nil { return capacity{},err }
 if s.Bsize<=0 || s.Blocks==0 || s.Bfree>s.Blocks || s.Bavail>s.Bfree || s.Blocks> ^uint64(0)/uint64(s.Bsize) { return capacity{},fmt.Errorf("invalid filesystem counters") }
 size:=uint64(s.Bsize)
 return capacity{s.Blocks*size,(s.Blocks-s.Bfree)*size,s.Bavail*size},nil
}

func readVolumes() []volume {
 // Root is always measured; extra local mounts are opt-in by discovery below.
 data,err:=os.ReadFile("/proc/self/mountinfo")
 paths:=[]string{"/"}
 if err==nil {paths=mountPaths(string(data))}
 result:=make([]volume,0,len(paths))
 for _,path:=range paths {
  c,e:=filesystemCapacity(path)
  if e!=nil {continue}
  label:="ROOT"
  if path!="/" {label=fmt.Sprintf("DATA %02d",len(result))}
  result=append(result,volume{label,c})
 }
 return result
}

func mountPaths(data string) []string {
 allowed:=map[string]bool{"ext2":true,"ext3":true,"ext4":true,"xfs":true,"btrfs":true,"f2fs":true,"vfat":true,"exfat":true,"ntfs3":true}
 type candidate struct{ device,path string }
 var candidates []candidate
 seen:=map[string]bool{}
 for _,line:=range strings.Split(data,"\n") {
  f:=strings.Fields(line)
  if len(f)<10 {continue}
  if f[4]=="/" {seen[f[2]]=true;continue}
  sep:=6
  for sep<len(f)&&f[sep]!="-" {sep++}
  if sep+3>=len(f)||!allowed[f[sep+1]]||f[3]!="/" {continue}
  // Ignore boot and container mounts; skip escaped paths rather than guessing.
  p:=f[4]
  if !strings.HasPrefix(p,"/")||strings.Contains(p,"\\")||p=="/boot"||strings.HasPrefix(p,"/boot/")||strings.HasPrefix(p,"/var/lib/docker/")||strings.HasPrefix(p,"/run/") {continue}
  candidates=append(candidates,candidate{f[2],p})
 }
 paths:=[]string{"/"}
 for _,c:=range candidates {
  if seen[c.device] {continue}
  seen[c.device]=true
  paths=append(paths,c.path)
  if len(paths)>=8 {break}
 }
 return paths
}

func sizeText(n uint64) string {
 v:=float64(n)
 unit:="B"
 for _,u:=range []string{"KiB","MiB","GiB","TiB","PiB"} {
  if v<1024 {break}
  v/=1024;unit=u
 }
 return fmt.Sprintf("%.1f%s",v,unit)
}

func storageLines(b *strings.Builder, volumes []volume, width, limit int, ascii bool) {
 b.WriteString("\x1b[1;36m// STORAGE\x1b[0m\r\n")
 if len(volumes)==0 {b.WriteString("N/A\r\n");return}
 for i,v:=range volumes {
  if i>=limit {break}
  p:=float64(v.used)/float64(v.total)
  barWidth:=max(0,min(20,width-57))
  full:=int(p*float64(barWidth)+0.5)
  full=max(0,min(barWidth,full))
  color:="36"
  if p>=.9 {color="31"} else if p>=.8 {color="33"}
  if barWidth>=5 {
   on,off:="─"," "
   if ascii {on,off="#","-"}
   fmt.Fprintf(b,"%-7s \x1b[1;%sm[%s%s]\x1b[0m %5.1f%% %s/%s FREE %s\r\n",v.label,color,strings.Repeat(on,full),strings.Repeat(off,barWidth-full),p*100,sizeText(v.used),sizeText(v.total),sizeText(v.available))
  } else {
   fmt.Fprintf(b,"%-7s %5.1f%% FREE %s\r\n",v.label,p*100,sizeText(v.available))
  }
 }
}

type traffic struct { rx,tx uint64 }
func readTraffic() (map[string]traffic,error) {
 f,e:=os.Open("/proc/net/dev");if e!=nil{return nil,e};defer f.Close()
 return parseTraffic(f)
}
func parseTraffic(r io.Reader) (map[string]traffic,error) {
 out:=map[string]traffic{}
 s:=bufio.NewScanner(r)
 for s.Scan() {
  name,rest,ok:=strings.Cut(s.Text(),":")
  if !ok {continue}
  name=strings.TrimSpace(name)
  if name=="lo"||strings.HasPrefix(name,"veth")||strings.HasPrefix(name,"docker")||strings.HasPrefix(name,"br-") {continue}
  f:=strings.Fields(rest);if len(f)<16 {continue}
  rx,e:=strconv.ParseUint(f[0],10,64);if e!=nil{continue}
  tx,e:=strconv.ParseUint(f[8],10,64);if e!=nil{continue}
  out[name]=traffic{rx,tx}
 }
 return out,s.Err()
}
func trafficRate(prev,next map[string]traffic,elapsed time.Duration)(float64,float64,bool) {
 if elapsed<=0||len(next)==0||len(prev)!=len(next){return 0,0,false}
 var rx,tx float64
 for name,n:=range next {
  p,ok:=prev[name]
  if !ok||n.rx<p.rx||n.tx<p.tx{return 0,0,false}
  rx+=float64(n.rx-p.rx);tx+=float64(n.tx-p.tx)
 }
 return rx/elapsed.Seconds(),tx/elapsed.Seconds(),true
}
func rateText(n float64) string {
 if n<0{return "N/A"}
 return sizeText(uint64(n))+"/s"
}
