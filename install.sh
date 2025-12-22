#!/bin/bash

# 任何命令失败则立即退出，防止不完整的安装
set -e

# === 步骤 1: 解析参数与定义路径 ===

# --- 解析参数 ---
FORCE_MODE=false
SOURCE_PATH_ARG=""

# 遍历所有传入的参数
for arg in "$@"; do
  case "$arg" in
    -f|--force)
      FORCE_MODE=true
      ;;
    *)
      # 将第一个非标志的参数识别为源路径
      if [ -z "$SOURCE_PATH_ARG" ]; then
        SOURCE_PATH_ARG="$arg"
      fi
      ;;
  esac
done

# 如果识别到 --force 标志，则打印提示信息
if [ "$FORCE_MODE" = true ]; then
    echo "ℹ️  检测到 '--force' 标志，将强制覆盖配置文件。"
fi

# --- 定义路径 ---
# 如果用户未提供路径参数，则默认为当前目录 "."
CRATE_SOURCE_PATH="${SOURCE_PATH_ARG:-.}"
CRATE_BINARY_PATH="${CRATE_SOURCE_PATH}/crate"
CRATE_CONFIG_PATH="${CRATE_SOURCE_PATH}/config.yaml"
CRATE_SPECS_PATH="${CRATE_SOURCE_PATH}/specs"

CRATE_INSTALL_DIR="/opt/crate"
CRATE_BIN_LINK="/usr/local/bin/crate"
CRATE_ETC_DIR="/etc/crate"

# === 步骤 2: 文件存在性检查 ===
echo "🔎 正在检查所需文件..."
if [ ! -f "${CRATE_BINARY_PATH}" ]; then
    echo "❌ 错误: 在路径 '${CRATE_BINARY_PATH}' 下找不到 'crate' 可执行文件。"
    exit 1
fi

if [ ! -f "${CRATE_CONFIG_PATH}" ]; then
    echo "❌ 错误: 在路径 '${CRATE_CONFIG_PATH}' 下找不到 'config.yaml' 配置文件。"
    exit 1
fi
echo "✅ 文件检查通过。"
echo ""

# === 步骤 3: 安装二进制文件 ===
echo "🚀 正在安装 crate 程序..."
sudo mkdir -p "${CRATE_INSTALL_DIR}"
sudo cp "${CRATE_BINARY_PATH}" "${CRATE_INSTALL_DIR}/"
sudo chmod 755 "${CRATE_INSTALL_DIR}/crate"
sudo ln -sf "${CRATE_INSTALL_DIR}/crate" "${CRATE_BIN_LINK}"
echo "✅ 程序已安装!"
echo ""

# === 步骤 4: 安装配置文件与SPEC ===
echo "📦 正在复制配置文件..."
sudo mkdir -p "${CRATE_ETC_DIR}"

# --- 智能处理主配置文件 config.yaml ---
TARGET_CONFIG_FILE="${CRATE_ETC_DIR}/config.yaml"
if [ -f "${TARGET_CONFIG_FILE}" ] && [ "$FORCE_MODE" = false ]; then
    echo "⚠️  警告: 主配置文件 '${TARGET_CONFIG_FILE}' 已存在。跳过复制。"
    echo "     请手动处理该文件，或使用 '--force' 标志运行安装脚本以强制覆盖。"
else
    if [ -f "${TARGET_CONFIG_FILE}" ]; then
        echo "  -> --force 模式: 正在覆盖主配置文件 '${TARGET_CONFIG_FILE}'..."
    else
        echo "  -> 正在复制主配置文件..."
    fi
    sudo cp "${CRATE_CONFIG_PATH}" "${TARGET_CONFIG_FILE}"
fi

# --- 处理 specs 目录 ---
if [ -d "${CRATE_SPECS_PATH}" ]; then
    echo "  -> 正在复制 'specs' 目录..."
    sudo cp -r "${CRATE_SPECS_PATH}" "${CRATE_ETC_DIR}/"
    echo "✅ 'specs' 目录已更新。"
fi
echo ""

# === 步骤 5: 完成 ===
echo "🎉 Crate 安装完成！"
echo "您现在可以运行: crate --help 获得帮助!"
echo ""
echo "📝 注意: 默认配置文件位于 ${CRATE_ETC_DIR}/config.yaml"
echo "      如果在非源码目录下运行，建议指定 -config ${CRATE_ETC_DIR}/config.yaml"