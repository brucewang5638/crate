#!/bin/bash
set -e

# ========== 参数接收（兼容独立运行）==========
ARCH=${1:-$(uname -m)}          # 架构: x86_64/aarch64（注意：noarch 包不受影响）
DIST=${2:-el7}                  # 发行版: el7/el8
VERSION=${3:-1.0.0}             # 版本号

# ========== 配置区 ==========
NAME="hk-service-smzjg"
BUILD_ROOT="$HOME/rpmbuild"
SOURCE_DIR="$BUILD_ROOT/SOURCES"
SPEC_DIR="$BUILD_ROOT/SPECS"
SPEC_FILE="hk-service-smzjg.spec"

# 原始文件夹所在的父目录
SRC_BASE="/opt/hk"
DIRS_TO_PACK=("hkservice" "config" "script" "res")

# ========== 显示构建参数 ==========
echo "=========================================="
echo "🔧 Services 构建配置"
echo "=========================================="
echo "  组件名称: $NAME"
echo "  版本号: $VERSION"
echo "  系统架构: $ARCH (构建为 noarch)"
echo "  发行版: $DIST"
echo "  源码路径: $SRC_BASE"
echo "  打包目录: ${DIRS_TO_PACK[*]}"
echo "=========================================="
echo ""

# ========== 初始化环境 ==========
echo "🧹 正在清理并初始化构建环境..."
mkdir -p $SOURCE_DIR $SPEC_DIR $BUILD_ROOT/{RPMS,SRPMS,BUILD,BUILDROOT}

# 修正 Spec 换行符并拷贝
if [ -f "$SPEC_FILE" ]; then
    sed -i 's/\r//g' "$SPEC_FILE"
    cp "$SPEC_FILE" "$SPEC_DIR/"
else
    echo "❌ 错误：找不到 Spec 文件 $SPEC_FILE" && exit 1
fi

# ========== 准备源码包 ==========
echo "📦 阶段 1: 正在封装多目录源码包..."

# 检查所有目录是否齐全
for dir in "${DIRS_TO_PACK[@]}"; do
    if [ ! -d "$SRC_BASE/$dir" ]; then
        echo "❌ 错误: 关键目录 $SRC_BASE/$dir 不存在！"
        exit 1
    fi
done

# 使用 -C 切换到父目录，直接打包四个文件夹
tar -czf "$SOURCE_DIR/hkservices.tar.gz" -C "$SRC_BASE" "${DIRS_TO_PACK[@]}"

echo "✅ 源码包已就绪: hkservices.tar.gz"

# ========== 执行构建 ==========
echo "🔨 阶段 2: 开始构建 RPM 包 -> ${NAME}"

rpmbuild -ba "$SPEC_DIR/$SPEC_FILE" \
    --define "pkg_name $NAME" \
    --define "version $VERSION" \
    --define "source_file hkservices.tar.gz" \
    --define "_topdir $BUILD_ROOT" \
    --define "dist .$DIST"

# 注意：services 包通常是 noarch，所以不需要 --target 参数

# ========== 结果展示 ==========
echo "--------------------------------------------"
echo "🎉 业务集成包构建成功"
echo "📍 RPM 位置:"
find "$BUILD_ROOT/RPMS" -name "${NAME}-*.rpm" -exec ls -lh {} \;
