//go:build linux

package main

import (
    "os"
    "strings"
    "testing"
)

func unitDirectives(t *testing.T, path string) map[string][]string {
    t.Helper()
    data,err:=os.ReadFile(path)
    if err!=nil {t.Fatal(err)}
    props:=map[string][]string{}
    for _,raw:=range strings.Split(string(data),"\n"){
        line:=strings.TrimSpace(raw)
        if line==""||strings.HasPrefix(line,"#")||strings.HasPrefix(line,"["){continue}
        k,v,ok:=strings.Cut(line,"=");if !ok {t.Fatalf("malformed unit line: %s",line)}
        props[k]=append(props[k],v)
    }
    return props
}

func expectUnitValue(t *testing.T, p map[string][]string, key,value string) {
    t.Helper()
    for _,v:=range p[key] {if v==value {return}}
    t.Fatalf("expected %s=%s, got %v",key,value,p[key])
}

func TestTTY1LoginRunsStockGettyDirectly(t *testing.T){
    p:=unitDirectives(t,"packaging/50-lockttyui-tty1.conf")
    // No ExecStart replacement: PID 1 launches the distro's /usr/sbin/agetty,
    // preserving SELinux getty_t instead of inheriting the UI service domain.
    if len(p["ExecStart"])!=0 {t.Fatalf("getty command overridden: %v",p["ExecStart"])}
    expectUnitValue(t,p,"ConditionPathExists","/run/lockttyui-tty1-login.ready")
    expectUnitValue(t,p,"ExecStartPre","-/usr/bin/rm -f /run/lockttyui-tty1-login.ready")
    expectUnitValue(t,p,"Restart","no")
    expectUnitValue(t,p,"OnSuccess","lockttyui-tty1.service")
    expectUnitValue(t,p,"OnFailure","lockttyui-tty1.service")
}

func TestTTY1DashboardUnprivilegedAndSingleConsole(t *testing.T){
    p:=unitDirectives(t,"packaging/lockttyui-tty1.service")
    expectUnitValue(t,p,"User","nobody")
    expectUnitValue(t,p,"NoNewPrivileges","yes")
    expectUnitValue(t,p,"TTYPath","/dev/tty1")
    expectUnitValue(t,p,"ExecStart","/usr/bin/lockttyui --exhibit --tty-fd")
    expectUnitValue(t,p,"OnSuccess","lockttyui-tty1-handoff.service")
    expectUnitValue(t,p,"OnFailure","lockttyui-tty1-handoff.service")
    expectUnitValue(t,p,"Before","getty.target")
    expectUnitValue(t,p,"Restart","no")
    if len(p["ExecStartPre"])>0 {t.Fatal("never block boot with a long-lived pre-exec")}
}

func TestTTY1HandoffGatesGettyStart(t *testing.T){
    p:=unitDirectives(t,"packaging/lockttyui-tty1-handoff.service")
    expectUnitValue(t,p,"Type","oneshot")
    expectUnitValue(t,p,"ExecStart","/usr/bin/touch /run/lockttyui-tty1-login.ready")
    expectUnitValue(t,p,"ExecStartPost","/usr/bin/systemctl --no-block start getty@tty1.service")
}

func TestRPMChangesOnlyTTY1(t *testing.T) {
    data,err:=os.ReadFile("packaging/lockttyui.spec");if err!=nil{t.Fatal(err)}
    s:=string(data)
    for _,required:=range []string{
        "getty@tty1.service.d/50-lockttyui.conf",
        "getty.target.wants/lockttyui-tty1.service",
        "lockttyui-tty1-handoff.service",
    }{if !strings.Contains(s,required){t.Fatalf("RPM missing %q",required)}}
    for _,forbidden:=range []string{"getty@tty2.service.d","getty@.service.d","setenforce 0","setsebool"}{
        if strings.Contains(s,forbidden){t.Fatalf("Unexpected global/SELinux change: %q",forbidden)}
    }
}
