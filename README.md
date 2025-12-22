# Crate 自动化构建工具

Crate 是一个现代化 Go 语言rpm包构建工具。它替代了传统的 Shell 脚本，提供原子化构建块构建、智能缓存、多架构支持以及一键发布 YUM 仓库的能力。

## ✨ 核心特性

*   **原子化构建**: 支持单独构建任意构建块（如 `redis`, `myservice`），无需全量跑。
*   **智能缓存**: 基于版本号自动检测缓存，避免重复构建耗时。
*   **强制构建**: 支持 `-force` 参数强制刷新缓存，适合开发调试。
*   **多架构支持**: 自动识别 `x86_64` (amd64) 和 `aarch64` (arm64)，适配 Centos 7 及 Kylin V10。
*   **一键发布**: 自动生成包含所有依赖 RPM 的 YUM 仓库 (`createrepo`/`repotrack`/`dnf`) 及安装脚本。
*   **Kylin 优化**: 针对 Kylin 系统自动切换至 `dnf` 和 `createrepo_c` 逻辑。

## 🚀 快速开始

### 1. 编译工具
```bash
go build -o crate ./cmd/crate
```

### 2. 常用命令 (最佳实践)

#### 🚀 全量构建 (首次运行/CI环境)
构建配置文件中列出的所有构建块。
```bash
./crate -build all
```

#### 🔄 服务迭代 (日常开发)
仅构建 Service 层构建块，并强制刷新缓存（忽略版本号检查）。
```bash
./crate -build services -force
```

#### 📦 正式发布 (提测/交付)
构建所有构建块，并生成最终的发布包（包含 repo 和 install.sh）。
```bash
./crate -build all -release
```

### 3. 所有选项
```text
  -build string
        指定构建的目标构建块名 (Name) 或 组名 (Group)，或者 'all'
  -config string
        配置文件路径 (默认 "config.yaml")
  -force
        强制重新构建，忽略缓存
  -release
        生成最终发布包 (包含仓库和安装脚本)
  -arch string
        覆盖默认架构 (例如: aarch64)
  -dist string
        覆盖发行版标识 (例如: el7)
```

## 📂 项目结构

```text
crate/
├── cmd/crate/          # 命令行入口 (main.go)
├── pkg/                # 核心逻辑
│   ├── builder/        # 构建器 (调用 rpmbuild)
│   ├── config/         # 配置解析
│   └── repo/           # 仓库生成 (repotrack/createrepo)
├── specs/              # RPM SPEC 文件 (静态命名，动态源码)
├── config.yaml         # 主配置文件
└── README.md           # 本文档
```

## ⚙️ 配置文件 (config.yaml)

```yaml
project_name: "my-project"
dist_default: "el7"
cache_dir: "~/.rpm_cache"  # 构建产物缓存路径
build_root: "~/rpmbuild"   # rpmbuild 工作目录

# 定义构建块组，方便批量构建
groups:
  infra: ["redis", "mysql", "nginx"]
  services: ["myservice", "gateway"]

# 构建块定义
components:
  - name: "redis"           # 构建块名 (Builder 内部标识)
    version: "6.2.6"
    type: "infra"
    spec: "specs/redis.spec"  # SPEC 文件路径
    source_dir: "/path/to/redis-src"       # 源码目录
    validate_exists: ["README.md"]         # (可选) 预检查文件
    pre_build: "make distclean"            # (可选) 预构建脚本
```

## 🛠️ 这里是如何工作的

1.  **Config**: 读取 `config.yaml`，解析构建块信息。
2.  **Pre-check**: 检查 `source_dir` 是否存在，执行 `pre_build` 脚本。
3.  **Cache**: 检查 `~/.rpm_cache` 是否已有 `redis-6.2.6-*.rpm`。
    *   如果有且无 `-force`: 直接跳过构建，使用缓存。
    *   如果无或有 `-force`: 进入构建流程。
4.  **Tarball**: 将 `source_dir` 打包为 `redis.tar.gz` (Source0)。
5.  **RPMBuild**: 调用 `rpmbuild`，传入 `version`, `comp_name` 等宏变量。
6.  **Release**: `-release` 模式下，收集所有 RPM，调用 `repotrack`/`dnf` 下载系统依赖，生成 `setup_repo.sh`，最后打包成最终交付物。
