# sv - Supervisor进程管理工具 🚀

一个基于Go语言开发的现代化Supervisor进程管理工具，采用模块化架构设计，支持序号操作和智能RPC配置，让进程管理更加便捷！

## ✨ 核心特性

- 🎯 **序号化操作** - 使用数字序号代替长进程名，提升运维效率
- 📊 **美观表格显示** - Unicode直线边框，完美对齐，彩色状态+图标
- 🔧 **灵活进程控制** - 支持单个、多个、范围、混合操作格式
- 🌐 **远程服务器管理** - 支持认证远程Supervisor服务器
- 🛠️ **智能配置检测** - 自动检测和配置Supervisor RPC功能
- 🚀 **短生命周期控制** - 工具只通过RPC控制Supervisor，不作为后台服务运行
- 💡 **智能错误处理** - 友好的中文错误提示和解决建议
- 🔄 **双模式架构** - RPC优先，命令行模式自动回退，确保兼容性
- ⚡ **极致性能** - Go语言开发，单二进制文件，超快启动速度

## 🏗️ 项目架构

项目采用清晰的模块化架构设计，职责分离：

```
sv/
├── main.go                    # 主程序入口（23行，极简设计）
├── build.sh                   # 构建脚本
├── pkg/                       # 核心包目录
│   ├── cli/                   # CLI应用层
│   │   ├── app.go            # CLI应用逻辑
│   │   └── renderer.go       # 渲染器
│   ├── supervisor/            # Supervisor核心功能
│   │   ├── rpc_client.go     # XML-RPC客户端
│   │   ├── config_detector.go # 配置检测器
│   │   ├── process_control.go # 进程控制
│   │   └── types.go          # 数据结构定义
│   └── utils/                 # 工具函数
│       └── common.go         # 通用工具函数
├── *.go                      # 测试文件（完整测试覆盖）
├── go.mod                    # Go模块依赖（Go 1.23.0+）
└── README.md                 # 项目文档
```

### 核心组件

- **CLI应用层**: 负责命令解析、参数验证和用户交互
- **业务逻辑层**: Supervisor通信、进程管理、配置检测
- **工具函数层**: 通用工具、数据结构、显示格式化
- **初始化层**: 检测配置并启动已有的Supervisor服务

## 🚀 快速开始

### 环境要求

- Go 1.23.0+
- Supervisor 3.x+

### 安装编译

```bash
# 克隆仓库
git clone https://github.com/x1t/sv.git
cd sv

# 安装依赖
go mod tidy

# 使用构建脚本（推荐，输出到dist目录）
./build.sh

# 或者手动编译
go build -ldflags="-s -w" -o sv main.go

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o sv-linux-amd64 main.go
GOOS=windows GOARCH=amd64 go build -o sv-windows-amd64.exe main.go
GOOS=darwin GOARCH=amd64 go build -o sv-darwin-amd64 main.go
```

### 从 GitHub Release 安装

安装脚本自动识别 Linux amd64/arm64，从 `x1t/sv` 下载最新正式 Release，
将二进制安装为 `/usr/local/bin/sv`。两个版本使用相同安装路径，请选择其中一个。

```sh
curl -fsSL https://raw.githubusercontent.com/x1t/sv/refs/heads/main/install.sh | sh
```

指定已发布版本或安装目录：

```sh
curl -fsSL https://raw.githubusercontent.com/x1t/sv/refs/heads/main/install.sh | sh -s -- --version v0.3.0 --install-dir /usr/bin
```

脚本支持 `curl` / `wget` 下载以及 `SV_VERSION`、`SV_INSTALL_DIR` 环境变量。
安装目录须已存在；没有写权限时会尝试 `sudo`，OpenWrt 可直接以 root 执行。
安装只复制二进制，不会启动 Supervisor；安装后使用 `sv --help`。

### 基本使用

```bash
# 查看所有进程状态（带序号和完美表格显示）
./sv status
./sv list      # 同status

# 重启序号为1的进程
./sv restart 1

# 停止序号为2、4、6的进程
./sv stop 2 4 6

# 启动序号1到3的所有进程
./sv start 1-3

# 使用进程名操作（兼容传统方式）
./sv restart nginx redis

# 混合使用各种格式
./sv restart 1 nginx 3-5

# 初始化Supervisor RPC并启动已有的Supervisor服务
sudo ./sv init
```

## 📋 命令参考

### 主要命令

| 命令 | 描述 | 示例 |
|------|------|------|
| `status` | 显示所有进程状态 | `./sv status` |
| `list` | 显示所有进程状态（同status） | `./sv list` |
| `start` | 启动指定进程 | `./sv start 1` |
| `stop` | 停止指定进程 | `./sv stop 1-3` |
| `restart` | 重启指定进程 | `./sv restart nginx` |
| `init` | 初始化并启动Supervisor RPC | `./sv init` |
| `help` | 显示帮助信息 | `./sv help` |

### 进程参数格式

| 格式 | 说明 | 示例 |
|------|------|------|
| `序号` | 使用数字序号 | `./sv restart 1` |
| `名称` | 使用进程名称 | `./sv restart myapp` |
| `多个` | 空格分隔多个参数 | `./sv restart 1 3 5` |
| `范围` | 横线表示序号范围 | `./sv restart 1-5` |
| `混合` | 混合使用各种格式 | `./sv restart 1 nginx 3-5` |

## 🔧 环境配置

### Supervisor连接配置

```bash
# Supervisor RPC服务器地址
export SUPERVISOR_HOST="http://localhost:9001/RPC2"

# 如果需要认证
export SUPERVISOR_USER="your_username"
export SUPERVISOR_PASSWORD="your_password"

# 同步控制操作超时（秒，可选）：start/stop/restart 默认 120s，
# 慢启动/慢停止服务可据此调大（查询 status 仍为 10s，不受影响）
# export SUPERVISOR_TIMEOUT="300"
```

### 配置示例

```bash
# 本地默认配置（先初始化RPC）
sudo ./sv init
./sv status

# 远程服务器配置
SUPERVISOR_HOST="http://192.168.1.100:9001/RPC2" ./sv status

# 带认证的配置
SUPERVISOR_HOST="http://remote-server:9001/RPC2" \
SUPERVISOR_USER="admin" \
SUPERVISOR_PASSWORD="secret123" \
./sv status
```

### 智能配置检测

工具通过显式命令管理Supervisor配置：
- **初始化**: `sv init` 扫描现有配置并补齐必要的RPC和HTTP服务器配置，然后重启已有的Supervisor服务
- **预览变更**: `sv configure rpc --dry-run` 只显示待补齐的配置，不修改文件
- **显式配置**: `sv configure rpc` 只修改配置，`--restart` 时才重启已有的Supervisor服务
- **连接回退**: 本地RPC不可用时，进程查询和控制可回退到`supervisorctl`

### 必需的Supervisor配置

确保`supervisord.conf`包含以下配置：

```ini
[inet_http_server]
port=127.0.0.1:9001
username=user
password=pass

[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

```

## 🔧 Supervisor初始化

```bash
# 补齐RPC配置并重启已有的Supervisor服务
sudo ./sv init

# sv本身不会常驻；Supervisor的服务由系统原有的init/procd/systemd管理
```

## 📊 状态说明

| 状态代码 | 状态名称 | 颜色 | 图标 |
|----------|----------|------|------|
| 20 | RUNNING | 绿色 | ✅ 运行中 |
| 10 | STARTING | 黄色 | 🚀 启动中 |
| 30 | STOPPING | 黄色 | ⏹️ 停止中 |
| 0 | STOPPED | 白色 | ⏸️ 已停止 |
| 100 | FATAL | 红色 | ❌ 致命错误 |
| 200 | BACKOFF | 黄色 | ⚠️ 重试中 |

## 🌟 输出示例

### 进程状态显示（最新版本）

```
🔍 Supervisor进程状态 (共5个进程)
┌──────┬──────────────────────┬─────────┬─────────┬─────────────────┐
│ 序号 │         名称         │  状态   │   PID   │    运行时间     │
├──────┼──────────────────────┼─────────┼─────────┼─────────────────┤
│ 1    │ agent:agent_00       │ RUNNING │ 3836860 │ 4小时50分钟06秒 │
│ 2    │ hysteria:hysteria_00 │ RUNNING │ 3870703 │ 2小时07分钟33秒 │
│ 3    │ iperf3:iperf3_00     │ RUNNING │ 3870876 │ 2小时07分钟18秒 │
│ 4    │ ss:ss_00             │ RUNNING │ 3870884 │ 2小时07分钟17秒 │
│ 5    │ xray8:xray8_00       │ RUNNING │ 3835299 │ 4小时56分钟50秒 │
└──────┴──────────────────────┴─────────┴─────────┴─────────────────┘

💡 提示: 使用 'sv start/stop/restart <序号>' 来控制进程
🔧 配置: 设置SUPERVISOR_HOST环境变量来指定Supervisor地址
```

### 智能配置检测输出

```
🔧 检测Supervisor配置...
✅ 找到配置文件: /etc/supervisor/supervisord.conf
⚠️  缺少[inet_http_server]配置
🛠️  正在添加RPC配置...
✅ 配置更新成功
🔄 正在重启Supervisor服务...
✅ Supervisor服务重启成功
```

## 🎯 使用场景

### 日常运维

```bash
# 快速查看所有服务状态
./sv status

# 重启web服务（序号1）
./sv restart 1

# 重启所有数据库服务（序号5-7）
./sv restart 5-7

# 查看特定服务状态
./sv status | grep nginx
```

### 批量操作

```bash
# 停止所有测试服务
./sv stop 8 10 12

# 启动所有核心服务
./sv start 1-4

# 重启多个指定服务
./sv restart 1 3 5-7 nginx

# 停止所有服务（使用范围）
./sv stop 1-20
```

### 问题排查

```bash
# 查看状态异常的服务
./sv status

# 重启问题服务
./sv restart 3

# 查看服务详细信息
./sv status
```

### 远程服务器管理

```bash
# 连接远程Supervisor服务器
SUPERVISOR_HOST="http://192.168.1.100:9001/RPC2" ./sv status

# 带认证的远程连接
SUPERVISOR_HOST="http://remote-server:9001/RPC2" \
SUPERVISOR_USER="admin" \
SUPERVISOR_PASSWORD="secret123" \
./sv restart 1-5
```

### Supervisor服务

```bash
# 初始化RPC并启动已有的Supervisor服务
sudo ./sv init
```

## 🛠️ 开发

### 开发环境设置

```bash
# 克隆仓库
git clone https://github.com/x1t/sv.git
cd sv

# 安装依赖
go mod tidy

# 开发运行
go run main.go status

# 运行所有测试
go test -v

# 按包运行测试
go test -v ./pkg/cli/
go test -v ./pkg/supervisor/
go test -v ./pkg/utils/

# 运行特定测试函数
go test -run TestMain_Help
go test -run TestProcessControl
go test -run TestConfigDetector

# 生成覆盖率报告
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# 基准测试
go test -bench=. -benchmem

# 依赖管理
go mod tidy      # 整理依赖
go mod verify    # 验证依赖
go mod graph     # 查看依赖图
```

### 编译选项

```bash
# 开发环境运行
go run main.go <command>

# 生产编译（减小二进制文件大小）
go build -ldflags="-s -w" -o sv main.go

# 使用构建脚本（输出到dist目录）
./build.sh

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o sv-linux-amd64 main.go
GOOS=windows GOARCH=amd64 go build -o sv-windows-amd64.exe main.go
GOOS=darwin GOARCH=amd64 go build -o sv-darwin-amd64 main.go

# 其他平台
GOOS=freebsd GOARCH=amd64 go build -o sv-freebsd-amd64 main.go
GOOS=openbsd GOARCH=amd64 go build -o sv-openbsd-amd64 main.go
```

### 依赖管理

```bash
# 整理依赖
go mod tidy

# 验证依赖
go mod verify

# 查看依赖图
go mod graph
```

### 项目结构详解

```
pkg/
├── cli/                   # CLI应用层
│   ├── app.go            # 主应用逻辑（84行）
│   └── renderer.go       # 渲染和格式化（116行）
├── supervisor/            # Supervisor核心功能
│   ├── rpc_client.go     # XML-RPC客户端（335行）
│   ├── types.go          # 数据结构定义（96行）
│   ├── config_detector.go # 配置检测和自动配置（299行）
│   └── process_control.go # 进程控制逻辑（119行）
└── utils/                 # 工具函数
    └── common.go         # 通用工具函数和数据结构（499行）
```

## 🔍 故障排除

### 连接问题

如果遇到连接错误：

1. 确保Supervisor服务正在运行
2. 检查端口配置（默认9001）
3. 确认防火墙设置
4. 验证认证信息
5. 首次使用或修改配置后执行`sudo ./sv init`

### 配置问题

工具会自动处理大部分配置问题：
- **检测现有配置**: 自动扫描Supervisor配置文件
- **添加缺失配置**: 自动添加RPC和HTTP服务器配置
- **配置验证**: 验证配置文件的正确性
- **优雅降级**: 配置失败时回退到命令行模式

### 双模式架构

工具采用智能双模式架构：
- **RPC模式**: 优先使用XML-RPC通信，性能更佳
- **命令行模式**: 自动回退到supervisorctl命令，确保兼容性
- **智能切换**: 透明模式切换，用户无感知

### 手动配置

如果自动配置失败，可以手动配置`supervisord.conf`：

```ini
[inet_http_server]
port=127.0.0.1:9001
username=user
password=pass

[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

```

### Supervisor启动问题

```bash
# 初始化配置并启动已有的Supervisor服务
sudo ./sv init

# Linux/OpenWrt上请查看Supervisor自身的服务日志
```

## 🤝 贡献

欢迎提交Issue和Pull Request！

### 开发指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

### 代码规范

- 遵循Go语言代码规范
- 所有用户界面使用中文
- 核心功能必须有测试覆盖
- 保持模块化架构设计
- 新功能应围绕简化Supervisor管理的核心价值

## 🔗 相关链接

- [Supervisor官方文档](http://supervisord.org/)
- [Go语言官网](https://golang.org/)
- [问题反馈](https://github.com/x1t/sv/issues)
- [项目主页](https://github.com/x1t/sv)

## 📄 许可证

MIT License

---

**享受便捷的进程管理体验！** 🎉

### 核心价值

记住这个工具的强大之处在于：
- **序号化操作** - 让进程管理更加高效
- **智能配置** - 自动检测和配置环境
- **完美显示** - 美观的表格和精确的时间格式
- **双模式架构** - 确保在各种环境下都能正常工作

让复杂的Supervisor进程管理变得简单高效！✨
