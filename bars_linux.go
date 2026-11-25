//go:build linux

package main

import (
 "fmt"
 "math"
 "sort"
 "strings"
 "time"
)

// A cell uses its background for full bars, so no block font is required.
// Optional half blocks improve resolution on terminals with suitable fonts.
func barRows(h []float64,w,rows int,scale float64,invert,ascii bool,color string) []string {
 out:=make([]strings.Builder,rows)
 for x:=0;x<w;x++ {
  lo,hi:=x*len(h)/w,(x+1)*len(h)/w
  if hi<=lo{hi=min(len(h),lo+1)}
  sum:=0.0; n:=0
  for _,v:=range h[lo:hi]{if v>=0&&!math.IsInf(v,0){sum+=v;n++}}
  fill:=0
  if n>0 && scale>0 {fill=int(math.Round(min(1,sum/float64(n)/scale)*float64(rows*2)))}
  for y:=0;y<rows;y++ {
   level:=rows-1-y;if invert{level=y}
   k:=fill-level*2
   if k>=2 {fmt.Fprintf(&out[y],"\x1b[4%sm \x1b[0m",color)
   }else if k==1{
    if ascii{fmt.Fprintf(&out[y],"\x1b[3%sm.\x1b[0m",color)}else{
     glyph:="▄";if invert{glyph="▀"}
     fmt.Fprintf(&out[y],"\x1b[3%sm%s\x1b[0m",color,glyph)
    }
   }else{out[y].WriteByte(' ')}
  }
 }
 lines:=make([]string,rows);for i:=range out{lines[i]=out[i].String()}
 return lines
}
func barHistory(b *strings.Builder,name string,h []float64,w,rows int,ascii bool,period time.Duration,live float64,color string){
 fmt.Fprintf(b,"\x1b[1;3%sm// %s\x1b[0m  %s\r\n",color,name,percentText(live))
 for y,line:=range barRows(h,w,rows,100,false,ascii,color){
  label:="";if y==0{label="100"};if y==rows-1{label="0"}
  fmt.Fprintf(b,"%3s |%s\r\n",label,line)
 }
 timeAxis(b,w,5,len(h),period)
}
func timeAxis(b *strings.Builder,w,offset,n int,period time.Duration){
 left:=fmt.Sprintf("-%gs",float64(n)*period.Seconds())
 fmt.Fprintf(b,"%s%s%sNOW\r\n",strings.Repeat(" ",offset),left,strings.Repeat(" ",max(1,w-len(left)-3)))
}
// Same scale, separate halves, no overlapping characters.
func mirroredBars(b *strings.Builder,a,z []float64,w,rows int,ascii bool,period time.Duration,first,second string){
 scale:=trafficScale(a,z)
 upper:=max(1,(rows+1)/2);lower:=max(1,rows-upper)
 for y,line:=range barRows(a,w,upper,scale,false,ascii,"6"){
  label:="";if y==0{label=sizeText(uint64(scale))}
  fmt.Fprintf(b,"%8s |%s\r\n",label,line)
 }
 sep:="─";if ascii{sep="-"}
 fmt.Fprintf(b,"%8s +%s\r\n","0",strings.Repeat(sep,w))
 for y,line:=range barRows(z,w,lower,scale,true,ascii,"4"){
  label:="";if y==lower-1{label=sizeText(uint64(scale))}
  fmt.Fprintf(b,"%8s |%s\r\n",label,line)
 }
 timeAxis(b,w,10,len(a),period)
}
func meter(v float64,w int,color string)string{
 if v<0{return strings.Repeat("-",w)}
 n:=max(0,min(w,int(math.Round(v/100*float64(w)))))
 return "\x1b[4"+color+"m"+strings.Repeat(" ",n)+"\x1b[0;37m"+strings.Repeat(".",w-n)+"\x1b[0m"
}
func coreMeterLines(cores map[int]*coreSeries,width,budget int)[]string{
 if budget<=0||len(cores)==0{return nil}
 ids:=make([]int,0,len(cores));for id:=range cores{ids=append(ids,id)};sort.Ints(ids)
 cols:=max(1,width/22);cell:=width/cols
 limit:=min(len(ids),cols*budget)
 if limit<len(ids){limit=max(0,limit-cols)}
 lines:=[]string{}
 for i:=0;i<limit;i+=cols{
  var b strings.Builder
  for j:=i;j<min(i+cols,limit);j++{
   c:=cores[ids[j]];v:=c.live
   color:="6";if v>=90{color="1"}else if v>=80{color="3"}
   text:=fmt.Sprintf("C%03d %4s %s",ids[j],percentText(v),meter(v,max(2,cell-14),color))
   b.WriteString(fitLine(text,cell))
  }
  lines=append(lines,b.String())
 }
 if limit<len(ids){lines=append(lines,fmt.Sprintf("+%d logical CPUs",len(ids)-limit))}
 return lines
}
