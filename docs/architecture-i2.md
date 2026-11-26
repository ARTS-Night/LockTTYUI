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

## I-2 implementation notes

The router emits fixed `LOCKTTYUI_EVENT=` messages to its unit journal. The
request, queue acceptance and request failure have distinct event IDs. Queue
acceptance means only that systemd accepted a job; the current report's unit
properties and classified manager journal provide separate observations, not a
proven per-request confirmation. Fallback selection and refusing to switch while
the other unit is not inactive also have fixed events. Logs contain no input,
identity or authentication data. The report accepts these exact messages only
from the three route units. Since the prelogin unit does not log a typed exit
reason, an administrative stop and Ctrl+] are not reliably distinguished.

A synchronous wait inside a handler ordered after its triggering unit risks
systemd job cycles. A one-shot verification timer could observe a failed or
skipped start without a daemon, but timeout, stale jobs, simultaneous removal,
unknown tty ownership and ensuring only one finite attempt need isolated systemd
and real-console tests. A watchdog requires a heartbeat and can kill a hung UI;
it must not reset or signal an unrelated authenticated session. No timer or
watchdog is added until that safety proof exists. If tty1 ownership or manager
state is unknown, do not claim recovery or force a terminal handoff.

`nobody` with its primary group works without account lifecycle scripts, but is
shared with other programs. DynamicUser reduces shared-account exposure and
avoids persistent users, yet tty access, SELinux domains, systemctl reads and
GPU access need testing on all four OSes. A dedicated system user provides a
stable isolated identity at the cost of package lifecycle and upgrade/removal
policy. Retain `nobody` temporarily; this is not an assertion that it provides
strong per-service isolation. Do not switch identities without those tests.
The router uses a fixed PATH, fixed actions and fixed systemctl executable; the
root-owned `/run` marker assumptions and check-then-act races remain. A private
root state directory or flock cannot be safely introduced without accounting
for upgrades, package removal and lock ordering. No lock was added here.

The 3,813,233-byte binary recorded at commit fea3957 was built in the Go 1.24
container without release stripping. It is one measurement, not an installed
package size or RSS measurement. Targets remain <5 MiB binary, <=10 MiB RSS,
<=1% of one CPU on average, zero new resident daemons and no VT-x/AMD-V
requirement; RSS, PSS, CPU, TUI subprocesses and long-term usage remain unmeasured.

### Failure handler change

A failed handoff or return oneshot now requests the existing fallback oneshot
through systemd OnFailure. The fallback first refuses to alter routing when
the dashboard or getty is active, starting, or otherwise not known inactive
or failed. This check is not atomic with the later job request and cannot
prove tty ownership. The tty1 getty retains its existing start limit; recovery may require a manual
reset from SSH or tty2 after the limit is reached. A failed fallback has no recursive failure handler.
The lease remains present when fallback requests getty because the getty unit
requires it; if fallback itself fails, the marker can remain and an
administrator must diagnose it. A successful `--no-block` enqueue still does
not prove startup; condition skips and job failures require further work.

### Router serialization and native getty limits

Route invocations use an exclusive advisory flock in a 0700 directory below
`/run`, validate ownership and mode, and wait at most three seconds. A lock
timeout returns failure; an existing OnFailure handler may request fallback.
The lock serializes the router only. Package lifecycle scripts and the systemd
manager do not take it: checks against their operations remain non-atomic. The
lock remains until the router exits, not until asynchronous jobs complete.

The fallback drop-in no longer adds the 3-starts-per-60-seconds limit, which
could stop a working getty after normal logout. It retains Restart=always and
the tty1 drop-in's existing 10-starts-per-20-seconds policy. Those limits are
still applied to getty starts, not independently to LockTTYUI recovery attempts.
`reset-failed` clears the counter once per fallback invocation and is not a
recovery-attempt budget. Thus repeated triggers and rapid successful logouts
remain unresolved; this change does not claim independent finite recovery.

Rootless Docker running `/usr/sbin/init` without privileged mode exited 255 in
this environment, and unshare could not write uid_map. These attempts did not
validate PID 1 unit behavior. An isolated VM or a delegated-cgroup test host is
needed for actual systemd tests; TTY, PAM and SELinux require separate tests.
