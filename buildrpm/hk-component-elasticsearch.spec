# ================= 核心防报错宏 =================
# 1. 彻底禁用构建后处理 (防止 cpio 报错、防止 strip、防止重打包)
%global _use_internal_dependency_generator 0
%global __find_requires %{nil}
%global __find_provides %{nil}
%global debug_package %{nil}
# 2. 禁止自动依赖扫描 (防止 osgi 刷屏)
%define __os_install_post %{nil}
%define __strip /bin/true
%define __jar_repack %{nil}
# ===============================================

Name:           hk-component-elasticsearch
Version:        %{version}
Release:        1%{?dist}
Summary:        HK 组件 - Elasticsearch (专用调优版)

License:        Proprietary
URL:            http://hongkexinxi.com/
Group:          Applications/System

Requires:       java-1.8.0-openjdk

Source0:        %{comp_name}.tar.gz
BuildRoot:      %_tmppath/%name-%version-root

%description
Elasticsearch 专用安装包。
包含系统参数优化(vm.max_map_count)、句柄限制调整以及专用 hk 用户创建。

%prep
# 解压源码
%setup -q -n %{comp_name}

%pre
# 1. 检查并创建组: hk
# 这里的逻辑是：如果 getent group hk 返回 0 (存在)，则不执行 || 后面的 groupadd
# 如果不存在，则执行 groupadd
getent group hk >/dev/null || groupadd -r hk

# 2. 检查并创建用户: hk
# 注意：这里直接用 /opt/hk 作为家目录，防止宏未解析
getent passwd hk >/dev/null || \
    useradd -r -g hk -d /opt/hk -s /sbin/nologin -c "HK Service User" hk
exit 0

%install
rm -rf %{buildroot}
# 定义安装目录
%define _instdir /opt/hk/component/%{comp_name}

# 1. 创建目录
install -d -m 755 %{buildroot}%{_instdir}

# 2. 拷贝所有文件
cp -a * %{buildroot}%{_instdir}/

%post
# =========================================================
# 1. ⚙️ 系统参数优化 (limits.conf)
# =========================================================
echo "⚙️ [ES] 配置 limits.conf ..."
# 临时生效
ulimit -n 150000

LIMITS_FILE="/etc/security/limits.conf"
# 幂等性检查：防止重复写入
grep -q "^\* soft nofile 150000" $LIMITS_FILE || echo "* soft nofile 150000" >> $LIMITS_FILE
grep -q "^\* hard nofile 150000" $LIMITS_FILE || echo "* hard nofile 150000" >> $LIMITS_FILE

# =========================================================
# 2. ⚙️ 内核参数优化 (sysctl.conf)
# =========================================================
echo "⚙️ [ES] 配置 vm.max_map_count ..."
SYSCTL_FILE="/etc/sysctl.conf"

grep -q "^vm.max_map_count=262144" $SYSCTL_FILE || echo "vm.max_map_count=262144" >> $SYSCTL_FILE
# 应用内核参数
sysctl -p >/dev/null 2>&1

=========================================================
# 3. 📂 外部数据目录处理
# =========================================================
# 如果有位于 /opt/hk 之外的数据目录，需要在这里手动授权
if [ ! -d "/data/elasticsearch" ]; then
    echo "📂 [ES] 创建外部数据目录 /data/elasticsearch ..."
    mkdir -p /data/elasticsearch
    # 必须手动授权，因为 %files 管不到这里
    chown -R hk:hk /data/elasticsearch
    chmod 755 /data/elasticsearch
fi

echo "✅ Elasticsearch 组件安装及配置完成。"

%postun
# 卸载时清理目录
if [ $1 -eq 0 ]; then
    rm -rf /opt/hk/component/%{comp_name}
    echo "🗑️ 组件 %{comp_name} 已移除。"
fi

%files
# =========================================================
# 🏆 最佳实践：利用 RPM 机制自动管理权限
# =========================================================
# 语法: %defattr(文件权限, 用户, 组, 目录权限)
# 这里强制指定所有文件的所有者为 hk:hk
%defattr(-, hk, hk, -)

# 包含组件目录
/opt/hk/component/%{comp_name}

%clean
rm -rf %{buildroot}
