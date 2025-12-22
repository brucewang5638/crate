# =========================================================
# Crate Example SPEC File
# =========================================================
# 最佳实践:
# 1. Name (包名) 应该是固定的，方便阅读和维护。
# 2. Source0 (源码) 应该是动态的 (%{comp_name}.tar.gz)，以便匹配 Builder 生成的 tar包。
# =========================================================

# 禁用构建时的自动依赖扫描 (防止很多不必要的 Requires 报错)
%global _use_internal_dependency_generator 0
%global __find_requires %{nil}
%global __find_provides %{nil}
%global debug_package %{nil}
%define __strip /bin/true

# [Static Name] 明确指定生成的 RPM 包名
Name:           example-component
Version:        %{version}
Release:        1%{?dist}
Summary:        这是一个 Crate 构建工具的示例构建块

License:        Proprietary
URL:            http://example.com/
Group:          Applications/System

# [Dynamic Source] 对应 Crate 根据 component.name 生成的 tar.gz
Source0:        %{comp_name}.tar.gz

BuildRoot:      %_tmppath/%name-%version-root

%description
这是一个示例 SPEC 文件，展示了如何配合 Crate 工具使用。
它会被安装到 /opt/example/ 目录下。

%prep
# 解压源码包
# -q: 安静模式
# -n: 指定解压后的目录名 (通常 Crate 打包时顶层目录就是 comp_name)
%setup -q -n %{comp_name}

%install
# 清理构建根目录
rm -rf %{buildroot}

# 定义安装路径
%define _instdir /opt/example/%{comp_name}

# 1. 创建目录
install -d -m 755 %{buildroot}%{_instdir}

# 2. 拷贝文件
# 将解压出来的所有文件拷贝到安装路径
cp -a * %{buildroot}%{_instdir}/

%post
# 安装后脚本 (可选)
echo "✅ Example component installed to %{_instdir}"
# 设置权限
chown -R root:root %{_instdir}

%files
%defattr(-,root,root,-)
# 包含的文件/目录
/opt/example/%{comp_name}

%clean
rm -rf %{buildroot}
