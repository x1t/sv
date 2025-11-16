# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🎯 项目概述

这是一个基于Go语言开发的现代化Supervisor进程管理工具，已经重构为清晰的模块化架构。该工具提供序号化的进程管理界面，支持远程操作、系统服务管理和智能配置检测。

### 核心特性
- 🎯 **序号操作**: 使用数字序号代替长进程名，操作更快速
- 📊 **美观显示**: 彩色状态显示，运行时间格式化
- 🔧 **灵活控制**: 支持单个、多个、范围操作
- 🌐 **远程管理**: 支持认证远程Supervisor服务器
- 🛠️ **智能配置**: 自动检测和配置Supervisor RPC功能
- 🔧 **系统服务**: 支持将工具自身安装为系统服务

## 📁 项目架构

### 模块化架构
项目采用分层模块化设计，职责清晰分离：

```
sv/
├── main.go                    # 主程序入口（23行）
├── pkg/                       # 核心包目录
│   ├── cli/                   # CLI应用层
│   │   ├── app.go            # CLI应用逻辑（84行）
│   │   └── renderer.go       # 渲染器（120行）
│   ├── supervisor/            # Supervisor核心功能
│   │   ├── rpc_client.go     # XML-RPC客户端（319行）
│   │   ├── types.go          # 数据结构定义（97行）
│   │   ├── config_detector.go # 配置检测器（300行）
│   │   ├── service_manager.go # 系统服务管理（304行）
│   │   └── process_control.go # 进程控制（120行）
│   └── utils/                 # 工具函数
│       └── common.go         # 通用工具函数（388行）
├── main_test.go              # 主函数测试（434行）
├── process_control_test.go   # 进程控制测试（254行）
├── supervisor_test.go        # Supervisor相关测试（337行）
├── utils_test.go             # 工具函数测试（331行）
├── service_test.go           # 服务管理测试（18行）
├── go.mod                    # Go模块依赖
├── go.sum                    # 依赖校验
└── README.md                 # 项目文档
```

### 架构层级

#### 1. **入口层 (main.go)**
- **职责**: 简化的程序入口，初始化并启动CLI应用
- **特点**: 从1349行重构到23行，极简化设计

#### 2. **CLI应用层 (pkg/cli/)**
- **CLIApp**: 整个CLI应用的运行逻辑和命令分发
- **CLIRenderer**: 命令行界面渲染、用户交互和输出格式化
- **命令支持**: status、list、start、stop、restart、service等

#### 3. **业务逻辑层 (pkg/supervisor/)**
- **RPCClient**: XML-RPC客户端，支持认证、超时、错误处理
- **ConfigDetector**: 智能检测和配置Supervisor RPC功能
- **ProcessController**: 进程控制，支持启动/停止/重启操作
- **ServiceManager**: 系统服务管理，支持守护进程模式

#### 4. **工具函数层 (pkg/utils/)**
- **ProcessInfo**: 进程信息数据结构和显示格式化
- **DisplayStatus**: 状态显示和表格格式化
- **ParseProcessIndices**: 进程参数解析（序号、名称、范围）
- **ParseSupervisorctlOutput**: supervisorctl命令输出解析

## 🚀 构建和测试

### 构建命令
```bash
# 开发构建
go run main.go <command>

# 生产构建
go build -ldflags="-s -w" -o sv main.go

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o sv-linux-amd64 main.go
GOOS=windows GOARCH=amd64 go build -o sv-windows-amd64.exe main.go
GOOS=darwin GOARCH=amd64 go build -o sv-darwin-amd64 main.go
```

### 测试运行
```bash
# 运行所有测试
go test -v

# 运行特定包的测试
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
```

### 依赖管理
```bash
# 整理依赖
go mod tidy

# 查看依赖图
go mod graph

# 验证依赖
go mod verify
```

## 🔧 开发环境配置

### 环境要求
- Go 1.23.0+ （必需，go.mod中明确指定）
- Supervisor 3.x+ （功能依赖）
- 标准Unix工具（用于系统服务管理）

### 环境变量配置
```bash
# Supervisor RPC连接配置
export SUPERVISOR_HOST="http://localhost:9001/RPC2"
export SUPERVISOR_USER="username"
export SUPERVISOR_PASSWORD="password"

# 开发调试
export SV_DEBUG=true
export SV_LOG_LEVEL=debug
```

### Supervisor配置要求
确保`supervisord.conf`包含以下配置：
```ini
[inet_http_server]
port=127.0.0.1:9001
username=user
password=pass

[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

[supervisord]
rpcinterface_files = supervisord
```

## 📋 功能开发指南

### 添加新的CLI命令
1. 在`pkg/cli/app.go`的`Run()`方法中添加命令处理逻辑
2. 在`pkg/cli/renderer.go`中添加相应的渲染方法
3. 创建对应的测试文件或更新现有测试
4. 更新帮助信息和使用说明

### 扩展Supervisor功能
1. 在`pkg/supervisor/`包中添加新的功能模块
2. 更新`pkg/supervisor/types.go`中的数据结构
3. 在`pkg/supervisor/rpc_client.go`中添加RPC调用方法
4. 编写对应的测试用例

### 添加新的工具函数
1. 在`pkg/utils/common.go`中添加通用工具函数
2. 确保函数具有良好的错误处理和参数验证
3. 添加对应的单元测试
4. 更新文档注释

### 系统服务功能开发
1. 在`pkg/supervisor/service_manager.go`中扩展服务管理功能
2. 使用`kardianos/service`库实现跨平台服务集成
3. 添加服务配置和生命周期管理
4. 测试不同平台的服务安装和运行

## 🛠️ 核心功能模块

### 双模式架构
- **RPC模式**: 优先使用XML-RPC与Supervisor通信，性能更好
- **命令行模式**: RPC失败时自动回退到supervisorctl命令，确保兼容性
- **智能切换**: 自动检测可用模式并透明切换

### 进程参数解析
支持多种参数格式，增强用户体验：
- **序号**: `./sv restart 1`
- **名称**: `./sv restart nginx`
- **多个**: `./sv restart 1 3 5`
- **范围**: `./sv restart 1-5`
- **混合**: `./sv restart 1 nginx 3-5`

### 智能配置检测
- **自动检测**: 检测现有Supervisor配置
- **缺失配置**: 自动添加必要的RPC和HTTP服务器配置
- **配置验证**: 验证配置正确性和可用性
- **错误恢复**: 配置失败时的优雅降级

### 系统服务集成
- **服务安装**: 将sv工具安装为系统服务
- **守护进程**: 支持后台运行模式
- **跨平台**: 支持Linux systemd、Windows服务等
- **生命周期**: 完整的服务启动、停止、重启管理

## 🎨 输出格式规范

### 进程状态显示格式
```
🔍 Supervisor进程状态 (共4个进程)
================================================================================
序号   名称                   状态         PID      运行时间            描述
--------------------------------------------------------------------------------
1    nginx                RUNNING    1234     2小时15分          ✅ 运行中
2    redis                RUNNING    5678     1天3小时           ✅ 运行中
3    mysql                STOPPED    -        已停止             ⏸️ 已停止
4    app                  STARTING   -        启动中...          🚀 启动中
================================================================================
```

### 状态编码和颜色
- **RUNNING (20)**: 绿色 (\x1b[32m) + ✅
- **STARTING (10)**: 黄色 (\x1b[33m) + 🚀
- **STOPPING (30)**: 黄色 (\x1b[33m) + ⏹️
- **STOPPED (0)**: 白色 + ⏸️
- **FATAL (100)**: 红色 (\x1b[31m) + ❌
- **BACKOFF (200)**: 黄色 (\x1b[33m) + ⚠️

### 操作反馈格式
- **成功操作**: ✅ 成功 / 🚀 启动成功 / ⏹️ 停止成功 / 🔄 重启成功
- **失败操作**: ❌ 失败 (详细错误信息)
- **提示信息**: 💡 使用提示 / 🔧 配置信息 / ⚠️ 警告信息

## 🔒 安全性和错误处理

### 输入验证
- 严格的进程名称验证，防止命令注入攻击
- 特殊字符过滤和边界检查
- 参数类型和范围验证

### 认证支持
- Basic认证支持
- 环境变量密码管理
- 不在配置文件中存储敏感信息

### 错误处理策略
- 统一的错误处理模式，使用`fmt.Errorf()`提供中文错误信息
- 优雅的降级处理（RPC失败时回退到命令行模式）
- 详细的错误日志和调试信息

### 并发安全
- HTTP客户端复用，避免连接泄露
- 合理的超时控制（默认10秒）
- 资源及时释放（response body关闭）

## 🌟 依赖管理

### 直接依赖
- `github.com/stretchr/testify v1.11.1` - 测试框架
- `github.com/kardianos/service v1.2.4` - 系统服务管理

### 间接依赖
- `golang.org/x/sys v0.34.0` - 系统调用
- `gopkg.in/yaml.v3 v3.0.1` - YAML解析
- `github.com/davecgh/go-spew v1.1.1` - 测试数据格式化
- `github.com/pmezard/go-difflib v1.0.0` - 测试差异比较

## 🚨 注意事项

### 开发注意事项
- **模块化架构**: 新功能应该添加到对应的包中，保持架构清晰
- **中文优先**: 所有用户界面使用中文，错误信息提供中文说明
- **向后兼容**: 保持命令行接口的向后兼容性
- **测试驱动**: 核心功能必须有对应的测试用例

### 部署注意事项
- **权限要求**: 系统服务功能需要适当的系统权限
- **平台兼容**: 注意不同平台的路径和权限差异
- **配置安全**: 确保Supervisor配置的安全性
- **日志管理**: 合理配置日志级别和输出位置

---

**记住**: 这个工具的核心价值在于**简化Supervisor管理**和**提升运维效率**。所有新功能开发都应该围绕这个核心价值展开，确保真正提升了用户体验。