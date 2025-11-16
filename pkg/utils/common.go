package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

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

// DisplayStatus 显示进程状态
func DisplayStatus(processes []ProcessInfo) {
	if len(processes) == 0 {
		fmt.Println("没有找到任何进程")
		return
	}

	// 创建使用Unicode直线边框的表格（与PM2一样的完美四边形边框）
	// 使用 WithTrimSpace(tw.Off) 来正确处理中文字符宽度，避免对齐问题
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleLight), // 使用直线Unicode边框（一致的┼分隔符）
		})),
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignCenter},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft}, // 默认左对齐
			},
		}),
		tablewriter.WithTrimSpace(tw.Off), // 关闭空格修剪，正确处理中文字符宽度
	)

	// 设置表头
	table.Header([]string{"序号", "名称", "状态", "PID", "运行时间"})

	// 准备数据
	var data [][]any
	for _, proc := range processes {
		pidStr := strconv.Itoa(proc.PID)
		if proc.PID == 0 {
			pidStr = "-"
		}

		// 添加带颜色的状态
		coloredStateName := fmt.Sprintf("%s%s%s", GetColorByState(proc.State), proc.StateName, "\x1b[0m")
		row := []any{
			proc.Index,
			proc.Name,
			coloredStateName, // 带颜色的状态
			pidStr,
			proc.Uptime,
		}
		data = append(data, row)
	}

	// 批量添加数据并渲染
	table.Bulk(data)
	table.Render()
}

// GetColorByState 根据状态获取颜色
func GetColorByState(state int) string {
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

// GetStateIcon 获取状态图标
func GetStateIcon(state int) string {
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

// GetStateValue 根据状态名称获取状态代码
func GetStateValue(stateName string) int {
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

// ProcessUptimeString 处理运行时间字符串，只保留时间部分（公开版本）
func ProcessUptimeString(uptime string) string {
	return processUptimeString(uptime)
}

// processUptimeString 处理运行时间字符串，只保留时间部分
func processUptimeString(uptime string) string {
	// 提取 "X days, X:X:X" 或 "X:X:X" 格式的运行时间
	// 例如：从 "30 days, 16:17:38" 中提取 "30天16小时17分钟38秒"
	// 或者从 "1:59:48" 中提取 "1小时59分钟48秒"

	// 检查是否包含 "days" 信息
	if strings.Contains(uptime, "days") {
		parts := strings.Split(uptime, "days")
		if len(parts) >= 2 {
			// 取逗号后的部分，即时间部分
			timePart := strings.TrimSpace(parts[1])
			if strings.HasPrefix(timePart, ",") {
				timePart = strings.TrimSpace(timePart[1:])
			}
			// 解析 "HH:MM:SS" 格式
			return parseTimeFormat(timePart)
		}
	}

	// 直接解析 "HH:MM:SS" 或 "MM:SS" 格式
	return parseTimeFormat(uptime)
}

// parseTimeFormat 解析时间格式
func parseTimeFormat(timeStr string) string {
	// 移除可能的额外描述信息，只保留 HH:MM:SS 格式
	timeStr = strings.TrimSpace(timeStr)

	// 只取第一个部分（时间部分）
	parts := strings.Split(timeStr, " ")
	timePart := parts[0]

	// 按冒号分割
	timeComponents := strings.Split(timePart, ":")

	if len(timeComponents) == 3 {
		// HH:MM:SS 格式
		hours, err1 := strconv.Atoi(timeComponents[0])
		mins, err2 := strconv.Atoi(timeComponents[1])
		secs, err3 := strconv.Atoi(timeComponents[2])

		if err1 == nil && err2 == nil && err3 == nil {
			if hours > 0 {
				return fmt.Sprintf("%d小时%02d分钟%02d秒", hours, mins, secs)
			} else {
				return fmt.Sprintf("%02d分钟%02d秒", mins, secs)
			}
		}
	} else if len(timeComponents) == 2 {
		// MM:SS 格式
		mins, err1 := strconv.Atoi(timeComponents[0])
		secs, err2 := strconv.Atoi(timeComponents[1])

		if err1 == nil && err2 == nil {
			return fmt.Sprintf("%02d分钟%02d秒", mins, secs)
		}
	}

	// 如果解析失败，返回原始字符串
	return timeStr
}

// GetStringValue 从interface{}获取string值
func GetStringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetIntValue 从interface{}获取int值
func GetIntValue(v interface{}) int {
	if i, ok := v.(int); ok {
		return i
	}
	return 0
}

// FormatUptime 格式化运行时间（秒转为可读格式）
func FormatUptime(seconds int) string {
	if seconds == 0 {
		return "已停止"
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secondsRemaining := seconds % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d小时%02d分%02d秒", days, hours, minutes, secondsRemaining)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时%02d分%02d秒", hours, minutes, secondsRemaining)
	} else if minutes > 0 {
		return fmt.Sprintf("%02d分钟%02d秒", minutes, secondsRemaining)
	} else {
		return fmt.Sprintf("%02d秒", secondsRemaining)
	}
}

// GetActionIcon 获取操作图标
func GetActionIcon(action string) string {
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

// IsValidProcessLine 检查行是否符合进程状态行的基本格式
func IsValidProcessLine(name string, rest string) bool {
	// 检查进程名是否符合基本格式（包含字母数字下划线等）
	if len(name) == 0 {
		return false
	}

	// 检查剩余部分是否包含常见的状态值
	restLower := strings.ToLower(rest)
	commonStates := []string{"running", "stopped", "starting", "stopping", "fatal", "backoff"}

	for _, state := range commonStates {
		if strings.Contains(restLower, state) {
			return true
		}
	}

	// 如果没有找到常见的状态，但rest包含pid或uptime等关键词，也认为是有效的
	if strings.Contains(restLower, "pid") || strings.Contains(restLower, "uptime") ||
	   strings.Contains(restLower, "not started") || strings.Contains(restLower, "exited") {
		return true
	}

	return false
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

			// 添加边界检查以避免索引越界
			if index-1 >= len(processes) {
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

// ParseSupervisorctlOutput 解析 supervisorctl status 命令的输出
func ParseSupervisorctlOutput(output string) []ProcessInfo {
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
		parts := strings.SplitN(lineCopy, " ", 2) // 只分割为两部分，确保进程名中的空格被保留
		if len(parts) >= 1 {
			name = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				rest = strings.TrimSpace(parts[1])
			} else {
				rest = ""
			}
		} else {
			continue // 跳过无法解析的行
		}

		// 检查行是否符合进程状态行的基本格式，避免解析无效行如 "invalid line without proper format"
		if !IsValidProcessLine(name, rest) {
			continue // 跳过无效行
		}

		// 解析剩余部分
		restFields := strings.Fields(rest)
		if len(restFields) < 1 {
			continue
		}

		stateName := restFields[0]
		pid := 0
		uptime := ""

		// 解析PID和运行时间
		for j, field := range restFields {
			if field == "pid" && j+1 < len(restFields) {
				// 保留原始的pid字段，不删除逗号，因为后续解析可能需要
				pidStr := restFields[j+1]
				if strings.HasSuffix(pidStr, ",") {
					pidStr = strings.TrimSuffix(pidStr, ",")
				}
				if p, err := strconv.Atoi(pidStr); err == nil {
					pid = p
				}
			}
			if field == "uptime" && j+1 < len(restFields) {
				// 只取uptime后面的第一个字段（时间部分），避免包含其他信息
				uptime = restFields[j+1]
				// 移除可能的逗号
				if strings.HasSuffix(uptime, ",") {
					uptime = strings.TrimSuffix(uptime, ",")
				}

				// 进一步处理运行时间格式，只保留时间部分
				processedUptime := processUptimeString(uptime)
				// 确保解析后的结果不为空
				if processedUptime != "" {
					uptime = processedUptime
				}
				break
			}
		}
		
		state := GetStateValue(stateName)

		// 创建进程信息时，如果uptime为空且状态不是RUNNING，尝试使用rest的剩余部分
		// 创建进程信息时，如果uptime为空且状态不是RUNNING，尝试使用rest的剩余部分
	if uptime == "" && !strings.Contains(strings.ToUpper(stateName), "RUNNING") {
			// 检查rest是否包含其他状态信息，如"Not started"
			if len(restFields) > 1 {
				// 重新构造从stateName开始的剩余部分
				stateIdx := -1
				for idx, field := range restFields {
					if field == stateName && stateIdx == -1 {
						stateIdx = idx
						break
					}
				}
				if stateIdx >= 0 && stateIdx+1 < len(restFields) {
					extraInfo := restFields[stateIdx+1:]
					if len(extraInfo) > 0 {
						// 拼接额外信息，但要排除PID相关字段
						var extraParts []string
						skipNext := false
						for _, part := range extraInfo {
							if skipNext {
								skipNext = false
								continue
							}
							if part == "pid" {
								skipNext = true // 跳过pid值
								continue
							}
							extraParts = append(extraParts, part)
						}
						if len(extraParts) > 0 {
							uptime = strings.Join(extraParts, " ")
						}
					}
				}
			}
		}

		processes = append(processes, ProcessInfo{
			Index:       i + 1,
			Name:        name, // 完整的进程名称，例如 "agent:agent_00"
			State:       state,
			StateName:   stateName,
			PID:         pid,
			Uptime:      uptime,
			Description: GetStateIcon(state),
			ExitStatus:  0,
		})
	}

	return processes
}

