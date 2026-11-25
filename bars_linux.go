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
 if w<=0||rows<=0{return nil}
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
func mirroredBars(b *strings.Builder,a,z []float64,w,rows int,ascii bool,period time.Duration,first,second string,scale float64){
 upper:=max(1,rows/2);lower:=upper
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
   value:=" N/A";if v>=0{value=fmt.Sprintf("%3.0f%%",v)}
   text:=fmt.Sprintf("C%03d %s %s",ids[j],value,meter(v,max(2,cell-11),color))
   b.WriteString(fitLine(text,cell))
  }
  lines=append(lines,b.String())
 }
 if limit<len(ids){lines=append(lines,fmt.Sprintf("+%d logical CPUs",len(ids)-limit))}
 return lines
}

func memoryMeterLines(hw hardwareInfo,width int,withBars bool)[]string{
 var out []string
 add:=func(label string,c capacity,color string){
  if c.total==0{return}
  p:=100*float64(c.used)/float64(c.total)
  out=append(out,fmt.Sprintf("\x1b[1;3%sm%s\x1b[0m %s / %s",color,label,sizeText(c.used),sizeText(c.total)))
  if withBars{out=append(out,meter(p,max(1,width),color))}
 }
 add("RAM",hw.ram,"4")
 if hw.swapKnown{
  if hw.swapCapacity.total==0{out=append(out,"SWAP OFF")}else{add("SWAP",hw.swapCapacity,"5")}
 }else if hw.swap!=""{out=append(out,hw.swap)}
 return out
}

 // Stacked RAM and swap contributions use a shared capacity denominator.
func stackedMemoryRows(ram,swap []float64,w,rows int,ascii bool)[]string{
 if w<=0||rows<=0{return nil}
 out:=make([]strings.Builder,rows)
 n:=min(len(ram),len(swap))
 for x:=0;x<w;x++{
  lo,hi:=x*n/w,(x+1)*n/w
  if hi<=lo{hi=min(n,lo+1)}
  var r,s float64;count:=0
  for i:=lo;i<hi;i++{
   if ram[i]>=0&&swap[i]>=0&&ram[i]+swap[i]<=100.000001{r+=ram[i];s+=swap[i];count++}
  }
  blue,total:=0,0
  if count>0{
   blue=int(math.Round(r/float64(count)*float64(rows*2)/100))
   total=int(math.Round((r+s)/float64(count)*float64(rows*2)/100))
   blue=min(rows*2,max(0,blue));total=min(rows*2,max(blue,total))
  }
  for y:=0;y<rows;y++{
   lower:=(rows-1-y)*2
   color:=func(level int)string{if level<blue{return "4"};if level<total{return "5"};return ""}
   a,z:=color(lower),color(lower+1)
   switch{
   case a=="":out[y].WriteByte(' ')
   case a==z:fmt.Fprintf(&out[y],"\x1b[4%sm \x1b[0m",a)
   case ascii:
    if z!=""{fmt.Fprintf(&out[y],"\x1b[3%s;4%sm=\x1b[0m",z,a)}else{fmt.Fprintf(&out[y],"\x1b[3%sm.\x1b[0m",a)}
   case z=="":fmt.Fprintf(&out[y],"\x1b[3%sm▄\x1b[0m",a)
   default:fmt.Fprintf(&out[y],"\x1b[3%s;4%sm▄\x1b[0m",a,z)
   }
  }
 }
 lines:=make([]string,rows);for i:=range out{lines[i]=out[i].String()};return lines
}
func stackedMemoryHistory(b *strings.Builder,ram,swap []float64,w,rows int,ascii bool,period time.Duration,hw hardwareInfo){
 value:="N/A"
 if hw.ram.total>0{
  total:=float64(hw.ram.total)+float64(hw.swapCapacity.total)
  value=percentText(100*(float64(hw.ram.used)+float64(hw.swapCapacity.used))/total)
 }
 fmt.Fprintf(b,"\x1b[1;34m// MEMORY\x1b[0m %s RAM+SWAP\r\n",value)
 for y,line:=range stackedMemoryRows(ram,swap,w,rows,ascii){
  label:="";if y==0{label="100"};if y==rows-1{label="0"}
  fmt.Fprintf(b,"%3s |%s\r\n",label,line)
 }
 timeAxis(b,w,5,len(ram),period)
}
