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
# Vendor dependency: do not create persistent enable links in /etc/systemd.
install -d %{buildroot}%{_unitdir}/getty.target.wants
ln -s ../lockttyui-tty1.service %{buildroot}%{_unitdir}/getty.target.wants/lockttyui-tty1.service

%post
%systemd_post lockttyui-preview.service
# Reload the vendor drop-in without restarting an active login or replacing tty2+.
if [ -d /run/systemd/system ]; then /usr/bin/systemctl daemon-reload >/dev/null 2>&1 || :; fi

%preun
%systemd_preun lockttyui-preview.service
# Final removal only: stop a pre-login dashboard but never interrupt agetty or a logged-in user.
if [ "$1" -eq 0 ] && [ -d /run/systemd/system ]; then
 /usr/bin/systemctl stop lockttyui-tty1.service >/dev/null 2>&1 || :
fi

%postun
%systemd_postun lockttyui-preview.service
# On final removal the original getty starts if idle; active tty1 login remains untouched.
if [ -d /run/systemd/system ]; then
 /usr/bin/systemctl daemon-reload >/dev/null 2>&1 || :
 if [ "$1" -eq 0 ]; then
  /usr/bin/rm -f /run/lockttyui-tty1-login.ready
  /usr/bin/systemctl --no-block start getty@tty1.service >/dev/null 2>&1 || :
 fi
fi

%files
%{_bindir}/lockttyui
%{_unitdir}/lockttyui-preview.service
%{_unitdir}/lockttyui-tty1.service
%{_unitdir}/lockttyui-tty1-handoff.service
%dir %{_unitdir}/getty.target.wants
%{_unitdir}/getty.target.wants/lockttyui-tty1.service
%dir %{_unitdir}/getty@tty1.service.d
%{_unitdir}/getty@tty1.service.d/50-lockttyui.conf
%doc README.md ROADMAP.md packaging/OPERATIONS.md
