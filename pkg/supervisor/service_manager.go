package supervisor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/kardianos/service"
)

const serviceName = "sv-supervisor-manager"

// ServiceManager 管理 sv 的系统服务生命周期。
type ServiceManager struct {
	out         io.Writer
	svcLogger   service.Logger
	svcService  service.Service
	svcProgram  *program
	executable  string
	symlinkPath string
}

type program struct {
	done      chan struct{}
	stopOnce  sync.Once
	startOnce sync.Once
	logger    service.Logger
}

func newProgram(logger service.Logger) *program {
	return &program{done: make(chan struct{}), logger: logger}
}

func (p *program) Start(_ service.Service) error {
	if p == nil {
		return fmt.Errorf("服务程序未初始化")
	}
	p.logInfo("SV服务正在启动...")
	p.startOnce.Do(func() { go p.run() })
	return nil
}

func (p *program) Stop(_ service.Service) error {
	if p == nil {
		return fmt.Errorf("服务程序未初始化")
	}
	p.logInfo("SV服务正在停止...")
	p.stopOnce.Do(func() { close(p.done) })
	return nil
}

func (p *program) run() {
	p.logInfo("SV服务已启动，正在后台运行...")
	<-p.done
	p.logInfo("SV服务已停止")
}

func (p *program) logInfo(format string, args ...interface{}) {
	if p.logger != nil {
		p.logger.Infof(format, args...)
	}
}

// NewServiceManager 创建服务管理器。
func NewServiceManager() *ServiceManager {
	return &ServiceManager{out: os.Stdout, symlinkPath: "/usr/local/bin/sv"}
}

// NewServiceManagerWithWriter 创建可注入输出流的服务管理器。
func NewServiceManagerWithWriter(out io.Writer) *ServiceManager {
	manager := NewServiceManager()
	if out != nil {
		manager.out = out
	}
	return manager
}

// HandleServiceCommand 处理 service 子命令。
func (sm *ServiceManager) HandleServiceCommand(args []string) error {
	if len(args) == 0 {
		sm.printServiceUsage()
		return fmt.Errorf("缺少服务操作")
	}
	if len(args) > 1 {
		return fmt.Errorf("服务操作不接受额外参数: %s", strings.Join(args[1:], " "))
	}

	switch args[0] {
	case "install":
		return sm.InstallService()
	case "uninstall":
		return sm.UninstallService()
	case "start":
		return sm.StartService()
	case "stop":
		return sm.StopService()
	case "restart":
		return sm.RestartService()
	case "status":
		return sm.CheckServiceStatus()
	default:
		sm.printServiceUsage()
		return fmt.Errorf("未知服务操作: %s", args[0])
	}
}

func (sm *ServiceManager) printServiceUsage() {
	_, _ = fmt.Fprintln(sm.out, `用法: sv service <action>

可用操作:
  install   安装 sv 为系统服务
  uninstall 卸载 sv 系统服务
  start     启动 sv 系统服务
  stop      停止 sv 系统服务
  restart   重启 sv 系统服务
  status    查看 sv 服务状态`)
}

func (sm *ServiceManager) setup() error {
	if sm == nil {
		return fmt.Errorf("服务管理器未初始化")
	}
	if sm.svcService != nil {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}
	sm.executable = executable
	config := &service.Config{
		Name:        serviceName,
		DisplayName: "SV Supervisor Manager",
		Description: "现代化 Supervisor 进程管理工具",
		Executable:  executable,
		Arguments:   []string{"daemon"},
	}

	instance := newProgram(nil)
	managedService, err := service.New(instance, config)
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}
	logger, err := managedService.Logger(nil)
	if err != nil {
		return fmt.Errorf("获取服务日志记录器失败: %w", err)
	}
	instance.logger = logger
	sm.svcLogger = logger
	sm.svcProgram = instance
	sm.svcService = managedService
	return nil
}

func (sm *ServiceManager) requireService() error {
	if sm.svcService != nil {
		return nil
	}
	return sm.setup()
}

// InstallService 安装系统服务并创建命令软链接。
func (sm *ServiceManager) InstallService() error {
	if err := sm.requireService(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(sm.out, "🔧 正在安装 SV 系统服务..."); err != nil {
		return err
	}
	if err := sm.svcService.Install(); err != nil {
		return fmt.Errorf("安装服务失败: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := sm.createSymlink(); err != nil {
			return fmt.Errorf("服务已安装，但创建命令软链接失败: %w", err)
		}
	}
	_, err := fmt.Fprintln(sm.out, "✅ SV 系统服务安装成功")
	return err
}

// UninstallService 卸载系统服务并移除由本程序创建的软链接。
func (sm *ServiceManager) UninstallService() error {
	if err := sm.requireService(); err != nil {
		return err
	}
	if err := sm.svcService.Uninstall(); err != nil {
		return fmt.Errorf("卸载服务失败: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := sm.removeSymlink(); err != nil {
			return fmt.Errorf("服务已卸载，但移除命令软链接失败: %w", err)
		}
	}
	_, err := fmt.Fprintln(sm.out, "✅ SV 系统服务卸载成功")
	return err
}

// StartService 启动系统服务。
func (sm *ServiceManager) StartService() error {
	if err := sm.requireService(); err != nil {
		return err
	}
	if err := sm.svcService.Start(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}
	_, err := fmt.Fprintln(sm.out, "✅ SV 系统服务启动成功")
	return err
}

// StopService 停止系统服务。
func (sm *ServiceManager) StopService() error {
	if err := sm.requireService(); err != nil {
		return err
	}
	if err := sm.svcService.Stop(); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}
	_, err := fmt.Fprintln(sm.out, "✅ SV 系统服务停止成功")
	return err
}

// RestartService 重启系统服务。
func (sm *ServiceManager) RestartService() error {
	if err := sm.requireService(); err != nil {
		return err
	}
	if err := sm.svcService.Restart(); err != nil {
		return fmt.Errorf("重启服务失败: %w", err)
	}
	_, err := fmt.Fprintln(sm.out, "✅ SV 系统服务重启成功")
	return err
}

// CheckServiceStatus 查询系统服务状态。
func (sm *ServiceManager) CheckServiceStatus() error {
	if err := sm.requireService(); err != nil {
		return err
	}
	status, err := sm.svcService.Status()
	if err != nil {
		return fmt.Errorf("获取服务状态失败: %w", err)
	}
	statusText := "⚠️ 其他状态"
	switch status {
	case service.StatusRunning:
		statusText = "✅ 运行中"
	case service.StatusStopped:
		statusText = "⏸️ 已停止"
	case service.StatusUnknown:
		statusText = "❓ 未知状态"
	}
	_, err = fmt.Fprintf(sm.out, "SV 系统服务状态: %s\n", statusText)
	return err
}

// RunServiceDaemon 运行服务守护进程。
func (sm *ServiceManager) RunServiceDaemon() error {
	if err := sm.requireService(); err != nil {
		return err
	}
	if err := sm.svcService.Run(); err != nil {
		return fmt.Errorf("服务运行失败: %w", err)
	}
	return nil
}

func (sm *ServiceManager) createSymlink() error {
	if err := sm.requireExecutable(); err != nil {
		return err
	}
	targetPath := sm.symlinkPath
	if targetPath == "" {
		return fmt.Errorf("软链接路径不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建软链接目录失败: %w", err)
	}

	info, err := os.Lstat(targetPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("目标路径已存在且不是软链接: %s", targetPath)
		}
		resolvedTarget, resolveErr := filepath.EvalSymlinks(targetPath)
		if resolveErr == nil {
			resolvedExecutable, executableErr := filepath.EvalSymlinks(sm.executable)
			if executableErr == nil && resolvedTarget == resolvedExecutable {
				return nil
			}
		}
		return fmt.Errorf("目标软链接已存在且指向其他文件: %s", targetPath)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("检查软链接目标失败: %w", err)
	}
	if err := os.Symlink(sm.executable, targetPath); err != nil {
		return fmt.Errorf("创建软链接失败: %w", err)
	}
	_, err = fmt.Fprintf(sm.out, "🔗 已创建软链接: %s -> %s\n", targetPath, sm.executable)
	return err
}

func (sm *ServiceManager) removeSymlink() error {
	if err := sm.requireExecutable(); err != nil {
		return err
	}
	info, err := os.Lstat(sm.symlinkPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检查软链接失败: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("拒绝删除非软链接路径: %s", sm.symlinkPath)
	}
	resolvedTarget, err := filepath.EvalSymlinks(sm.symlinkPath)
	if err != nil {
		return fmt.Errorf("解析软链接失败: %w", err)
	}
	resolvedExecutable, err := filepath.EvalSymlinks(sm.executable)
	if err != nil {
		return fmt.Errorf("解析可执行文件失败: %w", err)
	}
	if resolvedTarget != resolvedExecutable {
		return fmt.Errorf("拒绝删除指向其他文件的软链接: %s", sm.symlinkPath)
	}
	if err := os.Remove(sm.symlinkPath); err != nil {
		return fmt.Errorf("删除软链接失败: %w", err)
	}
	_, err = fmt.Fprintf(sm.out, "✅ 已删除软链接: %s\n", sm.symlinkPath)
	return err
}

func (sm *ServiceManager) requireExecutable() error {
	if sm.executable != "" {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("获取可执行文件状态失败: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("可执行文件路径指向目录: %s", executable)
	}
	sm.executable = executable
	return nil
}
