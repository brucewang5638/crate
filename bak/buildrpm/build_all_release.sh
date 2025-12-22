#!/bin/bash
set -e

# ========== 版本配置区（集中管理）==========
declare -A VERSIONS=(
    ["procmate"]="1.5.3"
    ["components"]="1.0.0"
    ["services"]="1.0.0"
)

# ========== 系统配置区 ==========
ARCH=${ARCH:-$(uname -m)}           # 支持环境变量覆盖
DIST=${DIST:-el7}                   # RPM 发行版标识
BUILD_ROOT="$HOME/rpmbuild"
CACHE_DIR="/root/.rpm_cache"

# 组件定义
declare -A COMPONENTS=(
    ["procmate"]="build_procmate.sh"
    ["components"]="build_components.sh"
    ["services"]="build_services.sh"
)

# ========== 参数解析 ==========
USE_CACHE=()
REBUILD=()
EXTERNAL_RPM_DIR=""
SHOW_VERSIONS=false

print_usage() {
    cat <<EOF
用法: $0 [选项]

选项:
  --use-cache <components>     从缓存复用指定组件的 RPM (逗号分隔)
                               可选值: procmate, components, services
  
  --rebuild <components>       重新构建指定组件 (逗号分隔)
                               可选值: procmate, components, services, all
  
  --external <目录>            直接使用指定目录下的现有 RPM (完全跳过构建)
  
  --versions                   显示当前版本配置并退出
  
  --arch <架构>                指定目标架构 (默认: 自动检测)
  
  --dist <发行版>              指定 RPM 发行版 (默认: el7)

环境变量:
  ARCH=<架构>                  覆盖系统架构 (x86_64/aarch64)
  DIST=<发行版>                覆盖发行版标识 (el7/el8)

示例:
  $0 --rebuild all                                    # 全量重新构建
  $0 --use-cache procmate,components --rebuild services  # 复用基础组件，只构建业务服务
  $0 --external /path/to/rpms                         # 完全使用外部 RPM
  $0 --versions                                       # 查看版本配置
  ARCH=aarch64 $0 --rebuild services                  # 指定架构构建

EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --use-cache)
            IFS=',' read -ra USE_CACHE <<< "$2"
            shift 2
            ;;
        --rebuild)
            IFS=',' read -ra REBUILD <<< "$2"
            shift 2
            ;;
        --external)
            EXTERNAL_RPM_DIR="$2"
            shift 2
            ;;
        --versions)
            SHOW_VERSIONS=true
            shift
            ;;
        --arch)
            ARCH="$2"
            shift 2
            ;;
        --dist)
            DIST="$2"
            shift 2
            ;;
        -h|--help)
            print_usage
            ;;
        *)
            echo "❌ 未知参数: $1"
            print_usage
            ;;
    esac
done

# 示例: hk-smzjg-release-el7-20251219
RELEASE_NAME="hk-smzjg-release-${DIST}-$(date +%Y%m%d)"
REPO_DIR="/root/${RELEASE_NAME}/hk-repo"

# ========== 显示版本配置 ==========
if [ "$SHOW_VERSIONS" = true ]; then
    echo "=========================================="
    echo "  📋 当前版本配置"
    echo "=========================================="
    echo "组件版本:"
    for comp in procmate components services; do
        printf "  %-12s : %s\n" "$comp" "${VERSIONS[$comp]}"
    done
    echo ""
    echo "系统参数:"
    echo "  架构 (ARCH) : $ARCH"
    echo "  发行版 (DIST): $DIST"
    echo "=========================================="
    exit 0
fi

# ========== 验证参数 ==========
for comp in "${USE_CACHE[@]}" "${REBUILD[@]}"; do
    if [[ "$comp" != "all" ]] && [[ ! ${COMPONENTS[$comp]+_} ]]; then
        echo "❌ 无效的组件名: $comp"
        print_usage
    fi
done

# ========== 初始化 ==========
echo "=========================================="
echo "🌟 HK-SMZJG 智能发布系统"
echo "=========================================="
echo "📋 构建配置:"
echo "   架构: $ARCH"
echo "   发行版: $DIST"
echo "   版本: procmate=${VERSIONS[procmate]}, components=${VERSIONS[components]}, services=${VERSIONS[services]}"
echo "=========================================="
echo ""

mkdir -p "$CACHE_DIR" "$REPO_DIR"

# ========== 主流程：外部 RPM 模式 ==========
if [ -n "$EXTERNAL_RPM_DIR" ]; then
    echo "🔨 模式: 外部 RPM 模式 (完全跳过构建)"
    if [ ! -d "$EXTERNAL_RPM_DIR" ]; then
        echo "❌ 错误: 目录 $EXTERNAL_RPM_DIR 不存在！" && exit 1
    fi
    
    rpm_count=$(find "$EXTERNAL_RPM_DIR" -name "*.rpm" | wc -l)
    if [ "$rpm_count" -eq 0 ]; then
        echo "⚠️  警告: 目录 $EXTERNAL_RPM_DIR 下没有找到 RPM 文件！"
        exit 1
    fi
    
    cp "$EXTERNAL_RPM_DIR"/*.rpm "$REPO_DIR/"
    echo "✅ 已汇聚 $rpm_count 个外部 RPM"

# ========== 主流程：构建+缓存模式 ==========
else
    echo "🔨 模式: 智能构建模式 (支持缓存复用)"
    
    # 处理 --rebuild all
    if [[ " ${REBUILD[@]} " =~ " all " ]]; then
        REBUILD=("procmate" "components" "services")
    fi
    
    # 默认行为：如果没有指定任何参数，则全量重新构建
    if [ ${#USE_CACHE[@]} -eq 0 ] && [ ${#REBUILD[@]} -eq 0 ]; then
        echo "ℹ️  未指定参数，默认全量重新构建"
        REBUILD=("procmate" "components" "services")
    fi
    
    # ========== 步骤1: 从缓存复用 RPM ==========
    for comp in "${USE_CACHE[@]}"; do
        echo ""
        echo "📦 正在从缓存复用组件: $comp (版本: ${VERSIONS[$comp]})"
        
        # 查找该组件在缓存中的 RPM（支持通配符匹配）
        case $comp in
            procmate)
                pattern="*procmate-*.rpm"
                ;;
            components)
                pattern="hk-component-*.rpm"
                ;;
            services)
                pattern="hk-service-*.rpm"
                ;;
        esac
        
        cached_rpms=$(find "$CACHE_DIR" -name "$pattern" 2>/dev/null)
        
        if [ -z "$cached_rpms" ]; then
            echo "⚠️  警告: 缓存中未找到 $comp 的 RPM，将强制重新构建"
            REBUILD+=("$comp")
        else
            cp $cached_rpms "$REPO_DIR/"
            count=$(echo "$cached_rpms" | wc -l)
            echo "   ✅ 已复用 $count 个缓存 RPM"
        fi
    done
    
    # ========== 步骤2: 重新构建指定组件 ==========
    for comp in "${REBUILD[@]}"; do
        script="${COMPONENTS[$comp]}"
        version="${VERSIONS[$comp]}"
        
        if [ ! -f "$script" ]; then
            echo "⚠️  跳过 $comp: 构建脚本 $script 不存在"
            continue
        fi
        
        echo ""
        echo "🔧 正在重新构建组件: $comp"
        echo "   参数: ARCH=$ARCH, DIST=$DIST, VERSION=$version"
        
        # 调用 build 脚本并传递参数
        bash "./$script" "$ARCH" "$DIST" "$version"
        
        # 将新构建的 RPM 复制到仓库和缓存
        case $comp in
            procmate)
                pattern="*procmate-*.rpm"
                ;;
            components)
                pattern="hk-component-*.rpm"
                ;;
            services)
                pattern="hk-service-*.rpm"
                ;;
        esac
        
        # 清理旧缓存（只清理当前版本对应的文件）
        # 保留其他版本以支持回滚
        find "$CACHE_DIR" -name "$pattern" -delete 2>/dev/null || true
        
        # 保存新 RPM 到缓存和仓库
        new_rpms=$(find "$BUILD_ROOT/RPMS" -name "$pattern" 2>/dev/null)
        if [ -z "$new_rpms" ]; then
            echo "⚠️  警告: 未找到构建产物 $pattern"
        else
            echo "$new_rpms" | while read rpm; do
                cp "$rpm" "$CACHE_DIR/"
                cp "$rpm" "$REPO_DIR/"
            done
            echo "   ✅ $comp 构建完成并已更新缓存"
        fi
    done
    
    echo ""
    echo "✅ 所有业务 RPM 准备就绪"
fi

# ========== 系统依赖采集 ==========
echo ""
echo "================================================"
echo "📦 阶段 2: 采集系统依赖"
echo "================================================"
DEPS=("mariadb-server" "java-1.8.0-openjdk-devel" "fontconfig" "createrepo" "deltarpm")

# 确保 repotrack 可用
if ! command -v repotrack &> /dev/null; then
    echo "   安装 yum-utils..."
    yum install -y yum-utils createrepo &> /dev/null
fi

for pkg in "${DEPS[@]}"; do
    echo "   -> $pkg"
    repotrack -p "$REPO_DIR" "$pkg" &> /dev/null
done

# ========== 构建仓库索引 ==========
echo ""
echo "================================================"
echo "📂 阶段 3: 生成仓库元数据"
echo "================================================"
createrepo "$REPO_DIR" &> /dev/null
echo "✅ 仓库索引创建完成"

# ========== 生成安装脚本 ==========
cat <<'SETUP_SCRIPT' > "/root/${RELEASE_NAME}/setup_repo.sh"
#!/bin/bash
set -e

CUR_DIR="$(cd "$(dirname "$0")"; pwd)"
REPO_PATH="${CUR_DIR}/hk-repo"

echo "=========================================="
echo "  HK-SMZJG 离线安装程序"
echo "=========================================="

echo "⚙️  配置本地离线源..."

# 1. 创建离线仓库配置 (不移动也不删除现有任何 .repo 文件)
cat > /etc/yum.repos.d/hk-local.repo <<EOR
[hk-local]
name=HK_Offline_Repository
baseurl=file://$REPO_PATH
enabled=1
gpgcheck=0
EOR

# 2. 清理缓存 (这一步通常很快，不需要忽略)
yum clean all &> /dev/null

# 3. 只建立本地源的缓存
# 加上 --disablerepo=* --enablerepo=hk-local
# 这样 yum 就不会尝试连接外网，也不会报错退出
echo "   正在生成本地缓存..."
yum makecache --disablerepo=* --enablerepo=hk-local &> /dev/null

# 4. 安装主程序
echo ""
echo "🚀 开始安装 hk-smzjg..."
# 这里保持原样，只使用本地源进行安装
yum install -y hk-service-smzjg --disablerepo=* --enablerepo=hk-local

echo ""
echo "=========================================="
echo "✅ 安装完成！"
echo ""
echo "后续操作:"
echo "  procmate status     # 检查状态"
echo "  procmate start all   # 启动服务"
SETUP_SCRIPT

chmod +x "/root/${RELEASE_NAME}/setup_repo.sh"

# ========== 生成版本清单 ==========
cat > "/root/${RELEASE_NAME}/VERSION_MANIFEST.txt" <<MANIFEST
========================================
HK-SMZJG 发布包版本清单
========================================
发布日期: $(date '+%Y-%m-%d %H:%M:%S')
构建架构: $ARCH
发行版本: $DIST

组件版本:
  procmate    : ${VERSIONS[procmate]}
  components  : ${VERSIONS[components]}
  services    : ${VERSIONS[services]}

RPM 列表:
$(find "$REPO_DIR" -name "*.rpm" -exec basename {} \; | grep -E "^(hk-|procmate)" | sort)

========================================
MANIFEST

# ========== 打包发布 ==========
echo ""
echo "================================================"
echo "🗜️  阶段 4: 生成最终交付包"
echo "================================================"
cd /root
tar -czf "${RELEASE_NAME}.tar.gz" "${RELEASE_NAME}" 2>/dev/null

# ========== 完成总结 ==========
echo ""
echo "================================================"
echo "🎉 发布包制作完成！"
echo "================================================"
echo "📦 文件位置: /root/${RELEASE_NAME}.tar.gz"
echo ""
echo "📊 内容统计:"
rpm_count=$(find "$REPO_DIR" -name "*.rpm" | wc -l)
size=$(du -sh "/root/${RELEASE_NAME}.tar.gz" | cut -f1)
echo "   - RPM 数量: $rpm_count"
echo "   - 包大小: $size"
echo "   - 架构: $ARCH"
echo "   - 发行版: $DIST"
echo ""
echo "📋 版本信息:"
echo "   - procmate: ${VERSIONS[procmate]}"
echo "   - components: ${VERSIONS[components]}"
echo "   - services: ${VERSIONS[services]}"
echo ""
echo "🚀 目标机器安装步骤:"
echo "   1. tar -xzf ${RELEASE_NAME}.tar.gz"
echo "   2. cd ${RELEASE_NAME}"
echo "   3. cat VERSION_MANIFEST.txt  # 查看版本清单"
echo "   4. ./setup_repo.sh           # 开始安装"
echo "================================================"

