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
