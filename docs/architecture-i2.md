# I-2 architecture and safety review

Status: proposed deployment design based on commit a7166e4; live TTY, SELinux,
AppArmor, boot and logout behavior are not yet validated. This document is not
a claim that the current implementation satisfies all transitions below.

## Boundaries and state transitions

PID 1 starts an unprivileged TUI on tty1 and separately starts the distribution's
getty@tty1; getty invokes stock agetty/login/PAM. The TUI never calls the router.
Only three root-owned oneshot units call the fixed router actions. The dashboard
uses `User=nobody` and its primary group; Debian commonly calls that group
`nogroup`, while EL commonly calls it `nobody`. This shared account is less
isolated than a dedicated or dynamic identity; changing it needs TTY and driver
access tests on all targets. No new daemon, SUID program or Go dependency is
introduced. The privileged router's fixed actions use the system manager and
root-owned runtime state; no UI input becomes a command or argument.

```text
BOOT -> DASHBOARD -> HANDOFF -> LOGIN -> SESSION -> RETURN -> DASHBOARD
                   \-> FALLBACK -> LOGIN
             DASHBOARD failure -> FALLBACK -> LOGIN
             any -> STOPPING / REMOVING
```

| State | Entry and owner | Next / failure and conflict handling |
| --- | --- | --- |
| BOOT | systemd pulls dashboard into getty.target; PID 1 owns scheduling | DASHBOARD if conditions pass; skipped conditions are not proof of a healthy login |
| DASHBOARD | non-root TUI gets tty1 with tty-fail; no forced hangup | Ctrl+] and normal exit request HANDOFF; failure requests FALLBACK; occupied tty must not be seized |
| HANDOFF | root oneshot checks dashboard inactive and creates login lease | queues stock getty; queued job is not evidence of start; failure needs independent recovery |
| LOGIN | systemd starts vendor getty; it owns tty1 and authentication | SESSION on authentication; getty failure routes to FALLBACK |
| SESSION | standard login/PAM and user shell; no dashboard ownership | getty clean exit requests RETURN; never kill a live session |
| RETURN | root oneshot checks inactive getty and systemd monitor status | known successful exit removes lease and queues DASHBOARD; uncertain status chooses FALLBACK |
| FALLBACK | root oneshot creates boot-local getty override and lease | systemd retries stock getty every 3 seconds; remain here until reboot/removal |
| STOPPING | systemd stops units; router checks system state | no new route should be queued; checks and starts are not atomic |
| REMOVING | package marks removal, stops dashboard and routers | package removes its drop-in, reloads and requests stock getty; errors require SSH/tty2 recovery |

Oneshots have 30 second startup timeouts; the dashboard has a 3 second stop
timeout. No reliable dashboard hang detection exists. `--no-block` only queues
jobs, and a failed job, a skipped condition, router failure, stale marker, or
start limit can leave neither service running. Automatic recovery from those
cases is not established. Do not add synchronous waits between ordered units:
that can deadlock systemd. Detection and recovery require an independently
validated design that cannot terminate an authenticated session. Until then,
use SSH or tty2 recovery steps in OPERATIONS.md.

## Threats and unresolved risks

Unauthenticated users can interact with the dashboard; compromise could spoof a
login prompt until the process and children are terminated. systemd's default
control-group termination, tty-fail and stock getty's terminal reset are
relevant but require real-console verification, including queued input, echo,
termios restoration and child cleanup. The router executes as root and writes
markers in `/run`, assuming a root-owned, non-writable directory. Redirection
follows symlinks; checks and actions are not atomic. Root or a compromised
privileged service can violate that assumption. Removal markers, asynchronous
jobs and check-then-act sequences leave race windows. No claim of race freedom
or complete fallback is made. SELinux/AppArmor are not disabled. TUI systemd
hardening includes no capabilities, NoNewPrivileges, read-only system and home
protection; more restrictive address families, syscall filters and dynamic users
need compatibility testing with Go, systemctl and GPU probes. PrivateNetwork
would alter host network metrics; do not add it blindly.

Keep existing RPM/DEB unit and getty drop-in paths, route helper, diagnostic CLI,
UI, packaging and build workflow. This change removes only the nonportable
explicit primary group. The existing architecture already implements the broad
I-2 separation; simplification, new hardening and marker migration remain open
until failures can be reproduced in isolated systemd and physical-console tests.
Design targets (CPU, RSS, size) are not measured guarantees. Review pre-login
telemetry as public information and retain SSH or tty2 for recovery.

## Observability and validation plan

Proposed fixed event vocabulary: BOOT_REQUESTED, DASHBOARD_STARTED,
DASHBOARD_EXITED, HANDOFF_REQUESTED, GETTY_REQUESTED, GETTY_ACTIVE,
RETURN_REQUESTED, FALLBACK_REQUESTED, STOPPING and REMOVING. These names are
not all emitted today. A request event must never be labeled as a successful
start; observe actual systemd state separately. LOGIN and SESSION must remain
UNKNOWN if session evidence is unavailable. Never record passwords, user input,
user names, IP addresses or arbitrary environment values. Existing read-only
report output is bounded and redacted, but does not yet establish these events
or prove logout recovery. Validate it using fabricated hostile log fields and
real systemd journals before using it as a security decision.

The change to the account group is a static portability fix; it does not prove
that either account can acquire an actual tty or access GPU telemetry. Test
AlmaLinux 9/10 and Debian 12/13 package installation, restart and removal in
isolated machines, then verify SELinux Enforcing, standard login, logout,
20 cycles, SSH/tty2 isolation, crash and missing-binary recovery on the actual
AlmaLinux 10 system. No complete compatibility or release-readiness claim is
made from Go or container tests.
