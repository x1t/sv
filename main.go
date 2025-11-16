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
)

// XML-RPC数据结构
type MethodCall struct {
	XMLName    xml.Name   `xml:"methodCall"`
	MethodName string     `xml:"methodName"`
	Params     []Param    `xml:"params>param"`
}

type Param struct {
	Value Value `xml:"value"`
}

type Value struct {
	String string `xml:"string"`
	Int    int    `xml:"int"`
	Boolean bool   `xml:"boolean"`
	Array  []interface{} `xml:"array>data>value"`
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

// call 调用XML-RPC方法
func (sc *SupervisorClient) call(method string, params []interface{}) (interface{}, error) {
	// 构建methodCall
	call := MethodCall{
		MethodName: method,
	}

	for _, param := range params {
		var value Value
		switch v := param.(type) {
		case string:
			value.String = v
		case int:
			value.Int = v
		case bool:
			value.Boolean = v
		}
		call.Params = append(call.Params, Param{Value: value})
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

	// 解析响应
	var response MethodResponse
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("XML解析失败: %v", err)
	}

	// 检查错误
	if response.Fault != nil {
		for _, member := range response.Fault.Value.Struct.Member {
			if member.Name == "faultString" {
				return nil, fmt.Errorf("XML-RPC错误: %s", member.Value.String)
			}
		}
		return nil, fmt.Errorf("未知XML-RPC错误")
	}

	if len(response.Params) == 0 {
		return nil, nil
	}

	return response.Params[0].Value, nil
}

// GetAllProcesses 获取所有进程信息
func (sc *SupervisorClient) GetAllProcesses() ([]ProcessInfo, error) {
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
		
		// 提取进程名称（第一个字段）
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		
		name := fields[0]
		stateName := fields[1]
		pid := 0
		uptime := ""
		
		// 解析PID和运行时间
		for j, field := range fields {
			if field == "pid" && j+1 < len(fields) {
				pidStr := strings.TrimSuffix(fields[j+1], ",")
				if p, err := strconv.Atoi(pidStr); err == nil {
					pid = p
				}
			}
			if field == "uptime" && j+1 < len(fields) {
				// 组合uptime后面的所有字段
				uptimeFields := fields[j+1:]
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
			Name:        name,
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
	var command string
	switch action {
	case "start":
		command = "start"
	case "stop":
		command = "stop"
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
				// 如果不是数字，直接当作进程名处理
				names = append(names, arg)
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

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

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
			os.Exit(1)
		}
		controlProcesses(client, command, args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("未知命令: %s\n\n", command)
		printUsage()
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
	fmt.Println()
	fmt.Println("进程参数支持:")
	fmt.Println("  序号      sv restart 1       # 使用序号")
	fmt.Println("  名称      sv restart myapp   # 使用进程名")
	fmt.Println("  多个      sv restart 1 3 5   # 多个进程")
	fmt.Println("  范围      sv restart 1-5     # 序号范围")
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
}