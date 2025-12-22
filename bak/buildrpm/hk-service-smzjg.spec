# 1. 禁止 RPM 尝试解压和重新打包 JAR 文件 (解决 cpio: open 失败的关键)
%define __jar_repack %{nil}
# 2. 禁止生成 debuginfo 包 (防止 strip 导致的文件损坏)
%global debug_package %{nil}
# 3. 禁止对文件执行 strip 操作 (保持 JAR 包原封不动)
%define __strip /bin/true
# 4. 禁止自动扫描依赖 (解决 osgi(...) 刷屏和构建缓慢)
%global _use_internal_dependency_generator 0
# 5. 禁用所有默认的安装后处理脚本 (核武器：彻底防止 RPM 修改你的文件)
%define __os_install_post %{nil}

# 变量定义
%define _installpath    /opt/hk
%define _procmate_dir   /etc/procmate/conf.d
%define _dst_fonts      /usr/share/fonts/custom
%define _logfile        /var/log/%{pkg_name}-install.log

# --- 接收来自命令行的参数 ---
%{!?pkg_name: %global pkg_name hk-smzjg}
%{!?version: %global version 1.0.0}
%{!?source_file: %global source_file hkservice.tar.gz}

Name:           %{pkg_name}
Version:        %{version}
Release:        1%{?dist}
Summary:        HK 自监管业务应用主程序 (集成多目录版)

License:        Proprietary
URL:            http://hongkexinxi.com/
BuildArch:      noarch

# --- 核心依赖声明 ---
Requires:       java-devel >= 1.8.0
Requires:       mariadb-server >= 5.5.68
Requires:       fontconfig
Requires:       procmate >= 1.0.0
Requires:       hk-component-redis
Requires:       hk-component-nacos
Requires:       hk-component-kafka
Requires:       hk-component-elasticsearch
Requires:       hk-component-nginx

Source0:        %{source_file}

%description
用于部署业务逻辑服务，包含 hkservice、config、script 和 res 资源。

%prep
# 🚀 关键修改：如果你的 tar 包解压后是四个同级文件夹，不需要 -n
# 如果 tar 包里没有顶层目录，直接用 %setup -c
%setup -q -c

%install
rm -rf %{buildroot}

# 1. 创建目标目录结构
install -d -m 755 %{buildroot}%{_installpath}/hkservice
install -d -m 755 %{buildroot}%{_installpath}/res
install -d -m 755 %{buildroot}%{_installpath}/script
install -d -m 755 %{buildroot}%{_procmate_dir}

# 2. 拷贝业务主程序、资源文件和脚本
# 按照你的意图，这些文件夹现在是同级的
cp -a hkservice/* %{buildroot}%{_installpath}/hkservice/
cp -a res/* %{buildroot}%{_installpath}/res/
cp -a script/* %{buildroot}%{_installpath}/script/

# 3. 安装 Procmate 配置文件
install -d -m 755 %{buildroot}%{_procmate_dir}
install -m 644 config/%{pkg_name}.yaml %{buildroot}%{_procmate_dir}/%{pkg_name}.yaml
%post
# =========================================================
# 💡 定义输出辅助函数 (兼容 TTY 和 自动化脚本)
# =========================================================
print_msg() {
    # 尝试写入当前终端(/dev/tty)以实现实时显示
    # 如果没有终端(如 CI 环境)，则回退到标准输出
    echo "$1" > /dev/tty 2>/dev/null || echo "$1"
}

# --- 1. 权限加固 ---
chown -R hk:hk %{_installpath}
chmod +x %{_installpath}/script/initdb.sh

# --- 2. 系统参数优化 ---
if [ $1 -eq 1 ]; then
    print_msg "⚙️ [1/3] 正在优化系统参数及创建必备目录..."

    # --- order 目录 ---
    mkdir -p /data/order/temp
    chmod -R 777 /data/order

    # --- 3. 字体安装 (来自 res 目录) ---
    if [ -d "%{_installpath}/res/fonts/win" ]; then
        print_msg "🔤 [2/3] 正在安装业务字体..."

        mkdir -p %{_dst_fonts}
        cp %{_installpath}/res/fonts/win/*.ttf %{_dst_fonts} 2>/dev/null || true
        fc-cache -fv >/dev/null 2>&1
    fi

    # --- 4. 数据库初始化 (来自 script 目录) ---
    print_msg "🚀 [3/3] 正在对关联的数据库初始化 (耗时较长，请耐心等待)..."

    bash %{_installpath}/script/initdb.sh >> %{_logfile} 2>&1 || echo "⚠️ 初始化脚本异常"
fi

# --- 5. 启动托管 ---
systemctl daemon-reload
systemctl try-restart procmate.service >/dev/null 2>&1

%preun
if [ $1 -eq 0 ]; then
    rm -f %{_procmate_dir}/%{pkg_name}.yaml
    systemctl try-restart procmate.service >/dev/null 2>&1
fi

%postun
if [ $1 -eq 0 ]; then
    # 只删除业务相关的目录，不要删顶层目录
    rm -rf %{_installpath}/hkservice
    rm -rf %{_installpath}/res
    rm -rf %{_installpath}/script
    echo "🗑️ 业务组件已卸载完成。"
fi

%files
%defattr(-,root,root,-)
%{_installpath}/hkservice
%{_installpath}/res
%{_installpath}/script
%config(noreplace) %{_procmate_dir}/%{pkg_name}.yaml

%clean
rm -rf %{buildroot}
