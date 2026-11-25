//go:build linux

package main

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTTY1GettyArguments(t *testing.T) {
	want := []string{"agetty", "-o", "-p -- \\u", "--noclear", "-", "linux"}
	if got := tty1GettyArgs(""); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected normal login, got %q", got)
	}
	if got := tty1GettyArgs("xterm-256color"); got[len(got)-1] != "xterm-256color" {
		t.Fatalf("valid TERM rejected: %q", got)
	}
	for _, term := range []string{"xterm;evil", "../console", strings.Repeat("x", 65)} {
		if got := tty1GettyArgs(term); got[len(got)-1] != "linux" {
			t.Fatalf("untrusted TERM accepted: %q", got)
		}
	}
	for _, arg := range want {
		if strings.Contains(arg, "--autologin") {
			t.Fatalf("autologin must not be enabled: %q", want)
		}
	}
}

func TestTTY1RejectsUnrelatedTerminal(t *testing.T) {
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTTY1FD(f) {
		t.Fatal("non-tty1 fd accepted")
	}
}

func TestTTY1VendorDropin(t *testing.T) {
	data, err := os.ReadFile("packaging/50-lockttyui-tty1.conf")
	if err != nil {
		t.Fatal(err)
	}
	active := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			active = append(active, line)
		}
	}
	s := strings.Join(active, "\n")
	for _, required := range []string{"[Service]", "ExecStart=", "ExecStart=-/usr/bin/lockttyui --tty1-bridge", "TTYPath=/dev/tty1", "Restart=always"} {
		if !strings.Contains(s, required) {
			t.Fatalf("missing tty1-only directive %q: %s", required, s)
		}
	}
	if strings.Contains(s, "getty@.service") || strings.Contains(s, "tty2") || strings.Contains(s, "--autologin") {
		t.Fatalf("drop-in changed another console or authentication: %s", s)
	}
}

type partialWriter struct{ bytes []byte; max int }
func (p *partialWriter) Write(b []byte) (int, error) {
	if len(b) > p.max {
		b = b[:p.max]
	}
	p.bytes = append(p.bytes, b...)
	return len(b), nil
}
type zeroWriter struct{}
func (zeroWriter) Write([]byte) (int,error) {return 0,nil}

func TestWriteFullHandlesPartialWrites(t *testing.T) {
	p := &partialWriter{max: 2}
	if err := writeFull(p, []byte("lockttyui")); err != nil {
		t.Fatal(err)
	}
	if string(p.bytes) != "lockttyui" {
		t.Fatalf("lost bytes: %q", p.bytes)
	}
	if err := writeFull(zeroWriter{}, []byte("x")); err != io.ErrShortWrite {
		t.Fatalf("zero-progress write must fail, got %v", err)
	}
}
