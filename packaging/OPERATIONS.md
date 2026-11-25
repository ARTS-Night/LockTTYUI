# Operations — experimental preview, not production login integration

RPM install is passive. No PAM, SSH, getty drop-ins, SELinux settings or persistent users are modified. The preview unit has no Install section and no automatic getty restart trigger.

Only test on a disposable AlmaLinux VM with SSH and a separate console already available. Never target tty1, a logged-in console or a remote serial console. The examples use tty9 exclusively; first verify it is unused with loginctl and systemctl. The fixed preview unit does not stop getty automatically. After verifying tty9 is unused, stop its getty explicitly from SSH.

After installing the locally built RPM:

    sudo systemctl stop getty@tty9.service
    sudo systemctl start lockttyui-preview.service

Exit with Ctrl+] or stop from SSH. Restore explicitly:

    sudo systemctl stop lockttyui-preview.service
    sudo systemctl start getty@tty9.service

The package registers only the fixed tty9 preview unit. RPM scriptlets reload systemd metadata and stop the preview on final removal; no automatic enable or restart is requested. Before removal, stop the preview and restore getty manually. An older experimental template must be stopped manually before upgrading.

No automatic pre-login cycle is installed. Do not enable this preview as a login replacement. Under PrivateNetwork network rates can be N/A; do not remove isolation merely to obtain network statistics.

## Production gates
- Confirm correct SELinux context and review dedicated policy if required.
- Confirm TTY handoff, input flushing and no residual credentials.
- Verify normal login/logout with unmodified getty/PAM, crash, SIGKILL, boot, manual stop and rollback.
- Verify no privilege escalation or private information on screen.
- Test physical font support, CPU overhead and disk / GPU visibility.
- Only after these tests design a deterministic automatic supervisor; no success/failure trigger loops.

## RPM local build
Requires Go 1.23+, rpm-build, systemd RPM macros and a Linux build host. From repository root:

    mkdir -p ~/rpmbuild/{SOURCES,SPECS}
    git archive --format=tar.gz --prefix=lockttyui-0.6.0/ HEAD > ~/rpmbuild/SOURCES/lockttyui-0.6.0.tar.gz
    cp packaging/lockttyui.spec ~/rpmbuild/SPECS/
    rpmbuild -ba ~/rpmbuild/SPECS/lockttyui.spec

Inspect RPM contents before installing with dnf install /absolute/path/to/package.rpm. Stop preview and restore getty before dnf remove lockttyui. License field is provisional for private development; review before publishing.

## Standalone installation
Install the RPM matching the machine architecture with `sudo dnf install ./lockttyui-0.6.0-1.el9.x86_64.rpm` (use the actual filename).
Run `lockttyui --exhibit` as an ordinary user. Go is needed only on the build host.
Docker, NVIDIA utilities and individual monitored services are optional. Systemd is recommended for service status; a failed query is not a healthy result.
The build disables CGO and external Go module resolution. Verify the resulting ELF has no interpreter or NEEDED entries before release.
A local RPM is not a hosted DNF repository. Public DNF repository hosting and upgrade tests remain release gates.

## GitHub build releases
Only pushes to the build branch run CI. After successful tests and static ELF checks, CI creates an unsigned prerelease with the RPM and SHA256SUMS. Download both from the same release, then run:

    sha256sum -c SHA256SUMS
    sudo dnf install ./lockttyui-*.rpm

If local RPM signature policy rejects this unsigned package, review the source and release provenance first; only then optionally use `sudo dnf --nogpgcheck install ./lockttyui-*.rpm` for this transaction. Never change global signature policy. A checksum downloaded with the RPM detects corruption but does not authenticate the publisher. Private repository access remains required while the repository is private.

## Automatic RPM setup and login safety
The RPM automatically installs the binary and the fixed tty9 preview unit, and registers it with systemd through RPM macros. It does not replace getty, PAM, SSH, or an active login terminal. Automatic login-screen takeover is intentionally gated until physical-console, crash, input-flush, SELinux, and uninstall rollback tests pass. Installing a package must not silently strand the machine without a working login.
The `build` branch is the only CI trigger. Each successful build creates an unsigned prerelease; the workflow does not publish a DNF repository or install packages on the target machine.
