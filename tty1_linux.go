//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// tty1Bridge runs under getty@tty1.service. It never authenticates anybody;
// on completion or UI failure it replaces itself with the distribution's agetty.
// The dashboard child runs without root credentials, using inherited tty fds.
func runTTY1Bridge() error {
	if os.Geteuid() != 0 {
		return errors.New("tty1 bridge must be launched by the system getty service")
	}
	if !isTTY1FD(os.Stdin) || !isTTY1FD(os.Stdout) {
		return errors.New("tty1 bridge refused: stdin/stdout are not /dev/tty1")
	}

	stop := make(chan os.Signal, 2)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(stop)

	executable, err := os.Executable()
	if err == nil {
		child := exec.Command(executable, "--exhibit", "--tty-fd")
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		child.Env = os.Environ()
		child.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: 65534,
				Gid: 65534,
				Groups: []uint32{},
			},
		}
		if err = child.Start(); err == nil {
			done := make(chan error, 1)
			go func() { done <- child.Wait() }()
			select {
			case err = <-done:
				// Ctrl+] exits successfully. A nonzero exit or SIGKILL also
				// falls back to agetty, rather than trapping the console.
			case received := <-stop:
				_ = child.Process.Signal(received)
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					_ = child.Process.Kill()
					<-done
				}
				// Never start a login prompt during a systemd stop request.
				return nil
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "LockTTYUI dashboard unavailable; restoring agetty: %v\n", err)
	}
	select {
	case <-stop:
		return nil
	default:
	}
	return execGettyTTY1()
}

func isTTY1FD(f *os.File) bool {
	fdInfo, err := f.Stat()
	if err != nil || fdInfo.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	ttyInfo, err := os.Stat("/dev/tty1")
	return err == nil && os.SameFile(fdInfo, ttyInfo)
}

func safeTerm(term string) string {
	if term == "" || len(term) > 64 {
		return "linux"
	}
	for _, c := range term {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return "linux"
		}
	}
	return term
}

func tty1GettyArgs(term string) []string {
	// Match EL9's normal getty login invocation; no autologin, no PAM changes.
	return []string{"agetty", "-o", "-p -- \\u", "--noclear", "tty1", safeTerm(term)}
}

func execGettyTTY1() error {
	// Remove any partially typed bytes before agetty receives credentials.
	// agetty itself resets terminal modes if the dashboard was force-killed.
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), 0x540B /* Linux TCFLSH */, uintptr(syscall.TCIFLUSH))
	fmt.Fprint(os.Stdout, "\x1b[0m\x1b[?25h\x1b[2J\x1b[H")
	return syscall.Exec("/usr/sbin/agetty", tty1GettyArgs(os.Getenv("TERM")), os.Environ())
}

