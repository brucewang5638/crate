#!/bin/bash
# ==============================================================================
# 脚本名称: promote.sh
#以此脚本实现 “Build Once, Promote Anywhere” (一次构建，随处晋级) 的理念。
#
# 功能描述:
#   该脚本用于将本地（通常是测试环境）的构建产物或数据库，同步到远端（打包环境）。
#   核心特性：
#   1. 批量传输: 无论文件多少，尽量合并为一个网络连接，解决“多次输密码”的痛点。
#   2. 数据库集成: 支持自动导出本地 MySQL 数据库并推送到远端。
#   3. 智能路径: Automatically handling relative vs absolute paths.
#
# 使用方法:
#   ./promote.sh [--db] [文件或目录路径...]
#
# 示例:
#   1. 仅同步文件:
#      ./promote.sh components/redis /opt/hk/bin/myapp
#   2. 仅同步数据库:
#      ./promote.sh --db
#   3. 同时做:
#      ./promote.sh --db components/redis
# ==============================================================================

# 遇到任何错误立即停止，防止错误扩大
set -e

# ==============================================================================
# 1. 配置部分 (Configuration)
#    你可以修改这里的默认值，或者通过环境变量覆盖它们
# ==============================================================================

# --- 远端环境配置 ---
# 目标机器的 IP 地址
REMOTE_HOST="${PROMOTE_HOST:-192.168.3.31}"
# 目标机器的 SSH 用户名
REMOTE_USER="${PROMOTE_USER:-root}"
# 项目在两台机器上的根目录 (保持一致，方便同步)
ROOT_DIR="${PROMOTE_ROOT:-/opt/hk}"

# --- 数据库配置 (本地) ---
DB_HOST="${MYSQL_HOST:-127.0.0.1}"
DB_USER="${MYSQL_USER:-root}"
# 注意: 建议通过 ~/.my.cnf 配置密码，避免在此明文写密码
DB_PASS="${MYSQL_PASS:-123@ASDFjkl}"
# 需要导出的数据库列表
TARGET_DBS=("hkinfo-core" "hkinfo_ticket" "icp_analysis")

# ==============================================================================
# 2. 基础函数 (Helpers)
# ==============================================================================

# 打印带颜色的日志，方便区分
log() {
    echo -e "\033[32m[PROMOTE] $1\033[0m" # 绿色文本
}

warn() {
    echo -e "\033[33m[WARN] $1\033[0m"    # 黄色文本
}

error() {
    echo -e "\033[31m[ERROR] $1\033[0m"   # 红色文本
    exit 1
}

# ==============================================================================
# 3. 核心逻辑: 文件同步 (File Sync)
# ==============================================================================

# 用于存储待传输的相对路径列表
REL_PATHS=()

# 解析用户输入的一个路径参数
# 目的：将用户输入的各种路径（绝对的、相对的、带斜杠的）都统一转换为
# 相对于 ROOT_DIR 的“相对路径”，以便后续批量处理。
collect_path() {
    local INPUT="$1"
    local REL=""

    # 去除末尾的斜杠 (例如 "dir/" -> "dir")，保证路径规范
    INPUT="${INPUT%/}"

    if [[ "$INPUT" == /* ]]; then
        # 情况A: 用户输入了绝对路径 (例如 /opt/hk/bin)
        # 检查它是否在我们管理的 ROOT_DIR (/opt/hk) 下面
        if [[ "$INPUT" == "$ROOT_DIR"* ]]; then
            # 截取掉前缀，得到相对路径
            REL="${INPUT#$ROOT_DIR/}"
        else
            # 如果路径在 /opt/hk 之外，无法批量合并，只能单独处理 (降级)
            warn "路径 '$INPUT' 不在项目根目录 '$ROOT_DIR' 下，将单独传输。"
            sync_single_absolute "$INPUT"
            return
        fi
    else
        # 情况B: 用户输入了相对路径 (例如 components/redis)
        REL="$INPUT"
    fi

    # 检查本地文件到底存不存在
    if [ ! -e "$ROOT_DIR/$REL" ]; then
        error "未找到本地文件: $ROOT_DIR/$REL"
    fi

    # 加入待处理列表
    REL_PATHS+=("$REL")
}

# 处理那些不在项目根目录下的“特殊”绝对路径
sync_single_absolute() {
    local ABS_PATH="$1"
    local PARENT=$(dirname "$ABS_PATH")
    log "📦 [单独模式] 同步: $ABS_PATH"

    # 远程创建父目录 -> rsync 同步
    ssh "$REMOTE_USER@$REMOTE_HOST" "mkdir -p $PARENT"
    rsync -avz "$ABS_PATH" "$REMOTE_USER@$REMOTE_HOST:$PARENT/"
}

# 批量执行文件同步
# 核心优势：无论有多少个文件，只建立一次 SSH 连接 = 输入一次密码！
run_file_sync_batch() {
    # 如果没有文件要传，直接返回
    if [ ${#REL_PATHS[@]} -eq 0 ]; then return; fi

    log "📦 [批量模式] 正在传输 ${#REL_PATHS[@]} 个路径..."

    # 技巧解释:
    # 1. (cd "$ROOT_DIR" ...) : 临时切换到根目录，确保 rsync 能找到相对路径的文件
    # 2. rsync -R : 关键参数！"Relative"模式。
    #    举例: 传输 "bin/app"，它会在远端自动创建 "bin" 目录并把 "app" 放进去。
    #    这使得我们不需要手动去远端 mkdir。
    (
        cd "$ROOT_DIR"
        rsync -avzR "${REL_PATHS[@]}" "$REMOTE_USER@$REMOTE_HOST:$ROOT_DIR/"
    )
}

# ==============================================================================
# 4. 核心逻辑: 数据库导出与同步 (DB Export & Sync)
# ==============================================================================

run_db_export_batch() {
    log "💾 启动数据库处理流程..."

    # 创建一个临时目录用于存放导出的 SQL
    # mktemp -d 保证目录名唯一，trap 保证脚本结束时自动删除它
    local TMP_BASE=$(mktemp -d -t crate_db_export_XXXXXX)
    trap 'rm -rf "$TMP_BASE"' RETURN

    # 我们在临时目录里模拟出远端的目录结构: script/initdb/
    # 这样解压时可以直接解压到正确位置
    local STAGING_DIR="$TMP_BASE/script/initdb"
    mkdir -p "$STAGING_DIR"

    local COUNT=0

    # 循环导出每一个定义的数据库
    for DB in "${TARGET_DBS[@]}"; do
        local DUMP="$STAGING_DIR/${DB}.sql"

        # 组装 mysqldump 命令参数
        local ARGS=("-h" "$DB_HOST" "-u" "$DB_USER")
        [ -n "$DB_PASS" ] && ARGS+=("-p$DB_PASS")
        # --databases: 导出文件中包含 'CREATE DATABASE' 和 'USE' 语句
        # --add-drop-database: 包含 'DROP DATABASE' (覆盖旧库)
        ARGS+=("--databases" "--add-drop-database" "$DB")

        # 执行导出 (2>/dev/null 隐藏密码警告等杂音)
        if mysqldump "${ARGS[@]}" > "$DUMP" 2>/dev/null; then
            log "   ✅ 导出成功: $DB"
            # 计数器 +1 (注意：Bash 中 set -e 模式下不要用 ((COUNT++)))
            COUNT=$((COUNT + 1))
        else
            warn "   ⚠️ 导出失败: $DB (可能数据库不存在? 跳过)"
            rm -f "$DUMP"
        fi
    done

    # 只要有至少一个数据库导出成功，就推送到远端
    if [ "$COUNT" -gt 0 ]; then
        log "🚀 [管道模式] 正在推送 SQL 文件到远端..."

        # 核心技巧: Tar Pipeline (管道传输)
        # 1. tar -C ... -czf - .  : 把所有 SQL 文件打包并压缩成数据流(stdout)
        # 2. | ssh ...           : 建立 SSH 连接，把数据流传过去
        # 3. tar -xzf - -C ...   : 在远端接收数据流并解压
        # 结果: 极快，且只需验证一次密码
        tar -C "$TMP_BASE" -czf - . | ssh "$REMOTE_USER@$REMOTE_HOST" "mkdir -p $ROOT_DIR/script/initdb && tar -xzf - -C $ROOT_DIR"

        log "   ✅ 数据库推送完成。"
    else
        log "   ⚠️ 没有导出任何数据，跳过推送。"
    fi
}

# ==============================================================================
# 5. 主程序入口 (Main)
# ==============================================================================

# 标志位: 是否需要处理数据库
DO_DB=false

# 解析命令行参数
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --db)
            DO_DB=true
            ;;
        -h|--help)
            grep '^# ' "$0" | cut -c 3- # 显示本文件头部的注释作为帮助
            exit 0
            ;;
        *)
            # 如果不是选项，就认为是文件路径
            collect_path "$1"
            ;;
    esac
    shift # 移动到下一个参数
done

# 如果用户什么都没输入
if [ ${#REL_PATHS[@]} -eq 0 ] && [ "$DO_DB" = false ]; then
    echo "未指定任何操作。"
    echo "用法: $0 [--db] <path1> <path2> ..."
    exit 1
fi

log "=== 开始执行晋级任务 ==="

# 步骤 1: 处理数据库 (如果用户加了 --db)
if [ "$DO_DB" = true ]; then
    run_db_export_batch
fi

# 步骤 2: 处理文件同步
run_file_sync_batch

log "🎉 所有任务圆满结束!"
