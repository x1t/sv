package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/x1t/sv/pkg/supervisor"
)

// CLIApp 负责整个 CLI 应用的运行逻辑。
type CLIApp struct {
	renderer *CLIRenderer
}

// NewCLIApp 创建使用系统标准流的 CLI 应用。
func NewCLIApp() *CLIApp {
	return NewCLIAppWithWriters(os.Stdout, os.Stderr)
}

// NewCLIAppWithWriters 创建可注入输出流的 CLI 应用，便于集成和测试。
func NewCLIAppWithWriters(out, errOut io.Writer) *CLIApp {
	return &CLIApp{renderer: NewCLIRenderer(out, errOut)}
}

// Run 使用 os.Args 执行 CLI。
func (app *CLIApp) Run() error {
	return app.RunArgs(os.Args[1:])
}

// RunArgs 执行传入的命令参数。
func (app *CLIApp) RunArgs(args []string) error {
	if app == nil || app.renderer == nil {
		return fmt.Errorf("CLI应用未初始化")
	}
	if len(args) == 0 {
		app.renderer.PrintUsage()
		return nil
	}

	command := strings.ToLower(strings.TrimSpace(args[0]))
	commandArgs := args[1:]
	switch command {
	case "help", "-h", "--help":
		app.renderer.PrintUsage()
		return nil
	case "init":
		if len(commandArgs) != 0 {
			return fmt.Errorf("init不接受额外参数")
		}
		return app.initSupervisor()
	case "configure":
		return app.configure(commandArgs)
	case "status", "list", "ls":
		if len(commandArgs) != 0 {
			return fmt.Errorf("%s不接受额外参数", command)
		}
		return app.renderer.ShowStatus(app.newSupervisorClient())
	case "start", "stop", "restart":
		if len(commandArgs) == 0 {
			return fmt.Errorf("用法: sv %s <进程序号|进程名称|范围>", command)
		}
		return app.renderer.ControlProcesses(app.newSupervisorClient(), command, commandArgs)
	default:
		app.renderer.PrintUsage()
		return fmt.Errorf("未知命令: %s", command)
	}
}

// initSupervisor 一次性准备并启动 Supervisor 的 RPC 服务。
// sv 本身不常驻,真正的后台进程始终是 supervisord。
func (app *CLIApp) initSupervisor() error {
	detector := supervisor.NewConfigDetector()
	message, err := detector.InitializeSupervisor()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(app.renderer.out, message); err != nil {
		return err
	}
	_, err = fmt.Fprintln(app.renderer.out, "✅ Supervisor RPC 初始化成功")
	return err
}

func (app *CLIApp) newSupervisorClient() *supervisor.RPCClient {
	detector := supervisor.NewConfigDetector()
	host, username, password := detector.ReadSupervisorConfig()
	return supervisor.NewRPCClient(host, username, password)
}

func (app *CLIApp) configure(args []string) error {
	if len(args) == 0 || strings.ToLower(args[0]) != "rpc" {
		return fmt.Errorf("用法: sv configure rpc [--dry-run] [--restart]")
	}

	dryRun := false
	restart := false
	for _, argument := range args[1:] {
		switch argument {
		case "--dry-run":
			dryRun = true
		case "--restart":
			restart = true
		default:
			return fmt.Errorf("未知配置参数: %s", argument)
		}
	}
	if dryRun && restart {
		return fmt.Errorf("--dry-run不能与--restart同时使用")
	}

	detector := supervisor.NewConfigDetector()
	message, err := detector.ConfigureRPC(dryRun)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(app.renderer.out, message); err != nil {
		return err
	}
	if restart {
		if err := detector.RestartSupervisor(); err != nil {
			return err
		}
		_, err := fmt.Fprintln(app.renderer.out, "Supervisor 已重启，RPC 配置已生效")
		return err
	}
	return nil
}
