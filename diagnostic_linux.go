//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

type diagnosticUnit struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Result  string `json:"result"`
	Restart string `json:"restart"`
}
type diagnosticReport struct {
	Schema       int              `json:"schema"`
	CollectedAt  string           `json:"collected_at_utc"`
	OS           string           `json:"os"`
	Version      string           `json:"version"`
	Architecture string           `json:"architecture"`
	Units        []diagnosticUnit `json:"units"`
	Recovery     string           `json:"post_logout_recovery"`
	Details      *detailReport    `json:"details,omitempty"`
}

var buildRevision = ""
var buildNumber = ""

func buildVersion() string {
	if buildRevision != "" && buildNumber != "" {
		return "0.6.0 (build " + safePropertyValue(buildNumber) + ", commit " + safePropertyValue(buildRevision) + ")"
	}
	if b, ok := debug.ReadBuildInfo(); ok {
		for _, s := range b.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return "0.6.0 (" + s.Value[:12] + ")"
			}
		}
	}
	return "0.6.0 (revision unavailable)"
}

// Only fixed property names and known values are emitted. Unit command lines,
// journal entries and arbitrary configuration can contain private information.
func inspectUnit(name string) diagnosticUnit {
	u := diagnosticUnit{Name: name, State: "UNKNOWN", Result: "UNKNOWN", Restart: "UNKNOWN"}
	data, status := diagnosticCommand("/usr/bin/systemctl", "--system", "--no-pager", "show", name, "--property=ActiveState,Result,Restart")
	if status != "OK" {
		return u
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "ActiveState":
			switch v {
			case "active", "inactive", "failed", "activating", "deactivating", "reloading", "maintenance":
				u.State = v
			}
		case "Result":
			switch v {
			case "success", "resources", "protocol", "timeout", "exit-code", "signal", "core-dump", "watchdog", "start-limit-hit", "dependency", "skipped", "condition", "assert":
				u.Result = v
			}
		case "Restart":
			switch v {
			case "no", "always", "on-success", "on-failure", "on-abnormal", "on-watchdog", "on-abort":
				u.Restart = v
			}
		}
	}
	return u
}
func diagnosticCLI(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if args[0] == "--version" {
		if len(args) != 1 {
			return true, fmt.Errorf("--version takes no arguments")
		}
		fmt.Println("LockTTYUI", buildVersion())
		return true, nil
	}
	if args[0] != "status" && args[0] != "doctor" && args[0] != "report" {
		return false, nil
	}
	jsonOutput := false
	full := false
	if len(args) > 1 {
		if args[0] == "report" && len(args) == 2 && args[1] == "--json" {
			jsonOutput = true
		} else if args[0] == "report" && len(args) == 2 && args[1] == "--full" {
			full = true
		} else {
			return true, fmt.Errorf("usage: lockttyui status | doctor | report [--json|--full]")
		}
	}
	report := diagnosticReport{Schema: 1, CollectedAt: time.Now().UTC().Format(time.RFC3339), OS: diagnosticOS(), Version: buildVersion(), Architecture: runtime.GOARCH, Recovery: "UNKNOWN: login/logout history has not been verified"}
	for _, name := range []string{"lockttyui-tty1.service", "lockttyui-tty1-handoff.service", "lockttyui-tty1-return.service", "lockttyui-tty1-fallback.service", "getty@tty1.service"} {
		report.Units = append(report.Units, inspectUnit(name))
	}
	if args[0] != "status" {
		d := collectDetails(full)
		report.Details = &d
	}
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return true, enc.Encode(report)
	}
	if args[0] == "report" {
		fmt.Println("===== BEGIN LOCKTTYUI REPORT =====")
	}
	fmt.Printf("Collected: %s\nOS: %s\nLockTTYUI: %s\nArchitecture: %s\n", report.CollectedAt, report.OS, report.Version, report.Architecture)
	for _, u := range report.Units {
		fmt.Printf("%s: state=%s result=%s restart=%s\n", u.Name, u.State, u.Result, u.Restart)
		if args[0] != "status" && u.State == "failed" {
			fmt.Printf("[FAIL] SERVICE_FAILED: %s; inspect its journal with appropriate privileges.\n", u.Name)
		}
	}
	if args[0] != "status" {
		fmt.Println("[UNKNOWN] POST_LOGOUT_RECOVERY_UNKNOWN: login/logout history was not checked; verify on the target system.")
		fmt.Println("[UNKNOWN] SELINUX_DENIAL: audit records are not inspected by this report.")
		fmt.Println("[UNKNOWN] TTY1_SESSION: session occupancy was not checked.")
	}
	if report.Details != nil {
		printDetails(*report.Details)
	}
	if args[0] == "report" {
		fmt.Println("===== END LOCKTTYUI REPORT =====")
	}
	return true, nil
}
