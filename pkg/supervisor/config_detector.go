package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	inetHTTPServerConfig = "[inet_http_server]\nport=127.0.0.1:9001\n"
	rpcInterfaceConfig   = "[rpcinterface:supervisor]\nsupervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface\n"
)

// ConfigDetector 负责查找和显式修改 Supervisor 配置。
type ConfigDetector struct {
	configPaths []string
}

// NewConfigDetector 创建配置检测器。
func NewConfigDetector() *ConfigDetector {
	paths := make([]string, 0, 3)
	if configuredPath := strings.TrimSpace(os.Getenv("SUPERVISOR_CONFIG")); configuredPath != "" {
		paths = append(paths, configuredPath)
	}
	paths = append(paths,
		"/etc/supervisor/supervisord.conf",
		"/etc/supervisord.conf",
	)
	return &ConfigDetector{configPaths: paths}
}

// FindConfigPath 返回第一个存在且为普通文件的主配置文件。
func (cd *ConfigDetector) FindConfigPath() (string, error) {
	if cd == nil {
		return "", fmt.Errorf("配置检测器未初始化")
	}
	for _, path := range cd.configPaths {
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("检查配置文件 %s 失败: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("配置路径不是普通文件: %s", path)
		}
		return path, nil
	}
	return "", fmt.Errorf("未找到 Supervisor 配置文件，请设置 SUPERVISOR_CONFIG")
}

// DetectAndEnableRPC 保留旧 API；配置修改仍需由调用方显式触发。
// 新代码应使用 ConfigureRPC，以便选择 dry-run 或显式重启策略。
func (cd *ConfigDetector) DetectAndEnableRPC() error {
	_, err := cd.ConfigureRPC(false)
	return err
}

// ConfigureRPC 检查并补齐 RPC 配置。它不会自动重启 Supervisor。
func (cd *ConfigDetector) ConfigureRPC(dryRun bool) (string, error) {
	configPath, err := cd.FindConfigPath()
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("读取配置文件失败: %w", err)
	}

	updated := string(content)
	changes := make([]string, 0, 2)
	if !hasInetHTTPServerContent(updated) {
		updated = appendConfigSection(updated, inetHTTPServerConfig)
		changes = append(changes, "inet_http_server")
	}
	if !hasRPCInterfaceContent(updated) {
		updated = appendConfigSection(updated, rpcInterfaceConfig)
		changes = append(changes, "rpcinterface:supervisor")
	}
	if len(changes) == 0 {
		return fmt.Sprintf("RPC配置已存在: %s", configPath), nil
	}
	if dryRun {
		return fmt.Sprintf("将更新 %s，新增配置段: %s", configPath, strings.Join(changes, ", ")), nil
	}

	backupPath, err := backupAndWriteConfig(configPath, []byte(updated))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("配置已更新: %s；备份: %s；请使用 --restart 或手动重启 Supervisor", configPath, backupPath), nil
}

// RestartSupervisor 显式尝试重启 Supervisor 服务。
func (cd *ConfigDetector) RestartSupervisor() error {
	commands := [][]string{
		{"systemctl", "restart", "supervisor"},
		{"systemctl", "restart", "supervisord"},
		{"service", "supervisor", "restart"},
		{"service", "supervisord", "restart"},
		{"/etc/init.d/supervisor", "restart"},
		{"/etc/init.d/supervisord", "restart"},
	}
	errors := make([]string, 0, len(commands))
	for _, command := range commands {
		if filepath.Base(command[0]) != command[0] {
			if _, err := os.Stat(command[0]); err != nil {
				continue
			}
		} else if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.CommandContext(ctx, command[0], command[1:]...).CombinedOutput()
		cancel()
		if err == nil {
			return nil
		}
		errors = append(errors, fmt.Sprintf("%s: %v (%s)", strings.Join(command, " "), err, strings.TrimSpace(string(output))))
	}
	if len(errors) == 0 {
		return fmt.Errorf("未找到可用的 Supervisor 服务管理命令")
	}
	return fmt.Errorf("重启 Supervisor 失败: %s", strings.Join(errors, "; "))
}

// HasInetHTTPServer 检查是否存在有效的 inet_http_server 配置。
func (cd *ConfigDetector) HasInetHTTPServer(configPath string) (bool, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	return hasInetHTTPServerContent(string(content)), nil
}

// HasRPCInterface 检查是否存在有效的 Supervisor RPC 工厂配置。
func (cd *ConfigDetector) HasRPCInterface(configPath string) (bool, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	return hasRPCInterfaceContent(string(content)), nil
}

// AddInetHTTPServerConfig 显式追加 inet_http_server 配置。
func (cd *ConfigDetector) AddInetHTTPServerConfig(configPath string) error {
	return updateConfig(configPath, func(content string) (string, bool) {
		if hasInetHTTPServerContent(content) {
			return content, false
		}
		return appendConfigSection(content, inetHTTPServerConfig), true
	})
}

// AddRPCInterfaceConfig 显式追加 Supervisor RPC 工厂配置。
func (cd *ConfigDetector) AddRPCInterfaceConfig(configPath string) error {
	return updateConfig(configPath, func(content string) (string, bool) {
		if hasRPCInterfaceContent(content) {
			return content, false
		}
		return appendConfigSection(content, rpcInterfaceConfig), true
	})
}

// ReadSupervisorConfig 读取 Supervisor 的连接配置。
func (cd *ConfigDetector) ReadSupervisorConfig() (host, username, password string) {
	host = DefaultSupervisorHost
	username = strings.TrimSpace(os.Getenv("SUPERVISOR_USER"))
	password = os.Getenv("SUPERVISOR_PASSWORD")
	if configuredHost := strings.TrimSpace(os.Getenv("SUPERVISOR_HOST")); configuredHost != "" {
		host = configuredHost
	}
	return host, username, password
}

func hasInetHTTPServerContent(content string) bool {
	return hasSectionKey(content, "inet_http_server", "port")
}

func hasRPCInterfaceContent(content string) bool {
	return hasSectionKey(content, "rpcinterface:supervisor", "supervisor.rpcinterface_factory")
}

func hasSectionKey(content, expectedSection, expectedKey string) bool {
	section := ""
	for _, rawLine := range strings.Split(content, "\n") {
		line := activeConfigLine(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != expectedSection {
			continue
		}
		key, value, hasValue := strings.Cut(line, "=")
		if hasValue && strings.EqualFold(strings.TrimSpace(key), expectedKey) && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func activeConfigLine(rawLine string) string {
	line := strings.TrimSpace(rawLine)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
		return ""
	}
	for _, marker := range []string{"#", ";"} {
		if index := strings.Index(line, marker); index >= 0 {
			line = strings.TrimSpace(line[:index])
		}
	}
	return line
}

func appendConfigSection(content, section string) string {
	content = strings.TrimRight(content, "\r\n")
	if content != "" {
		content += "\n\n"
	}
	return content + strings.TrimRight(section, "\r\n") + "\n"
}

func updateConfig(configPath string, update func(string) (string, bool)) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	updated, changed := update(string(content))
	if !changed {
		return nil
	}
	_, err = backupAndWriteConfig(configPath, []byte(updated))
	return err
}

func backupAndWriteConfig(configPath string, content []byte) (string, error) {
	info, err := os.Lstat(configPath)
	if err != nil {
		return "", fmt.Errorf("检查配置文件失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("配置文件必须是普通文件: %s", configPath)
	}
	if info.Mode().Perm()&0200 == 0 {
		return "", fmt.Errorf("配置文件不可写: %s", configPath)
	}

	backupPath := configPath + ".bak." + strconv.FormatInt(time.Now().UnixNano(), 10)
	backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return "", fmt.Errorf("创建配置备份失败: %w", err)
	}
	backupOK := false
	defer func() {
		if !backupOK {
			_ = backup.Close()
			_ = os.Remove(backupPath)
		}
	}()

	source, err := os.Open(configPath)
	if err != nil {
		return "", fmt.Errorf("打开配置文件失败: %w", err)
	}
	_, copyErr := io.Copy(backup, source)
	closeSourceErr := source.Close()
	if copyErr != nil {
		return "", fmt.Errorf("写入配置备份失败: %w", copyErr)
	}
	if closeSourceErr != nil {
		return "", fmt.Errorf("关闭配置文件失败: %w", closeSourceErr)
	}
	if err := backup.Sync(); err != nil {
		return "", fmt.Errorf("同步配置备份失败: %w", err)
	}
	if err := backup.Close(); err != nil {
		return "", fmt.Errorf("关闭配置备份失败: %w", err)
	}
	backupOK = true

	directory := filepath.Dir(configPath)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(configPath)+".tmp-")
	if err != nil {
		return "", fmt.Errorf("创建临时配置文件失败: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("设置临时配置权限失败: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("写入临时配置失败: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("同步临时配置失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("关闭临时配置失败: %w", err)
	}
	if err := os.Rename(temporaryPath, configPath); err != nil {
		return "", fmt.Errorf("原子替换配置文件失败: %w", err)
	}
	return backupPath, nil
}
