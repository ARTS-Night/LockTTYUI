//go:build linux

package main

import (
 "bufio"
 "flag"
 "fmt"
 "math"
 "os"
 "os/signal"
 "strconv"
 "strings"
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

func readCPU() (cpu, error) {
 f,e:=os.Open("/proc/stat")
 if e!=nil{return cpu{},e}
 defer f.Close()
 s:=bufio.NewScanner(f)
 if !s.Scan(){return cpu{},fmt.Errorf("missing CPU counters")}
 a:=strings.Fields(s.Text())
 if len(a)<9||a[0]!="cpu"{return cpu{},fmt.Errorf("invalid CPU counters")}
 var c cpu
 for i:=1;i<=8;i++{
  v,e:=strconv.ParseUint(a[i],10,64)
  if e!=nil{return cpu{},e}
  c.total+=v
  if i==4||i==5{c.idle+=v}
 }
 return c,nil
}
func cpuUsage(a,b cpu)float64{
 if b.total<=a.total||b.idle<a.idle{return -1}
 d:=b.total-a.total
 i:=b.idle-a.idle
 if i>d{return -1}
 return 100*float64(d-i)/float64(d)
}
func memory()(float64,error){
 f,e:=os.Open("/proc/meminfo")
 if e!=nil{return -1,e}
 defer f.Close()
 s:=bufio.NewScanner(f)
 var total,avail uint64
 found:=false
 for s.Scan(){
  a:=strings.Fields(s.Text())
  if len(a)<2{continue}
  switch a[0]{
  case "MemTotal:":total,e=strconv.ParseUint(a[1],10,64)
  case "MemAvailable:":avail,e=strconv.ParseUint(a[1],10,64);found=true
  default:continue
  }
  if e!=nil{return -1,e}
 }
 if e=s.Err();e!=nil{return -1,e}
 if total==0||!found||avail>total{return -1,fmt.Errorf("invalid memory counters")}
 return 100*float64(total-avail)/float64(total),nil
}
func push(h []float64,v float64)[]float64{return append(h[1:],v)}

// Each column represents a time bucket; connect consecutive valid samples.
func plot(h []float64,w,rows int,ascii bool)[][]rune{
 g:=make([][]rune,rows)
 for y:=range g{g[y]=[]rune(strings.Repeat(" ",w))}
 if w<1||rows<2||len(h)==0{return g}
 prev:=-1
 for x:=0;x<w;x++{
  lo,hi:=x*len(h)/w,(x+1)*len(h)/w
  if hi<=lo{hi=lo+1}
  if hi>len(h){hi=len(h)}
  sum:=0.0
  n:=0
  for _,v:=range h[lo:hi]{if v>=0&&v<=100{sum+=v;n++}}
  if n==0{prev=-1;continue}
  y:=rows-1-int(math.Round(sum/float64(n)*float64(rows-1)/100))
  if prev<0{
   if ascii{g[y][x]='*'}else{g[y][x]='─'}
  }else if prev==y{
   if ascii{g[y][x]='-'}else{g[y][x]='─'}
  }else{
   top,bottom:=prev,y
   if top>bottom{top,bottom=bottom,top}
   for j:=top;j<=bottom;j++{if ascii{g[j][x]='|'}else{g[j][x]='│'}}
   if ascii{g[prev][x]='+';g[y][x]='+'}else if y<prev{g[prev][x]='┘';g[y][x]='┌'}else{g[prev][x]='┐';g[y][x]='└'}
  }
  prev=y
 }
 return g
}
func graph(b *strings.Builder,name string,h []float64,w,rows int,ascii bool,period time.Duration){
 color:="36"
 if strings.HasPrefix(name,"MEMORY"){color="34"}
 v:=h[len(h)-1]
 value:="N/A"
 if v>=0{value=fmt.Sprintf("%.1f%%",v)}
 fmt.Fprintf(b,"\x1b[1;%sm// %s\x1b[0m  %s\r\n",color,name,value)
 if w<1||rows<2{return}
 axis,corner,horizontal:="│","└","─"
 if ascii{axis,corner,horizontal="|","+","-"}
 g:=plot(h,w,rows,ascii)
 for y,row:=range g{
  label:="   "
  if y==0||y==rows-1||y==(rows-1)/2{label=fmt.Sprintf("%3d",int(math.Round(float64(rows-1-y)*100/float64(rows-1))))}
  fmt.Fprintf(b,"\x1b[0;37m%s %s\x1b[1;%sm%s\r\n",label,axis,color,string(row))
 }
 fmt.Fprintf(b,"\x1b[0;37m    %s%s\r\n",corner,strings.Repeat(horizontal,w))
 left:=fmt.Sprintf("-%gs",float64(len(h))*period.Seconds())
 fmt.Fprintf(b,"     %s%sNOW\r\n",left,strings.Repeat(" ",max(1,w-len(left)-3)))
}
func run()error{
 exhibit:=flag.Bool("exhibit",false,"ignore keys except Ctrl+]")
 glyphTest:=flag.Bool("glyph-test",false,"print supported glyph candidates for visual inspection")
 ascii:=flag.Bool("ascii",false,"ASCII fallback")
 interval:=flag.Duration("interval",100*time.Millisecond,"display refresh interval (100ms to 5s)")
 sample:=flag.Duration("sample",500*time.Millisecond,"metric sample interval (100ms to 1s)")
 flag.Parse()
 if *glyphTest{fmt.Println("ASCII: + - | *\nUnicode: ─ │ ┌ ┐ └ ┘ ├ ┤\nOptional (not used): ▁ ▂ ▃ ▄ ▅ ▆ ▇ █ ⣿");return nil}
 if *interval<100*time.Millisecond||*interval>5*time.Second||*sample<100*time.Millisecond||*sample>time.Second{return fmt.Errorf("interval must be 100ms..5s; sample must be 100ms..1s")}
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
 raw.Cc[syscall.VTIME]=1
 sig:=make(chan os.Signal,1)
 signal.Notify(sig,syscall.SIGINT,syscall.SIGTERM,syscall.SIGHUP,syscall.SIGQUIT)
 defer signal.Stop(sig)
 if e=ioctl(fd,syscall.TCSETS,unsafe.Pointer(&raw));e!=nil{return e}
 defer func(){
  ioctl(fd,syscall.TCSETS,unsafe.Pointer(&old))
  fmt.Fprint(tty,"\x1b[0m\x1b[?25h\x1b[2J\x1b[H")
 }()
 fmt.Fprint(tty,"\x1b[?25l\x1b[2J")
 ch,mh:=make([]float64,int(time.Minute / *sample)),make([]float64,int(time.Minute / *sample))
 for i:=range ch{ch[i]=-1;mh[i]=-1}
 prev,pe:=readCPU()
 next:=time.Now()
 nextSample:=next.Add(*sample)
 v,m:=-1.0,-1.0
 var buf [128]byte
 for{
  select{case <-sig:return nil;default:}
  now:=time.Now()
  if !now.Before(nextSample){
   nextSample=nextSample.Add(*sample)
   if !nextSample.After(now){nextSample=now.Add(*sample)}
   cur,ce:=readCPU()
   v=-1.0
   if pe==nil&&ce==nil{v=cpuUsage(prev,cur)}
   prev,pe=cur,ce
   ch=push(ch,v)
   var me error
   m,me=memory()
   if me!=nil{m=-1}
   mh=push(mh,m)
  }
  if !now.Before(next){
   next=next.Add(*interval)
   if !next.After(now){next=now.Add(*interval)}
   var ws struct{Row,Col,X,Y uint16}
   if e=ioctl(fd,syscall.TIOCGWINSZ,unsafe.Pointer(&ws));e!=nil{return e}
   w:=int(ws.Col)-7
   if w>110{w=110}
   clock:=clockLines(now,int(ws.Col)-1,int(ws.Row),*ascii)
   rows:=(int(ws.Row)-14-(len(clock)-1))/2
   if rows>16{rows=16}
   var b strings.Builder
   b.WriteString("\x1b[H\x1b[36m")
   fmt.Fprintf(&b,"LOCKTTYUI // STANDBY\r\n")
   for _,line:=range clock{fmt.Fprintf(&b,"\x1b[1;36m%s\x1b[0m\r\n",line)}
   fmt.Fprintf(&b,"\x1b[37m%s\r\n\r\n",now.Format("2006-01-02 Mon"))
   if w>=20&&rows>=2{
    graph(&b,"CPU / UTILIZATION",ch,w,rows,*ascii,*sample)
    b.WriteString("\r\n")
    graph(&b,"MEMORY / UTILIZATION",mh,w,rows,*ascii,*sample)
   }else{fmt.Fprintf(&b,"CPU: %.1f%%\r\nMEM: %.1f%%\r\n",v,m)}
   b.WriteString("\r\nDISPLAY MODE")
   if !*exhibit{b.WriteString(" / q to exit")}
   b.WriteString("\x1b[0m")
   frame:=strings.ReplaceAll(b.String(),"\r\n","\x1b[K\r\n")+"\x1b[K\x1b[J"
   if _,e=fmt.Fprint(tty,frame);e!=nil{return e}
  }
  n,e:=syscall.Read(int(fd),buf[:])
  if e!=nil{return e}
  for _,k:=range buf[:n]{if k==29||(!*exhibit&&(k=='q'||k==3)){return nil}}
 }
}
func main(){if e:=run();e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}}
