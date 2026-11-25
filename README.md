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

Standalone exhibit mode is not a session lock. It returns to the invoking shell. No PAM or SSH changes, no root requirement, no Docker socket, no identity/address disclosure. Physical console glyphs and SELinux confinement still require validation.

See packaging/OPERATIONS.md for passive RPM build and isolated tty9 preview. Do not use the preview as a production login replacement; it does not automatically hand off or restore getty. PrivateNetwork can make network rates unavailable.

## Status

Development branch: responsive bar dashboard and preview packaging. Build, tests, RPM generation, physical console validation and production authentication integration must be verified before release.
