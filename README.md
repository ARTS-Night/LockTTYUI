# LockTTYUI

Read-only Linux console telemetry dashboard. Go 1.23+, no third-party Go dependencies.

## Build and run

    go test ./...
    go vet ./...
    go build -o lockttyui .
    ./lockttyui --exhibit

Bar graphs are the only graph style. Ctrl+] exits exhibit mode. Use --ascii for font compatibility, --glyph-test to inspect supported glyphs, --interval and --sample to tune the default 50ms display / 250ms sampling cadence. History uses one-second averages over 60 seconds.

CPU, memory, swap, local storage, disk I/O, network, and optional hardware statistics are displayed subject to available screen space. Optional NVIDIA metrics use /usr/bin/nvidia-smi with a timeout; Docker counts are visible populated systemd cgroup scopes, not an authoritative container inventory.

## Safety

Standalone exhibit mode is not a session lock. It returns to the invoking shell. The RPM provides a **TTY1-only** pre-login dashboard: Ctrl+] hands off to the normal agetty login, and the dashboard returns after logout. TTY2+ and SSH use unmodified standard login. No PAM changes, no Docker socket, no identity/address disclosure. The dashboard child uses an unprivileged UID; physical console behavior and SELinux confinement require VM validation.

See packaging/OPERATIONS.md for automatic TTY1 integration, install/uninstall rollback, exact VM test steps and manual isolated tty9 preview. The preview is not a login replacement. PrivateNetwork can make preview network rates unavailable.

## Status

Development branch: responsive bar dashboard and TTY1 getty handoff integration. CI build, RPM install/removal, physical console behavior and SELinux protection must be verified before production deployment.

## Resource bounds

Three single-entry graph caches, 60 samples per series and six events; caches are replaced on history changes and terminal resize. Hardware collection has one in-flight worker. No persistent monitoring database or cron job is installed. The RPM installs a TTY1-only getty drop-in and a disabled tty9 preview unit; it never restarts an active session during package installation. The preview service's memory and task limits are provisional and need measurement on real hardware.
