//go:build linux

package main

import (
 "fmt"
 "strings"
 "time"
 "unicode/utf8"
)

// All dashboard strings use single-cell ASCII or box-drawing glyphs.
func visibleWidth(s string) int {
 n:=0
 for len(s)>0 {
  if strings.HasPrefix(s,"\x1b[") {
   i:=2
   for i<len(s)&&(s[i]<'@'||s[i]>'~'){i++}
   if i<len(s){s=s[i+1:];continue}
  }
  _,size:=utf8.DecodeRuneInString(s);s=s[size:];n++
 }
 return n
}
func fitLine(s string,w int) string {
 if w<=0{return ""}
 var b strings.Builder
 n:=0
 for len(s)>0 {
  if strings.HasPrefix(s,"\x1b[") {
   i:=2
   for i<len(s)&&(s[i]<'@'||s[i]>'~'){i++}
   if i<len(s){b.WriteString(s[:i+1]);s=s[i+1:];continue}
  }
  if n>=w{break}
  _,size:=utf8.DecodeRuneInString(s);b.WriteString(s[:size]);s=s[size:];n++
 }
 return b.String()+"\x1b[0m"+strings.Repeat(" ",w-n)
}
func centerLine(s string,w int) string {
 return strings.Repeat(" ",max(0,(w-visibleWidth(s))/2))+s
}
func panel(lines []string,w int,ascii bool) []string {
 tl,tr,bl,br,h,v:="┌","┐","└","┘","─","│"
 if ascii{tl,tr,bl,br,h,v="+","+","+","+","-","|"}
 edge:="\x1b[0;34m"
 out:=[]string{edge+tl+strings.Repeat(h,w-2)+tr+"\x1b[0m"}
 for _,line:=range lines {out=append(out,edge+v+"\x1b[0m "+fitLine(line,w-4)+" "+edge+v+"\x1b[0m")}
 return append(out,edge+bl+strings.Repeat(h,w-2)+br+"\x1b[0m")
}
func splitLines(s string) []string{return strings.Split(strings.TrimSuffix(s,"\r\n"),"\r\n")}
func joinPanels(a,b []string,aw int) []string {
 out:=make([]string,max(len(a),len(b)))
 for i:=range out {
  l,r:="",""
  if i<len(a){l=a[i]};if i<len(b){r=b[i]}
  out[i]=fitLine(l,aw)+"  "+r
 }
 return out
}
func percentText(v float64) string {if v<0{return "N/A"};return fmt.Sprintf("%.1f%%",v)}
func storagePanel(volumes []volume,w,limit int,ascii bool) []string {
 lines:=[]string{"\x1b[1;36m// STORAGE\x1b[0m"}
 if len(volumes)==0{return append(lines,"N/A")}
 for i,v:=range volumes {
  if i>=limit{break}
  if v.total==0{continue}
  p:=float64(v.used)/float64(v.total)
  color:="36";if p>=.9{color="31"}else if p>=.8{color="33"}
  bar:=max(1,w-2)
  full:=min(bar,max(0,int(p*float64(bar)+.5)))
  on,off:="─","·";if ascii{on,off="#","-"}
  lines=append(lines,fmt.Sprintf("%-8s %s",v.label,percentText(p*100)),
   fmt.Sprintf("\x1b[1;%sm[%s\x1b[0;37m%s\x1b[1;%sm]\x1b[0m",color,strings.Repeat(on,full),strings.Repeat(off,bar-full),color),
   fmt.Sprintf("%s / %s   FREE %s",sizeText(v.used),sizeText(v.total),sizeText(v.available)))
 }
 if len(volumes)>limit{lines=append(lines,fmt.Sprintf("+%d more volumes",len(volumes)-limit))}
 return lines
}

func dashboard(now time.Time,width,height int,ascii,exhibit bool,period time.Duration,ch,mh,rh,th []float64,volumes []volume,info systemInfo,lastSample time.Time,live [4]float64,cores map[int]*coreSeries,hw hardwareInfo,dh,wh []float64,dr,dw float64) string {
 w:=min(140,width-1)
 if w<1||height<1{return "\x1b[H\x1b[J"}
 lines:=[]string{"\x1b[1;36m/// LOCKTTYUI // TELEMETRY\x1b[0m"}
 clock:=clockLines(now,w,height,ascii)
 lines=append(lines,clock...)
 lines=append(lines,centerLine(now.Format("2006-01-02 Mon"),w),telemetryLine(info,w,now,lastSample),"")
 footer:="DISPLAY MODE / q to exit"
 if exhibit{footer="CONSOLE // STANDBY  |  Ctrl+] LOGIN"}
 cpuVal,memVal:=live[0],live[1]
 rx,tx:=live[2],live[3]
 compact:=[]string{fmt.Sprintf("CPU %s   MEMORY %s",percentText(cpuVal),percentText(memVal)),fmt.Sprintf("RX %s  TX %s",rateText(rx),rateText(tx))}
 if w<40||height<24 {
  lines=append([]string{"LOCKTTYUI",now.Format("15:04:05")},compact...)
  for _,v:=range volumes{if v.total>0{lines=append(lines,fmt.Sprintf("%s %s FREE %s",v.label,percentText(100*float64(v.used)/float64(v.total)),sizeText(v.available)));break}}
 }else{
  // Reserve footer and choose graph height from actual panel row costs.
  side:=w>=76
  pw:=w
  if side{pw=(w-2)/2}
  remaining:=height-len(lines)-2
  limit:=min(3,max(1,(remaining-15)/3))
  storage:=storagePanel(volumes,w-4,limit,ascii)
  storage=append(storage,fmt.Sprintf("I/O R %s  W %s",rateText(dr),rateText(dw)))
  storageRows:=len(storage)+2
  extras:=0
  if hw.cpu!=""||hw.swap!=""{extras++}
  if hw.gpu!=""{extras++}
  remaining-=extras
  factor:=2;if side{factor=1}
  rows:=min(10,(remaining-storageRows-5-5*factor)/(factor+1))
  if rows>=2 {
   var a,b,n strings.Builder
   coreGraph(&a,cores,ch,pw-9,rows,ascii,period,cpuVal)
   graph(&b,"MEMORY",mh,pw-9,rows,ascii,period,memVal)
   al,bl:=splitLines(a.String()),splitLines(b.String())
   if hw.cpu!=""||hw.swap!=""{al=append(al,hw.cpu);bl=append(bl,hw.swap)}
   ap,bp:=panel(al,pw,ascii),panel(bl,pw,ascii)
   if side{lines=append(lines,joinPanels(ap,bp,pw)...)}else{lines=append(lines,ap...);lines=append(lines,bp...)}
   fmt.Fprintf(&n,"\x1b[1;36m// NETWORK RX %s\x1b[1;34m  TX %s\x1b[0m\r\n",rateText(rx),rateText(tx))
   trafficGraph(&n,rh,th,w-14,rows,ascii,period)
   lines=append(lines,panel(splitLines(n.String()),w,ascii)...)
   if remaining-(rows+5)*(factor+1)-storageRows>=5 {
    var disk strings.Builder
    trafficGraph(&disk,dh,wh,w-14,3,ascii,period,"R","W")
    storage=append(storage,splitLines(disk.String())...)
   }
   lines=append(lines,panel(storage,w,ascii)...)
   if hw.gpu!=""{lines=append(lines,"\x1b[1;36m// "+hw.gpu+"\x1b[0m")}
  }else{
   lines=append(lines,compact...)
   lines=append(lines,panel(storage,w,ascii)...)
  }
 }
 if len(lines)>=height{lines=lines[:max(0,height-1)]}
 lines=append(lines,footer)
 var b strings.Builder
 b.WriteString("\x1b[H")
 for i,line:=range lines {
  if i>0{b.WriteString("\r\n")}
  b.WriteString(fitLine(line,w));b.WriteString("\x1b[K")
 }
 b.WriteString("\x1b[J")
 return b.String()
}
