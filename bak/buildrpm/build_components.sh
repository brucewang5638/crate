#!/bin/bash
set -e

# ========== 参数接收 ==========
ARCH=${1:-$(uname -m)}          # 架构: x86_64/aarch64
DIST=${2:-el7}                  # 发行版: el7/el8
VERSION=${3:-1.0.0}             # 版本号

# ========== 配置区 ==========
# 需要打包的组件列表
COMPONENTS=("redis" "nacos" "kafka" "elasticsearch" "nginx")

BUILD_ROOT="$HOME/rpmbuild"
SOURCE_DIR="$BUILD_ROOT/SOURCES"
SPEC_DIR="$BUILD_ROOT/SPECS"

# 默认通用 SPEC 文件名
GENERIC_SPEC_NAME="hk-component-generic.spec"
# 组件源码所在的父目录
SRC_COMP_BASE="/opt/hk/component"

# ========== 显示构建参数 ==========
echo "=========================================="
echo "🔧 中间件组件智能构建系统"
echo "=========================================="
echo "  目标组件: ${COMPONENTS[*]}"
echo "  版本号: $VERSION"
echo "  系统架构: $ARCH"
echo "  源码路径: $SRC_COMP_BASE"
echo "=========================================="
echo ""

# ========== 初始化环境 ==========
echo "🧹 正在初始化构建环境..."
mkdir -p "$SOURCE_DIR" "$SPEC_DIR" "$BUILD_ROOT"/{RPMS,SRPMS,BUILD,BUILDROOT}

# 1. 拷贝通用 SPEC 到 SPEC 目录
if [ -f "$GENERIC_SPEC_NAME" ]; then
    sed -i 's/\r//g' "$GENERIC_SPEC_NAME"
    cp "$GENERIC_SPEC_NAME" "$SPEC_DIR/"
else
    echo "❌ 错误：找不到通用 Spec 文件 $GENERIC_SPEC_NAME"
    exit 1
fi

# 2. 扫描并拷贝当前目录下所有的专用 SPEC (如 hk-component-redis.spec)
# 注意：这里加了双引号保护，防止文件名带空格导致错误
ls hk-component-*.spec 2>/dev/null | grep -v "$GENERIC_SPEC_NAME" | while read -r specific_spec; do
    echo "📄 发现专用 SPEC: $specific_spec"
    sed -i 's/\r//g' "$specific_spec"
    cp "$specific_spec" "$SPEC_DIR/"
done

# ========== 循环构建流程 ==========
echo "🚀 开始批量构建任务..."

for COMP in "${COMPONENTS[@]}"; do
    echo ""
    echo "--------------------------------------------"
    echo "▶️  正在处理组件: [ $COMP ]"
    
    # --- 阶段 1: 准备源码 ---
    COMP_SRC_DIR="$SRC_COMP_BASE/$COMP"
    
    if [ ! -d "$COMP_SRC_DIR" ]; then
        echo "⚠️  警告: 找不到组件目录 $COMP_SRC_DIR"
        echo "   跳过该组件构建..."
        continue
    fi
    
    echo "📦 阶段 1: 封装源码包 -> $COMP.tar.gz"
    # 使用 -C 切换目录，确保 tar 包解压后直接是组件名目录
    tar -czf "$SOURCE_DIR/$COMP.tar.gz" -C "$SRC_COMP_BASE" "$COMP"
    
    # --- 阶段 2: 智能选择 SPEC ---
    # 规则：如果存在 hk-component-{组件名}.spec，则优先使用，否则用通用模板
    SPEC_TARGET="$SPEC_DIR/hk-component-$COMP.spec"
    
    if [ -f "$SPEC_TARGET" ]; then
        USE_SPEC="$SPEC_TARGET"
        # 这里的 basename 调用和引号是容易出错的地方
        echo "🎯 策略: 使用【专用 SPEC】 -> $(basename "$USE_SPEC")"
    else
        USE_SPEC="$SPEC_DIR/$GENERIC_SPEC_NAME"
        echo "🔄 策略: 使用【通用 SPEC】 -> $(basename "$USE_SPEC")"
    fi
    
    # --- 阶段 3: 执行 RPM 构建 ---
    echo "🔨 阶段 3: 执行构建..."
    
    # 这里的 define 即使在专用 SPEC 里没用到也不会报错，为了兼容性统一传入
    rpmbuild -ba "$USE_SPEC" \
        --define "comp_name $COMP" \
        --define "version $VERSION" \
        --define "_topdir $BUILD_ROOT" \
        --define "dist .$DIST" \
        --target "$ARCH"

    echo "✅ 组件 $COMP 构建成功"
done

# ========== 结果展示 ==========
echo ""
echo "=========================================="
echo "🎉 批量构建任务完成！"
echo "📍 RPM 产物清单:"
# 确保这一行的引号是成对的
find "$BUILD_ROOT/RPMS" -name "hk-component-*-${VERSION}*.rpm" -exec ls -lh {} \;
echo "=========================================="
