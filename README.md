# LockTTYUI

Read-only Linux console telemetry dashboard. Go 1.23+, no third-party Go dependencies.

## Install from GitHub Releases (AlmaLinux 9 x86_64)

The RPM is distributed through GitHub Releases rather than a DNF repository.
The supported distribution method is a local RPM installed with DNF.
Build-15 is the default validated release; higher build numbers are test
candidates until explicitly promoted. Download the RPM and SHA256SUMS
from the matching GitHub release and verify them before installation:

```bash
sha256sum -c SHA256SUMS
sudo dnf install ./lockttyui-0.6.0-15.el9.x86_64.rpm
```

Alternatively, use the repository's readable helper to fetch the validated
default or explicitly select a test build, verify the asset digest and
install or upgrade without restarting a logged-in TTY1:

```bash
curl -fL -o lockttyui-install.sh \
  https://raw.githubusercontent.com/ARTS-Night/LockTTYUI/main/scripts/install.sh
# Inspect the script before running it.
less lockttyui-install.sh
bash lockttyui-install.sh install
```

Use `bash lockttyui-install.sh install build-16` to explicitly select a test
build (the helper will not downgrade an installed newer version), `bash lockttyui-install.sh status` to inspect its state,
or `bash lockttyui-install.sh remove` to uninstall. This script requires
`curl`, `python3`, `sha256sum`, DNF and sudo. It never executes downloaded
code as a shell script and **never restarts an active TTY1 login**.

**First activation:** Reboot at a convenient time with SSH or TTY2 available. The RPM does not stop an existing login to take over TTY1.

Ctrl+] opens the standard login. After logout, the dashboard returns.
An upgrade of an already-running dashboard takes effect the next time it
restarts (normally after a login/logout cycle).

**Security limitation:** The release RPM is currently unsigned. An SHA-256
digest fetched from the same trusted GitHub repository verifies download
integrity but does **not** constitute independent publisher authentication.
Review the release URL and script before executing. Test with an SSH/TTY2
recovery path available; production stability requires further distro/VM testing.

## Build and run

    go test ./...
    go vet ./...
    go build -o lockttyui .
    ./lockttyui --exhibit

The dashboard uses a modern monochrome palette: black console background, white headings and graphs, grey frames and secondary data, and restrained green/red status indicators. No blue, cyan, magenta or yellow theme colors. Ctrl+] exits exhibit mode. Use --ascii for font compatibility, --glyph-test to inspect supported glyphs, --interval and --sample to tune the default 50ms display / 250ms sampling cadence. History uses one-second averages over 60 seconds. CPU and memory panels balance graph heights against each other in side-by-side layouts, so memory's paging detail does not leave unused blank space beneath the CPU chart. At idle a subtle grey 50% guide shows the CPU graph scale without implying CPU activity.

CPU, memory, swap, local storage, disk I/O, network, and optional hardware statistics are displayed subject to available screen space. NETWORK shows DOWNLOAD (RX) and UPLOAD (TX); STORAGE shows READ and WRITE. The MEMORY I/O section displays /proc/vmstat pgpgin/pgpgout in bytes/sec and swap page-in/page-out in bytes/sec. These are paging/storage-backed events, **not DRAM bandwidth**. Green signals inbound/read activity; red signals outbound/write activity; idle is grey and missing data is ?. A 900ms pulse hold prevents distracting flicker. The small icons update on the existing metric timer, not an additional render loop. On Linux virtual consoles, external kernel printk messages can overwrite terminal cells even when LockTTYUI has not changed. The renderer therefore performs an in-place full repaint every 750ms to repair console interference, while retaining low-overhead delta updates between repairs. It does not change the system-wide kernel console log level; exceptionally frequent console messages can still be briefly visible. Optional NVIDIA metrics use /usr/bin/nvidia-smi with a timeout; Docker counts are visible populated systemd cgroup scopes, not an authoritative container inventory.

## Safety

Standalone exhibit mode is not a session lock. It returns to the invoking shell. The RPM provides a **TTY1-only** pre-login dashboard in its own unprivileged systemd service: Ctrl+] stops the dashboard, then systemd launches the unmodified distro agetty in its normal SELinux context. On logout, systemd starts the dashboard again. TTY2+ and SSH use unmodified standard login. No PAM changes, no Docker socket, no identity/address disclosure. The dashboard service runs as user nobody; SELinux Enforcing login/logout behavior and physical console handoff require VM validation.

See packaging/OPERATIONS.md for automatic TTY1 integration, install/uninstall rollback, exact VM test steps and manual isolated tty9 preview. The preview is not a login replacement. PrivateNetwork can make preview network rates unavailable.

## Status

Development branch: responsive dashboard and separate TTY1 display/getty services. Automated RPM tests cannot prove physical console handoff or SELinux enforcing user-shell transitions; validate in an AlmaLinux test VM before production deployment.

## Resource bounds

Three single-entry graph caches, 60 samples per series and six events; caches are replaced on history changes and terminal resize. Hardware collection has one in-flight worker. No persistent monitoring database or cron job is installed. The RPM installs a TTY1-only getty drop-in, two handoff/display systemd services and a disabled tty9 preview unit; it never restarts an active login session during package installation. The preview service's memory and task limits are provisional and need measurement on real hardware.
