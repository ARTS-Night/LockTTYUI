//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type detailUnit struct {
	Name       string            `json:"name"`
	Status     string            `json:"collection"`
	Properties map[string]string `json:"properties"`
}
type historyItem struct {
	Unit  string `json:"unit"`
	Time  string `json:"time_utc"`
	Event string `json:"event"`
}
type detailReport struct {
	Units           []detailUnit  `json:"units"`
	History         []historyItem `json:"history"`
	JournalStatus   string        `json:"journal_status"`
	TTY1            string        `json:"tty1"`
	Token           string        `json:"handoff_token"`
	Fallback        string        `json:"fallback_marker"`
	Removing        string        `json:"removal_marker"`
	RuntimeOverride string        `json:"runtime_override"`
	TokenAge        string        `json:"handoff_token_age"`
	VendorDropIn    string        `json:"vendor_dropin"`
	AdminDropIn     string        `json:"admin_dropin_directory"`
	Package         string        `json:"package"`
	Limits          string        `json:"privacy_limits"`
}
type limitedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := 262144 - b.Len()
	if left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		_, _ = b.Buffer.Write(p)
	}
	if n > left {
		b.overflow = true
	}
	return n, nil
}
func diagnosticCommand(path string, args ...string) ([]byte, string) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "COMMAND_MISSING"
		}
		if errors.Is(err, os.ErrPermission) {
			return nil, "PERMISSION_DENIED"
		}
		return nil, "UNAVAILABLE"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "SYSTEMD_COLORS=0", "SYSTEMD_PAGER=cat"}
	cmd.WaitDelay = 100 * time.Millisecond
	var out, stderr limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, "TIMEOUT"
	}
	if out.overflow || stderr.overflow {
		return nil, "OUTPUT_LIMIT"
	}
	if err != nil {
		s := strings.ToLower(stderr.String())
		if strings.Contains(s, "permission denied") || strings.Contains(s, "access denied") {
			return nil, "PERMISSION_DENIED"
		}
		return nil, "COMMAND_FAILED"
	}
	if out.Len() == 0 {
		return nil, "NO_DATA"
	}
	return out.Bytes(), "OK"
}

var detailProperties = []string{"LoadState", "ActiveState", "SubState", "UnitFileState", "Result", "ExecMainCode", "ExecMainStatus", "MainPID", "ControlPID", "NRestarts", "Restart", "RestartUSec", "StartLimitIntervalUSec", "StartLimitBurst", "NeedDaemonReload"}

func safePropertyValue(s string) string {
	if len(s) == 0 || len(s) > 80 {
		return "UNKNOWN"
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune(" ._-", c)) {
			return "REDACTED"
		}
	}
	return s
}
func collectDetails(full bool) detailReport {
	d := detailReport{Limits: "Raw journal messages, command lines, unit paths, session identities and arbitrary settings are excluded.", History: []historyItem{}}
	names := []string{"lockttyui-tty1.service", "lockttyui-tty1-handoff.service", "lockttyui-tty1-return.service", "lockttyui-tty1-fallback.service", "getty@tty1.service"}
	for _, name := range names {
		u := detailUnit{Name: name, Properties: map[string]string{}}
		data, status := diagnosticCommand("/usr/bin/systemctl", "--system", "--no-pager", "show", name, "--property="+strings.Join(detailProperties, ","))
		u.Status = status
		if status == "OK" {
			for _, line := range strings.Split(string(data), "\n") {
				k, v, ok := strings.Cut(line, "=")
				if !ok {
					continue
				}
				for _, allowed := range detailProperties {
					if k == allowed {
						u.Properties[k] = safePropertyValue(v)
						break
					}
				}
			}
		}
		d.Units = append(d.Units, u)
	}
	lines := "100"
	if full {
		lines = "500"
	}
	args := []string{"--no-pager", "--boot=0", "--output=json", "--lines=" + lines, "--quiet"}
	for _, n := range names {
		args = append(args, "--unit="+n)
	}
	data, status := diagnosticCommand("/usr/bin/journalctl", args...)
	d.JournalStatus = status
	if os.Geteuid() != 0 {
		d.JournalStatus += " (journal visibility may be restricted)"
	}
	if status == "OK" {
		for _, line := range bytes.Split(data, []byte{'\n'}) {
			var e struct {
				Unit        string `json:"_SYSTEMD_UNIT"`
				ObjectUnit  string `json:"OBJECT_SYSTEMD_UNIT"`
				ManagerUnit string `json:"UNIT"`
				PID         string `json:"_PID"`
				Message     string `json:"MESSAGE"`
				Timestamp   string `json:"__REALTIME_TIMESTAMP"`
			}
			if json.Unmarshal(line, &e) != nil {
				continue
			}
			if e.PID == "1" && e.ManagerUnit != "" {
				e.Unit = e.ManagerUnit
			} else if e.ObjectUnit != "" {
				e.Unit = e.ObjectUnit
			}
			known := false
			for _, n := range names {
				if e.Unit == n {
					known = true
				}
			}
			if !known {
				continue
			}
			event := classifyJournal(e.Message)
			if event == "" {
				continue
			}
			micros, err := strconv.ParseInt(e.Timestamp, 10, 64)
			if err != nil {
				continue
			}
			d.History = append(d.History, historyItem{e.Unit, time.UnixMicro(micros).UTC().Format(time.RFC3339), event})
		}
	}
	d.TTY1 = "UNKNOWN"
	if fi, err := os.Stat("/dev/tty1"); err == nil {
		if fi.Mode()&os.ModeCharDevice != 0 {
			d.TTY1 = "character device present"
		} else {
			d.TTY1 = "not a character device"
		}
	} else if errors.Is(err, os.ErrNotExist) {
		d.TTY1 = "absent"
	} else if errors.Is(err, os.ErrPermission) {
		d.TTY1 = "PERMISSION_DENIED"
	}
	d.Token = "UNKNOWN"
	if fi, err := os.Lstat("/run/lockttyui-tty1-login.ready"); err == nil {
		d.Token = "present"
		if fi.Mode()&os.ModeSymlink != 0 {
			d.Token = "symlink (unexpected)"
		}
		age := time.Since(fi.ModTime())
		if age >= 0 {
			d.TokenAge = age.Round(time.Second).String()
		} else {
			d.TokenAge = "UNKNOWN"
		}
	} else if errors.Is(err, os.ErrNotExist) {
		d.Token = "absent"
	} else if errors.Is(err, os.ErrPermission) {
		d.Token = "PERMISSION_DENIED"
	}
	d.Fallback = pathState("/run/lockttyui-tty1-fallback")
	d.Removing = pathState("/run/lockttyui-tty1-removing")
	d.RuntimeOverride = pathState("/run/systemd/system/getty@tty1.service.d/90-lockttyui-fallback.conf")
	d.VendorDropIn = pathState("/usr/lib/systemd/system/getty@tty1.service.d/50-lockttyui.conf")
	d.AdminDropIn = pathState("/etc/systemd/system/getty@tty1.service.d")
	if strings.HasPrefix(diagnosticOS(), "debian ") {
		data, status = diagnosticCommand("/usr/bin/dpkg-query", "-W", "-f=${db:Status-Abbrev} ${Version} ${Architecture}", "lockttyui")
	} else {
		data, status = diagnosticCommand("/usr/bin/rpm", "-q", "--qf", "%{VERSION}-%{RELEASE}.%{ARCH}", "lockttyui")
	}
	d.Package = status
	if status == "OK" {
		d.Package = safePackageValue(strings.TrimSpace(string(data)))
	}
	return d
}
func safePackageValue(s string) string {
	if len(s) == 0 || len(s) > 100 {
		return "UNKNOWN"
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune(" ._+-:~", c)) {
			return "REDACTED"
		}
	}
	return s
}
func pathState(path string) string {
	fi, err := os.Lstat(path)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return "symlink"
		}
		return "present"
	}
	if errors.Is(err, os.ErrNotExist) {
		return "absent"
	}
	if errors.Is(err, os.ErrPermission) {
		return "PERMISSION_DENIED"
	}
	return "UNKNOWN"
}
func classifyJournal(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "start request repeated too quickly"):
		return "start limit reached"
	case strings.Contains(s, "permission denied"):
		return "permission denied"
	case strings.Contains(s, "timed out"):
		return "timeout"
	case strings.Contains(s, "failed with result"):
		return "service failed"
	case strings.Contains(s, "main process exited"):
		return "main process exited"
	case strings.Contains(s, "starting "):
		return "starting"
	case strings.Contains(s, "started "):
		return "started"
	case strings.Contains(s, "stopped "):
		return "stopped"
	case strings.Contains(s, "deactivated successfully"):
		return "deactivated"
	}
	return ""
}
func printDetails(d detailReport) {
	fmt.Println("[Effective unit properties]")
	for _, u := range d.Units {
		fmt.Printf("%s: %s\n", u.Name, u.Status)
		for _, k := range detailProperties {
			if v, ok := u.Properties[k]; ok {
				fmt.Printf("  %s=%s\n", k, v)
			}
		}
	}
	fmt.Printf("TTY1: %s\nHandoff token: %s\nPackage: %s\nJournal: %s\n", d.TTY1, d.Token, d.Package, d.JournalStatus)
	fmt.Printf("Fallback marker: %s\nRemoval marker: %s\nRuntime fallback override: %s\n", d.Fallback, d.Removing, d.RuntimeOverride)
	fmt.Printf("Token age: %s\nVendor drop-in: %s\nAdmin drop-in directory: %s\n", d.TokenAge, d.VendorDropIn, d.AdminDropIn)
	for _, h := range d.History {
		fmt.Printf("%s %s %s\n", h.Time, h.Unit, h.Event)
	}
	fmt.Println(d.Limits)
}

// Accept only known distributions and numeric release identifiers.
func diagnosticOS() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "UNKNOWN"
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(data) > 16384 {
		return "UNKNOWN"
	}
	id, version := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, "\"")
		if k == "ID" {
			switch v {
			case "almalinux", "debian":
				id = v
			}
		}
		if k == "VERSION_ID" {
			valid := len(v) > 0 && len(v) < 16
			for _, r := range v {
				if !(r >= '0' && r <= '9' || r == '.') {
					valid = false
				}
			}
			if valid {
				version = v
			}
		}
	}
	if id == "" {
		id = "other"
	}
	if version == "" {
		version = "UNKNOWN"
	}
	return id + " " + version
}
