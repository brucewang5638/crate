%global debug_package %{nil}

Name:           procmate
Version:        %{version}
Release:        1%{?dist}
Summary:        HK 进程管理监控工具

License:        Proprietary
Source0:        %{comp_name}.tar.gz

Requires:       systemd

%description
Procmate 是轻量级的进程管理工具，用于业务组件的监控与自愈。

%prep
%setup -q -c

%install
rm -rf %{buildroot}
# 强制创建目标文件系统结构
install -d -m 755 %{buildroot}/opt/procmate
install -d -m 755 %{buildroot}/usr/local/bin
install -d -m 755 %{buildroot}/etc/procmate/conf.d
install -d -m 755 %{buildroot}/etc/systemd/system

# 安装文件与软链接
install -p -m 755 procmate %{buildroot}/opt/procmate/procmate
ln -sf /opt/procmate/procmate %{buildroot}/usr/local/bin/procmate
install -p -m 644 config.yaml %{buildroot}/etc/procmate/config.yaml

%post
if [ $1 -eq 1 ]; then
    # 动态注入 Service 脚本
    cat <<EOF > /etc/systemd/system/procmate.service
[Unit]
Description=Procmate Process Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/procmate watch
Restart=on-failure
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable procmate >/dev/null 2>&1 || :
    echo "✅ Procmate 服务已注册并启用开机自启。"
fi

%preun
if [ $1 -eq 0 ]; then
    systemctl stop procmate.service >/dev/null 2>&1 || :
    systemctl disable procmate.service >/dev/null 2>&1 || :
fi

%files
%defattr(-,root,root,-)
/opt/procmate/procmate
/usr/local/bin/procmate
%dir /etc/procmate
%dir /etc/procmate/conf.d
%config(noreplace) /etc/procmate/config.yaml

%clean
rm -rf %{buildroot}
