package supervisor

import (
	"fmt"
	"strings"
	"unicode"
)

// ProcessController delegates process operations to a Supervisor client.
// The variadic constructor keeps the zero-argument form available for callers
// that want to use the default local endpoint.
type ProcessController struct {
	client *RPCClient
}

func NewProcessController(clients ...*RPCClient) *ProcessController {
	var client *RPCClient
	if len(clients) > 0 {
		client = clients[0]
	}
	if client == nil {
		client = NewRPCClient(DefaultSupervisorHost, "", "")
	}
	return &ProcessController{client: client}
}

func (pc *ProcessController) ControlProcess(action, processName string) error {
	if pc == nil || pc.client == nil {
		return fmt.Errorf("Supervisor客户端未初始化")
	}
	return pc.client.ControlProcess(action, processName)
}

func validateProcessAction(action string) error {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start", "stop", "restart":
		return nil
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}
}

func validateProcessName(processName string) error {
	if strings.TrimSpace(processName) == "" {
		return fmt.Errorf("进程名称不能为空")
	}
	if strings.TrimSpace(processName) != processName {
		return fmt.Errorf("进程名称不能包含首尾空白")
	}
	if strings.HasPrefix(processName, "-") {
		return fmt.Errorf("进程名称不能以连字符开头")
	}
	for _, r := range processName {
		if unicode.IsControl(r) || r == '\x00' {
			return fmt.Errorf("进程名称包含控制字符")
		}
	}
	return nil
}
