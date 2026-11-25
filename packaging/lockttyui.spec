Name:           lockttyui
Version:        0.6.0
Release:        %{?build_release}%{!?build_release:1}%{?dist}
Summary:        Read-only Linux console telemetry dashboard with tty1 login handoff
License:        Proprietary
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.23
BuildRequires:  systemd-rpm-macros
# tty1 is the only modified getty instance; agetty remains the login handler.
Requires:      systemd
Requires:      /usr/sbin/agetty

%global debug_package %{nil}

%description
Standalone console telemetry dashboard. The RPM adds a vendor drop-in only for
getty@tty1.service; the next start shows the dashboard, then hands off to the
normal agetty login. tty2+, SSH and PAM are not modified.

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
%dir %{_unitdir}/getty@tty1.service.d
%{_unitdir}/getty@tty1.service.d/50-lockttyui.conf
install -D -m 0644 packaging/50-lockttyui-tty1.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/50-lockttyui.conf

%post
%systemd_post lockttyui-preview.service
# Reload the vendor drop-in without restarting an active login or replacing tty2+.
if [ -d /run/systemd/system ]; then /usr/bin/systemctl daemon-reload >/dev/null 2>&1 || :; fi

%preun
%systemd_preun lockttyui-preview.service

%postun
%systemd_postun lockttyui-preview.service
# After removal, the distro getty template is again used for tty1 on next start.
if [ -d /run/systemd/system ]; then /usr/bin/systemctl daemon-reload >/dev/null 2>&1 || :; fi

%files
%{_bindir}/lockttyui
%{_unitdir}/lockttyui-preview.service
%doc README.md ROADMAP.md packaging/OPERATIONS.md
