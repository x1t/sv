package supervisor

import (
	"fmt"
	"os"
	"path/filepath"
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

// createSymlink 创建到 /usr/local/bin 的符号链接
func (sm *ServiceManager) createSymlink() error {
	// 获取当前可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	// 获取文件状态，确认是普通文件
	fileInfo, err := os.Stat(exePath)
	if err != nil {
		return fmt.Errorf("获取可执行文件状态失败: %v", err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("可执行文件路径指向目录: %s", exePath)
	}

	// 目标符号链接路径
	targetPath := "/usr/local/bin/sv"

	// 检查是否有权限写入目标目录
	binDir := filepath.Dir(targetPath)
	// 尝试创建一个临时文件来检查写权限
	testFile := filepath.Join(binDir, ".sv_permissions_test")
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		// 如果无法写入，可能是没有权限，需要以sudo运行
		if os.IsPermission(err) {
			return fmt.Errorf("没有权限写入 %s 目录，请以sudo身份运行: %v", binDir, err)
		}
		// 如果目录不存在，则需要创建
		if os.IsNotExist(err) {
			// 检查父目录权限
			parentDir := filepath.Dir(binDir)
			testParentFile := filepath.Join(parentDir, ".sv_permissions_test")
			if err := os.WriteFile(testParentFile, []byte(""), 0644); err != nil {
				if os.IsPermission(err) {
					return fmt.Errorf("没有权限写入 %s 目录，请以sudo身份运行", parentDir)
				}
			} else {
				// 清理测试文件
				os.Remove(testParentFile)
			}
		}
	} else {
		// 清理测试文件
		os.Remove(testFile)
	}

	// 检查目标路径是否已存在
	if _, err := os.Lstat(targetPath); err == nil {
		// 检查是否已经是符号链接并指向当前可执行文件
		if linkDest, linkErr := os.Readlink(targetPath); linkErr == nil {
			if linkDest == exePath {
				// 已存在且指向正确的路径，无需操作
				return nil
			} else {
				// 存在但指向不同路径，先删除
				if removeErr := os.Remove(targetPath); removeErr != nil {
					return fmt.Errorf("删除现有符号链接失败: %v", removeErr)
				}
			}
		} else {
			// 是普通文件而不是符号链接，需要删除
			if removeErr := os.Remove(targetPath); removeErr != nil {
				return fmt.Errorf("删除现有文件失败: %v", removeErr)
			}
		}
	}

	// 创建 /usr/local/bin 目录（如果不存在）
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 创建符号链接
	if err := os.Symlink(exePath, targetPath); err != nil {
		// 如果权限错误，提示用户以sudo运行
		if os.IsPermission(err) {
			return fmt.Errorf("创建符号链接失败，请以sudo身份运行: %v", err)
		}
		return fmt.Errorf("创建符号链接失败: %v", err)
	}

	fmt.Printf("✅ 已创建符号链接: %s -> %s\n", targetPath, exePath)
	return nil
}

// removeSymlink 删除到 /usr/local/bin 的符号链接
func (sm *ServiceManager) removeSymlink() error {
	targetPath := "/usr/local/bin/sv"

	// 检查目标路径是否存在
	if _, err := os.Lstat(targetPath); os.IsNotExist(err) {
		// 符号链接不存在，无需操作
		return nil
	}

	// 删除符号链接
	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("删除符号链接失败: %v", err)
	}

	fmt.Printf("✅ 已删除符号链接: %s\n", targetPath)
	return nil
}

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

	// 为Unix/Linux系统创建符号链接到/usr/local/bin
	if runtime.GOOS != "windows" {
		fmt.Println("🔗 正在创建符号链接...")
		if err := sm.createSymlink(); err != nil {
			// 如果符号链接创建失败，输出警告但不中断服务安装
			fmt.Printf("⚠️  创建符号链接失败: %v\n", err)
			fmt.Println("💡 提示: 如需将命令添加到PATH，可手动执行: sudo ln -s $(which sv) /usr/local/bin/sv")
		}
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

	// 为Unix/Linux系统移除符号链接
	if runtime.GOOS != "windows" {
		fmt.Println("🔗 正在移除符号链接...")
		if err := sm.removeSymlink(); err != nil {
			// 如果符号链接移除失败，输出警告但不中断服务卸载
			fmt.Printf("⚠️  移除符号链接失败: %v\n", err)
		}
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