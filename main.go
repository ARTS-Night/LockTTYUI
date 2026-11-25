//go:build linux

package main

import (
 "flag"
 "fmt"
 "os"
 "os/signal"
 "syscall"
 "time"
 "unsafe"
)

func ioctl(fd uintptr, req uintptr, p unsafe.Pointer) error {
 _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(p))
 if e != 0 { return e }
 return nil
}

type cpu struct{ total, idle uint64 }

func cpuUsage(a,b cpu)float64{
 if b.total<=a.total||b.idle<a.idle{return -1}
 d:=b.total-a.total
 i:=b.idle-a.idle
 if i>d{return -1}
 return 100*float64(d-i)/float64(d)
}
func memory()(float64,error){
 ram,_,_:=readMemoryCapacity()
 if ram.total==0{return -1,fmt.Errorf("invalid memory counters")}
 return 100*float64(ram.used)/float64(ram.total),nil
}
func push(h []float64,v float64)[]float64{if len(h)>0{copy(h,h[1:]);h[len(h)-1]=v};return h}

// Commit one averaged sample per fixed history interval.
type historyBucket struct{sum float64; count int}
func (b *historyBucket) add(v float64){if v>=0{b.sum+=v;b.count++}}
func (b *historyBucket) take()float64{v:=-1.0;if b.count>0{v=b.sum/float64(b.count)};*b=historyBucket{};return v}

func run()error{
 exhibit:=flag.Bool("exhibit",false,"ignore keys except Ctrl+]")
 glyphTest:=flag.Bool("glyph-test",false,"print supported glyph candidates for visual inspection")
 ascii:=flag.Bool("ascii",false,"ASCII fallback")
 interval:=flag.Duration("interval",50*time.Millisecond,"display refresh interval (50ms to 5s)")
 sample:=flag.Duration("sample",250*time.Millisecond,"metric sample interval (100ms to 1s)")
 flag.Parse()
 if *glyphTest{fmt.Println("ASCII: + - | *\nUnicode: ─ │ ┌ ┐ └ ┘ ├ ┤\nBars: ▄ ▀ █; ASCII mode uses background colors");return nil}
 if *interval<50*time.Millisecond||*interval>5*time.Second||*sample<100*time.Millisecond||*sample>time.Second{return fmt.Errorf("interval must be 50ms..5s; sample must be 100ms..1s")}
 tty,e:=os.OpenFile("/dev/tty",os.O_RDWR,0)
 if e!=nil{return e}
 defer tty.Close()
 fd:=tty.Fd()
 var old syscall.Termios
 if e=ioctl(fd,syscall.TCGETS,unsafe.Pointer(&old));e!=nil{return e}
 raw:=old
 raw.Iflag &^= syscall.BRKINT|syscall.ICRNL|syscall.INPCK|syscall.ISTRIP|syscall.IXON
 raw.Oflag &^= syscall.OPOST
 raw.Lflag &^= syscall.ECHO|syscall.ICANON|syscall.IEXTEN|syscall.ISIG
 raw.Cflag &^= syscall.CSIZE|syscall.PARENB
 raw.Cflag |= syscall.CS8
 raw.Cc[syscall.VMIN]=0
 raw.Cc[syscall.VTIME]=0
 sig:=make(chan os.Signal,1)
 signal.Notify(sig,syscall.SIGINT,syscall.SIGTERM,syscall.SIGHUP,syscall.SIGQUIT)
 defer signal.Stop(sig)
 if e=ioctl(fd,syscall.TCSETS,unsafe.Pointer(&raw));e!=nil{return e}
 defer func(){
  ioctl(fd,syscall.TCSETS,unsafe.Pointer(&old))
  fmt.Fprint(tty,"\x1b[0m\x1b[?25h\x1b[2J\x1b[H")
 }()
 fmt.Fprint(tty,"\x1b[?25l\x1b[2J")
 ch,mh:=make([]float64,60),make([]float64,60)
 sh:=make([]float64,60)
 for i:=range ch{ch[i]=-1;mh[i]=-1;sh[i]=-1}
 prev,corePrev,prevOK:=readCPUSnapshot()
 cores:=map[int]*coreSeries{}
 updateCores(cores,nil,corePrev)
 next:=time.Now()
 nextSample:=next.Add(*sample)
 v,m:=-1.0,-1.0
 volumes:=readVolumes()
 nextStorage:=next.Add(5*time.Second)
 info:=readSystemInfo()
 var lastSample time.Time
 nextInfo:=next.Add(time.Second)
 lastFrame:=""
 var lastRenderKey struct{row,col uint16;second int64;sample time.Time;read bool;revision uint64}
 var revision uint64
 netPrev,_:=readTraffic()
 netTime:=next
 rx,tx:=-1.0,-1.0
 rh,th:=make([]float64,len(ch)),make([]float64,len(ch))
 for i:=range rh{rh[i]=-1;th[i]=-1}
 var buckets [5]historyBucket
 var lastRamTotal,lastSwapTotal uint64
 var lastSwapKnown bool
 netScale:=rateScale{}
 netScale.update(1024)
 diskPrev:=readDiskTraffic()
 diskTime:=next
 dr,dw:=-1.0,-1.0
 hw:=readHardware()
 nextHistory:=next.Add(time.Second)
 var buf [128]byte
 for{
  select{case <-sig:return nil;default:}
  now:=time.Now()
  if !now.Before(nextSample){
   nextSample=nextSample.Add(*sample)
   if !nextSample.After(now){nextSample=now.Add(*sample)}
   cur,coreNext,curOK:=readCPUSnapshot()
   v=-1.0
   if prevOK&&curOK{v=cpuUsage(prev,cur)}
   prev,prevOK=cur,curOK
   buckets[0].add(v)
   updateCores(cores,corePrev,coreNext)
   corePrev=coreNext
   hw.ram,hw.swapCapacity,hw.swapKnown=readMemoryCapacity()
   m=-1
   if hw.ram.total>0&&hw.swapKnown{
    m=combinedMemoryPercent(hw)
    if lastRamTotal!=hw.ram.total||lastSwapTotal!=hw.swapCapacity.total||lastSwapKnown!=hw.swapKnown{
     for i:=range mh{mh[i]=-1;sh[i]=-1}
     buckets[1]=historyBucket{};buckets[4]=historyBucket{}
     lastRamTotal,lastSwapTotal,lastSwapKnown=hw.ram.total,hw.swapCapacity.total,hw.swapKnown
    }
    total:=float64(hw.ram.total)+float64(hw.swapCapacity.total)
    buckets[1].add(100*float64(hw.ram.used)/total)
    buckets[4].add(100*float64(hw.swapCapacity.used)/total)
   }
   netCur,ne:=readTraffic()
   rx,tx=-1,-1
   if ne==nil { if a,b,ok:=trafficRate(netPrev,netCur,now.Sub(netTime));ok{rx,tx=a,b} }
   netPrev,netTime=netCur,now
   buckets[2].add(rx);buckets[3].add(tx)
   diskNext:=readDiskTraffic()
   dr,dw=-1,-1
   if a,b,ok:=trafficRate(diskPrev,diskNext,now.Sub(diskTime));ok{dr,dw=a,b}
   diskPrev,diskTime=diskNext,now
   lastSample=now
  }
  if !now.Before(nextHistory){
   ch=push(ch,buckets[0].take());mh=push(mh,buckets[1].take());sh=push(sh,buckets[4].take());rh=push(rh,buckets[2].take());th=push(th,buckets[3].take())
   revision++
   netScale.update(trafficScale(rh,th))
   nextHistory=nextHistory.Add(time.Second)
   if !nextHistory.After(now){nextHistory=now.Add(time.Second)}
  }
  if !now.Before(nextInfo){info=readSystemInfo();revision++;nextInfo=now.Add(time.Second)}
  if !now.Before(nextStorage){
   volumes=readVolumes()
   nextHW:=readHardware()
   nextHW.ram,nextHW.swapCapacity,nextHW.swapKnown=hw.ram,hw.swapCapacity,hw.swapKnown
   hw=nextHW
   revision++
   nextStorage=now.Add(5*time.Second)
  }
  if !now.Before(next){
   next=next.Add(*interval)
   if !next.After(now){next=now.Add(*interval)}
   var ws struct{Row,Col,X,Y uint16}
   if e=ioctl(fd,syscall.TIOCGWINSZ,unsafe.Pointer(&ws));e!=nil{return e}
   key:=struct{row,col uint16;second int64;sample time.Time;read bool;revision uint64}{ws.Row,ws.Col,now.Unix(),lastSample,!lastSample.IsZero()&&now.Sub(lastSample)<150*time.Millisecond,revision}
   if key!=lastRenderKey||lastFrame==""{
   lastRenderKey=key
   frame:=dashboard(now,int(ws.Col),int(ws.Row),*ascii,*exhibit,time.Second,ch,mh,sh,rh,th,volumes,info,lastSample,[4]float64{v,m,rx,tx},cores,hw,dr,dw,netScale.value)
   if frame!=lastFrame {
    if _,e=fmt.Fprint(tty,frame);e!=nil{return e}
    lastFrame=frame
   }
   }
  }
  n,e:=syscall.Read(int(fd),buf[:])
  if e!=nil{return e}
  for _,k:=range buf[:n]{if k==29||(!*exhibit&&(k=='q'||k==3)){return nil}}
  time.Sleep(10*time.Millisecond)
 }
}
func main(){if e:=run();e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}}
