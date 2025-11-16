package supervisor

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kardianos/service"
)

// ServiceManager 系统服务管理器
type ServiceManager struct {
	svcLogger  service.Logger
	svcService service.Service
	svcProgram *program
}

// Program 实现service.Interface接口
type program struct {
	done chan struct{}
}

// Start 服务启动回调
func (p *program) Start(s service.Service) error {
	svcLogger.Infof("SV服务正在启动...")
	go p.run()
	return nil
}

// Stop 服务停止回调
func (p *program) Stop(s service.Service) error {
	svcLogger.Infof("SV服务正在停止...")
	close(p.done)
	return nil
}

// run 服务主循环
func (p *program) run() {
	svcLogger.Infof("SV服务已启动，正在后台运行...")

	// 这里可以实现sv的守护进程功能
	// 比如定期监控Supervisor状态、自动重启异常进程等
	// 目前保持简单，只是保持服务运行
	<-p.done
	svcLogger.Infof("SV服务已停止")
}

var (
	svcLogger  service.Logger
	svcService service.Service
	svcProgram *program
)

// NewServiceManager 创建新的服务管理器
func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

// HandleServiceCommand 处理service子命令
func (sm *ServiceManager) HandleServiceCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: sv service <action>")
		fmt.Println()
		fmt.Println("可用操作:")
		fmt.Println("  install   安装sv为系统服务")
		fmt.Println("  uninstall 卸载sv系统服务")
		fmt.Println("  start     启动sv系统服务")
		fmt.Println("  stop      停止sv系统服务")
		fmt.Println("  restart   重启sv系统服务")
		fmt.Println("  status    查看sv服务状态")
		return
	}

	action := args[0]

	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("❌ 获取可执行文件路径失败: %v\n", err)
		return
	}

	// 创建服务配置
	svcConfig := &service.Config{
		Name:        "sv-supervisor-manager",
		DisplayName: "SV Supervisor Manager",
		Description: "现代化Supervisor进程管理工具",
		Executable:  exePath,
		Arguments:   []string{"daemon"},
	}

	// 创建程序实例
	programInstance := &program{}
	svcProgram = programInstance

	// 创建服务实例
	s, err := service.New(programInstance, svcConfig)
	if err != nil {
		fmt.Printf("❌ 创建服务失败: %v\n", err)
		return
	}

	svcService = s

	// 获取日志记录器
	svcLogger, err = s.Logger(nil)
	if err != nil {
		fmt.Printf("❌ 获取日志记录器失败: %v\n", err)
		return
	}

	switch action {
	case "install":
		sm.InstallService()
	case "uninstall":
		sm.UninstallService()
	case "start":
		sm.StartService()
	case "stop":
		sm.StopService()
	case "restart":
		sm.RestartService()
	case "status":
		sm.CheckServiceStatus()
	default:
		fmt.Printf("❌ 未知操作: %s\n\n", action)
		fmt.Println("可用操作: install, uninstall, start, stop, restart, status")
	}
}

// InstallService 安装服务
func (sm *ServiceManager) InstallService() {
	fmt.Println("🔧 正在安装SV系统服务...")

	err := svcService.Install()
	if err != nil {
		fmt.Printf("❌ 安装失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务安装成功!")
	fmt.Println()
	fmt.Println("💡 使用以下命令管理服务:")
	fmt.Println("  启动服务: sv service start")
	fmt.Println("  停止服务: sv service stop")
	fmt.Println("  重启服务: sv service restart")
	fmt.Println("  查看状态: sv service status")
	fmt.Println()
	fmt.Println("🔧 也可以使用系统标准命令:")
	if isLinux() {
		fmt.Println("  sudo systemctl start sv-supervisor-manager")
		fmt.Println("  sudo systemctl enable sv-supervisor-manager")
		fmt.Println("  sudo systemctl status sv-supervisor-manager")
	} else if isWindows() {
		fmt.Println("  net start sv-supervisor-manager")
		fmt.Println("  sc config sv-supervisor-manager start= auto")
	}
}

// UninstallService 卸载服务
func (sm *ServiceManager) UninstallService() {
	fmt.Println("🗑️  正在卸载SV系统服务...")

	err := svcService.Uninstall()
	if err != nil {
		fmt.Printf("❌ 卸载失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务卸载成功!")
}

// StartService 启动服务
func (sm *ServiceManager) StartService() {
	fmt.Println("🚀 正在启动SV系统服务...")

	err := svcService.Start()
	if err != nil {
		fmt.Printf("❌ 启动失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务启动成功!")
}

// StopService 停止服务
func (sm *ServiceManager) StopService() {
	fmt.Println("⏹️  正在停止SV系统服务...")

	err := svcService.Stop()
	if err != nil {
		fmt.Printf("❌ 停止失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务停止成功!")
}

// RestartService 重启服务
func (sm *ServiceManager) RestartService() {
	fmt.Println("🔄 正在重启SV系统服务...")

	err := svcService.Restart()
	if err != nil {
		fmt.Printf("❌ 重启失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务重启成功!")
}

// CheckServiceStatus 检查服务状态
func (sm *ServiceManager) CheckServiceStatus() {
	fmt.Println("📊 正在查询SV系统服务状态...")

	status, err := svcService.Status()
	if err != nil {
		fmt.Printf("❌ 获取状态失败: %v\n", err)
		return
	}

	var statusStr string
	switch status {
	case service.StatusRunning:
		statusStr = "✅ 运行中"
	case service.StatusStopped:
		statusStr = "⏸️ 已停止"
	case service.StatusUnknown:
		statusStr = "❓ 未知状态"
	default:
		statusStr = "⚠️ 其他状态"
	}

	fmt.Printf("SV系统服务状态: %s\n", statusStr)

	if status == service.StatusRunning {
		fmt.Println()
		fmt.Println("💡 服务正在后台运行，可以使用以下命令:")
		fmt.Println("  sv status          # 查看Supervisor进程状态")
		fmt.Println("  sv restart 1       # 重启序号为1的进程")
		fmt.Println("  sv service stop    # 停止SV服务")
	}
}

// RunServiceDaemon 运行服务守护进程
func (sm *ServiceManager) RunServiceDaemon() {
	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		logFatal("获取可执行文件路径失败: %v", err)
	}

	// 创建服务配置
	svcConfig := &service.Config{
		Name:        "sv-supervisor-manager",
		DisplayName: "SV Supervisor Manager",
		Description: "现代化Supervisor进程管理工具",
		Executable:  exePath,
		Arguments:   []string{"daemon"},
	}

	// 创建程序实例
	programInstance := &program{}
	svcProgram = programInstance

	// 创建服务实例
	s, err := service.New(programInstance, svcConfig)
	if err != nil {
		logFatal("创建服务失败: %v", err)
	}

	svcService = s

	// 获取日志记录器
	svcLogger, err = s.Logger(nil)
	if err != nil {
		logFatal("获取日志记录器失败: %v", err)
	}

	// 运行服务
	err = s.Run()
	if err != nil {
		logFatal("服务运行失败: %v", err)
	}
}

// logFatal 记录致命错误并退出
func logFatal(format string, args ...interface{}) {
	if svcLogger != nil {
		svcLogger.Errorf(format, args...)
	}
	fmt.Printf("❌ "+format+"\n", args...)
	os.Exit(1)
}

// isLinux 检查是否为Linux系统
func isLinux() bool {
	return runtime.GOOS == "linux"
}

// isWindows 检查是否为Windows系统
func isWindows() bool {
	return runtime.GOOS == "windows"
}