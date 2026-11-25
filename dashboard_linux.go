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
  color:="6";if p>=.9{color="1"}else if p>=.8{color="3"}
  lines=append(lines,fmt.Sprintf("%-8s %s",v.label,percentText(p*100)),
   meter(p*100,max(1,w),color),
   fmt.Sprintf("%s / %s   FREE %s",sizeText(v.used),sizeText(v.total),sizeText(v.available)))
 }
 if len(volumes)>limit{lines=append(lines,fmt.Sprintf("+%d more volumes",len(volumes)-limit))}
 return lines
}

func dashboard(now time.Time,width,height int,ascii,exhibit bool,period time.Duration,ch,mh,sh,rh,th []float64,volumes []volume,info systemInfo,lastSample time.Time,live [4]float64,cores map[int]*coreSeries,hw hardwareInfo,dr,dw float64,netScale float64) string {
 w:=min(512,width-1)
 height=min(256,height)
 if w<1||height<1{return "\x1b[H\x1b[J"}
 head:="\x1b[1;36m/// LOCKTTYUI // TELEMETRY\x1b[0m"
 pulse:=systemIndicator(hw,ascii)
 if visibleWidth(head)+visibleWidth(pulse)+2>w{head="\x1b[1;36mLOCKTTYUI\x1b[0m"}
 if visibleWidth(head)+visibleWidth(pulse)+2<=w{head+=strings.Repeat(" ",w-visibleWidth(head)-visibleWidth(pulse))+pulse}
 lines:=[]string{head}
 clock:=clockLines(now,w,height,ascii)
 lines=append(lines,clock...)
 lines=append(lines,centerLine(now.Format("2006-01-02 Mon"),w),telemetryLine(info,w,now,lastSample),"")
 footer:="DISPLAY MODE / q to exit"
 if exhibit{footer="CONSOLE // STANDBY  |  Ctrl+] LOGIN"}
 cpuVal,memVal:=live[0],live[1]
 rx,tx:=live[2],live[3]
 compact:=[]string{fmt.Sprintf("CPU %s   RAM+SWAP %s",percentText(cpuVal),percentText(memVal)),fmt.Sprintf("RX %s  TX %s",rateText(rx),rateText(tx))}
 if w<40||height<24 {
  lines=append([]string{head,now.Format("15:04:05")},compact...)
  for _,v:=range volumes{if v.total>0{lines=append(lines,fmt.Sprintf("%s %s FREE %s",v.label,percentText(100*float64(v.used)/float64(v.total)),sizeText(v.available)));break}}
 }else{
  // Fit complete panels, reducing optional detail before using compact mode.
  side:=w>=76
  pw:=w
  if side{pw=(w-2)/2}
  budget:=height-len(lines)-1
  fitted:=false
  for detail:=3;detail>=0&&!fitted;detail--{
   // Calculate the exact panel cost before rendering any history.
   coreLines:=[]string{}
   if detail>=1{coreLines=coreMeterLines(cores,pw-4,detail+1+max(0,(height-48)/12))}
   if hw.cpu!=""{coreLines=append(coreLines,hw.cpu)}
   memLines:=memoryMeterLines(hw,pw-4,detail>=1)
   storage:=storagePanel(volumes,w-4,max(1,detail+max(0,(height-48)/12)),ascii)
   storage=append(storage,fmt.Sprintf("I/O R %s  W %s",rateText(dr),rateText(dw)))
   extra:=len(storage)+2
   if hw.gpu!=""&&detail>=1{extra+=4}
   status:=statusLines(hw.statuses,hw.docker,w-4,ascii,hw.checking)
   if detail>=1&&len(status)>0{extra+=len(status)+2}
   // Reserve event space before choosing graph height, so large graphs
   // cannot consume the entire event panel budget.
   eventLines:=[]string(nil)
   if detail>=1&&budget>=30{eventLines=hw.events.lines(min(3,max(1,(height-30)/10)))}
   eventCost:=0
   if len(eventLines)>0{eventCost=len(eventLines)+2}
   // CPU and RAM share height; network uses an even or odd row count.
   // Select geometry using integers only, then draw once.
   for rows:=max(2,budget);rows>=2;rows--{
    cpuRows,memRows:=rows+5+len(coreLines),rows+5+len(memLines)
    upperRows:=cpuRows+memRows
    if side{upperRows=max(cpuRows,memRows)}
    networkRows:=rows+5
    if upperRows+networkRows+extra+eventCost>budget{continue}
    var a,b,n strings.Builder
    barHistory(&a,fmt.Sprintf("CPU (%d LOGICAL)",len(cores)),ch,pw-9,rows,ascii,period,cpuVal,"6")
    stackedMemoryHistory(&b,mh,sh,pw-9,rows,ascii,period,hw)
    al,bl:=splitLines(a.String()),splitLines(b.String())
    al=append(al,coreLines...)
    bl=append(bl,memLines...)
    if side{for len(al)<len(bl){al=append(al,"")};for len(bl)<len(al){bl=append(bl,"")}}
    ap,bp:=panel(al,pw,ascii),panel(bl,pw,ascii)
    candidate:=[]string{}
    if side{candidate=append(candidate,joinPanels(ap,bp,pw)...)}else{candidate=append(candidate,ap...);candidate=append(candidate,bp...)}
    fmt.Fprintf(&n,"\x1b[1;36m// NETWORK RX %s\x1b[1;34m  TX %s\x1b[0m\r\n",rateText(rx),rateText(tx))
    mirroredBars(&n,rh,th,w-14,rows,ascii,period,"RX","TX",netScale)
    candidate=append(candidate,panel(splitLines(n.String()),w,ascii)...)
    candidate=append(candidate,panel(storage,w,ascii)...)
    if hw.gpu!=""&&detail>=1{candidate=append(candidate,panel([]string{"\x1b[1;36m// GPU\x1b[0m",hw.gpu},w,ascii)...)}
    if detail>=1&&len(status)>0{candidate=append(candidate,panel(status,w,ascii)...)}
    if len(eventLines)>0{candidate=append(candidate,panel(eventLines,w,ascii)...)}
    if len(candidate)<=budget{lines=append(lines,candidate...);fitted=true;break}
   }
  }
  if !fitted{
   lines=append([]string{head,now.Format("15:04:05")},compact...)
   if len(volumes)>0&&volumes[0].total>0{v:=volumes[0];lines=append(lines,fmt.Sprintf("%s %s FREE %s",v.label,percentText(100*float64(v.used)/float64(v.total)),sizeText(v.available)))}
  }
 }
 if len(lines)>=height{lines=lines[:max(0,height-1)]}
 for len(lines)<height-1{lines=append(lines,"")}
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
