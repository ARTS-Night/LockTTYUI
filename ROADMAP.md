# LockTTYUI development plan

## v0.2 — prototype
- [x] CPU/memory history, Unicode step graphs and ASCII fallback.
- [x] Hyper-V AlmaLinux 9.9: DynamicUser TTY9 display with SELinux enforcing.
- [x] Observed TUI -> stock getty -> TUI service cycle.
- [x] User reports no authenticated shell exposed after forced termination.

The temporary systemd experiment is not production-ready. The successful OnSuccess/OnFailure configuration still needs deterministic stop/disable behavior, input flushing, authentication testing, crash recovery and SELinux confinement review (observed unconfined_service_t). Never modify PAM or SSH or disable SELinux. Do not add persistent users. No claim of locking an existing authenticated session.

## v0.3 — visual refresh
- [x] Outline clock, cyan CPU and blue memory graphs.
- [x] Compact clock and ASCII fallback.
- [x] User reports the outline design works on the development VM.
- [ ] Confirm automated build/test results.

## v0.4 — metrics
- [x] Initial aggregate RX/TX rate display and counter delta tests; no addresses, endpoints, ports or packet contents.
- [ ] Network history and host statistics under PrivateNetwork; current collector observes its own namespace only.
- [x] Initial root and selected local filesystem capacity, anonymized labels and duplicate filtering.
- [ ] Validate capacity display, mount namespace visibility, and narrow consoles.
- Optional GPU detection.
- Preserve network isolation: do not assume /proc/net/dev exposes host counters under PrivateNetwork.

## v0.5 — integration
- Harden non-root systemd display, preserve stock getty/PAM, explicit activation.
- Verify normal login/logout, terminal handoff, residual input, repeated cycles, crash/stop/rollback.
- Resolve duplicate trigger warning and ensure disabling does not relaunch services.

## v0.6 — distribution
- RPM, DNF installation/removal, CI and release artifacts.
- Installation must not enable a login modification by default.

## Later
Declarative templates and layout configuration; optional offline browser-based editor.
Docker metrics only via separately reviewed least-privilege collection; no direct Docker socket access by the untrusted display.
