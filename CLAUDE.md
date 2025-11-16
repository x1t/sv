# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🎯 项目概述

这是一个基于Go语言开发的现代化Supervisor进程管理工具，采用清晰的模块化架构。该工具通过序号化操作简化进程管理，支持智能配置检测、远程管理和系统服务集成。

### 核心价值
- **序号操作**: 使用数字序号代替长进程名，提升运维效率
- **智能双模式**: RPC优先，命令行模式回退
- **自动配置**: 智能检测和配置Supervisor环境
- **中文界面**: 完全中文化的用户体验

## 🏗️ 核心架构

### 极简入口 (main.go - 23行)
```go
func main() {
    app := cli.NewCLIApp()
    err := app.Run()
    if err != nil {
        os.Exit(1)
    }
}
```
从1349行重构到23行，体现单一职责原则。

### 三层模块化架构

#### 1. CLI应用层 (`pkg/cli/`)
- **`app.go`**: CLI应用逻辑和命令分发
- **`renderer.go`**: 用户界面渲染、输出格式化、交互提示
- **核心职责**: 命令解析、参数验证、用户交互

#### 2. 业务逻辑层 (`pkg/supervisor/`)
- **`rpc_client.go`**: XML-RPC客户端，支持认证、超时、错误处理
- **`config_detector.go`**: 智能配置检测和自动配置Supervisor
- **`service_manager.go`**: 系统服务管理，跨平台守护进程
- **`process_control.go`**: 进程控制逻辑（启动/停止/重启）
- **`types.go`**: XML-RPC数据结构定义
- **核心职责**: Supervisor通信、进程管理、配置管理

#### 3. 工具函数层 (`pkg/utils/`)
- **`common.go`**: ProcessInfo数据结构、状态显示格式化、参数解析
- **核心职责**: 通用工具、数据结构、显示格式化

### 双模式通信架构
- **RPC模式**: 优先使用XML-RPC，高性能通信
- **命令行模式**: 自动回退到`supervisorctl`命令，确保兼容性
- **智能切换**: 透明模式切换，用户无感知

## 🚀 常用开发命令

### 构建和运行
```bash
# 开发环境运行
go run main.go status
go run main.go restart 1

# 生产构建
go build -ldflags="-s -w" -o sv main.go

# 使用构建脚本（输出到dist目录）
./build.sh

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o sv-linux-amd64 main.go
GOOS=windows GOARCH=amd64 go build -o sv-windows-amd64.exe main.go
GOOS=darwin GOARCH=amd64 go build -o sv-darwin-amd64 main.go
```

### 测试执行
```bash
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

## 🔧 开发环境配置

### 必需环境
- Go 1.23.0+ (go.mod中明确指定)
- Supervisor 3.x+ (功能依赖)

### 环境变量
```bash
# Supervisor RPC连接
export SUPERVISOR_HOST="http://localhost:9001/RPC2"
export SUPERVISOR_USER="username"
export SUPERVISOR_PASSWORD="password"

# 开发调试
export SV_DEBUG=true
export SV_LOG_LEVEL=debug
```

### Supervisor配置要求
确保`supervisord.conf`包含：
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

## 📋 开发指南

### 添加新CLI命令
1. 在`pkg/cli/app.go`的`Run()`方法中添加命令处理逻辑
2. 在`pkg/cli/renderer.go`中添加渲染方法
3. 更新帮助信息
4. 添加测试用例

### 扩展Supervisor功能
1. 在`pkg/supervisor/`包中添加功能模块
2. 更新`pkg/supervisor/types.go`数据结构
3. 在`pkg/supervisor/rpc_client.go`中添加RPC方法
4. 编写对应测试

### 进程参数解析
支持多种操作格式：
```bash
./sv restart 1           # 序号
./sv restart nginx       # 名称  
./sv restart 1 3 5       # 多个
./sv restart 1-5         # 范围
./sv restart 1 nginx 3-5 # 混合
```

### 符号链接功能
- 安装服务时自动创建 `/usr/local/bin/sv` 符号链接
- 权限检测机制，自动处理权限不足情况
- 卸载服务时自动移除符号链接
- 跨平台兼容（仅Unix/Linux系统）

## 🎨 UI/UX 规范

### 状态显示系统
- 使用`tablewriter`库实现美观表格显示
- Unicode直线边框（与PM2保持一致），表头居中，数据左对齐
- 彩色状态显示 + 直观图标系统
- 支持中文字符宽度对齐（`WithTrimSpace(tw.Off)`配置）

### 状态编码规范
- **RUNNING (20)**: 绿色 + ✅
- **STARTING (10)**: 黄色 + 🚀  
- **STOPPING (30)**: 黄色 + ⏹️
- **STOPPED (0)**: 白色 + ⏸️
- **FATAL (100)**: 红色 + ❌
- **BACKOFF (200)**: 黄色 + ⚠️

### 错误处理
- 所有用户界面使用中文
- 提供详细错误信息和解决建议
- 统一的错误处理模式，使用`fmt.Errorf()`

## 🔒 安全注意事项

### 输入验证
- 严格进程名称验证，防止命令注入
- 参数类型和范围验证
- 特殊字符过滤

### 认证支持
- Basic认证支持
- 环境变量密码管理
- 不在配置文件中存储敏感信息

## 🌟 依赖管理

### 直接依赖
- `github.com/kardianos/service v1.2.4` - 系统服务管理
- `github.com/olekukonko/tablewriter v1.1.2` - 表格渲染
- `github.com/stretchr/testify v1.11.1` - 测试框架

### 构建优化
使用`-ldflags="-s -w"`标志减小二进制文件大小

## 🚨 重要提醒

### 文件忽略规则
当前项目中的以下文件夹已被忽略：
- `sv` - 编译后的可执行文件
- `supervisor/` - 意外下载的依赖包
- `tablewriter/` - 意外下载的依赖包

### 模块化开发原则
- 新功能必须添加到对应包中，保持架构清晰
- 核心功能必须有完整测试覆盖
- 保持向后兼容性

### 核心价值导向
所有功能开发应围绕**简化Supervisor管理**和**提升运维效率**的核心价值展开。

---

记住：这个工具的强大之处在于**序号化操作**和**智能配置**，让复杂的进程管理变得简单高效！