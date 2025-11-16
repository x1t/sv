package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kardianos/service"
)

// XML-RPC数据结构
type MethodCall struct {
	XMLName    xml.Name   `xml:"methodCall"`
	MethodName string     `xml:"methodName"`
	Params     []Param    `xml:"params>param"`
}

// handleServiceCommand 处理service子命令
func handleServiceCommand(args []string) {
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
	svcProgram = &program{}

	// 创建服务实例
	s, err := service.New(svcProgram, svcConfig)
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
		installService()
	case "uninstall":
		uninstallService()
	case "start":
		startService()
	case "stop":
		stopService()
	case "restart":
		restartService()
	case "status":
		checkServiceStatus()
	default:
		fmt.Printf("❌ 未知操作: %s\n\n", action)
		fmt.Println("可用操作: install, uninstall, start, stop, restart, status")
	}
}

// installService 安装服务
func installService() {
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

// uninstallService 卸载服务
func uninstallService() {
	fmt.Println("🗑️  正在卸载SV系统服务...")
	
	err := svcService.Uninstall()
	if err != nil {
		fmt.Printf("❌ 卸载失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务卸载成功!")
}

// startService 启动服务
func startService() {
	fmt.Println("🚀 正在启动SV系统服务...")
	
	err := svcService.Start()
	if err != nil {
		fmt.Printf("❌ 启动失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务启动成功!")
}

// stopService 停止服务
func stopService() {
	fmt.Println("⏹️  正在停止SV系统服务...")
	
	err := svcService.Stop()
	if err != nil {
		fmt.Printf("❌ 停止失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务停止成功!")
}

// restartService 重启服务
func restartService() {
	fmt.Println("🔄 正在重启SV系统服务...")
	
	err := svcService.Restart()
	if err != nil {
		fmt.Printf("❌ 重启失败: %v\n", err)
		return
	}

	fmt.Println("✅ SV系统服务重启成功!")
}

// checkServiceStatus 检查服务状态
func checkServiceStatus() {
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

// runServiceDaemon 运行服务守护进程
func runServiceDaemon() {
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
	svcProgram = &program{}

	// 创建服务实例
	s, err := service.New(svcProgram, svcConfig)
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
	return strings.Contains(strings.ToLower(os.Getenv("GOOS")), "linux") || 
		   (os.PathSeparator == '/' && os.Getenv("WINDIR") == "")
}

// isWindows 检查是否为Windows系统  
func isWindows() bool {
	return strings.Contains(strings.ToLower(os.Getenv("GOOS")), "windows") ||
		   os.Getenv("WINDIR") != ""
}

type Param struct {
	Value Value `xml:"value"`
}

type Value struct {
	String  string      `xml:"string,omitempty"`
	Int     int         `xml:"int,omitempty"`
	Boolean bool        `xml:"boolean,omitempty"`
	Array   ArrayValues `xml:"array,omitempty"`
}

// ArrayValues 用于处理数组值
type ArrayValues struct {
	Data ArrayData `xml:"data"`
}

type ArrayData struct {
	Values []Value `xml:"value"`
}

type MethodResponse struct {
	XMLName xml.Name `xml:"methodResponse"`
	Params  []Param  `xml:"params>param"`
	Fault   *Fault   `xml:"fault"`
}

type Fault struct {
	Value struct {
		Struct struct {
			Member []struct {
				Name  string `xml:"name"`
				Value Value  `xml:"value"`
			} `xml:"member"`
		} `xml:"struct"`
	} `xml:"value"`
}

// ProcessInfo 表示一个进程的信息
type ProcessInfo struct {
	Index       int
	Name        string
	Group       string
	State       int
	StateName   string
	PID         int
	Uptime      string
	Description string
	ExitStatus  int
}

// 全局服务管理变量
var (
	svcLogger service.Logger
	svcService service.Service
	svcProgram *program
)

// program 实现service.Interface接口
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

// SupervisorClient Supervisor RPC客户端
type SupervisorClient struct {
	host     string
	username string
	password string
	client   *http.Client
}

// NewSupervisorClient 创建新的Supervisor客户端
func NewSupervisorClient(host, username, password string) *SupervisorClient {
	return &SupervisorClient{
		host:     host,
		username: username,
		password: password,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// EnhancedValue 用于更好地表示XML-RPC响应值
type EnhancedValue struct {
	XMLName xml.Name    `xml:"value"`
	String  string      `xml:"string"`
	Int     int         `xml:"int"`
	Boolean bool        `xml:"boolean"`
	Double  float64     `xml:"double"`
	Array   EnhancedArray `xml:"array"`
	Struct  EnhancedStruct `xml:"struct"`
}

// EnhancedArray 表示XML-RPC数组
type EnhancedArray struct {
	Data EnhancedData `xml:"data"`
}

// EnhancedData 包含数组的数据
type EnhancedData struct {
	Values []EnhancedValue `xml:"value"`
}

// EnhancedStruct 表示XML-RPC结构体
type EnhancedStruct struct {
	Members []EnhancedMember `xml:"member"`
}

// EnhancedMember 表示结构体成员
type EnhancedMember struct {
	Name  string        `xml:"name"`
	Value EnhancedValue `xml:"value"`
}

// call 调用XML-RPC方法
func (sc *SupervisorClient) call(method string, params []interface{}) (interface{}, error) {
	// 构建methodCall
	call := MethodCall{
		MethodName: method,
	}

	if params != nil {
		for _, param := range params {
			var value Value
			switch v := param.(type) {
			case string:
				value.String = v
			case int:
				value.Int = v
			case bool:
				value.Boolean = v
			case []interface{}:  // 处理数组参数
				value.Array = ArrayValues{
					Data: ArrayData{
						Values: make([]Value, len(v)),
					},
				}
				for i, item := range v {
					switch iv := item.(type) {
					case string:
						value.Array.Data.Values[i] = Value{String: iv}
					case int:
						value.Array.Data.Values[i] = Value{Int: iv}
					case bool:
						value.Array.Data.Values[i] = Value{Boolean: iv}
					}
				}
			}
			call.Params = append(call.Params, Param{Value: value})
		}
	}

	// 序列化为XML
	xmlData, err := xml.MarshalIndent(call, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("XML序列化失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", sc.host, bytes.NewBuffer(xmlData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("User-Agent", "sv-supervisor-client/1.0")

	// 添加认证
	if sc.username != "" && sc.password != "" {
		req.SetBasicAuth(sc.username, sc.password)
	}

	// 发送请求
	resp, err := sc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP错误: %d, %s", resp.StatusCode, string(body))
	}

	// 为了正确解析响应，我们需要使用EnhancedValue结构
	// 重新定义MethodResponse使用EnhancedValue
	enhanedResponse := struct {
		XMLName xml.Name      `xml:"methodResponse"`
		Params  []struct {
			Value EnhancedValue `xml:"param>value"`
		} `xml:"params"`
		Fault *struct {
			Value struct {
				Struct struct {
					Member []struct {
						Name  string        `xml:"name"`
						Value EnhancedValue `xml:"value"`
					} `xml:"member"`
				} `xml:"struct"`
			} `xml:"value"`
		} `xml:"fault"`
	}{}

	if err := xml.Unmarshal(body, &enhanedResponse); err != nil {
		return nil, fmt.Errorf("XML解析失败: %v", err)
	}

	// 检查错误
	if enhanedResponse.Fault != nil {
		for _, member := range enhanedResponse.Fault.Value.Struct.Member {
			if member.Name == "faultString" {
				return nil, fmt.Errorf("XML-RPC错误: %s", member.Value.String)
			}
		}
		return nil, fmt.Errorf("未知XML-RPC错误")
	}

	if len(enhanedResponse.Params) == 0 {
		return nil, nil
	}

	// 解析并返回数据
	return parseEnhancedValue(enhanedResponse.Params[0].Value), nil
}

// parseEnhancedValue 将EnhancedValue转换为Go类型
func parseEnhancedValue(ev EnhancedValue) interface{} {
	if ev.String != "" {
		return ev.String
	}
	if ev.Int != 0 || (ev.String == "" && !ev.Boolean && ev.Double == 0 && ev.Array.Data.Values == nil && ev.Struct.Members == nil) {
		// 如果int不是0，或者这是唯一设置的字段，则返回int
		return ev.Int
	}
	if ev.Boolean {
		return ev.Boolean
	}
	if ev.Double != 0 {
		return ev.Double
	}
	if ev.Array.Data.Values != nil {
		// 解析数组
		result := make([]interface{}, len(ev.Array.Data.Values))
		for i, val := range ev.Array.Data.Values {
			result[i] = parseEnhancedValue(val)
		}
		return result
	}
	if ev.Struct.Members != nil {
		// 解析结构体
		result := make(map[string]interface{})
		for _, member := range ev.Struct.Members {
			result[member.Name] = parseEnhancedValue(member.Value)
		}
		return result
	}
	return nil
}

// 更新 getAllProcesses 方法使用新的解析方法
func (sc *SupervisorClient) GetAllProcesses() ([]ProcessInfo, error) {
	// 首先尝试使用RPC调用
	result, err := sc.call("supervisor.getAllProcessInfo", nil)
	if err != nil {
		// 如果RPC调用失败，回退到使用命令行方式
		fmt.Printf("⚠️  RPC调用失败: %v, 尝试使用命令行工具\n", err)
		return sc.getAllProcessesViaCommand()
	}

	// 将结果转换为适当的类型
	if processesData, ok := result.([]interface{}); ok {
		processes := make([]ProcessInfo, len(processesData))
		for i, procData := range processesData {
			if procMap, ok := procData.(map[string]interface{}); ok {
				processes[i] = parseProcessInfoFromMap(procMap, i+1)
			}
		}
		return processes, nil
	}

	fmt.Println("⚠️  无法解析RPC响应数据，使用命令行工具作为回退")
	return sc.getAllProcessesViaCommand()
}

// parseProcessInfoFromMap 从map解析进程信息
func parseProcessInfoFromMap(procMap map[string]interface{}, index int) ProcessInfo {
	name := ""
	if n, ok := procMap["name"]; ok {
		if s, ok := n.(string); ok {
			name = s
		}
	}

	group := ""
	if g, ok := procMap["group"]; ok {
		if s, ok := g.(string); ok {
			group = s
		}
	}

	state := 0
	if s, ok := procMap["state"]; ok {
		if f, ok := s.(float64); ok {
			state = int(f)
		} else if i, ok := s.(int); ok {
			state = i
		}
	}

	stateName := ""
	if sn, ok := procMap["statename"]; ok {
		if s, ok := sn.(string); ok {
			stateName = s
		}
	}

	pid := 0
	if p, ok := procMap["pid"]; ok {
		if f, ok := p.(float64); ok {
			pid = int(f)
		} else if i, ok := p.(int); ok {
			pid = i
		}
	}

	description := ""
	if d, ok := procMap["description"]; ok {
		if s, ok := d.(string); ok {
			description = s
		}
	}

	// 生成完整进程名称 (group:name)
	fullName := name
	if group != "" && name != "" {
		fullName = group + ":" + name
	}

	// 生成状态描述
	var uptime string
	if pid > 0 {
		// 如果有PID，尝试从描述中提取运行时间
		uptime = description
	} else {
		uptime = "已停止"
	}

	return ProcessInfo{
		Index:       index,
		Name:        fullName,  // 使用完整进程名称
		Group:       group,
		State:       state,
		StateName:   stateName,
		PID:         pid,
		Uptime:      uptime,
		Description: getStateIcon(state),
		ExitStatus:  0,
	}
}


// ProcessInfoRPC 定义从RPC获取的进程信息结构
type ProcessInfoRPC struct {
	Name        string  `xml:"name"`
	Group       string  `xml:"group"`
	Start       float64 `xml:"start"`
	Stop        float64 `xml:"stop"`
	Now         float64 `xml:"now"`
	State       int     `xml:"state"`
	StateName   string  `xml:"statename"`
	SpawnErr    string  `xml:"spawnerr"`
	ExitStatus  int     `xml:"exitstatus"`
	Logfile     string  `xml:"logfile"`
	StdoutLogfile string `xml:"stdout_logfile"`
	StderrLogfile string `xml:"stderr_logfile"`
	Pid         int     `xml:"pid"`
	Description string  `xml:"description"`
}

// parseProcessInfoFromValue 从Value解析进程信息
func parseProcessInfoFromValue(procValue Value, index int) ProcessInfo {
	// 目前的解析方法是基于手工解析Value结构体
	// 但更好的方法是重新设计XML解析结构以直接处理Supervisor的响应
	// 下面是一个更完整的解析方法

	// 对于结构体，Supervisor在XML-RPC响应中使用了特定的格式
	// 我们需要遍历Value数组来找到键值对
	// 在当前的XML结构定义下，这需要一个更复杂的方法

	// 为简单起见，暂时使用回退逻辑，但我们会改进call方法以返回更好的数据结构
	// 更好的方法是修改call函数来直接解析响应并返回map
	name := ""
	group := ""
	state := 0
	stateName := ""
	pid := 0
	description := ""

	// 这里需要更完整的解析逻辑，但暂时依赖命令行回退
	// 一旦我们有了完整的解析器，这部分将被替换
	return ProcessInfo{
		Index:       index,
		Name:        name,
		Group:       group,
		State:       state,
		StateName:   stateName,
		PID:         pid,
		Uptime:      description,
		Description: getStateIcon(state),
		ExitStatus:  0,
	}
}

// 为了更好地处理RPC响应，我们需要修改call方法以返回可解析的数据结构
// 但当前的实现会在出错时自动回退到命令行方法，这也是一种合理的实现


// parseProcessInfoRPC 从RPC响应解析进程信息
func parseProcessInfoRPC(procMap map[string]interface{}, index int) ProcessInfo {
	// 由于valueToMap函数可能不能完全解析复杂结构
	// 我们需要在GetAllProcesses中直接处理Value结构
	// 这个函数暂时保留，但可能需要重构
	name := ""
	group := ""
	state := 0
	stateName := ""
	pid := 0
	description := ""

	return ProcessInfo{
		Index:       index,
		Name:        name,
		Group:       group,
		State:       state,
		StateName:   stateName,
		PID:         pid,
		Uptime:      description,
		Description: getStateIcon(state),
		ExitStatus:  0,
	}
}

// valueToMap 将Value转换为map[string]interface{}（用于解析RPC响应）
func valueToMap(value Value) map[string]interface{} {
	result := make(map[string]interface{})

	if value.String != "" {
		return map[string]interface{}{"value": value.String}
	}
	if value.Int != 0 || (value.String == "" && value.Boolean == false && len(value.Array.Data.Values) == 0) {
		// 处理Int为0的情况
		return map[string]interface{}{"value": value.Int}
	}
	if value.Boolean {
		return map[string]interface{}{"value": value.Boolean}
	}
	if len(value.Array.Data.Values) > 0 {
		// 处理数组
		arrayResult := make([]interface{}, len(value.Array.Data.Values))
		for i, v := range value.Array.Data.Values {
			arrayResult[i] = extractValueContent(v)
		}
		return map[string]interface{}{"array": arrayResult}
	}

	return result
}

// extractValueContent 提取Value中的内容
func extractValueContent(value Value) interface{} {
	if value.String != "" {
		return value.String
	}
	if value.Boolean {
		return value.Boolean
	}
	if len(value.Array.Data.Values) > 0 {
		// 处理嵌套数组或结构
		arrayResult := make([]interface{}, len(value.Array.Data.Values))
		for i, v := range value.Array.Data.Values {
			arrayResult[i] = extractValueContent(v)
		}
		return arrayResult
	}
	// 默认返回int（包括0）
	return value.Int
}

// getAllProcessesViaCommand 通过命令行方式获取进程信息（回退方案）
func (sc *SupervisorClient) getAllProcessesViaCommand() ([]ProcessInfo, error) {
	// 尝试使用 supervisorctl 命令获取真实数据
	fmt.Println("正在获取Supervisor进程状态...")
	cmd := exec.Command("supervisorctl", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 即使有错误，output中通常也包含有用的信息
		outputStr := string(output)
		if strings.Contains(outputStr, "RUNNING") || strings.Contains(outputStr, "STOPPED") {
			fmt.Println("⚠️  获取到进程数据，但可能存在一些状态问题")
			return parseSupervisorctlOutput(outputStr), nil
		}
		fmt.Printf("❌ supervisorctl 命令失败: %v\n", err)
		return nil, fmt.Errorf("无法获取进程信息: supervisorctl 命令失败")
	}

	fmt.Println("✅ 成功获取真实进程数据")
	return parseSupervisorctlOutput(string(output)), nil
}

// getStringValue 从interface{}获取string值
func getStringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// getIntValue 从interface{}获取int值  
func getIntValue(v interface{}) int {
	if i, ok := v.(int); ok {
		return i
	}
	return 0
}

// formatUptime 格式化运行时间（秒转为可读格式）
func formatUptime(seconds int) string {
	if seconds == 0 {
		return "已停止"
	}
	
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	
	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时%d分", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%d分钟", minutes)
	} else {
		return "不到1分钟"
	}
}

// parseSupervisorctlOutput 解析 supervisorctl status 命令的输出
func parseSupervisorctlOutput(output string) []ProcessInfo {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	processes := make([]ProcessInfo, 0, len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析行格式: "group:name  state    pid  uptime"
		// 例如: "agent:agent_00                   RUNNING   pid 988995, uptime 30 days, 16:17:38"

		// 使用正则表达式或更精确的方式提取进程名称（第一个字段）
		// 我们需要确保第一个字段是完整的名称（包含冒号）
		lineCopy := strings.TrimSpace(line)
		if lineCopy == "" {
			continue
		}

		// 找到第一个非空格序列作为进程名
		var name string
		var rest string
		for j, char := range lineCopy {
			if char == ' ' {
				name = strings.TrimSpace(lineCopy[:j])
				rest = strings.TrimSpace(lineCopy[j+1:])
				break
			}
		}
		if name == "" {
			name = lineCopy // 如果整行都是名称
		}

		// 解析剩余部分
		restFields := strings.Fields(rest)
		if len(restFields) < 2 {
			continue
		}

		stateName := restFields[0]
		pid := 0
		uptime := ""

		// 解析PID和运行时间
		for j, field := range restFields {
			if field == "pid" && j+1 < len(restFields) {
				pidStr := strings.TrimSuffix(restFields[j+1], ",")
				if p, err := strconv.Atoi(pidStr); err == nil {
					pid = p
				}
			}
			if field == "uptime" && j+1 < len(restFields) {
				// 组合uptime后面的所有字段
				uptimeFields := restFields[j+1:]
				for k, uptimeField := range uptimeFields {
					if strings.HasSuffix(uptimeField, ",") {
						uptimeFields[k] = strings.TrimSuffix(uptimeField, ",")
					}
				}
				uptime = strings.Join(uptimeFields, " ")
				break
			}
		}

		state := getStateValue(stateName)

		processes = append(processes, ProcessInfo{
			Index:       i + 1,
			Name:        name, // 完整的进程名称，例如 "agent:agent_00"
			State:       state,
			StateName:   stateName,
			PID:         pid,
			Uptime:      uptime,
			Description: getStateIcon(state),
			ExitStatus:  0,
		})
	}

	return processes
}

// getStateValue 根据状态名称获取状态代码
func getStateValue(stateName string) int {
	switch strings.ToUpper(stateName) {
	case "RUNNING":
		return 20
	case "STARTING":
		return 10
	case "STOPPING":
		return 30
	case "STOPPED":
		return 0
	case "FATAL":
		return 100
	case "BACKOFF":
		return 200
	default:
		return 0
	}
}

// ControlProcess 控制进程（启动/停止/重启）
func (sc *SupervisorClient) ControlProcess(action, processName string) error {
	var methodName string
	switch action {
	case "start":
		methodName = "supervisor.startProcess"
	case "stop":
		methodName = "supervisor.stopProcess"
	case "restart":
		// 重启是先停止再启动
		err := sc.ControlProcess("stop", processName)
		if err != nil {
			return fmt.Errorf("停止进程失败: %v", err)
		}
		time.Sleep(1 * time.Second) // 等待一下再启动
		return sc.ControlProcess("start", processName)
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}

	// 调用RPC方法
	_, err := sc.call(methodName, []interface{}{processName, true}) // true表示wait for the process to finish the action

	if err != nil {
		// 如果RPC调用失败，回退到使用命令行方式
		fmt.Printf("⚠️  RPC调用失败: %v, 尝试使用命令行工具\n", err)
		return sc.controlProcessViaCommand(action, processName)
	}

	return nil
}

// controlProcessViaCommand 通过命令行方式控制进程（回退方案）
func (sc *SupervisorClient) controlProcessViaCommand(action, processName string) error {
	var command string
	switch action {
	case "start":
		command = "start"
	case "stop":
		command = "stop"
	case "restart":
		// 重启是先停止再启动
		err := sc.controlProcessViaCommand("stop", processName)
		if err != nil {
			return fmt.Errorf("停止进程失败: %v", err)
		}
		time.Sleep(1 * time.Second) // 等待一下再启动
		return sc.controlProcessViaCommand("start", processName)
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}

	// 使用 supervisorctl 命令控制进程
	cmd := exec.Command("supervisorctl", command, processName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s进程失败: %v", action, err)
	}

	// 检查输出是否成功
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR") {
		return fmt.Errorf("%s进程失败: %s", action, outputStr)
	}

	return nil
}

// DisplayStatus 显示进程状态
func DisplayStatus(processes []ProcessInfo) {
	if len(processes) == 0 {
		fmt.Println("没有找到任何进程")
		return
	}

	fmt.Printf("%-4s %-20s %-10s %-8s %-15s %s\n", "序号", "名称", "状态", "PID", "运行时间", "描述")
	fmt.Println(strings.Repeat("-", 80))

	for _, proc := range processes {
		statusColor := getColorByState(proc.State)
		pidStr := strconv.Itoa(proc.PID)
		if proc.PID == 0 {
			pidStr = "-"
		}

		fmt.Printf("%-4d %-20s %s%-10s%s %-8s %-15s %s\n",
			proc.Index,
			proc.Name,
			statusColor, proc.StateName, "\x1b[0m",
			pidStr,
			proc.Uptime,
			getStateIcon(proc.State))
	}
}

// getColorByState 根据状态获取颜色
func getColorByState(state int) string {
	switch state {
	case 20: // RUNNING
		return "\x1b[32m" // 绿色
	case 10: // STARTING
		return "\x1b[33m" // 黄色
	case 30: // STOPPING
		return "\x1b[33m" // 黄色
	case 100: // FATAL
		return "\x1b[31m" // 红色
	default:
		return "\x1b[37m" // 白色
	}
}

// getStateIcon 获取状态图标
func getStateIcon(state int) string {
	switch state {
	case 20: // RUNNING
		return "✅ 运行中"
	case 10: // STARTING
		return "🚀 启动中"
	case 30: // STOPPING
		return "⏹️ 停止中"
	case 0: // STOPPED
		return "⏸️ 已停止"
	case 100: // FATAL
		return "❌ 致命错误"
	case 200: // BACKOFF
		return "⚠️ 重试中"
	default:
		return "❓ 未知"
	}
}

// ParseProcessIndices 解析进程索引参数
func ParseProcessIndices(args []string, processes []ProcessInfo) ([]string, error) {
	var names []string
	var invalidIndices []int

	for _, arg := range args {
		// 检查是否为范围格式 (如: 1-5)
		if strings.Contains(arg, "-") {
			parts := strings.Split(arg, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("无效的范围格式: %s", arg)
			}

			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("无效的范围数字: %s", arg)
			}

			if start < 1 || end > len(processes) || start > end {
				return nil, fmt.Errorf("范围超出有效区间: %s", arg)
			}

			for i := start; i <= end; i++ {
				names = append(names, processes[i-1].Name)
			}
		} else {
			// 单个数字
			index, err := strconv.Atoi(arg)
			if err != nil {
				// 如果不是数字，检查是否为进程名（可能为简写或完整名称）
				// 首先检查是否为完整名称（包含冒号）
				if strings.Contains(arg, ":") {
					// 这是一个完整的进程名，直接添加
					names = append(names, arg)
				} else {
					// 这是一个简写名，尝试找到匹配的完整进程名
					found := false
					for _, proc := range processes {
						// 检查是否与组名:进程名匹配
						if strings.Contains(proc.Name, ":") && (proc.Name == arg ||
							strings.Split(proc.Name, ":")[1] == arg) {
							names = append(names, proc.Name)
							found = true
							break
						}
						// 或者直接匹配整个进程名
						if proc.Name == arg {
							names = append(names, proc.Name)
							found = true
							break
						}
					}
					if !found {
						// 如果找不到完全匹配，将原参数添加进去，让后续调用处理错误
						names = append(names, arg)
					}
				}
				continue
			}

			if index < 1 || index > len(processes) {
				invalidIndices = append(invalidIndices, index)
				continue
			}

			names = append(names, processes[index-1].Name)
		}
	}

	if len(invalidIndices) > 0 {
		return nil, fmt.Errorf("无效的进程序号: %v (有效范围: 1-%d)", invalidIndices, len(processes))
	}

	return names, nil
}

// readSupervisorConfig 读取supervisor配置获取连接信息
func readSupervisorConfig() (host, username, password string) {
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

// detectAndEnableRPC 检测并开启Supervisor RPC功能
func detectAndEnableRPC() error {
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
			enabled, err := hasInetHTTPServer(configPath)
			if err != nil {
				fmt.Printf("⚠️  检查配置文件失败: %v\n", err)
				continue
			}

			if !enabled {
				// 如果没有启用inet_http_server，则添加配置
				fmt.Printf("🔧 未发现inet_http_server配置，正在添加...\n")
				err = addInetHTTPServerConfig(configPath)
				if err != nil {
					fmt.Printf("❌ 添加inet_http_server配置失败: %v\n", err)
					continue
				} else {
					fmt.Printf("✅ inet_http_server配置已添加\n")
					configModified = true
				}
			}

			// 检查RPC接口配置
			rpcEnabled, err := hasRPCInterface(configPath)
			if err != nil {
				fmt.Printf("⚠️  检查RPC接口配置失败: %v\n", err)
				continue
			}

			if !rpcEnabled {
				// 如果没有启用RPC接口，则添加配置
				fmt.Printf("🔧 未发现RPC接口配置，正在添加...\n")
				err = addRPCInterfaceConfig(configPath)
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
				if err := restartSupervisor(); err != nil {
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

// restartSupervisor 尝试重启Supervisor服务
func restartSupervisor() error {
	// 尝试使用systemctl重启supervisor (在大多数Linux系统上)
	cmd := exec.Command("systemctl", "restart", "supervisor")
	if err := cmd.Run(); err != nil {
		// 如果systemctl失败，尝试使用service命令
		cmd = exec.Command("service", "supervisor", "restart")
		if err := cmd.Run(); err != nil {
			// 如果还是失败，尝试直接kill supervisord进程，让系统服务管理器重启它
			return err
		}
	}
	return nil
}

// hasInetHTTPServer 检查配置文件是否已启用inet_http_server
func hasInetHTTPServer(configPath string) (bool, error) {
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

// hasRPCInterface 检查配置文件是否已启用RPC接口
func hasRPCInterface(configPath string) (bool, error) {
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

// addInetHTTPServerConfig 添加inet_http_server配置
func addInetHTTPServerConfig(configPath string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	contentStr := string(content)

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
			return os.WriteFile(configPath, []byte(newContent), 0644)
		}
	}

	// 如果没找到unix_http_server或位置不明确，则添加到文件开头
	newContent := inetConfig + contentStr
	return os.WriteFile(configPath, []byte(newContent), 0644)
}

// addRPCInterfaceConfig 添加RPC接口配置
func addRPCInterfaceConfig(configPath string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// 添加RPC接口配置
	rpcConfig := `
[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

`

	// 检查是否存在[supervisorctl]段，如果存在则在其前添加
	supervisorctlPos := strings.Index(contentStr, "[supervisorctl]")
	if supervisorctlPos != -1 {
		newContent := contentStr[:supervisorctlPos] + rpcConfig + contentStr[supervisorctlPos:]
		return os.WriteFile(configPath, []byte(newContent), 0644)
	}

	// 如果没找到supervisorctl位置，则添加到文件末尾
	newContent := contentStr + rpcConfig
	return os.WriteFile(configPath, []byte(newContent), 0644)
}

// run 程序运行逻辑，提取出来便于测试
func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	command := os.Args[1]
	args := os.Args[2:]

	// 检查是否是service子命令
	if command == "service" {
		handleServiceCommand(args)
		return nil
	}

	// 对于与Supervisor交互的命令，检测并开启RPC功能
	if command == "status" || command == "list" || command == "start" || command == "stop" || command == "restart" {
		// 尝试检测并开启RPC功能
		err := detectAndEnableRPC()
		if err != nil {
			fmt.Printf("⚠️  检测/开启RPC功能时出错: %v\n", err)
			// 继续执行，因为可能RPC已在其他地方配置，或者会回退到命令行模式
		}
	}

	// 读取Supervisor连接配置
	host, username, password := readSupervisorConfig()

	// 创建Supervisor客户端
	client := NewSupervisorClient(host, username, password)

	switch command {
	case "status", "list":
		showStatus(client)
	case "start", "stop", "restart":
		if len(args) == 0 {
			fmt.Printf("用法: sv %s <进程序号|进程名称|范围>\n", command)
			fmt.Println("示例:")
			fmt.Printf("  sv %s 1        # 控制序号为1的进程\n", command)
			fmt.Printf("  sv %s myapp    # 控制名为myapp的进程\n", command)
			fmt.Printf("  sv %s 1 3 5   # 控制多个进程\n", command)
			fmt.Printf("  sv %s 1-5     # 控制序号1到5的进程\n", command)
			return fmt.Errorf("参数不足")
		}
		controlProcesses(client, command, args)
	case "daemon":
		// 守护进程模式，由系统服务管理器调用
		runServiceDaemon()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("未知命令: %s\n\n", command)
		printUsage()
		return fmt.Errorf("未知命令: %s", command)
	}
	
	return nil
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func showStatus(client *SupervisorClient) {
	processes, err := client.GetAllProcesses()
	if err != nil {
		fmt.Printf("⚠️  获取进程状态失败: %v\n", err)
		fmt.Println("这是演示模式，显示模拟数据:")
		processes, _ = client.GetAllProcesses()
	}

	fmt.Printf("\n🔍 Supervisor进程状态 (共%d个进程)\n", len(processes))
	fmt.Println(strings.Repeat("=", 80))
	DisplayStatus(processes)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\n💡 提示: 使用 'sv start/stop/restart <序号>' 来控制进程")
	fmt.Println("🔧 配置: 设置SUPERVISOR_HOST环境变量来指定Supervisor地址")
}

func controlProcesses(client *SupervisorClient, action string, args []string) {
	// 首先获取所有进程信息
	processes, err := client.GetAllProcesses()
	if err != nil {
		fmt.Printf("⚠️  获取进程信息失败: %v\n", err)
		fmt.Println("这是演示模式，将使用模拟数据:")
		processes, _ = client.GetAllProcesses()
	}

	// 解析进程名称
	processNames, err := ParseProcessIndices(args, processes)
	if err != nil {
		fmt.Printf("❌ 解析进程参数失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🎯 正在执行 '%s' 操作...\n", action)

	// 执行控制操作
	var successCount, failCount int
	for _, name := range processNames {
		fmt.Printf("  %s 进程 %s ... ", getActionIcon(action), name)
		err := client.ControlProcess(action, name)
		if err != nil {
			fmt.Printf("❌ 失败 (%v)\n", err)
			failCount++
		} else {
			fmt.Printf("✅ 成功\n")
			successCount++
		}
	}

	fmt.Printf("\n📊 操作完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)
	
	if failCount > 0 {
		fmt.Println("💡 提示: 请确保Supervisor正在运行并且配置正确")
	}
}

func getActionIcon(action string) string {
	switch action {
	case "start":
		return "🚀 启动"
	case "stop":
		return "⏹️ 停止"
	case "restart":
		return "🔄 重启"
	default:
		return "⚙️ 操作"
	}
}

func printUsage() {
	fmt.Println("sv - Supervisor进程管理工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  sv status                    # 显示所有进程状态")
	fmt.Println("  sv list                     # 显示所有进程状态（同status）")
	fmt.Println("  sv start <进程>              # 启动进程")
	fmt.Println("  sv stop <进程>               # 停止进程") 
	fmt.Println("  sv restart <进程>            # 重启进程")
	fmt.Println("  sv service <action>          # 服务管理")
	fmt.Println()
	fmt.Println("进程参数支持:")
	fmt.Println("  序号      sv restart 1       # 使用序号")
	fmt.Println("  名称      sv restart myapp   # 使用进程名")
	fmt.Println("  多个      sv restart 1 3 5   # 多个进程")
	fmt.Println("  范围      sv restart 1-5     # 序号范围")
	fmt.Println()
	fmt.Println("服务管理:")
	fmt.Println("  install   安装sv为系统服务")
	fmt.Println("  uninstall 卸载sv系统服务")
	fmt.Println("  start     启动sv系统服务")
	fmt.Println("  stop      停止sv系统服务")
	fmt.Println("  restart   重启sv系统服务")
	fmt.Println("  status    查看sv服务状态")
	fmt.Println()
	fmt.Println("环境变量:")
	fmt.Println("  SUPERVISOR_HOST              # Supervisor RPC地址 (默认: http://localhost:9001/RPC2)")
	fmt.Println("  SUPERVISOR_USER              # 用户名 (可选)")
	fmt.Println("  SUPERVISOR_PASSWORD          # 密码 (可选)")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  sv status                    # 查看所有进程状态")
	fmt.Println("  sv list                      # 查看所有进程状态（同status）")
	fmt.Println("  sv restart 1                 # 重启序号为1的进程")
	fmt.Println("  sv stop 2 4 6               # 停止序号2、4、6的进程")
	fmt.Println("  sv start 1-3                # 启动序号1到3的进程")
	fmt.Println("  sv restart myapp nginx      # 重启指定名称的进程")
	fmt.Println("  sv service install           # 安装为系统服务")
	fmt.Println("  sv service start             # 启动系统服务")
}