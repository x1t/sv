package supervisor

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ProcessController 负责控制Supervisor进程（启动/停止/重启）
type ProcessController struct{}

// NewProcessController 创建新的进程控制器
func NewProcessController() *ProcessController {
	return &ProcessController{}
}

// ControlProcess 控制进程（启动/停止/重启）
func (pc *ProcessController) ControlProcess(action, processName string) error {
	// 验证进程名称，防止命令注入
	// 检查是否包含可能用于命令注入的特殊字符
	if strings.ContainsAny(processName, "|;&`$()<>[]{}\\\"'") {
		return fmt.Errorf("进程名称包含非法字符")
	}

	// 检查进程名是否只包含字母数字、冒号、下划线、连字符和点号（标准进程名格式）
	// 避免包含可能导致shell解释的字符
	for _, r := range processName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			 r == ':' || r == '_' || r == '-' || r == '.') {
			// 如果包含非标准字符，可能是恶意输入
			return fmt.Errorf("进程名称包含非法字符")
		}
	}

	var command string
	switch action {
	case "start":
		command = "start"
	case "stop":
		command = "stop"
	case "restart":
		// 重启是先停止再启动
		err := pc.controlProcessViaCommand("stop", processName)
		if err != nil {
			return fmt.Errorf("停止进程失败: %v", err)
		}
		time.Sleep(1 * time.Second) // 等待一下再启动
		return pc.controlProcessViaCommand("start", processName)
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}

	// 使用 supervisorctl 命令控制进程，使用参数化方式避免命令注入
	cmd := exec.Command("supervisorctl", command, processName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s进程失败: %v, 输出: %s", action, err, string(output))
	}

	// 检查输出是否成功
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR") {
		return fmt.Errorf("%s进程失败: %s", action, outputStr)
	}

	return nil
}

// controlProcessViaCommand 通过命令行方式控制进程
func (pc *ProcessController) controlProcessViaCommand(action, processName string) error {
	// 验证进程名称，防止命令注入
	// 检查是否包含可能用于命令注入的特殊字符
	if strings.ContainsAny(processName, "|;&`$()<>[]{}\\\"'") {
		return fmt.Errorf("进程名称包含非法字符")
	}

	// 检查进程名是否只包含字母数字、冒号、下划线、连字符和点号（标准进程名格式）
	// 避免包含可能导致shell解释的字符
	for _, r := range processName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			 r == ':' || r == '_' || r == '-' || r == '.') {
			// 如果包含非标准字符，可能是恶意输入
			return fmt.Errorf("进程名称包含非法字符")
		}
	}

	var command string
	switch action {
	case "start":
		command = "start"
	case "stop":
		command = "stop"
	case "restart":
		// 重启是先停止再启动
		err := pc.controlProcessViaCommand("stop", processName)
		if err != nil {
			return fmt.Errorf("停止进程失败: %v", err)
		}
		time.Sleep(1 * time.Second) // 等待一下再启动
		return pc.controlProcessViaCommand("start", processName)
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}

	// 使用 supervisorctl 命令控制进程，使用参数化方式避免命令注入
	cmd := exec.Command("supervisorctl", command, processName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s进程失败: %v, 输出: %s", action, err, string(output))
	}

	// 检查输出是否成功
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR") {
		return fmt.Errorf("%s进程失败: %s", action, outputStr)
	}

	return nil
}