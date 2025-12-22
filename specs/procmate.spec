Name:           {{.Name}}
Version:        {{.Version}}
Release:        1%{?dist}
Summary:        Placeholder SPEC for {{.Name}}

License:        Proprietary
Source0:        %{name}-%{version}.tar.gz
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)

%description
Placeholder description.

%prep
%setup -q

%build

%install
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/opt/hk/{{.Name}}

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(-,root,root,-)
/opt/hk/{{.Name}}

%changelog
