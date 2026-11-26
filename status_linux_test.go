//go:build linux
package main
import (
 "strings"
 "testing"
)
func TestServiceStatus(t *testing.T){
 data:="Id=docker.service\nLoadState=loaded\nActiveState=active\n\nId=sshd.service\nLoadState=loaded\nActiveState=failed\n\nId=nginx.service\nLoadState=not-found\nActiveState=inactive\n\nId=secret.service\nLoadState=loaded\nActiveState=active"
 items:=parseServiceStatus(data)
 if len(items)!=2||items[0].color!="2"||items[1].color!="1"{t.Fatal(items)}
 for _,ascii:=range []bool{true,false}{
  lines:=statusLines(items,"",60,ascii)
  for _,line:=range lines{if visibleWidth(line)>60{t.Fatal(line)}}
  if strings.Contains(strings.Join(lines,""),"NGINX"){t.Fatal(lines)}
 }
 if len(statusLines(nil,"",60,true))!=0{t.Fatal("empty panel")}
}

func TestFailedCount(t *testing.T){
 for _,v:=range []string{"","N/A","-1","2\n3"}{if _,ok:=parseFailedCount(v);ok{t.Fatal(v)}}
 if n,ok:=parseFailedCount(" 3\n");!ok||n!=3{t.Fatal(n,ok)}
 for _,v:=range []struct{hw hardwareInfo;want string}{
 {hardwareInfo{},"N/A"},
 {hardwareInfo{checking:true,failedKnown:true},"CHECKING"},
 {hardwareInfo{failedKnown:true},"00 ERR"},
 {hardwareInfo{failedKnown:true,failedUnits:3},"03 ERR"},
 }{if !strings.Contains(systemIndicator(v.hw,true),v.want){t.Fatal(v)}}
}

func TestDebianStatusAliasesAndAppArmor(t *testing.T){
 data:="Id=ssh.service\nLoadState=loaded\nActiveState=active\n\nId=ssh.service\nLoadState=loaded\nActiveState=active"
 items:=parseServiceStatus(data)
 if len(items)!=1||items[0].label!="SSH"||items[0].state!="ACTIVE"{t.Fatal(items)}
 if item,ok:=apparmorStatus("Y\n");!ok||item.state!="ENABLED"||item.color=="2"{t.Fatal(item,ok)}
 if _,ok:=apparmorStatus("unknown");ok{t.Fatal("unknown AppArmor state")}
}
