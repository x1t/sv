package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/x1t/sv/pkg/supervisor"
	"github.com/x1t/sv/pkg/utils"
)

// CLIRenderer 负责命令行界面的渲染和交互
type CLIRenderer struct{}

// NewCLIRenderer 创建新的命令行界面渲染器
func NewCLIRenderer() *CLIRenderer {
	return &CLIRenderer{}
}

// ShowStatus 显示Supervisor进程状态
func (cr *CLIRenderer) ShowStatus(client *supervisor.RPCClient) {
	processes, err := client.GetAllProcesses()
	if err != nil {
		fmt.Printf("⚠️  获取进程状态失败: %v\n", err)
		fmt.Println("这是演示模式，显示模拟数据:")
		processes, _ = client.GetAllProcesses()
	}

	fmt.Printf("\n🔍 Supervisor进程状态 (共%d个进程)\n", len(processes))
	fmt.Println(strings.Repeat("=", 80))
	utils.DisplayStatus(processes)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\n💡 提示: 使用 'sv start/stop/restart <序号>' 来控制进程")
	fmt.Println("🔧 配置: 设置SUPERVISOR_HOST环境变量来指定Supervisor地址")
}

// ControlProcesses 控制多个进程（启动/停止/重启）
func (cr *CLIRenderer) ControlProcesses(client *supervisor.RPCClient, action string, args []string) {
	// 首先获取所有进程信息
	processes, err := client.GetAllProcesses()
	if err != nil {
		fmt.Printf("⚠️  获取进程信息失败: %v\n", err)
		fmt.Println("这是演示模式，将使用模拟数据:")
		processes, _ = client.GetAllProcesses()
	}

	// 解析进程名称
	processNames, err := utils.ParseProcessIndices(args, processes)
	if err != nil {
		fmt.Printf("❌ 解析进程参数失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🎯 正在执行 '%s' 操作...\n", action)

	// 初始化进程控制器
	ctrl := supervisor.NewProcessController()

	// 执行控制操作
	var successCount, failCount int
	for _, name := range processNames {
		fmt.Printf("  %s 进程 %s ... ", utils.GetActionIcon(action), name)
		err := ctrl.ControlProcess(action, name)
		if err != nil {
			fmt.Printf("❌ 失败 (%v)\n", err)
			failCount++
		} else {
			fmt.Printf("✅ 成功\n")
			successCount++
		}
	}

	fmt.Printf("\n📊 操作完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)

	if failCount > 0 {
		fmt.Println("💡 提示: 请确保Supervisor正在运行并且配置正确")
	}
}

// PrintUsage 打印使用说明
func (cr *CLIRenderer) PrintUsage() {
	fmt.Println("sv - Supervisor进程管理工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  sv status                    # 显示所有进程状态")
	fmt.Println("  sv list                     # 显示所有进程状态（同status）")
	fmt.Println("  sv start <进程>              # 启动进程")
	fmt.Println("  sv stop <进程>               # 停止进程")
	fmt.Println("  sv restart <进程>            # 重启进程")
	fmt.Println("  sv service <action>          # 服务管理")
	fmt.Println()
	fmt.Println("进程参数支持:")
	fmt.Println("  序号      sv restart 1       # 使用序号")
	fmt.Println("  名称      sv restart myapp   # 使用进程名")
	fmt.Println("  多个      sv restart 1 3 5   # 多个进程")
	fmt.Println("  范围      sv restart 1-5     # 序号范围")
	fmt.Println()
	fmt.Println("服务管理:")
	fmt.Println("  install   安装sv为系统服务")
	fmt.Println("  uninstall 卸载sv系统服务")
	fmt.Println("  start     启动sv系统服务")
	fmt.Println("  stop      停止sv系统服务")
	fmt.Println("  restart   重启sv系统服务")
	fmt.Println("  status    查看sv服务状态")
	fmt.Println()
	fmt.Println("环境变量:")
	fmt.Println("  SUPERVISOR_HOST              # Supervisor RPC地址 (默认: http://localhost:9001/RPC2)")
	fmt.Println("  SUPERVISOR_USER              # 用户名 (可选)")
	fmt.Println("  SUPERVISOR_PASSWORD          # 密码 (可选)")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  sv status                    # 查看所有进程状态")
	fmt.Println("  sv list                      # 查看所有进程状态（同status）")
	fmt.Println("  sv restart 1                 # 重启序号为1的进程")
	fmt.Println("  sv stop 2 4 6               # 停止序号2、4、6的进程")
	fmt.Println("  sv start 1-3                # 启动序号1到3的进程")
	fmt.Println("  sv restart myapp nginx      # 重启指定名称的进程")
	fmt.Println("  sv service install           # 安装为系统服务")
	fmt.Println("  sv service start             # 启动系统服务")
}