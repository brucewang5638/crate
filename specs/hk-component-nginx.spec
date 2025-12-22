# 禁用自动依赖扫描（防止跨架构安装失败的关键！）
%global _use_internal_dependency_generator 0
%global __find_requires %{nil}
%global __find_provides %{nil}
%global debug_package %{nil}
%define __strip /bin/true
%define __jar_repack %{nil}

Name:           hk-component-nginx
Version:        %{version}
Release:        1%{?dist}
Summary:        HK 组件 - Nginx

License:        Proprietary
URL:            http://hongkexinxi.com/
Group:          Applications/System

# 假设你的源码 tar 包命名规则为 component_name.tar.gz
Source0:        %{comp_name}.tar.gz
BuildRoot:      %_tmppath/%name-%version-root

%description
这是自监管平台一体化安装包中的 %{comp_name} 组件。
由通用模板构建，旨在降低多组件适配成本。

%prep
# -q 代表解压前不要给我新建目录，-n 指定解压后的目录名
%setup -q -n %{comp_name}

%install
# 清理旧的构建目录
rm -rf %{buildroot}

# 1. 定义目标安装路径
%define _instdir /opt/hk/component/%{comp_name}
install -d -m 755 %{buildroot}%{_instdir}

# 2. 将解压后的所有内容拷贝到安装目录
cp -a * %{buildroot}%{_instdir}/

%post
# 安装后的初始化（意图：设置权限）
chown -R root:root /opt/hk/component/%{comp_name}
chmod -R 755 /opt/hk/component/%{comp_name}

echo "✅ 组件 %{comp_name} 安装成功。"
echo "📍 安装路径: /opt/hk/component/%{comp_name}"

%postun
# 只有在完全卸载时才删除目录 ($1 == 0)
if [ $1 -eq 0 ]; then
    rm -rf /opt/hk/component/%{comp_name}
    echo "🗑️ 组件 %{comp_name} 已完全移除。"
fi

%files
%defattr(-,root,root,-)
# 核心：将该路径下的所有文件归入 RPM 管理
/opt/hk/component/%{comp_name}

%clean
rm -rf %{buildroot}
