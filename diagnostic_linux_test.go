//go:build linux

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDiagnosticCommandsDoNotRequireTTY(t *testing.T) {
	if os.Getenv("LOCKTTYUI_DIAGNOSTIC_CHILD") == "1" {
		handled, err := diagnosticCLI([]string{"report", "--json"})
		if !handled || err != nil {
			os.Exit(2)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDiagnosticCommandsDoNotRequireTTY$")
	cmd.Env = append(os.Environ(), "LOCKTTYUI_DIAGNOSTIC_CHILD=1")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	if !strings.Contains(string(b), `"post_logout_recovery": "UNKNOWN`) {
		t.Fatalf("missing unknown recovery: %s", b)
	}
	if strings.ContainsRune(string(b), '\x1b') {
		t.Fatal("unexpected terminal escape")
	}
}

func TestDiagnosticRedaction(t *testing.T) {
	for _, value := range []string{"secret\nInjected=1", "\x1b[31msecret", "/home/alice/token"} {
		if got := safePropertyValue(value); got != "REDACTED" {
			t.Fatalf("unsafe value accepted: %q", got)
		}
	}
	if got := classifyJournal("Failed with result 'exit-code'. user=alice token=secret"); got != "service failed" {
		t.Fatal(got)
	}
	if got := classifyJournal("user=alice token=secret"); got != "" {
		t.Fatalf("unexpected log output: %q", got)
	}
}
