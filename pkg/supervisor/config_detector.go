package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ConfigDetector 负责检测和配置Supervisor配置
type ConfigDetector struct{}

// NewConfigDetector 创建新的配置检测器
func NewConfigDetector() *ConfigDetector {
	return &ConfigDetector{}
}

// DetectAndEnableRPC 检测并开启Supervisor RPC功能
func (cd *ConfigDetector) DetectAndEnableRPC() error {
	// 检查默认的supervisor配置文件位置
	configPaths := []string{
		"/etc/supervisor/supervisord.conf",
		"/etc/supervisor/conf.d/*.conf",
		"/etc/supervisord.conf",
	}

	// 标记是否修改了配置文件
	configModified := false

	// 简单地检查和修改第一个存在的配置文件
	for _, configPath := range configPaths {
		// 简化处理：只处理主配置文件，不处理通配符路径
		if strings.Contains(configPath, "*") {
			continue
		}

		if _, err := os.Stat(configPath); err == nil {
			// 配置文件存在，检查是否有inet_http_server配置
			enabled, err := cd.HasInetHTTPServer(configPath)
			if err != nil {
				fmt.Printf("⚠️  检查配置文件失败: %v\n", err)
				continue
			}

			if !enabled {
				// 如果没有启用inet_http_server，则添加配置
				fmt.Printf("🔧 未发现inet_http_server配置，正在添加...\n")
				err = cd.AddInetHTTPServerConfig(configPath)
				if err != nil {
					fmt.Printf("❌ 添加inet_http_server配置失败: %v\n", err)
					continue
				} else {
					fmt.Printf("✅ inet_http_server配置已添加\n")
					configModified = true
				}
			}

			// 检查RPC接口配置
			rpcEnabled, err := cd.HasRPCInterface(configPath)
			if err != nil {
				fmt.Printf("⚠️  检查RPC接口配置失败: %v\n", err)
				continue
			}

			if !rpcEnabled {
				// 如果没有启用RPC接口，则添加配置
				fmt.Printf("🔧 未发现RPC接口配置，正在添加...\n")
				err = cd.AddRPCInterfaceConfig(configPath)
				if err != nil {
					fmt.Printf("❌ 添加RPC接口配置失败: %v\n", err)
					continue
				} else {
					fmt.Printf("✅ RPC接口配置已添加\n")
					configModified = true
				}
			}

			// 如果配置被修改，提示用户重启Supervisor服务
			if configModified {
				fmt.Printf("💡 提示: 配置已修改，需要重启Supervisor服务以应用更改\n")
				// 尝试重启Supervisor服务
				if err := cd.RestartSupervisor(); err != nil {
					fmt.Printf("⚠️  无法自动重启Supervisor服务: %v\n", err)
					fmt.Println("💡 请手动重启Supervisor服务以应用配置更改")
				} else {
					fmt.Println("✅ Supervisor服务已重启，配置生效")
				}
			}

			return nil
		}
	}

	fmt.Println("⚠️  未找到supervisor配置文件")
	return fmt.Errorf("未找到supervisor配置文件")
}

// RestartSupervisor 尝试重启Supervisor服务
func (cd *ConfigDetector) RestartSupervisor() error {
	// 尝试使用systemctl重启supervisor (在大多数Linux系统上)
	cmd := exec.Command("systemctl", "restart", "supervisor")
	if err := cmd.Run(); err != nil {
		fmt.Printf("systemctl restart supervisor 失败: %v\n", err)
		// 如果systemctl失败，尝试使用service命令
		cmd = exec.Command("service", "supervisor", "restart")
		if err := cmd.Run(); err != nil {
			fmt.Printf("service restart supervisor 失败: %v\n", err)
			// 如果还是失败，返回错误而不是继续尝试
			return fmt.Errorf("无法重启supervisor服务: systemctl和service命令都失败了")
		}
	}
	return nil
}

// HasInetHTTPServer 检查配置文件是否已启用inet_http_server
func (cd *ConfigDetector) HasInetHTTPServer(configPath string) (bool, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}

	contentStr := string(content)

	// 检查是否有未注释的inet_http_server配置
	lines := strings.Split(contentStr, "\n")

	inInetSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检查段开始
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[]")
			if section == "inet_http_server" {
				inInetSection = true
			} else {
				inInetSection = false
			}
		}

		// 如果在inet_http_server段中且找到了port配置，则认为已启用
		if inInetSection && strings.HasPrefix(trimmed, "port=") && !strings.HasPrefix(line, ";") && !strings.HasPrefix(line, "#") {
			return true, nil
		}
	}

	return false, nil
}

// HasRPCInterface 检查配置文件是否已启用RPC接口
func (cd *ConfigDetector) HasRPCInterface(configPath string) (bool, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}

	contentStr := string(content)

	// 检查是否有rpcinterface:supervisor配置
	lines := strings.Split(contentStr, "\n")

	inRPCSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检查段开始
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[]")
			if section == "rpcinterface:supervisor" {
				inRPCSection = true
			} else {
				inRPCSection = false
			}
		}

		// 如果在rpcinterface:supervisor段中且找到了factory配置，则认为已启用
		if inRPCSection && strings.Contains(trimmed, "rpcinterface_factory") && !strings.HasPrefix(line, ";") && !strings.HasPrefix(line, "#") {
			return true, nil
		}
	}

	return false, nil
}

// AddInetHTTPServerConfig 添加inet_http_server配置
func (cd *ConfigDetector) AddInetHTTPServerConfig(configPath string) error {
	// 先检查配置文件权限
	info, err := os.Stat(configPath)
	if err != nil {
		return err
	}

	// 检查是否可写
	if info.Mode()&0200 == 0 { // 检查用户是否可写
		return fmt.Errorf("配置文件不可写: %s", configPath)
	}

	// 读取现有内容
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// 检查是否已经存在inet_http_server配置
	if strings.Contains(contentStr, "[inet_http_server]") {
		return nil // 已存在，无需添加
	}

	// 添加inet_http_server配置
	inetConfig := `
[inet_http_server]
port=127.0.0.1:9001

`

	// 检查是否存在[unix_http_server]段，如果存在则在其后添加
	unixPos := strings.Index(contentStr, "[unix_http_server]")
	if unixPos != -1 {
		// 找到unix_http_server段的结束位置（下一个段开始的位置）
		nextSectionPos := strings.Index(contentStr[unixPos+1:], "[")
		if nextSectionPos != -1 {
			nextSectionPos += unixPos + 1
			newContent := contentStr[:nextSectionPos] + inetConfig + contentStr[nextSectionPos:]
			return os.WriteFile(configPath, []byte(newContent), info.Mode())
		}
	}

	// 如果没找到unix_http_server或位置不明确，则添加到文件开头
	newContent := inetConfig + contentStr
	return os.WriteFile(configPath, []byte(newContent), info.Mode())
}

// AddRPCInterfaceConfig 添加RPC接口配置
func (cd *ConfigDetector) AddRPCInterfaceConfig(configPath string) error {
	// 先检查配置文件权限
	info, err := os.Stat(configPath)
	if err != nil {
		return err
	}

	// 检查是否可写
	if info.Mode()&0200 == 0 { // 检查用户是否可写
		return fmt.Errorf("配置文件不可写: %s", configPath)
	}

	// 读取现有内容
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// 检查是否已经存在RPC接口配置
	if strings.Contains(contentStr, "[rpcinterface:supervisor]") {
		return nil // 已存在，无需添加
	}

	// 添加RPC接口配置
	rpcConfig := `
[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

`

	// 检查是否存在[supervisorctl]段，如果存在则在其前添加
	supervisorctlPos := strings.Index(contentStr, "[supervisorctl]")
	if supervisorctlPos != -1 {
		newContent := contentStr[:supervisorctlPos] + rpcConfig + contentStr[supervisorctlPos:]
		return os.WriteFile(configPath, []byte(newContent), info.Mode())
	}

	// 如果没找到supervisorctl位置，则添加到文件末尾
	newContent := contentStr + rpcConfig
	return os.WriteFile(configPath, []byte(newContent), info.Mode())
}

// ReadSupervisorConfig 读取supervisor配置获取连接信息
func (cd *ConfigDetector) ReadSupervisorConfig() (host, username, password string) {
	// 默认值
	host = "http://localhost:9001/RPC2"
	username = ""
	password = ""

	// 尝试从环境变量读取
	if h := os.Getenv("SUPERVISOR_HOST"); h != "" {
		host = h
	}
	if u := os.Getenv("SUPERVISOR_USER"); u != "" {
		username = u
	}
	if p := os.Getenv("SUPERVISOR_PASSWORD"); p != "" {
		password = p
	}

	// 也可以从配置文件读取，这里简化处理
	return
}