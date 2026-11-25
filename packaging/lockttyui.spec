Name:           lockttyui
Version:        0.6.0
Release:        %{?build_release}%{!?build_release:1}%{?dist}
Summary:        Read-only Linux console telemetry dashboard
License:        Proprietary
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.23
BuildRequires:  systemd-rpm-macros
# Optional integration uses systemctl; no Go runtime or GPU/Docker dependency.
Recommends:     systemd

%global debug_package %{nil}

%description
Standalone console telemetry dashboard. Installation does not change authentication
or enable any service.

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
install -D -m 0644 packaging/lockttyui-preview@.service %{buildroot}%{_unitdir}/lockttyui-preview@.service

%files
%{_bindir}/lockttyui
%{_unitdir}/lockttyui-preview@.service
%doc README.md ROADMAP.md packaging/OPERATIONS.md
