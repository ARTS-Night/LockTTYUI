# LockTTYUI operations — AlmaLinux 9 (TTY1 only)

## Automatic integration on RPM installation

The RPM ships a vendor-level, **instance-specific** drop-in at
`/usr/lib/systemd/system/getty@tty1.service.d/50-lockttyui.conf`.
There is **no** modification of `getty@.service`, `getty@tty2.service` or higher terminals,
PAM, SSH, login policy, or `/etc/systemd`.

- Install with `sudo dnf install ./lockttyui-*.rpm` (review and verify the RPM first).
- RPM reloads systemd's unit definitions, **but never restarts a running getty or user session**.
- On the next normal **getty@tty1.service** start (typically the next boot), the tty1
  bridge starts an unprivileged telemetry child and displays LockTTYUI.
- Press **Ctrl+]** on tty1 to leave the dashboard and run the distro's normal `agetty`
  login. Enter a username/password using the normal login and PAM mechanisms.
- After the user logs out, the existing systemd getty restart policy runs the
  dashboard again. TTY2+ remain the distro's unmodified login consoles; SSH remains
  unchanged. Use Ctrl+Alt+F2 to reach tty2 at the physical console.

The tty1 bridge itself runs under the privileged getty service solely to hand off
to `/usr/sbin/agetty`. The dashboard subprocess runs as UID/GID 65534 with no
supplementary groups, using inherited tty file descriptors. This is not auto-login:
the dashboard never requests or handles a password. If the display exits with an
error, fails to launch, or is killed, the bridge falls back to ordinary agetty.
A stop signal from systemd shuts the child down without unexpectedly launching login.

The override is suitable for the **text-only virtual-console** environment used
for this project. Graphical login managers, custom tty1 getty overrides, SELinux
policy, virtual terminal behavior, and local fonts must be reviewed before use
outside a disposable AlmaLinux test VM. The package does not forcibly switch
`graphical.target` to `multi-user.target` or seize an active console.

## Verify in a disposable VM before deploying

Keep SSH and a separate working TTY2 login available. Do not try the following
on an actively used console. Before reboot, inspect without changing the session:

    systemctl cat getty@tty1.service
    systemctl cat getty@tty2.service
    systemd-analyze verify getty@tty1.service
    systemctl get-default

The tty1 override must appear only in the first output; the second must still
use the stock getty template. On a test VM, after logging out of tty1, restart
the service **from SSH or tty2** to test immediately:

    sudo systemctl restart getty@tty1.service

Validation checklist:
1. tty1 displays LockTTYUI; tty2 can log in without it.
2. Ctrl+] shows the normal agetty username/password prompt; no automatic login.
3. After a normal user login and logout on tty1, dashboard returns.
4. Killing only the unprivileged display child lets the bridge show agetty.
5. `systemctl stop getty@tty1.service` does not start a second agetty.
6. A system reboot returns tty1 dashboard and preserves tty2/SSH access.
7. SELinux enforcing, CPU/GPU visibility, resize, keyboard flush and failure
   conditions are checked on the actual console.

None of these physical-console conditions is proven by a Docker CI build alone.

## Revert / uninstall

The RPM owns its drop-in under `/usr/lib`, not a file under `/etc`. On final
removal, that file is removed and systemd is reloaded; the distributor's normal
tty1 getty definition is restored for its **next** start. No other gettys change.

    sudo dnf remove lockttyui
    systemctl cat getty@tty1.service

If immediate restoration is needed, from **SSH or tty2** (never during an active
tty1 login), restart the existing getty:

    sudo systemctl restart getty@tty1.service

After removal, test standard login on tty1 and tty2. If a machine already has
custom tty1 settings, the package cannot safely override or delete them; review
the unit effective configuration first.

## Manual tty9 preview (developer-only)

The separate `lockttyui-preview.service` remains disabled and is never enabled
or started by the RPM. It is for manual experiments on a known-free tty9:

    sudo systemctl stop getty@tty9.service
    sudo systemctl start lockttyui-preview.service

Restore after stopping the preview:

    sudo systemctl stop lockttyui-preview.service
    sudo systemctl start getty@tty9.service

The preview is intentionally not a login manager. Under `PrivateNetwork`,
network telemetry may be unavailable.

## Local RPM build

Go 1.23+, `rpm-build`, and `systemd-rpm-macros` are required only on the build
host. From the repository root:

    mkdir -p ~/rpmbuild/{SOURCES,SPECS}
    git archive --format=tar.gz --prefix=lockttyui-0.6.0/ HEAD > ~/rpmbuild/SOURCES/lockttyui-0.6.0.tar.gz
    cp packaging/lockttyui.spec ~/rpmbuild/SPECS/
    rpmbuild -bb ~/rpmbuild/SPECS/lockttyui.spec

Review RPM contents and dependencies before installing. The binary is built
with CGO disabled; no third-party Go modules are required. Docker,
NVIDIA utilities, and individual monitored system services are optional.

## GitHub Actions releases

Only a push to the `build` branch triggers the RPM workflow. It builds inside
AlmaLinux 9, runs Go tests/vet, tests RPM installation and removal, checks
static ELF dependencies, and publishes unsigned prereleases with SHA256SUMS
only after those checks pass.

Download both from the same release and run:

    sha256sum -c SHA256SUMS
    sudo dnf install ./lockttyui-*.rpm

A matching checksum does not authenticate an unsigned publisher. Inspect the
source and provenance; if a verified local RPM requires a one-off exception,
use `sudo dnf --nogpgcheck install ./lockttyui-*.rpm` only after review. Do not
change global signature policy. There is not yet a hosted DNF repository.
