# CLAUDE.md - Supervisor进程管理工具开发指南

## 🎯 项目概述

这是一个基于Go语言开发的现代化Supervisor进程管理工具，提供序号化的进程管理界面，支持远程操作和美观的状态显示。

### 核心特性
- 🎯 **序号操作**: 使用数字序号代替长进程名，操作更快速
- 📊 **美观显示**: 彩色状态显示，运行时间格式化
- 🔧 **灵活控制**: 支持单个、多个、范围操作
- 🌐 **远程管理**: 支持认证远程Supervisor服务器
- 💡 **智能提示**: 友好的错误提示和使用说明

## 📁 项目架构

### 文件结构
```
sv/
├── main.go                    # 主程序文件（609行）
├── main_test.go              # 主函数测试套件
├── process_control_test.go   # 进程控制测试
├── supervisor_test.go        # Supervisor相关测试
├── utils_test.go             # 工具函数测试
├── go.mod                    # Go模块依赖
├── go.sum                    # 依赖校验
├── .gitignore                # Git忽略规则
└── README.md                 # 项目文档
```

### 核心架构组件

#### 1. SupervisorClient (64-80行)
- **职责**: XML-RPC客户端，与Supervisor通信
- **特性**: 支持认证、超时控制、错误处理
- **方法**: `call()`, `GetAllProcesses()`, `ControlProcess()`

#### 2. 数据模型
- **ProcessInfo**: 进程信息结构（52-62行）
- **MethodCall/MethodResponse**: XML-RPC数据结构（17-49行）
- **Fault**: 错误处理结构（40-49行）

#### 3. 业务逻辑层
- **状态获取**: `GetAllProcesses()` (162-181行)
- **进程控制**: `ControlProcess()` (304-337行)
- **输出解析**: `parseSupervisorctlOutput()` (220-281行)

#### 4. 展示层
- **状态显示**: `DisplayStatus()` (340-364行)
- **颜色控制**: `getColorByState()` (367-380行)
- **图标系统**: `getStateIcon()` (383-400行)

## 🛠️ 开发规范

### 代码风格
- **文件组织**: 单文件架构，所有功能在main.go中
- **函数命名**: 使用驼峰命名法，清晰表达功能
- **错误处理**: 统一使用`fmt.Errorf()`，提供中文错误信息
- **注释风格**: 使用中文注释，说明函数用途和参数

### 错误处理模式
```go
// 标准错误处理模式
func someFunction() error {
    if err != nil {
        return fmt.Errorf("操作失败: %v", err)
    }
    return nil
}
```

### 输出格式规范
- **状态信息**: 使用中文emoji图标（✅🚀⏹️⏸️❌⚠️）
- **表格显示**: 统一的对齐格式（80字符宽度）
- **颜色编码**: 
  - RUNNING: 绿色 (\x1b[32m)
  - STARTING/STOPPING: 黄色 (\x1b[33m)
  - FATAL: 红色 (\x1b[31m)

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
```

### 测试运行
```bash
# 运行所有测试
go test -v

# 运行特定测试
go test -run TestMain_Help

# 生成覆盖率报告
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 🔧 环境配置

### 开发环境设置
```bash
# 设置Go 1.21+
go version

# 安装测试依赖
go mod tidy

# 配置环境变量（可选）
export SUPERVISOR_HOST="http://localhost:9001/RPC2"
export SUPERVISOR_USER="username"
export SUPERVISOR_PASSWORD="password"
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

## 📋 功能开发指南

### 添加新命令
1. 在`main()`函数中添加case分支
2. 实现对应的处理函数
3. 添加测试用例
4. 更新帮助信息

### 添加新状态类型
1. 更新`getStateValue()`函数
2. 添加对应的颜色代码
3. 更新`getStateIcon()`函数
4. 添加测试验证

### 修改XML-RPC协议
1. 更新`MethodCall/MethodResponse`结构
2. 修改`call()`方法的序列化逻辑
3. 更新错误处理逻辑

## 🎨 输出格式规范

### 进程状态显示
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

### 操作反馈
- **成功**: ✅ 成功 / 🚀 启动 / ⏹️ 停止 / 🔄 重启
- **失败**: ❌ 失败 (错误信息)
- **提示**: 💡 提示信息 / 🔧 配置信息

## 🌟 最佳实践

### 新功能开发
1. **先写测试**: 使用testify框架编写测试用例
2. **保持单文件**: 维持单文件架构，不拆分模块
3. **中文优先**: 所有用户界面使用中文
4. **错误容错**: 提供降级处理（如演示模式）

### 性能优化
1. **HTTP客户端**: 复用HTTP客户端，避免连接泄露
2. **超时控制**: 设置合理的超时时间（10秒）
3. **内存管理**: 及时关闭资源（response body）
4. **错误处理**: 避免panic，使用error返回

### 代码质量
1. **测试覆盖**: 确保核心功能有测试覆盖
2. **错误处理**: 所有函数都应正确处理错误
3. **代码复用**: 避免重复代码，提取公共函数
4. **文档完善**: 为复杂函数添加注释

## 🚨 注意事项

### 环境依赖
- Go 1.21+ 是必须的
- 需要Supervisor服务支持
- XML-RPC接口必须启用

### 兼容性
- 支持Supervisor 3.x+
- 支持Linux/Windows平台
- 向后兼容旧版本命令格式

### 安全考虑
- 密码通过环境变量传递
- 支持Basic认证
- 不存储敏感信息在配置文件中

---

**记住**: 这个工具的核心价值在于**简化Supervisor管理**，所有功能都应围绕这个核心价值展开。开发新功能时要考虑是否真正提升了用户体验。