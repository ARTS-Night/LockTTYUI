//go:build linux

package main

import (
 "strings"
 "testing"
 "time"
)

func TestMountPaths(t *testing.T) {
 input:="1 0 8:1 / / rw - ext4 /dev/a rw\n2 1 8:2 / /data rw - xfs /dev/b rw\n3 1 8:2 / /duplicate rw - xfs /dev/b rw\n4 1 0:4 / /remote rw - nfs server:/ rw\n5 1 8:1 / /same rw - ext4 /dev/a rw"
 got:=mountPaths(input)
 if strings.Join(got,",")!="/,/data" {t.Fatal(got)}
}
func TestTrafficRate(t *testing.T) {
 prev:=map[string]traffic{"eth0":{100,200}}
 next:=map[string]traffic{"eth0":{300,300}}
 rx,tx,ok:=trafficRate(prev,next,2*time.Second)
 if !ok||rx!=100||tx!=50 {t.Fatal(rx,tx,ok)}
 if _,_,ok:=trafficRate(next,prev,time.Second);ok{t.Fatal("counter reset")}
 if _,_,ok:=trafficRate(prev,nil,time.Second);ok{t.Fatal("empty interfaces")}
}
func TestStorageLabels(t *testing.T) {
 var b strings.Builder
 storageLines(&b,[]volume{{"ROOT",capacity{100,50,40}}},80,1,true)
 if !strings.Contains(b.String(),"50.0%")||!strings.Contains(b.String(),"FREE 40.0B"){t.Fatal(b.String())}
}
