package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

const (
	StateStopped  = 0
	StateStarting = 10
	StateRunning  = 20
	StateStopping = 30
	StateFatal    = 100
	StateBackoff  = 200
)

// ProcessInfo 表示一个 Supervisor 进程的信息。
type ProcessInfo struct {
	Index         int
	Name          string
	Group         string
	Start         float64
	Stop          float64
	Now           float64
	State         int
	StateName     string
	SpawnErr      string
	PID           int
	Logfile       string
	StdoutLogfile string
	StderrLogfile string
	Uptime        string
	Description   string
	ExitStatus    int
}

// DisplayStatus 保留旧的 stdout API，新的调用方应优先使用 RenderStatus。
func DisplayStatus(processes []ProcessInfo) {
	_ = RenderStatus(os.Stdout, processes)
}

// RenderStatus 将进程状态渲染到指定输出流。
func RenderStatus(w io.Writer, processes []ProcessInfo) error {
	if w == nil {
		return fmt.Errorf("状态输出流不能为空")
	}
	if len(processes) == 0 {
		_, err := fmt.Fprintln(w, "没有找到任何进程")
		return err
	}

	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleLight),
		})),
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignCenter},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
		tablewriter.WithTrimSpace(tw.Off),
	)

	table.Header([]string{"序号", "名称", "状态", "PID", "运行时间"})
	rows := make([][]any, 0, len(processes))
	colors := colorOutputEnabled(w)
	for _, process := range processes {
		pid := "-"
		if process.PID > 0 {
			pid = strconv.Itoa(process.PID)
		}
		state := process.StateName
		if strings.TrimSpace(state) == "" {
			state = "UNKNOWN"
		}
		if colors {
			state = GetColorByState(process.State) + state + "\x1b[0m"
		}
		rows = append(rows, []any{process.Index, process.Name, state, pid, process.Uptime})
	}

	if err := table.Bulk(rows); err != nil {
		return fmt.Errorf("写入状态表格失败: %w", err)
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("渲染状态表格失败: %w", err)
	}
	return nil
}

func colorOutputEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// GetColorByState 根据状态返回 ANSI 颜色。
func GetColorByState(state int) string {
	switch state {
	case StateRunning:
		return "\x1b[32m"
	case StateStarting, StateStopping, StateBackoff:
		return "\x1b[33m"
	case StateFatal:
		return "\x1b[31m"
	default:
		return "\x1b[37m"
	}
}

// GetStateIcon 获取状态图标。
func GetStateIcon(state int) string {
	switch state {
	case StateRunning:
		return "✅ 运行中"
	case StateStarting:
		return "🚀 启动中"
	case StateStopping:
		return "⏹️ 停止中"
	case StateStopped:
		return "⏸️ 已停止"
	case StateFatal:
		return "❌ 致命错误"
	case StateBackoff:
		return "⚠️ 重试中"
	default:
		return "❓ 未知"
	}
}

// GetStateValue 根据状态名称获取状态代码。
func GetStateValue(stateName string) int {
	switch strings.ToUpper(strings.TrimSpace(stateName)) {
	case "RUNNING":
		return StateRunning
	case "STARTING":
		return StateStarting
	case "STOPPING":
		return StateStopping
	case "STOPPED":
		return StateStopped
	case "FATAL":
		return StateFatal
	case "BACKOFF":
		return StateBackoff
	default:
		return 0
	}
}

// ProcessUptimeString 将 supervisorctl 的运行时间转成中文可读形式。
func ProcessUptimeString(uptime string) string {
	return processUptimeString(uptime)
}

func processUptimeString(uptime string) string {
	original := strings.TrimSpace(uptime)
	if original == "" {
		return ""
	}

	dayCount := 0
	timePart := original
	fields := strings.SplitN(original, ",", 2)
	if len(fields) == 2 {
		dayText := strings.TrimSpace(fields[0])
		dayWords := strings.Fields(dayText)
		if len(dayWords) == 2 && (dayWords[1] == "day" || dayWords[1] == "days") {
			parsedDays, err := strconv.Atoi(dayWords[0])
			if err != nil || parsedDays < 0 {
				return original
			}
			dayCount = parsedDays
			timePart = strings.TrimSpace(fields[1])
		}
	}

	parts := strings.Split(timePart, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return original
	}
	values := make([]int, len(parts))
	for i, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 {
			return original
		}
		values[i] = value
	}

	var hours, minutes, seconds int
	if len(values) == 3 {
		hours, minutes, seconds = values[0], values[1], values[2]
	} else {
		minutes, seconds = values[0], values[1]
	}
	if minutes >= 60 || seconds >= 60 {
		return original
	}
	return formatDurationParts(dayCount, hours, minutes, seconds)
}

func formatDurationParts(days, hours, minutes, seconds int) string {
	switch {
	case days > 0:
		return fmt.Sprintf("%d天%d小时%02d分%02d秒", days, hours, minutes, seconds)
	case hours > 0:
		return fmt.Sprintf("%d小时%02d分%02d秒", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%d分%02d秒", minutes, seconds)
	default:
		return fmt.Sprintf("%d秒", seconds)
	}
}

// GetStringValue 从 interface{} 获取 string 值。
func GetStringValue(value interface{}) string {
	result, _ := value.(string)
	return result
}

// GetIntValue 从 interface{} 获取 int 值。
func GetIntValue(value interface{}) int {
	result, _ := value.(int)
	return result
}

// FormatUptime 将秒数格式化为中文可读形式。
func FormatUptime(seconds int) string {
	if seconds < 0 {
		return "无效时长"
	}
	if seconds == 0 {
		return "已停止"
	}

	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	seconds %= 60
	return formatDurationParts(days, hours, minutes, seconds)
}

// GetActionIcon 获取操作图标。
func GetActionIcon(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
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

// IsValidProcessLine 判断 supervisorctl 输出是否为一个有效进程行。
func IsValidProcessLine(name, rest string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return false
	}
	return GetStateValue(fields[0]) != 0 || strings.EqualFold(fields[0], "STOPPED")
}

// ParseProcessIndices 将序号、名称和序号范围解析为唯一进程名称列表。
func ParseProcessIndices(args []string, processes []ProcessInfo) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("未提供进程参数")
	}

	byIndex := make(map[int]string, len(processes))
	byName := make(map[string]string, len(processes))
	for index, process := range processes {
		actualIndex := process.Index
		if actualIndex <= 0 {
			actualIndex = index + 1
		}
		byIndex[actualIndex] = process.Name
		byName[process.Name] = process.Name
	}

	result := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	appendName := func(name string) {
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}

	for _, raw := range args {
		argument := strings.TrimSpace(raw)
		if argument == "" {
			return nil, fmt.Errorf("进程参数不能为空")
		}
		if startText, endText, hasRange := strings.Cut(argument, "-"); hasRange {
			start, startErr := strconv.Atoi(strings.TrimSpace(startText))
			end, endErr := strconv.Atoi(strings.TrimSpace(endText))
			if startErr == nil && endErr == nil {
				if start <= 0 || end <= 0 || start > end {
					return nil, fmt.Errorf("无效的进程范围 %q", argument)
				}
				for index := start; index <= end; index++ {
					name, exists := byIndex[index]
					if !exists {
						return nil, fmt.Errorf("未找到序号为%d的进程", index)
					}
					appendName(name)
				}
				continue
			}
		}

		if index, err := strconv.Atoi(argument); err == nil {
			name, exists := byIndex[index]
			if !exists {
				return nil, fmt.Errorf("未找到序号为%d的进程", index)
			}
			appendName(name)
			continue
		}

		if name, exists := byName[argument]; exists {
			appendName(name)
			continue
		}

		matches := make([]string, 0, 1)
		for name := range byName {
			if strings.HasSuffix(name, ":"+argument) {
				matches = append(matches, name)
			}
		}
		if len(matches) == 1 {
			appendName(matches[0])
			continue
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("进程名称 %q 不唯一，请使用完整名称", argument)
		}
		return nil, fmt.Errorf("未找到进程 %q", argument)
	}

	return result, nil
}

// ParseSupervisorctlOutput 解析 supervisorctl status 的真实文本输出。
func ParseSupervisorctlOutput(output string) []ProcessInfo {
	result := make([]ProcessInfo, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 1024), 1<<20)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || (GetStateValue(fields[1]) == 0 && !strings.EqualFold(fields[1], "STOPPED")) {
			continue
		}

		process := ProcessInfo{
			Index:     len(result) + 1,
			Name:      fields[0],
			StateName: strings.ToUpper(fields[1]),
			State:     GetStateValue(fields[1]),
			PID:       0,
			Uptime:    "已停止",
		}
		rest := fields[2:]
		for index := 0; index < len(rest); index++ {
			switch strings.ToUpper(rest[index]) {
			case "PID":
				if index+1 < len(rest) {
					process.PID, _ = strconv.Atoi(strings.TrimSuffix(rest[index+1], ","))
				}
			case "UPTIME":
				if index+1 < len(rest) {
					uptime := strings.Join(rest[index+1:], " ")
					process.Uptime = processUptimeString(strings.TrimSpace(strings.TrimSuffix(uptime, ",")))
				}
			}
		}
		if process.State == StateStopped {
			process.Uptime = "已停止"
		}
		process.Description = GetStateIcon(process.State)
		result = append(result, process)
	}
	return result
}
