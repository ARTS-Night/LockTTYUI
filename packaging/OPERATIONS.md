# LockTTYUI operations — AlmaLinux 9, TTY1 only

## SELinux fix: v0.6.0 build 9 and later

Build 8 had a login regression: the privileged LockTTYUI bridge directly
executed /usr/sbin/agetty under the UI service's SELinux
`unconfined_service_t` domain. Authentication succeeded but SELinux denied
the following transition to a normal user shell (`unconfined_t`). Do **not**
disable SELinux or add broad allow rules to work around this.

The corrected design uses two distinct services:

1. `lockttyui-tty1.service` runs an unprivileged, read-only dashboard (nobody)
   using the open /dev/tty1; it never handles credentials.
2. On Ctrl+] (or UI failure) `lockttyui-tty1-handoff.service` records a
   temporary /run handoff token and starts `getty@tty1.service` through
   systemd. The default vendor `getty@.service` ExecStart is untouched,
   so **systemd starts the distribution agetty with its normal SELinux
   transition**. Passwords and shells use the normal PAM/SELinux path.
3. A clean logout stops the getty service (Restart=no only for tty1);
   OnSuccess starts `lockttyui-tty1.service` again.

Only `getty@tty1.service` has an instance-specific vendor drop-in.
The stock `getty@.service`, tty2+, SSH, and PAM remain unchanged.
An ephemeral token at `/run/lockttyui-tty1-login.ready` gates getty
startup so the two services do not claim the console simultaneously.
The vendor symlink in `getty.target.wants` starts the dashboard on boot.
Systemd is reloaded during install but an active user session is never
restarted by the RPM.

## Installation / upgrade (test VM)

Keep an SSH session or TTY2 available. Use the RPM downloaded from the
latest passing GitHub Releases prerelease; check SHA256SUMS before installing.
Update from build 8 (or install fresh) with:

    sudo dnf upgrade ./lockttyui-0.6.0-N.el9.x86_64.rpm

Use `dnf install ./...` if it is not already installed. On build8-to-new
upgrades, the old bridge may still be serving tty1 until the service is
switched. **Only from tty2 or SSH with no logged-in tty1 session**, do:

    sudo systemctl stop getty@tty1.service
    sudo systemctl start lockttyui-tty1.service

The dashboard should appear on tty1. Press Ctrl+] and verify a normal login
with SELinux **Enforcing**. Run `id -Z` in the resulting shell. Its user
domain should normally be `unconfined_t`, not `unconfined_service_t`.
After `exit` / logout the dashboard should return.

Do not run `systemctl restart getty@tty1.service` to force the dashboard:
the new unit deliberately requires a handoff token and preserves stock getty
as its sole ExecStart.

## Console printk messages overlapping the dashboard

Linux kernel printk can write directly to the foreground virtual console. For
example, Hyper-V's `hv_balloon` may report a memory-balloon floor warning.
This is **not** a LockTTYUI event and does not mean the console is compromised.

LockTTYUI repaints the complete frame in-place at most every 750ms. This
repairs persistent artifacts left behind by asynchronous kernel prints and
keeps low-overhead delta updates between repaints. A message may be briefly
visible until the next repair. Kernel journal collection remains unchanged.

There is **no automatic global printk suppression** in the RPM or TTY1 unit:
it could hide useful diagnostics from other virtual consoles. For a host-wide
manual opt-in that hides warning-level printk from the physical console but
still records messages in the kernel journal, save the original level and use:

    cat /proc/sys/kernel/printk
    sudo dmesg -n 4

This is not TTY1-specific: it changes the console printing level globally.
To restore, run `sudo dmesg -n ORIGINAL_LEVEL` with the original first number.
Avoid lowering the console log level on production systems where visible kernel
warnings are an operational requirement.

## Diagnosis (TTY2 or SSH)

    rpm -q lockttyui
    getenforce
    systemctl cat getty@tty1.service
    systemctl status lockttyui-tty1.service lockttyui-tty1-handoff.service getty@tty1.service --no-pager
    sudo journalctl -b -u lockttyui-tty1.service -u lockttyui-tty1-handoff.service -u getty@tty1.service -n 120 --no-pager
    sudo ausearch -m AVC,USER_AVC -ts recent

Do not paste passwords, tokens, private system logs, or personal information
into public issues. Go tests and RPM container tests **cannot** verify live
SELinux domain transitions or physical console ownership; those need an
AlmaLinux 9 VM with enforcing SELinux.

## Rollback / removal

With no tty1 user session, use SSH or tty2:

    sudo dnf remove lockttyui
    sudo systemctl daemon-reload
    sudo systemctl start getty@tty1.service

The final removal script stops the pre-login dashboard, removes the handoff
token, restores the distributor's instance configuration and starts the
stock getty if idle. A logged-in tty1 session is not restarted.
This does not remove unrelated /etc/systemd drop-ins.

## Manual tty9 preview

`lockttyui-preview.service` remains an opt-in preview (never auto-enabled).
Only use it on a verified vacant tty9:

    sudo systemctl stop getty@tty9.service
    sudo systemctl start lockttyui-preview.service

To revert:

    sudo systemctl stop lockttyui-preview.service
    sudo systemctl start getty@tty9.service

## Local RPM and release workflow

Go 1.23+, rpm-build, systemd-rpm-macros are required on the build host.
GitHub Actions on the `build` branch builds in AlmaLinux 9 and tests Go,
RPM install/uninstall, static ELF dependencies and SHA256 checks, then
publishes an **unsigned** prerelease only after all checks succeed.
A checksum detects accidental alteration but does not authenticate the
unsigned publisher. There is no hosted DNF repository yet.
