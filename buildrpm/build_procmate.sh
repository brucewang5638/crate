#!/bin/bash
set -e

# ========== 参数接收（兼容独立运行）==========
ARCH=${1:-$(uname -m)}          # 架构: x86_64/aarch64
DIST=${2:-el7}                  # 发行版: el7/el8
VERSION=${3:-1.5.3}             # 版本号

# ========== 架构名称标准化 ==========
# uname -m 返回 x86_64，但文件通常命名为 amd64
if [ "$ARCH" == "x86_64" ]; then
    FILE_ARCH="amd64"
elif [ "$ARCH" == "aarch64" ]; then
    FILE_ARCH="arm64"
else
    FILE_ARCH="$ARCH"
fi

# ========== 配置区 ==========
NAME="procmate"
BUILD_ROOT="$HOME/rpmbuild"
SOURCE_DIR="$BUILD_ROOT/SOURCES"
SPEC_DIR="$BUILD_ROOT/SPECS"

# 源码文件位置
SRC_BASE=/opt/hk/procmate_${VERSION}_linux_${FILE_ARCH}

# ========== 显示构建参数 ==========
echo "=========================================="
echo "🔧 Procmate 构建配置"
echo "=========================================="
echo "  组件名称: $NAME"
echo "  版本号: $VERSION"
echo "  系统架构: $ARCH"
echo "  发行版: $DIST"
echo "  源码路径: $SRC_BASE"
echo "=========================================="
echo ""

# ========== 初始化环境 ==========
echo "🧹 正在初始化构建环境..."
mkdir -p $SOURCE_DIR $SPEC_DIR $BUILD_ROOT/{RPMS,SRPMS,BUILD,BUILDROOT}
cp "procmate.spec" "$SPEC_DIR/"

# ========== 准备源码包 ==========
echo "--------------------------------------------"
echo "📦 阶段 1: 准备源码包 -> $NAME"

# 检查源码目录是否存在
if [ ! -d "$SRC_BASE" ]; then
    echo "⚠️  警告: 标准路径 $SRC_BASE 不存在"
    echo "   尝试使用通用路径..."
    # 回退到不带架构的路径
    SRC_BASE=/root/procmate_${VERSION}_linux_amd64
    if [ ! -d "$SRC_BASE" ]; then
        echo "❌ 错误: 找不到源码目录！"
        echo "   已尝试路径:"
        echo "   - /root/procmate_${VERSION}_linux_${ARCH}"
        echo "   - /root/procmate_${VERSION}_linux_amd64"
        exit 1
    fi
fi

# 检查关键文件是否存在
for file in "procmate" "config.yaml"; do
    if [ ! -f "$SRC_BASE/$file" ]; then
        echo "❌ 错误: 关键文件 $SRC_BASE/$file 不存在！"
        exit 1
    fi
done

# 制作 Source0 压缩包
tar -czf "$SOURCE_DIR/$NAME.tar.gz" -C "$SRC_BASE" procmate config.yaml

echo "✅ 源码包已就绪"

# ========== 执行构建 ==========
echo "🔨 阶段 2: 构建 RPM 包 -> $NAME"

rpmbuild -ba "$SPEC_DIR/${NAME}.spec" \
    --define "pkg_name $NAME" \
    --define "version $VERSION" \
    --define "_topdir $BUILD_ROOT" \
    --define "dist .$DIST" \
    --target "${ARCH}"

# ========== 结果展示 ==========
echo "--------------------------------------------"
echo "🎉 Procmate RPM 构建成功"
echo "📍 RPM 位置:"
find $BUILD_ROOT/RPMS -name "${NAME}-${VERSION}*.rpm" -exec ls -lh {} \;
