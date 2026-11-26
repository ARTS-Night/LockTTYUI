//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Exercise the shipped router using a private state directory and fake PID 1.
// Neither real TTYs nor the host systemd manager are touched.
func TestTTY1RouteTransitions(t *testing.T) {
	original, err := os.ReadFile("packaging/lockttyui-tty1-route")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, action, ui, getty, phase, result, code, status string
		initialToken, initialFallback, removing              bool
		token, fallback, native                              bool
		start                                                string
	}{
		{"normal-handoff", "handoff", "inactive", "inactive", "running", "", "", "", false, false, false, true, false, false, "getty@tty1.service"},
		{"busy-ui", "handoff", "active", "inactive", "running", "", "", "", false, false, false, false, false, false, ""},
		{"normal-logout", "return", "inactive", "inactive", "running", "success", "exited", "0", true, false, false, false, false, false, "lockttyui-tty1.service"},
		{"live-login", "return", "inactive", "active", "running", "success", "exited", "0", true, false, false, true, false, false, ""},
		{"login-starting", "return", "inactive", "activating", "running", "success", "exited", "0", true, false, false, true, false, false, ""},
		{"ignored-nonzero", "return", "inactive", "inactive", "running", "success", "exited", "1", true, false, false, true, true, true, "getty@tty1.service"},
		{"ignored-kill", "return", "inactive", "inactive", "running", "success", "killed", "KILL", true, false, false, true, true, true, "getty@tty1.service"},
		{"getty-failed", "return", "inactive", "failed", "running", "exit-code", "exited", "1", true, false, false, true, true, true, "getty@tty1.service"},
		{"unknown-status", "return", "inactive", "inactive", "running", "", "", "", true, false, false, true, true, true, "getty@tty1.service"},
		{"fallback-live-getty", "fallback", "failed", "active", "running", "", "", "", false, false, false, false, false, false, ""},
		{"fallback-getty-starting", "fallback", "failed", "activating", "running", "", "", "", false, false, false, false, false, false, ""},
		{"fallback-getty-unknown", "fallback", "failed", "unknown", "running", "", "", "", false, false, false, false, false, false, ""},
		{"unknown-manager", "handoff", "inactive", "inactive", "unknown", "", "", "", false, false, false, false, false, false, ""},
		{"crash-fallback", "fallback", "failed", "inactive", "running", "", "", "", false, false, false, true, true, true, "getty@tty1.service"},
		{"fallback-logout", "return", "failed", "inactive", "running", "success", "exited", "0", true, true, false, true, true, true, "getty@tty1.service"},
		{"erase-handoff", "handoff", "inactive", "inactive", "running", "", "", "", false, false, true, false, false, false, ""},
		{"erase-fallback", "fallback", "failed", "inactive", "running", "", "", "", false, false, true, false, false, false, ""},
		{"erase-return", "return", "inactive", "inactive", "running", "success", "exited", "0", true, false, true, true, false, false, ""},
		{"shutdown", "handoff", "inactive", "inactive", "stopping", "", "", "", false, false, false, false, false, false, ""},
		{"boot", "handoff", "inactive", "inactive", "starting", "", "", "", false, false, false, true, false, false, "getty@tty1.service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fake := filepath.Join(dir, "systemctl")
			trace := filepath.Join(dir, "trace")
			script := filepath.Join(dir, "route")
			fixture := "#!/bin/sh\nset -eu\ncase \"$1\" in\nis-system-running) printf '%s\\n' \"$MOCK_PHASE\";;\nshow) case \"$5\" in lockttyui-tty1.service) printf '%s\\n' \"$MOCK_UI\";; getty@tty1.service) printf '%s\\n' \"$MOCK_GETTY\";; *) exit 9;; esac;;\ndaemon-reload|reset-failed) printf '%s\\n' \"$1\" >> \"$MOCK_TRACE\";;\n--no-block) [ \"$2\" = start ] && printf '%s\\n' \"$3\" >> \"$MOCK_TRACE\";;\n*) exit 9;;\nesac\n"
			if err := os.WriteFile(fake, []byte(fixture), 0700); err != nil {
				t.Fatal(err)
			}
			source := strings.Replace(string(original), "state=/run\n", "state='"+dir+"'\n", 1)
			source = strings.Replace(source, "systemctl=/usr/bin/systemctl\n", "systemctl='"+fake+"'\n", 1)
			if err := os.WriteFile(script, []byte(source), 0700); err != nil {
				t.Fatal(err)
			}
			token := filepath.Join(dir, "lockttyui-tty1-login.ready")
			fallback := filepath.Join(dir, "lockttyui-tty1-fallback")
			removing := filepath.Join(dir, "lockttyui-tty1-removing")
			dropin := filepath.Join(dir, "systemd/system/getty@tty1.service.d/90-lockttyui-fallback.conf")
			for path, exists := range map[string]bool{token: tc.initialToken, fallback: tc.initialFallback, removing: tc.removing} {
				if exists {
					if err := os.WriteFile(path, nil, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			cmd := exec.Command("/bin/sh", script, tc.action)
			cmd.Env = append(os.Environ(), "MOCK_UI="+tc.ui, "MOCK_GETTY="+tc.getty, "MOCK_PHASE="+tc.phase, "MOCK_TRACE="+trace,
				"MONITOR_UNIT=getty@tty1.service", "MONITOR_SERVICE_RESULT="+tc.result, "MONITOR_EXIT_CODE="+tc.code, "MONITOR_EXIT_STATUS="+tc.status)
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("route: %v %s", err, b)
			}
			for path, want := range map[string]bool{token: tc.token, fallback: tc.fallback, dropin: tc.native} {
				_, err := os.Stat(path)
				if (err == nil) != want {
					t.Fatalf("state %s: got %v want %v", filepath.Base(path), err == nil, want)
				}
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
			if tc.native {
				b, err := os.ReadFile(dropin)
				if err != nil {
					t.Fatal(err)
				}
				for _, directive := range []string{"OnSuccess=\n", "OnFailure=\n", "Restart=always", "RestartSec=3s"} {
					if !strings.Contains(string(b), directive) {
						t.Fatalf("native fallback missing %q", directive)
					}
				}
				if strings.Contains(string(b), "StartLimitIntervalSec=") || strings.Contains(string(b), "StartLimitBurst=") {
					t.Fatal("fallback must not override getty start limits")
				}
				if strings.Contains(string(b), "ExecStart=") {
					t.Fatal("fallback must not override the vendor getty command")
				}
			}
			b, err := os.ReadFile(trace)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			want := tc.start
			if tc.native {
				want = "daemon-reload\nreset-failed\n" + want
			}
			if strings.TrimSpace(string(b)) != want {
				t.Fatalf("trace: got %q want %q", b, want)
			}
		})
	}
}

func TestRouteReportsRejectedStart(t *testing.T) {
	original, err := os.ReadFile("packaging/lockttyui-tty1-route")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "systemctl")
	script := filepath.Join(dir, "route")
	fixture := "#!/bin/sh\ncase \"$1\" in is-system-running) echo running;; show) echo inactive;; --no-block) exit 1;; *) exit 9;; esac\n"
	if err := os.WriteFile(fake, []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(original), "state=/run\n", "state='"+dir+"'\n", 1)
	source = strings.Replace(source, "systemctl=/usr/bin/systemctl\n", "systemctl='"+fake+"'\n", 1)
	if err := os.WriteFile(script, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("/bin/sh", script, "handoff").CombinedOutput()
	if err == nil {
		t.Fatal("rejected start must fail")
	}
	if !strings.Contains(string(out), "LOCKTTYUI_EVENT=GETTY_START_REQUEST_FAILED") || strings.Contains(string(out), "LOCKTTYUI_EVENT=GETTY_START_QUEUED") {
		t.Fatalf("incorrect events: %s", out)
	}
}

// This models the OnFailure dispatch explicitly; it does not emulate systemd.
func TestRejectedHandoffThenFallback(t *testing.T) {
	original, err := os.ReadFile("packaging/lockttyui-tty1-route")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "systemctl")
	script := filepath.Join(dir, "route")
	trace := filepath.Join(dir, "trace")
	fixture := `#!/bin/sh
case "$1" in
 is-system-running) echo running ;;
 show) echo inactive ;;
 daemon-reload|reset-failed) exit 0 ;;
 --no-block) if [ ! -e "$MOCK_TRACE" ]; then : > "$MOCK_TRACE"; exit 1; fi; exit 0 ;;
 *) exit 9 ;;
esac
`
	if err = os.WriteFile(fake, []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(original), "state=/run\n", "state='"+dir+"'\n", 1)
	source = strings.Replace(source, "systemctl=/usr/bin/systemctl\n", "systemctl='"+fake+"'\n", 1)
	if err = os.WriteFile(script, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	run := func(action string) error {
		cmd := exec.Command("/bin/sh", script, action)
		cmd.Env = append(os.Environ(), "MOCK_TRACE="+trace)
		return cmd.Run()
	}
	if err = run("handoff"); err == nil {
		t.Fatal("first request unexpectedly succeeded")
	}
	if err = run("fallback"); err != nil {
		t.Fatalf("fallback request failed: %v", err)
	}
	for _, path := range []string{"lockttyui-tty1-login.ready", "lockttyui-tty1-fallback", "systemd/system/getty@tty1.service.d/90-lockttyui-fallback.conf"} {
		if _, err = os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
}

func TestRejectedDashboardThenFallback(t *testing.T) {
	original, err := os.ReadFile("packaging/lockttyui-tty1-route")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "systemctl")
	script := filepath.Join(dir, "route")
	trace := filepath.Join(dir, "trace")
	fixture := `#!/bin/sh
case "$1" in
 is-system-running) echo running ;;
 show) echo inactive ;;
 daemon-reload|reset-failed) exit 0 ;;
 --no-block) if [ ! -e "$MOCK_TRACE" ]; then : > "$MOCK_TRACE"; exit 1; fi; exit 0 ;;
 *) exit 9 ;;
esac
`
	if err = os.WriteFile(fake, []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(original), "state=/run\n", "state='"+dir+"'\n", 1)
	source = strings.Replace(source, "systemctl=/usr/bin/systemctl\n", "systemctl='"+fake+"'\n", 1)
	if err = os.WriteFile(script, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(dir, "lockttyui-tty1-login.ready")
	if err = os.WriteFile(token, nil, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(action string) error {
		cmd := exec.Command("/bin/sh", script, action)
		cmd.Env = append(os.Environ(), "MOCK_TRACE="+trace, "MONITOR_UNIT=getty@tty1.service", "MONITOR_SERVICE_RESULT=success", "MONITOR_EXIT_CODE=exited", "MONITOR_EXIT_STATUS=0")
		return cmd.Run()
	}
	if err = run("return"); err == nil {
		t.Fatal("dashboard request unexpectedly succeeded")
	}
	if err = run("fallback"); err != nil {
		t.Fatalf("fallback request failed: %v", err)
	}
	if _, err = os.Stat(token); err != nil {
		t.Fatalf("fallback must restore login lease: %v", err)
	}
	if _, err = os.Stat(filepath.Join(dir, "lockttyui-tty1-fallback")); err != nil {
		t.Fatal(err)
	}
}

func TestRouteLockTimeoutPreservesState(t *testing.T) {
	original, err := os.ReadFile("packaging/lockttyui-tty1-route")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	lockdir := filepath.Join(dir, "lockttyui-private")
	if err = os.Mkdir(lockdir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(lockdir, "route.lock"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	script := filepath.Join(dir, "route")
	source := strings.Replace(string(original), "state=/run\n", "state='"+dir+"'\n", 1)
	if err = os.WriteFile(script, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command("/bin/sh", script, "handoff").CombinedOutput()
	if err == nil || !strings.Contains(string(b), "ROUTE_LOCK_TIMEOUT") {
		t.Fatalf("expected bounded lock timeout: %v %s", err, b)
	}
	if _, err = os.Stat(filepath.Join(dir, "lockttyui-tty1-login.ready")); !os.IsNotExist(err) {
		t.Fatal("lock loser must not change login lease")
	}
}
