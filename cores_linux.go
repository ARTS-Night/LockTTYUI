//go:build linux

package main

import (
 "bufio"
 "fmt"
 "io"
 "os"
 "sort"
 "strconv"
 "strings"
 "time"
)

type coreSeries struct {
 id int
 history []float64
 bucket historyBucket
 live float64
}

func parseCores(r io.Reader) map[int]cpu {
 result:=map[int]cpu{}
 s:=bufio.NewScanner(r)
 for s.Scan(){
  f:=strings.Fields(s.Text())
  if len(f)<9 || !strings.HasPrefix(f[0],"cpu") || f[0]=="cpu"{continue}
  id,e:=strconv.Atoi(f[0][3:])
  if e!=nil||id<0{continue}
  var c cpu
  valid:=true
  for i:=1;i<=8;i++ {
   v,e:=strconv.ParseUint(f[i],10,64)
   if e!=nil{valid=false;break}
   c.total+=v
   if i==4||i==5{c.idle+=v}
  }
  if valid{result[id]=c}
 }
 return result
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
   s=&coreSeries{id:id,live:-1,history:make([]float64,60)}
   for i:=range s.history{s.history[i]=-1}
   series[id]=s
  }
  s.live=-1
  if p,ok:=prev[id];ok{s.live=cpuUsage(p,c);s.bucket.add(s.live)}
 }
}
var coreColors=[]string{"1;36","1;34","1;32","1;35","1;33","1;31","0;36","0;34","0;32","0;35","0;33","0;31"}
func coreGraph(b *strings.Builder,series map[int]*coreSeries,total []float64,w,rows int,ascii bool,period time.Duration,live float64){
 if len(series)==0||w<1||rows<2{graph(b,"CPU",total,w,rows,ascii,period,live);return}
 ids:=make([]int,0,len(series))
 for id:=range series{ids=append(ids,id)}
 sort.Ints(ids)
 fmt.Fprintf(b,"\x1b[1;36m// CPU\x1b[0m %s  %d LOGICAL\r\n",percentText(live),len(ids))
 type cell struct{glyph rune; color string; count int}
 grid:=make([][]cell,rows)
 for y:=range grid{grid[y]=make([]cell,w)}
 for _,id:=range ids{
  color:=coreColors[id%len(coreColors)]
  p:=plot(series[id].history,w,rows,ascii)
  for y,row:=range p{for x,g:=range row{
   if g==' '{continue}
   c:=&grid[y][x]
   if c.count==0{c.glyph=g;c.color=color}else{
    if c.glyph!=g{if ascii{c.glyph='+'}else{c.glyph='┼'};c.color="1;37"}else if c.count==1{c.color="1;36"}
   }
   c.count++
  }}
 }
 axis,corner,horizontal:="│","└","─"
 if ascii{axis,corner,horizontal="|","+","-"}
 for y,row:=range grid{
  label:="   "
  if y==0||y==rows-1||y==(rows-1)/2{label=fmt.Sprintf("%3d",(100*(rows-1-y)+(rows-1)/2)/(rows-1))}
  fmt.Fprintf(b,"\x1b[0;37m%s %s",label,axis)
  current:=""
  for _,c:=range row{
   if c.count==0{b.WriteByte(' ');continue}
   if c.color!=current{fmt.Fprintf(b,"\x1b[%sm",c.color);current=c.color}
   b.WriteRune(c.glyph)
  }
  b.WriteString("\x1b[0m\r\n")
 }
 fmt.Fprintf(b,"\x1b[0;37m    %s%s\r\n",corner,strings.Repeat(horizontal,w))
 left:=fmt.Sprintf("-%gs",float64(len(total))*period.Seconds())
 fmt.Fprintf(b,"     %s%sNOW\r\n",left,strings.Repeat(" ",max(1,w-len(left)-3)))
}
