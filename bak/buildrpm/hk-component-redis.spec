# ================= 核心防报错宏 =================
%global _use_internal_dependency_generator 0
%global __find_requires %{nil}
%global __find_provides %{nil}
%global debug_package %{nil}
%define __os_install_post %{nil}
%define __strip /bin/true
# ===============================================

Name:           hk-component-redis
Version:        %{version}
Release:        1%{?dist}
Summary:        HK 组件 - Redis (专用版)
License:        Proprietary
Group:          Applications/System
Source0:        %{comp_name}.tar.gz
BuildRoot:      %_tmppath/%name-%version-root

%description
Redis 组件，包含 /data/redis 目录初始化。

%prep
%setup -q -n %{comp_name}

%install
rm -rf %{buildroot}
%define _instdir /opt/hk/component/%{comp_name}
install -d -m 755 %{buildroot}%{_instdir}
cp -a * %{buildroot}%{_instdir}/

%post
# 1. 基础权限
chown -R root:root /opt/hk/component/%{comp_name}
chmod -R 755 /opt/hk/component/%{comp_name}

# 2. 🚀 Redis 专属逻辑：创建数据目录
if [ ! -d "/data/redis" ]; then
    echo "📂 [Redis] 创建数据目录 /data/redis ..."
    mkdir -p /data/redis
    # 根据你的运行用户修改这里的权限，假设是 root 或 redis
    chmod 755 /data/redis
fi

echo "✅ Redis 安装配置完成。"

%files
%defattr(-,root,root,-)
/opt/hk/component/%{comp_name}

%clean
rm -rf %{buildroot}
