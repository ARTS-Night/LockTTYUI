# LockTTYUI

**Your Linux server, at a glance — before you log in.**

A lightweight, read-only system telemetry dashboard for the Linux text console.
LockTTYUI turns TTY1 into a live status display while keeping the distribution's
normal login, PAM and security policies in place.

[![Linux packages](https://github.com/ARTS-Night/LockTTYUI/actions/workflows/build-rpm.yml/badge.svg?branch=finish-build18)](https://github.com/ARTS-Night/LockTTYUI/actions/workflows/build-rpm.yml)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
![Platform](https://img.shields.io/badge/Linux-x86__64-555555?logo=linux)
![Status](https://img.shields.io/badge/Status-Release%20candidate-555555)

[Installation](#installation) · [Usage](#usage) · [Diagnostics](#diagnostics) · [Compatibility](#compatibility) · [Build from source](#build-from-source)

## Features

- **Live system overview:** CPU and per-core activity, RAM, swap, storage, disk I/O, network traffic, uptime and available hardware metrics.
- **Console-first design:** responsive monochrome layout, 60-second graphs, activity indicators and an ASCII fallback.
- **Normal Linux login:** press Ctrl+] to open the stock login prompt; a confirmed clean logout returns to the dashboard.
- **Lightweight by design:** one dashboard process, bounded history, cached graphs and no third-party Go dependencies. Routing helpers run only during transitions.
- **TTY1 only:** SSH, other virtual terminals and distribution authentication are left unchanged.
- **Easy maintenance:** native RPM and DEB packages, with an SSH-friendly diagnostic report in the current release candidate.

> [!NOTE]
> LockTTYUI is a **pre-login dashboard**, not a session locker. Running it
> inside an existing shell does not lock that session or provide authentication.

## Usage

```text
Boot
  |
  v
LockTTYUI dashboard
  |  Ctrl+]
  v
Standard Linux login (agetty / login / PAM)
  |
  v
User shell
  |  exit
  v
LockTTYUI dashboard
```

| Action | Result |
| --- | --- |
| Boot the server | Display the dashboard on TTY1 |
| Ctrl + ] | Hand off to the normal login prompt |
| Log in | Use the normal Linux shell |
| `exit` on TTY1 | Return to the dashboard after a confirmed clean logout |
| Connect using SSH | Continue using SSH normally, independently of TTY1 |

If a handoff fails or a clean exit cannot be confirmed, the intended fallback is
the distribution's standard getty until the next boot. A failed dashboard must
not become an alternative authentication mechanism.

## Compatibility

Requires an x86_64/amd64 Linux text-console server with systemd 252 or later.
A graphical display manager or a non-systemd init is outside the current scope.

| Distribution | Package | Build 20 package CI | Build 20 console test |
| --- | --- | --- | --- |
| AlmaLinux 9 | RPM | Passed | One successful login/logout on 9.9 (systemd 252) |
| AlmaLinux 10 | RPM | Passed | Retest pending |
| Debian 12 | DEB | Passed | Pending |
| Debian 13 | DEB | Passed | Pending |

**Validation record:** November 26, 2026 — [build 20 source](https://github.com/ARTS-Night/LockTTYUI/commit/5748f93c40ef143ba3589b0fa00f30ecb1ba2ac4) · [CI results](https://github.com/ARTS-Night/LockTTYUI/actions/runs/45804154531) · [test PR #2](https://github.com/ARTS-Night/LockTTYUI/pull/2).

Build 20 is a test candidate, **not a published GitHub Release**.
Package CI is not proof of live console ownership, PAM or SELinux/AppArmor
behavior. Repeated logout, fault recovery and live removal tests remain pending.

## Installation

> [!IMPORTANT]
> Use SSH or TTY2 for installation and removal, and keep that recovery path
> available. Test in a VM before using a production server. Packages are
> currently unsigned; checksums from the same repository verify integrity,
> not independent publisher authentication.

### 1. Choose the package

Visit [GitHub Releases](https://github.com/ARTS-Night/LockTTYUI/releases).
Select a matching OS package and its release's `SHA256SUMS`.
Here, `N` denotes the actual release build number.

| Distribution | Package filename |
| --- | --- |
| AlmaLinux 9 | `lockttyui-0.6.0-N.el9.x86_64.rpm` |
| AlmaLinux 10 | `lockttyui-0.6.0-N.el10.x86_64.rpm` |
| Debian 12 | `lockttyui_0.6.0-N+deb12_amd64.deb` |
| Debian 13 | `lockttyui_0.6.0-N+deb13_amd64.deb` |

The AlmaLinux 9 installer helper still defaults to build 15, the previously
validated release. Builds 18 and 19 have a known logout-return regression;
the build 20 fix is currently available from the test branch and CI artifacts,
not as a release download. Do not assume the newest published package includes
the fix.

### 2. Verify and install

In the download directory:

```bash
sha256sum --ignore-missing -c SHA256SUMS
```

Confirm that your exact package filename is reported as `OK` before continuing.
Replace the example filename below with the package you downloaded.

**AlmaLinux**

```bash
sudo dnf install ./filename.rpm
```

**Debian**

```bash
sudo apt install ./filename.deb
```

### 3. Activate

```bash
sudo reboot
```

Reboot at a suitable time. No manual `systemctl enable` is required.
Installation does not intentionally stop an existing TTY1 login.
After an upgrade, the running dashboard picks up the new binary when
restarted; reboot clears any boot-local fallback state.

<details>
<summary>Optional release installer — AlmaLinux 9 only</summary>

The helper downloads a specific published RPM, checks its GitHub asset digest,
and uses DNF. It does not downgrade a newer installed version.

```bash
git clone https://github.com/ARTS-Night/LockTTYUI.git
cd LockTTYUI
git checkout build-15
less scripts/install.sh
bash scripts/install.sh install
```

Use `bash scripts/install.sh status`, `bash scripts/install.sh remove`, or
`bash scripts/install.sh install build-N` to explicitly select a published
test release. Requires curl, Python 3, sha256sum, sudo and DNF.
The helper does not install unpublished PR artifacts.

</details>

## Diagnostics

From SSH or TTY2, on build 19 or later containing the diagnostic CLI:

```bash
sudo lockttyui report
```

The report includes package and OS information, effective unit states,
handoff/fallback markers and classified journal events. Raw journal messages,
command lines, session identities and input are excluded. Review the output
before sharing it in a [GitHub issue](https://github.com/ARTS-Night/LockTTYUI/issues).

A healthy logout in build 20 looks like this (abridged from the AlmaLinux 9 VM test):

```text
lockttyui-tty1-return.service RETURN_CLEAN
lockttyui-tty1-return.service DASHBOARD_START_QUEUED
lockttyui-tty1.service started

lockttyui-tty1.service: state=active result=success restart=no
getty@tty1.service: state=inactive result=success restart=no
Handoff token: absent
Fallback marker: absent
Runtime fallback override: absent
```

A queued start alone is not proof of a running dashboard. The report does not
establish TTY1 session occupancy or SELinux audit results; `UNKNOWN` indicates
an unverified check, not necessarily an error.

Additional commands:

```bash
lockttyui --version
lockttyui status
sudo lockttyui doctor
sudo lockttyui report --json
```

For older releases without this CLI, see [operations and troubleshooting](packaging/OPERATIONS.md).

## Uninstall and recover

Run from SSH or TTY2, preferably with no active TTY1 login.

```bash
# AlmaLinux
sudo dnf remove lockttyui

# Debian (instead)
sudo apt remove lockttyui
```

The package removal restores the stock TTY1 configuration. If the standard
login does not return after successful removal, run:

```bash
sudo systemctl daemon-reload &&
sudo systemctl start getty@tty1.service
```

Do not forcibly stop a logged-in getty or disable SELinux, AppArmor or PAM.
See [operations](packaging/OPERATIONS.md) for detailed recovery procedures.

## Build from source

Requires Go 1.23+ and Git. The following runs a standalone preview in your
current terminal without installing system services.

```bash
git clone --branch finish-build18 https://github.com/ARTS-Night/LockTTYUI.git
cd LockTTYUI
go test ./...
go vet ./...
go build -trimpath -o lockttyui .
./lockttyui --exhibit
```

Ctrl+] exits the preview back to the shell. This is not a session lock.

| Option | Purpose |
| --- | --- |
| `--ascii` | ASCII fallback for limited console fonts |
| `--glyph-test` | Inspect supported characters |
| `--interval 100ms` | Set the display refresh interval (default 50ms) |
| `--sample 500ms` | Set the metric sampling interval (default 250ms) |

<details>
<summary>Build the test RPM locally — AlmaLinux 9 / 10</summary>

From SSH or TTY2 on the matching AlmaLinux version, in the cloned repository:

```bash
sudo dnf install -y golang rpm-build systemd-rpm-macros binutils git
git checkout --detach 5748f93c40ef143ba3589b0fa00f30ecb1ba2ac4
TOP=$(mktemp -d)
mkdir -p "$TOP"/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}
git archive --format=tar.gz --prefix=lockttyui-0.6.0/ HEAD \
  > "$TOP/SOURCES/lockttyui-0.6.0.tar.gz"
rpmbuild -bb --define "_topdir $TOP" --define "build_release 20" \
  packaging/lockttyui.spec
ls "$TOP"/RPMS/x86_64/
```

If the build succeeds, install the matching RPM from that directory using
DNF, as above. Reboot and test on a disposable VM. This is the same source
revision as the successful build 20 package CI, not a signed release.

</details>

For DEB packaging, see [build-deb.sh](packaging/build-deb.sh) and the
[Linux packages workflow](.github/workflows/build-rpm.yml).

## Design and safety

- The dashboard runs as an unprivileged account (`nobody`, using its primary group), with no added capabilities.
- Systemd starts the original distribution `getty@tty1.service`; LockTTYUI never handles passwords or replaces PAM.
- A TTY1-specific login token coordinates the display and getty. The dashboard uses `tty-fail` instead of forcibly taking an occupied console.
- The handoff, return and fallback helpers are one-shot services, not resident daemons.
- No Docker socket access, persistent monitoring database or cron job is required.
- No deliberate change to SSH, TTY2+, the kernel console log level or distribution authentication policy.

<details>
<summary>Telemetry and resource details</summary>

The dashboard uses white and grey text/graphs with restrained green/red status
indicators. History uses one-second averages over 60 seconds; RAM/swap samples
retain the capacity percentages observed at the time of sampling. A subtle
50% CPU graph guide is a scale reference, not activity.

NETWORK displays RX/download and TX/upload. STORAGE displays read and write.
MEMORY I/O uses `/proc/vmstat` paging and swap counters, not DRAM bandwidth.
Green denotes inbound/read activity; red denotes outbound/write activity.
Idle is grey and unavailable data is `?`. A 900ms pulse hold reduces flicker.

Three single-entry graph caches, bounded history and a single in-flight
hardware worker keep resource use bounded by the displayed metrics and
terminal geometry. Rendering uses delta updates and a full repaint every
750ms to repair console interference from kernel printk. No host-wide printk
suppression is applied. Optional NVIDIA metrics use `nvidia-smi` with a timeout.
Docker counts describe visible populated systemd cgroup scopes, not an
authoritative container inventory. The optional tty9 preview can have network
rates unavailable due to PrivateNetwork. Resource limits are provisional;
CPU and memory performance targets have not been measured on all targets.

</details>

<details>
<summary>Build 20: logout regression and why the handlers must remain separate</summary>

In builds 18–19, getty's `OnSuccess` and `OnFailure` both targeted the same
return service. Systemd 252/257 can count these as two triggering candidates
and omit `MONITOR_*`. The strict return check then selects fallback even on a
clean logout (`RETURN_MONITOR_MISSING`).

[Commit 5748f93](https://github.com/ARTS-Night/LockTTYUI/commit/5748f93c40ef143ba3589b0fa00f30ecb1ba2ac4)
separates the existing success and failure handlers, preserving the strict
exit-status check. Do not combine them again. Missing or inconsistent
monitor information still selects fallback; stale service properties are not
used as a substitute for the triggering exit status.

The AlmaLinux 9 VM reproduced the problem in build 19 and recorded
`RETURN_CLEAN` and a running dashboard after logout with build 20.
See [operations](packaging/OPERATIONS.md) for the technical details.

</details>

## Project status

Development is focused on console compatibility, reliable logout recovery and
documentation rather than additional resident services. Pull requests targeting
`build` run package CI without publishing; successful pushes to `build` may
publish unsigned prerelease packages.

- [Releases](https://github.com/ARTS-Night/LockTTYUI/releases)
- [Operations and troubleshooting](packaging/OPERATIONS.md)
- [Roadmap](ROADMAP.md)
- [Report an issue](https://github.com/ARTS-Night/LockTTYUI/issues)
