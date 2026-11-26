Name:           lockttyui
Version:        0.6.0
Release:        %{?build_release}%{!?build_release:1}%{?dist}
Summary:        Console telemetry splash with isolated tty1 login handoff
License:        Proprietary
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.23
BuildRequires:  systemd-rpm-macros
# tty1 is the only modified getty instance; agetty remains the login handler.
Requires:      systemd
Requires:      /usr/sbin/agetty

%global debug_package %{nil}

%description
Console telemetry display with a dedicated tty1 service. Systemd launches the
unmodified distro agetty as a separate getty@tty1 service when the display ends;
tty2+, SSH and PAM are unchanged.

%prep
%setup -q

%build
export CGO_ENABLED=0 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
go build -mod=readonly -trimpath -ldflags="-s -w" -o lockttyui .

%check
export CGO_ENABLED=0 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
go test -mod=readonly ./...
go vet -mod=readonly ./...

%install
install -D -m 0755 lockttyui %{buildroot}%{_bindir}/lockttyui
install -D -m 0644 packaging/lockttyui-preview.service %{buildroot}%{_unitdir}/lockttyui-preview.service
install -D -m 0644 packaging/50-lockttyui-tty1.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/50-lockttyui.conf
install -D -m 0644 packaging/lockttyui-tty1.service %{buildroot}%{_unitdir}/lockttyui-tty1.service
install -D -m 0644 packaging/lockttyui-tty1-handoff.service %{buildroot}%{_unitdir}/lockttyui-tty1-handoff.service
install -D -m 0644 packaging/lockttyui-tty1-fallback.service %{buildroot}%{_unitdir}/lockttyui-tty1-fallback.service
install -D -m 0644 packaging/lockttyui-tty1-return.service %{buildroot}%{_unitdir}/lockttyui-tty1-return.service
install -D -m 0755 packaging/lockttyui-tty1-route %{buildroot}%{_libexecdir}/lockttyui-tty1-route
# Vendor dependency: do not create persistent enable links in /etc/systemd.
install -d %{buildroot}%{_unitdir}/getty.target.wants
ln -s ../lockttyui-tty1.service %{buildroot}%{_unitdir}/getty.target.wants/lockttyui-tty1.service

%post
%systemd_post lockttyui-preview.service
# Adopt a pre-existing stock getty (including builds 9-15, which consumed
# their token at startup). Never stop/restart a live login during installation.
if [ -d /run/systemd/system ]; then
 seeded=0
 state=$(/usr/bin/systemctl show -p ActiveState --value getty@tty1.service 2>/dev/null || :)
 case "$state" in
  active|activating|reloading)
   if [ ! -e /run/lockttyui-tty1-login.ready ]; then
    /usr/bin/touch /run/lockttyui-tty1-login.ready && seeded=1
   fi
   ;;
 esac
 /usr/bin/systemctl daemon-reload >/dev/null 2>&1 || :
 # If the pre-existing getty exited during the reload, reconcile the lease.
 if [ "$seeded" -eq 1 ] && [ "$(/usr/bin/systemctl show -p ActiveState --value getty@tty1.service 2>/dev/null || :)" = inactive ]; then
  # Exit status is uncertain during adoption; conservatively retain login.
  /usr/bin/systemctl --no-block start lockttyui-tty1-fallback.service >/dev/null 2>&1 || :
 fi
fi

%preun
%systemd_preun lockttyui-preview.service
# Final removal only: stop a pre-login dashboard but never interrupt agetty or a logged-in user.
if [ "$1" -eq 0 ] && [ -d /run/systemd/system ]; then
 # Prevent any queued OnSuccess/OnFailure router from starting a replacement
 # while the package is being erased. Keep this marker until daemon-reload.
 /usr/bin/touch /run/lockttyui-tty1-removing || exit 1
 if ! /usr/bin/systemctl stop lockttyui-tty1-handoff.service lockttyui-tty1-return.service lockttyui-tty1-fallback.service lockttyui-tty1.service; then
  /usr/bin/rm -f /run/lockttyui-tty1-removing
  /usr/bin/systemctl --no-block start lockttyui-tty1-fallback.service >/dev/null 2>&1 || :
  echo "Cannot stop LockTTYUI safely; refusing package removal." >&2
  exit 1
 fi
fi

%postun
%systemd_postun lockttyui-preview.service
 # On final removal remove only our own runtime fallback drop-in before reload.
# Active tty1 login remains untouched.
if [ -d /run/systemd/system ]; then
 if [ "$1" -eq 0 ]; then
  /usr/bin/rm -f /run/systemd/system/getty@tty1.service.d/90-lockttyui-fallback.conf
 fi
 if /usr/bin/systemctl daemon-reload; then
  if [ "$1" -eq 0 ]; then
   /usr/bin/rm -f /run/lockttyui-tty1-login.ready /run/lockttyui-tty1-fallback /run/lockttyui-tty1-removing
   /usr/bin/systemctl reset-failed getty@tty1.service >/dev/null 2>&1 || :
   /usr/bin/systemctl --no-block start getty@tty1.service || echo "Start stock getty manually from SSH or tty2." >&2
  fi
 else
  echo "systemd reload failed; run systemctl daemon-reload and restore tty1 from SSH or tty2." >&2
 fi
fi

%files
%{_bindir}/lockttyui
%{_unitdir}/lockttyui-preview.service
%{_unitdir}/lockttyui-tty1.service
%{_unitdir}/lockttyui-tty1-handoff.service
%{_unitdir}/lockttyui-tty1-return.service
%{_unitdir}/lockttyui-tty1-fallback.service
%{_libexecdir}/lockttyui-tty1-route
%dir %{_unitdir}/getty.target.wants
%{_unitdir}/getty.target.wants/lockttyui-tty1.service
%dir %{_unitdir}/getty@tty1.service.d
%{_unitdir}/getty@tty1.service.d/50-lockttyui.conf
%doc README.md ROADMAP.md packaging/OPERATIONS.md
