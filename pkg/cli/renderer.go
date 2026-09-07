package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/x1t/sv/pkg/supervisor"
	"github.com/x1t/sv/pkg/utils"
)

// CLIRenderer 负责命令行界面的渲染和交互。
type CLIRenderer struct {
	out    io.Writer
	errOut io.Writer
}

// NewCLIRenderer 创建命令行渲染器。
// 第一个 writer 是标准输出，第二个 writer 是错误输出；未提供时使用系统标准流。
func NewCLIRenderer(writers ...io.Writer) *CLIRenderer {
	renderer := &CLIRenderer{out: os.Stdout, errOut: os.Stderr}
	if len(writers) > 0 && writers[0] != nil {
		renderer.out = writers[0]
	}
	if len(writers) > 1 && writers[1] != nil {
		renderer.errOut = writers[1]
	} else if len(writers) == 1 && writers[0] != nil {
		renderer.errOut = writers[0]
	}
	return renderer
}

// ShowStatus 显示 Supervisor 进程状态。
func (cr *CLIRenderer) ShowStatus(client *supervisor.RPCClient) error {
	if client == nil {
		return fmt.Errorf("Supervisor客户端未初始化")
	}
	processes, err := client.GetAllProcesses()
	if err != nil {
		return fmt.Errorf("获取进程状态失败: %w", err)
	}

	if _, err := fmt.Fprintf(cr.out, "\n🔍 Supervisor进程状态 (共%d个进程)\n", len(processes)); err != nil {
		return err
	}
	if err := utils.RenderStatus(cr.out, processes); err != nil {
		return err
	}
	_, err = fmt.Fprintln(cr.out, "\n💡 提示: 使用 'sv start/stop/restart <序号>' 来控制进程")
	return err
}

// ControlProcesses 控制多个进程（启动、停止或重启）。
func (cr *CLIRenderer) ControlProcesses(client *supervisor.RPCClient, action string, args []string) error {
	if client == nil {
		return fmt.Errorf("Supervisor客户端未初始化")
	}
	processes, err := client.GetAllProcesses()
	if err != nil {
		return fmt.Errorf("获取进程信息失败: %w", err)
	}

	processNames, err := utils.ParseProcessIndices(args, processes)
	if err != nil {
		return fmt.Errorf("解析进程参数失败: %w", err)
	}

	if _, err := fmt.Fprintf(cr.out, "🎯 正在执行 '%s' 操作...\n", action); err != nil {
		return err
	}
	controller := supervisor.NewProcessController(client)
	failures := make([]string, 0)
	for _, name := range processNames {
		if _, err := fmt.Fprintf(cr.out, "  %s 进程 %s ... ", utils.GetActionIcon(action), name); err != nil {
			return err
		}
		if err := controller.ControlProcess(action, name); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			if _, writeErr := fmt.Fprintln(cr.out, "❌ 失败"); writeErr != nil {
				return writeErr
			}
			continue
		}
		if _, err := fmt.Fprintln(cr.out, "✅ 成功"); err != nil {
			return err
		}
	}

	successCount := len(processNames) - len(failures)
	if _, err := fmt.Fprintf(cr.out, "\n📊 操作完成: 成功 %d 个，失败 %d 个\n", successCount, len(failures)); err != nil {
		return err
	}
	if len(failures) > 0 {
		return fmt.Errorf("部分进程操作失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

// PrintUsage 打印使用说明。
func (cr *CLIRenderer) PrintUsage() {
	_, _ = fmt.Fprintln(cr.out, `sv - Supervisor进程管理工具

用法:
  sv status                    # 显示所有进程状态
  sv list                      # 显示所有进程状态（同 status）
  sv ls                        # list 的简写
  sv start <进程>              # 启动进程
  sv stop <进程>               # 停止进程
  sv restart <进程>            # 重启进程
  sv configure rpc             # 检查并补齐 RPC 配置
  sv configure rpc --dry-run   # 只预览配置变更
  sv configure rpc --restart   # 配置后显式重启 Supervisor
  sv service <action>          # 服务管理
  sv daemon                    # 运行服务守护进程

进程参数支持:
  序号      sv restart 1
  名称      sv restart myapp
  多个      sv restart 1 3 5
  范围      sv restart 1-5

服务操作:
  install   安装 sv 为系统服务
  uninstall 卸载 sv 系统服务
  start     启动 sv 系统服务
  stop      停止 sv 系统服务
  restart   重启 sv 系统服务
  status    查看 sv 服务状态

环境变量:
  SUPERVISOR_HOST              # Supervisor RPC 地址（默认: http://localhost:9001/RPC2）
  SUPERVISOR_USER              # 用户名（可选）
  SUPERVISOR_PASSWORD          # 密码（可选）`)
}
