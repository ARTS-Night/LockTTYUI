//go:build linux

package main

import (
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
)

// Exercise the shipped shell router with a private state directory and fake
// systemctl. No host systemd actions or real TTY writes occur in these tests.
func TestTTY1RouteTransitions(t *testing.T) {
 original,err:=os.ReadFile("packaging/lockttyui-tty1-route")
 if err!=nil{t.Fatal(err)}
 for _,tc:=range []struct{
  name,action,ui,getty string
  initialToken,initialFallback,removing bool
  token,fallback bool
  start string
 }{
  {"normal-handoff","handoff","inactive","inactive",false,false,false,true,false,"getty@tty1.service"},
  {"busy-ui","handoff","active","inactive",false,false,false,false,false,""},
  {"normal-logout","return","inactive","inactive",true,false,false,false,false,"lockttyui-tty1.service"},
  {"live-login","return","inactive","active",true,false,false,true,false,""},
  {"login-starting","return","inactive","activating",true,false,false,true,false,""},
  {"crash-fallback","fallback","failed","inactive",false,false,false,true,true,"getty@tty1.service"},
  {"fallback-logout","return","failed","inactive",true,true,false,true,true,"getty@tty1.service"},
  {"erase-handoff","handoff","inactive","inactive",false,false,true,false,false,""},
  {"erase-fallback","fallback","failed","inactive",false,false,true,false,false,""},
  {"erase-return","return","inactive","inactive",true,false,true,true,false,""},
 }{
  t.Run(tc.name,func(t *testing.T){
   dir:=t.TempDir()
   fake:=filepath.Join(dir,"systemctl")
   trace:=filepath.Join(dir,"trace")
   script:=filepath.Join(dir,"route")
   fixture:="#!/bin/sh\nset -eu\ncase \"$1\" in\nshow) case \"$5\" in lockttyui-tty1.service) printf '%s\\n' \"$MOCK_UI\";; getty@tty1.service) printf '%s\\n' \"$MOCK_GETTY\";; *) exit 9;; esac;;\n--no-block) [ \"$2\" = start ] && printf '%s\\n' \"$3\" >> \"$MOCK_TRACE\";;\n*) exit 9;;\nesac\n"
   if err:=os.WriteFile(fake,[]byte(fixture),0700);err!=nil{t.Fatal(err)}
   source:=strings.Replace(string(original),"state=/run\n","state='"+dir+"'\n",1)
   source=strings.Replace(source,"systemctl=/usr/bin/systemctl\n","systemctl='"+fake+"'\n",1)
   if err:=os.WriteFile(script,[]byte(source),0700);err!=nil{t.Fatal(err)}
   token:=filepath.Join(dir,"lockttyui-tty1-login.ready")
   fallback:=filepath.Join(dir,"lockttyui-tty1-fallback")
   removing:=filepath.Join(dir,"lockttyui-tty1-removing")
   for path,exists:=range map[string]bool{token:tc.initialToken,fallback:tc.initialFallback,removing:tc.removing}{
    if exists{if err:=os.WriteFile(path,nil,0600);err!=nil{t.Fatal(err)}}
   }
   cmd:=exec.Command("/bin/sh",script,tc.action)
   cmd.Env=append(os.Environ(),"MOCK_UI="+tc.ui,"MOCK_GETTY="+tc.getty,"MOCK_TRACE="+trace)
   if b,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("route: %v %s",err,b)}
   for path,want:=range map[string]bool{token:tc.token,fallback:tc.fallback}{
    _,err:=os.Stat(path)
    if (err==nil)!=want{t.Fatalf("state %s: got %v want %v",filepath.Base(path),err==nil,want)}
    if err!=nil&&!os.IsNotExist(err){t.Fatal(err)}
   }
   b,err:=os.ReadFile(trace)
   if err!=nil&&!os.IsNotExist(err){t.Fatal(err)}
   if strings.TrimSpace(string(b))!=tc.start{t.Fatalf("start: got %q want %q",b,tc.start)}
  })
 }
}
