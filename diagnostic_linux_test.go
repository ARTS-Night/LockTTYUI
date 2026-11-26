//go:build linux

package main

import (
	"encoding/json"
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
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDiagnosticCommandsDoNotRequireTTY$")
	cmd.Env = append(os.Environ(), "LOCKTTYUI_DIAGNOSTIC_CHILD=1")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	var report diagnosticReport
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, b)
	}
	if report.Schema != 1 || report.Details == nil || len(report.Units) != 5 || !strings.HasPrefix(report.Recovery, "UNKNOWN") {
		t.Fatalf("invalid schema: %+v", report)
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
