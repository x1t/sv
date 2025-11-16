package cli

import (
	"fmt"
	"os"

	"sv/pkg/supervisor"
)

// CLIApp 负责整个CLI应用的运行逻辑
type CLIApp struct {
	renderer *CLIRenderer
}

// NewCLIApp 创建新的CLI应用
func NewCLIApp() *CLIApp {
	return &CLIApp{
		renderer: NewCLIRenderer(),
	}
}

// Run 程序运行逻辑
func (app *CLIApp) Run() error {
	if len(os.Args) < 2 {
		app.renderer.PrintUsage()
		return nil
	}

	command := os.Args[1]
	args := os.Args[2:]

	// 检查是否是service子命令
	if command == "service" {
		sm := supervisor.NewServiceManager()
		sm.HandleServiceCommand(args)
		return nil
	}

	// 对于与Supervisor交互的命令，检测并开启RPC功能
	if command == "status" || command == "list" || command == "start" || command == "stop" || command == "restart" {
		// 尝试检测并开启RPC功能
		cd := supervisor.NewConfigDetector()
		err := cd.DetectAndEnableRPC()
		if err != nil {
			fmt.Printf("⚠️  检测/开启RPC功能时出错: %v\n", err)
			// 继续执行，因为可能RPC已在其他地方配置，或者会回退到命令行模式
		}
	}

	// 读取Supervisor连接配置
	cd := supervisor.NewConfigDetector()
	host, username, password := cd.ReadSupervisorConfig()

	// 创建Supervisor客户端
	client := supervisor.NewRPCClient(host, username, password)

	switch command {
	case "status", "list":
		app.renderer.ShowStatus(client)
	case "start", "stop", "restart":
		if len(args) == 0 {
			fmt.Printf("用法: sv %s <进程序号|进程名称|范围>\n", command)
			fmt.Println("示例:")
			fmt.Printf("  sv %s 1        # 控制序号为1的进程\n", command)
			fmt.Printf("  sv %s myapp    # 控制名为myapp的进程\n", command)
			fmt.Printf("  sv %s 1 3 5   # 控制多个进程\n", command)
			fmt.Printf("  sv %s 1-5     # 控制序号1到5的进程\n", command)
			return fmt.Errorf("参数不足")
		}
		app.renderer.ControlProcesses(client, command, args)
	case "daemon":
		// 守护进程模式，由系统服务管理器调用
		sm := supervisor.NewServiceManager()
		sm.RunServiceDaemon()
	case "help", "-h", "--help":
		app.renderer.PrintUsage()
	default:
		fmt.Printf("未知命令: %s\n\n", command)
		app.renderer.PrintUsage()
		return fmt.Errorf("未知命令: %s", command)
	}

	return nil
}