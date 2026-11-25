# Operations — experimental preview, not production login integration

RPM install is passive. No PAM, SSH, getty drop-ins, SELinux settings or persistent users are modified. The preview unit has no Install section and no automatic getty restart trigger.

Only test on a disposable AlmaLinux VM with SSH and a separate console already available. Never target tty1, a logged-in console or a remote serial console. The examples use tty9 exclusively; first verify it is unused with loginctl and systemctl. Starting the preview conflicts with getty on that TTY, so it can stop that getty.

After installing the locally built RPM:

    sudo systemctl start lockttyui-preview@tty9.service

Exit with Ctrl+] or stop from SSH. Restore explicitly:

    sudo systemctl stop lockttyui-preview@tty9.service
    sudo systemctl start getty@tty9.service

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
A local RPM is not a hosted DNF repository. Repository signing, publishing and upgrade tests remain release gates.
