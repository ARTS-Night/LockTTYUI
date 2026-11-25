Name:           lockttyui
Version:        0.5.0
Release:        1%{?dist}
Summary:        Read-only Linux console telemetry dashboard
License:        Proprietary
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.23

%description
Standalone console telemetry dashboard. Installation does not change authentication
or enable any service.

%prep
%setup -q

%build
CGO_ENABLED=0 go build -trimpath -o lockttyui .

%check
go test ./...
go vet ./...

%install
install -D -m 0755 lockttyui %{buildroot}%{_bindir}/lockttyui
install -D -m 0644 packaging/lockttyui-preview@.service %{buildroot}%{_unitdir}/lockttyui-preview@.service

%files
%{_bindir}/lockttyui
%{_unitdir}/lockttyui-preview@.service
%doc README.md ROADMAP.md packaging/OPERATIONS.md
