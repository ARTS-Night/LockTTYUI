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
 bg,fg:=barPalette(color)
 fullCell:="\x1b["+bg+"m \x1b[0m"
 glyph:="▄";if invert{glyph="▀"};if ascii{glyph="."}
 halfCell:="\x1b["+fg+"m"+glyph+"\x1b[0m"
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
   if k>=2 {out[y].WriteString(fullCell)
   }else if k==1{out[y].WriteString(halfCell)
   }else{out[y].WriteByte(' ')}
  }
 }
 lines:=make([]string,rows);for i:=range out{lines[i]=compressGraphLine(out[i].String())}
 return lines
}
func barHistory(b *strings.Builder,name string,h []float64,w,rows int,ascii bool,period time.Duration,live float64,color string){
 fmt.Fprintf(b,"\x1b[1;3%sm// %s\x1b[0m  %s\r\n",color,name,percentText(live))
 for y,line:=range barRows(h,w,rows,100,false,ascii,color){
  label:=""
  if y==0{label="100"}
  if rows>=4&&y==rows/2{
   label="50"
   if strings.TrimSpace(line)==""{line=cpuReferenceLine(w,ascii)}
  }
  fmt.Fprintf(b,"%3s |%s\r\n",label,line)
 }
 axis:="─";corner:="└";if ascii{axis="-";corner="+"}
 fmt.Fprintf(b,"%3s %s%s\r\n","0",corner,strings.Repeat(axis,w))
 timeAxis(b,w,5,len(h),period)
}
// One subdued mid-scale guide remains visible at zero CPU load without
// faking any CPU use. Unlike a full grid, it does not compete with the bars.
func cpuReferenceLine(w int,ascii bool)string{
 var b strings.Builder
 glyph:="·";if ascii{glyph="."}
 b.Grow(w)
 for x:=0;x<w;x++{
  if x%4==0{b.WriteString(glyph)}else{b.WriteByte(' ')}
 }
 return "\x1b[90m"+b.String()+"\x1b[0m"
}
func timeAxis(b *strings.Builder,w,offset,n int,period time.Duration){
 left:=fmt.Sprintf("-%gs",float64(n)*period.Seconds())
 fmt.Fprintf(b,"%s%s%sNOW\r\n",strings.Repeat(" ",offset),left,strings.Repeat(" ",max(1,w-len(left)-3)))
}
// Same scale, separate halves, no overlapping characters.
func mirroredBars(b *strings.Builder,a,z []float64,w,rows int,ascii bool,period time.Duration,first,second string,scale float64){
 upper:=max(1,rows/2);lower:=max(1,rows-upper)
 for y,line:=range barRows(a,w,upper,scale,false,ascii,"7"){
  label:="";if y==0{label=sizeText(uint64(scale))}
  fmt.Fprintf(b,"%8s |%s\r\n",label,line)
 }
 sep:="─";if ascii{sep="-"}
 fmt.Fprintf(b,"%8s +%s\r\n","0",strings.Repeat(sep,w))
 for y,line:=range barRows(z,w,lower,scale,true,ascii,"8"){
  label:="";if y==lower-1{label=sizeText(uint64(scale))}
  fmt.Fprintf(b,"%8s |%s\r\n",label,line)
 }
 timeAxis(b,w,10,len(a),period)
}
func meter(v float64,w int,color string)string{
 if v<0{return strings.Repeat("-",w)}
 n:=max(0,min(w,int(math.Round(v/100*float64(w)))))
 bg,_:=barPalette(color)
 return "\x1b["+bg+"m"+strings.Repeat(" ",n)+"\x1b[0;90m"+strings.Repeat(".",w-n)+"\x1b[0m"
}
func coreMeterLines(cores map[int]*coreSeries,width,budget int,caches ...*dashboardCache)[]string{
 if budget<=0||len(cores)==0{return nil}
 var ids []int
 if len(caches)>0&&caches[0]!=nil{ids=caches[0].ids}
 valid:=len(ids)==len(cores)
 if valid{for _,id:=range ids{if _,ok:=cores[id];!ok{valid=false;break}}}
 if !valid{ids=make([]int,0,len(cores));for id:=range cores{ids=append(ids,id)};sort.Ints(ids);if len(caches)>0&&caches[0]!=nil{caches[0].ids=ids}}
 cols:=max(1,width/22);cell:=width/cols
 limit:=min(len(ids),cols*budget)
 if limit<len(ids){limit=max(0,limit-cols)}
 lines:=[]string{}
 for i:=0;i<limit;i+=cols{
  var b strings.Builder
  for j:=i;j<min(i+cols,limit);j++{
   c:=cores[ids[j]];v:=c.live
   color:="7";if v>=90{color="1"}
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
  heading:="\x1b[1;97m";if color=="8"{heading="\x1b[90m"}
  out=append(out,fmt.Sprintf("%s%s\x1b[0m %s / %s",heading,label,sizeText(c.used),sizeText(c.total)))
  if withBars{out=append(out,meter(p,max(1,width),color))}
 }
 add("RAM",hw.ram,"7")
 if hw.swapKnown{
  if hw.swapCapacity.total==0{out=append(out,"SWAP OFF")}else{add("SWAP",hw.swapCapacity,"8")}
 }else{out=append(out,"SWAP N/A")}
 return out
}

 // Stacked RAM and swap contributions use a shared capacity denominator.
func stackedMemoryRows(ram,swap []float64,w,rows int,ascii bool)[]string{
 if w<=0||rows<=0{return nil}
 out:=make([]strings.Builder,rows)
 // Six possible cells; precompute their escapes instead of formatting per pixel.
 cells:=[6]string{" ","\x1b[37m▄\x1b[0m","\x1b[90m▄\x1b[0m","\x1b[47m \x1b[0m","\x1b[100m \x1b[0m","\x1b[37;100m▄\x1b[0m"}
 if ascii{cells[5]="\x1b[37;100m=\x1b[0m";cells[1]="\x1b[37m.\x1b[0m";cells[2]="\x1b[90m.\x1b[0m"}
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
   // Preserve visibility of nonzero swap at the expense of one half-cell
   // of quantization; numeric labels remain exact.
   if s>0&&total==blue{
    if total<rows*2{total++}else{blue--}
   }
   if r>0&&blue==0&&total>=2{blue=1}
  }
  for y:=0;y<rows;y++{
   lower:=(rows-1-y)*2
   idx:=0
   switch {
   case lower+1<blue:idx=3
   case lower<blue&&lower+1<total:idx=5
   case lower<blue:idx=1
   case lower+1<total:idx=4
   case lower<total:idx=2
   }
   out[y].WriteString(cells[idx])
  }
 }
 lines:=make([]string,rows);for i:=range out{lines[i]=compressGraphLine(out[i].String())};return lines
}
func stackedMemoryHistory(b *strings.Builder,ram,swap []float64,w,rows int,ascii bool,period time.Duration,hw hardwareInfo){
 value:=percentText(combinedMemoryPercent(hw))
 fmt.Fprintf(b,"\x1b[1;97m// MEMORY\x1b[0m %s RAM+SWAP\r\n",value)
 for y,line:=range stackedMemoryRows(ram,swap,w,rows,ascii){
  label:="";if y==0{label="100"}
  fmt.Fprintf(b,"%3s |%s\r\n",label,line)
 }
 axis:="─";corner:="└";if ascii{axis="-";corner="+"}
 fmt.Fprintf(b,"%3s %s%s\r\n","0",corner,strings.Repeat(axis,w))
 timeAxis(b,w,5,len(ram),period)
}

// Monochrome bar foreground/background. Green and red are reserved for status.
func barPalette(color string)(background,foreground string){
 switch color {
 case "1":return "41","31"  // Alert (red)
 case "2":return "42","32"  // Healthy (green)
 case "8":return "100","90" // Muted grey
 default:return "47","37"  // White
 }
}
