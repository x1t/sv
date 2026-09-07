package supervisor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	serviceName    = "sv-supervisor-manager"
	sysvInitDir    = "/etc/init.d"
	systemdUnitDir = "/etc/systemd/system"
	symlinkDefault = "/usr/local/bin/sv"
)

var (
	sysvStartRunlevels = [...]string{"2", "3", "4", "5"}
	sysvStopRunlevels  = [...]string{"0", "1", "6"}
)

// writeLine 向输出流写一行文本(镜像 sv-rs `write_line`)。
func writeLine(w io.Writer, text string) error {
	if _, err := io.WriteString(w, text+"\n"); err != nil {
		return fmt.Errorf("写入服务输出: %w", err)
	}
	return nil
}

// ServiceManager 管理 sv 的系统服务生命周期(自写,镜像 sv-rs `ServiceManager`)。
type ServiceManager struct {
	out            io.Writer
	executable     string
	symlinkPath    string
	unitPath       string
	initScriptPath string
	backend        ServiceBackend
}

// NewServiceManager 使用真实环境构造:解析当前可执行文件并探测后端。
func NewServiceManager() *ServiceManager {
	return newServiceManager(os.Stdout)
}

// NewServiceManagerWithWriter 创建可注入输出流的服务管理器。
func NewServiceManagerWithWriter(out io.Writer) *ServiceManager {
	if out == nil {
		out = os.Stdout
	}
	return newServiceManager(out)
}

func newServiceManager(out io.Writer) *ServiceManager {
	executable, _ := os.Executable()
	if resolved, err := filepath.Abs(executable); err == nil {
		executable = resolved
	}
	return &ServiceManager{
		out:            out,
		executable:     executable,
		symlinkPath:    symlinkDefault,
		unitPath:       filepath.Join(systemdUnitDir, serviceName+".service"),
		initScriptPath: filepath.Join(sysvInitDir, serviceName),
		backend:        detectBackend(),
	}
}

// forTesting 注入全部落地路径与后端,便于单元测试在临时目录内验证(镜像 sv-rs `for_testing`)。
func forTesting(executable, symlinkPath, unitPath, initScriptPath string, backend ServiceBackend, out io.Writer) *ServiceManager {
	if out == nil {
		out = io.Discard
	}
	return &ServiceManager{
		out:            out,
		executable:     executable,
		symlinkPath:    symlinkPath,
		unitPath:       unitPath,
		initScriptPath: initScriptPath,
		backend:        backend,
	}
}

// unitFileName 返回 unit 文件的 basename(systemctl 按名操作)。
func (sm *ServiceManager) unitFileName() string {
	if base := filepath.Base(sm.unitPath); base != "." && base != "" {
		return base
	}
	return serviceName + ".service"
}

// HandleServiceCommand 处理 `sv service <action>` 子命令。
func (sm *ServiceManager) HandleServiceCommand(args []string) error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	if len(args) == 0 {
		_ = printServiceUsage(sm.out)
		return fmt.Errorf("缺少服务操作")
	}
	if len(args) > 1 {
		return fmt.Errorf("服务操作不接受额外参数: %s", strings.Join(args[1:], " "))
	}
	switch args[0] {
	case "install":
		return sm.install()
	case "uninstall":
		return sm.uninstall()
	case "start":
		return sm.start()
	case "stop":
		return sm.stop()
	case "restart":
		return sm.restart()
	case "status":
		return sm.status()
	default:
		_ = printServiceUsage(sm.out)
		return fmt.Errorf("未知服务操作: %s", args[0])
	}
}

// printServiceUsage 打印 service 用法(镜像 sv-rs `print_service_usage`)。
func printServiceUsage(out io.Writer) error {
	return writeLine(out, "用法: sv service <action>\n\n可用操作:\n  install   安装 sv 为系统服务\n  uninstall 卸载 sv 系统服务\n  start     启动 sv 系统服务\n  stop      停止 sv 系统服务\n  restart   重启 sv 系统服务\n  status    查看 sv 服务状态")
}

// ensureAbsent 拒绝写入已存在的路径(含软链接),避免覆盖既有服务文件或跟随软链。
func ensureAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("目标路径已存在: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查目标路径失败: %w", err)
	}
	return nil
}

// writeUnitFile 写 systemd unit 文件(仅当不存在)。
func (sm *ServiceManager) writeUnitFile() error {
	if err := ensureAbsent(sm.unitPath); err != nil {
		return err
	}
	if dir := filepath.Dir(sm.unitPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建服务目录失败: %w", err)
		}
	}
	content := systemdUnitText(sm.executable)
	if err := os.WriteFile(sm.unitPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入服务单元文件失败: %w", err)
	}
	return nil
}

// writeInitScript 写 SysV init 脚本(仅当不存在)并置 0755。
func (sm *ServiceManager) writeInitScript() error {
	if err := ensureAbsent(sm.initScriptPath); err != nil {
		return err
	}
	if dir := filepath.Dir(sm.initScriptPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建服务目录失败: %w", err)
		}
	}
	content := sysvInitText(sm.executable)
	if err := os.WriteFile(sm.initScriptPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("写入服务脚本失败: %w", err)
	}
	return nil
}

// runSystemctl 执行 systemctl 子命令;非零退出视为失败(镜像 sv-rs `run_systemctl`)。
func (sm *ServiceManager) runSystemctl(args []string) (string, error) {
	result, err := runCommandWithTimeout("systemctl", args, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("systemctl: %w", err)
	}
	text := strings.TrimSpace(string(result.output))
	if result.code == 0 {
		return text, nil
	}
	if result.code < 0 {
		return "", fmt.Errorf("systemctl %s: 信号终止", strings.Join(args, " "))
	}
	return "", fmt.Errorf("systemctl %s: exit status %d (%s)", strings.Join(args, " "), result.code, text)
}

// serviceControl 对后端执行 start/stop/restart。
func (sm *ServiceManager) serviceControl(action string) error {
	if sm.backend == BackendSystemd {
		_, err := sm.runSystemctl([]string{action, sm.unitFileName()})
		return err
	}
	script := sm.initScriptPath
	result, err := runCommandWithTimeout(script, []string{action}, 30*time.Second)
	if err != nil {
		return fmt.Errorf("%s %s: %w", script, action, err)
	}
	text := strings.TrimSpace(string(result.output))
	if result.code == 0 {
		return nil
	}
	if result.code < 0 {
		return fmt.Errorf("signal: killed (%s)", text)
	}
	return fmt.Errorf("exit status %d (%s)", result.code, text)
}

// runDaemon 以服务守护进程方式常驻,响应 SIGTERM/SIGINT 优雅退出(镜像 sv-rs `run_daemon`)。
func (sm *ServiceManager) runDaemon() error {
	if sm.executable == "" {
		return fmt.Errorf("服务程序未初始化")
	}
	if err := writeLine(sm.out, "SV服务已启动，正在后台运行..."); err != nil {
		return err
	}
	waitForSignal()
	return writeLine(sm.out, "SV服务已停止")
}

// install 安装系统服务并创建命令软链接(镜像 sv-rs `install`)。
// 任何已存在的服务文件或软链接都会拒绝覆盖。
func (sm *ServiceManager) install() error {
	if err := writeLine(sm.out, "🔧 正在安装 SV 系统服务..."); err != nil {
		return err
	}
	var created []string
	switch sm.backend {
	case BackendSystemd:
		if err := sm.writeUnitFile(); err != nil {
			return fmt.Errorf("安装服务失败: %w", err)
		}
		created = append(created, sm.unitPath)
		if _, err := sm.runSystemctl([]string{"daemon-reload"}); err != nil {
			sm.rollbackInstall(created)
			return fmt.Errorf("安装服务失败: %w", err)
		}
		if _, err := sm.runSystemctl([]string{"enable", sm.unitFileName()}); err != nil {
			sm.rollbackInstall(created)
			return fmt.Errorf("安装服务失败: %w", err)
		}
	case BackendSysV:
		if err := sm.writeInitScript(); err != nil {
			return fmt.Errorf("安装服务失败: %w", err)
		}
		created = append(created, sm.initScriptPath)
		if err := createSysVRunlevelLinks(sm.initScriptPath); err != nil {
			sm.rollbackInstall(created)
			return fmt.Errorf("安装服务失败: %w", err)
		}
	}
	// createSymlink 在软链已存在且指向本程序时视为幂等成功(不重复建也不报错),
	// 与 sv-rs `create_symlink` 语义一致。
	if _, err := sm.createSymlink(); err != nil {
		sm.rollbackInstall(created)
		return fmt.Errorf("服务已安装，但创建命令软链接失败: %w", err)
	}
	return writeLine(sm.out, "✅ SV 系统服务安装成功")
}

// rollbackInstall 清理本次安装已创建的服务资产(镜像 sv-rs `rollback_assets`)。
func (sm *ServiceManager) rollbackInstall(created []string) {
	if sm.backend == BackendSystemd {
		_, _ = sm.runSystemctl([]string{"disable", sm.unitFileName()})
	}
	for _, path := range created {
		_ = os.Remove(path)
	}
	if sm.backend == BackendSysV {
		_ = removeSysVRunlevelLinks(sm.initScriptPath)
	}
	if sm.backend == BackendSystemd {
		_, _ = sm.runSystemctl([]string{"daemon-reload"})
	}
	// 回滚只清理本次创建的软链:以当前可执行路径为候选。
	owned := make([]string, 0, 1)
	if sm.executable != "" {
		owned = append(owned, sm.executable)
	}
	_ = sm.removeSymlink(owned)
}

// uninstall 卸载系统服务并移除本程序创建的软链接(镜像 sv-rs `uninstall`)。
// 幂等:服务未安装且无本程序残留时输出「服务未安装,无需卸载」并成功。
func (sm *ServiceManager) uninstall() error {
	serviceFilePresent := false
	serviceFile := ""
	switch sm.backend {
	case BackendSystemd:
		serviceFile = sm.unitPath
	case BackendSysV:
		serviceFile = sm.initScriptPath
	}
	if serviceFile != "" {
		if _, err := os.Lstat(serviceFile); err == nil {
			serviceFilePresent = true
		}
	}

	if !serviceFilePresent {
		return sm.uninstallNotRegistered()
	}

	// 归属门控:仅当服务文件里记录的可执行路径确为本工具安装(等于当前 sv,
	// 或等于命令软链当前指向的路径,覆盖升级换路径/旧 exe 已删除)才卸载;
	// 否则是占用本服务名的异源文件,拒绝 stop/disable/删除,避免误伤。
	if recorded := sm.recordedServiceExecutable(); !sm.serviceFileOwned(recorded) {
		return fmt.Errorf("拒绝卸载非本程序注册的服务文件: %s", serviceFile)
	}

	owned := sm.uninstallSymlinkOwned()
	if sm.backend == BackendSystemd {
		if _, err := sm.runSystemctl([]string{"stop", sm.unitFileName()}); err != nil {
			return fmt.Errorf("卸载服务失败: %w", err)
		}
		if _, err := sm.runSystemctl([]string{"disable", sm.unitFileName()}); err != nil {
			return fmt.Errorf("卸载服务失败: %w", err)
		}
		if info, err := os.Lstat(sm.unitPath); err == nil {
			if info.IsDir() {
				return fmt.Errorf("拒绝删除目录: %s", sm.unitPath)
			}
			if err := os.Remove(sm.unitPath); err != nil {
				return fmt.Errorf("卸载服务失败: %w", err)
			}
		}
		if _, err := sm.runSystemctl([]string{"daemon-reload"}); err != nil {
			return fmt.Errorf("卸载服务失败: %w", err)
		}
	} else {
		if _, err := os.Lstat(sm.initScriptPath); err == nil {
			if err := sm.serviceControl("stop"); err != nil {
				return fmt.Errorf("卸载服务失败: %w", err)
			}
		}
		if err := removeSysVRunlevelLinks(sm.initScriptPath); err != nil {
			return fmt.Errorf("卸载服务失败: %w", err)
		}
		if info, err := os.Lstat(sm.initScriptPath); err == nil {
			if info.IsDir() {
				return fmt.Errorf("拒绝删除目录: %s", sm.initScriptPath)
			}
			if err := os.Remove(sm.initScriptPath); err != nil {
				return fmt.Errorf("卸载服务失败: %w", err)
			}
		}
	}
	if err := sm.removeSymlink(owned); err != nil {
		return fmt.Errorf("服务已卸载，但移除命令软链接失败: %w", err)
	}
	return writeLine(sm.out, "✅ SV 系统服务卸载成功")
}

// uninstallNotRegistered 处理服务未安装的情形:清理残留 rc 链接/软链或 no-op。
func (sm *ServiceManager) uninstallNotRegistered() error {
	cleaned := false
	if sm.backend == BackendSysV && anySysVRunlevelLinkExists(sm.initScriptPath) {
		if err := removeSysVRunlevelLinks(sm.initScriptPath); err == nil {
			cleaned = true
		}
	}
	owned := sm.uninstallSymlinkOwned()
	if _, err := os.Lstat(sm.symlinkPath); err == nil {
		if err := sm.removeSymlink(owned); err != nil {
			return fmt.Errorf("服务未安装，清理残留命令软链接失败: %w", err)
		}
		cleaned = true
	}
	if cleaned {
		return writeLine(sm.out, "✅ 服务未安装，已清理残留文件/软链接")
	}
	return writeLine(sm.out, "服务未安装，无需卸载")
}

// start/stop/restart 分别启动/停止/重启系统服务。
func (sm *ServiceManager) start() error {
	if err := sm.serviceControl("start"); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}
	return writeLine(sm.out, "✅ SV 系统服务启动成功")
}

func (sm *ServiceManager) stop() error {
	if err := sm.serviceControl("stop"); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}
	return writeLine(sm.out, "✅ SV 系统服务停止成功")
}

func (sm *ServiceManager) restart() error {
	if err := sm.serviceControl("restart"); err != nil {
		return fmt.Errorf("重启服务失败: %w", err)
	}
	return writeLine(sm.out, "✅ SV 系统服务重启成功")
}

// status 查询系统服务状态(镜像 sv-rs `status`)。
func (sm *ServiceManager) status() error {
	var state string
	switch sm.backend {
	case BackendSystemd:
		result, err := runCommandWithTimeout("systemctl", []string{"is-active", sm.unitFileName()}, 15*time.Second)
		if err != nil {
			return fmt.Errorf("获取服务状态失败: %w", err)
		}
		// is-active 对非 active 态(含 inactive/failed/未加载)返回非零退出码,
		// 需按输出文本而非退出码判定,避免「已停止」被误报为错误。
		switch strings.TrimSpace(string(result.output)) {
		case "active":
			state = "✅ 运行中"
		case "inactive":
			state = "⏸️ 已停止"
		default:
			if result.code < 0 {
				return fmt.Errorf("获取服务状态失败: signal: killed")
			}
			state = "❓ 未知状态"
		}
	case BackendSysV:
		result, err := runCommandWithTimeout(sm.initScriptPath, []string{"status"}, 15*time.Second)
		if err != nil {
			return fmt.Errorf("获取服务状态失败: %w", err)
		}
		switch result.code {
		case 0:
			state = "✅ 运行中"
		case 3:
			state = "⏸️ 已停止"
		default:
			if result.code < 0 {
				return fmt.Errorf("获取服务状态失败: signal: killed")
			}
			state = "❓ 未知状态"
		}
	}
	return writeLine(sm.out, "SV 系统服务状态: "+state)
}

// RunServiceDaemon 运行服务守护进程(兼容公开 API,供 `sv daemon` 使用)。
func (sm *ServiceManager) RunServiceDaemon() error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	return sm.runDaemon()
}

// InstallService 安装系统服务并创建命令软链接(公开兼容包装)。
func (sm *ServiceManager) InstallService() error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	return sm.install()
}

// UninstallService 卸载系统服务并移除命令软链接(公开兼容包装)。
func (sm *ServiceManager) UninstallService() error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	return sm.uninstall()
}

// StartService 启动系统服务(公开兼容包装)。
func (sm *ServiceManager) StartService() error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	return sm.start()
}

// StopService 停止系统服务(公开兼容包装)。
func (sm *ServiceManager) StopService() error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	return sm.stop()
}

// RestartService 重启系统服务(公开兼容包装)。
func (sm *ServiceManager) RestartService() error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	return sm.restart()
}

// CheckServiceStatus 查询系统服务状态(公开兼容包装)。
func (sm *ServiceManager) CheckServiceStatus() error {
	if sm == nil || sm.out == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	return sm.status()
}
