//go:build linux

package main

import (
 "syscall"
 "time"
 "unsafe"
)

// ppoll is available on Linux amd64 and arm64 without external modules.
type pollDescriptor struct{fd int32; events,revents int16}
func waitTTY(fd uintptr,wait time.Duration)(bool,error){
 p:=pollDescriptor{fd:int32(fd),events:1}
 ts:=syscall.NsecToTimespec(int64(wait))
 _,_,err:=syscall.Syscall6(syscall.SYS_PPOLL,uintptr(unsafe.Pointer(&p)),1,uintptr(unsafe.Pointer(&ts)),0,0,0)
 if err==syscall.EINTR{return false,nil}
 if err!=0{return false,err}
 if p.revents&0x38!=0{return false,syscall.EIO}
 return p.revents&1!=0,nil
}
