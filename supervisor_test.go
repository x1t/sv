package main

import (
	"testing"

	"github.com/x1t/sv/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// TestNewSupervisorClient 测试创建Supervisor客户端
func TestNewSupervisorClient(t *testing.T) {
	host := "http://localhost:9001/RPC2"
	username := "user"
	password := "pass"

	client := NewSupervisorClient(host, username, password)

	assert.Equal(t, host, client.host)
	assert.Equal(t, username, client.username)
	assert.Equal(t, password, client.password)
	assert.NotNil(t, client.client)
}

// TestGetStateValue 测试状态值转换
func TestGetStateValue(t *testing.T) {
	testCases := []struct {
		stateName string
		expected  int
	}{
		{"RUNNING", 20},
		{"running", 20},  // 测试小写
		{"Running", 20},  // 测试混合大小写
		{"STARTING", 10},
		{"STOPPING", 30},
		{"STOPPED", 0},
		{"FATAL", 100},
		{"BACKOFF", 200},
		{"UNKNOWN", 0},   // 未知状态返回0
		{"", 0},          // 空字符串返回0
	}

	for _, tc := range testCases {
		result := utils.GetStateValue(tc.stateName)
		assert.Equal(t, tc.expected, result, "State name: %s", tc.stateName)
	}
}

// TestGetStringValue 测试从interface{}获取string值
func TestGetStringValue(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected string
	}{
		{"hello", "hello"},
		{"", ""},
		{"世界", "世界"},
		{123, ""},
		{nil, ""},
		{true, ""},
		{[]string{"test"}, ""},
	}

	for _, tc := range testCases {
		result := utils.GetStringValue(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %v", tc.input)
	}
}

// TestGetIntValue 测试从interface{}获取int值
func TestGetIntValue(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected int
	}{
		{42, 42},
		{0, 0},
		{-1, -1},
		{"123", 0},
		{nil, 0},
		{true, 0},
		{3.14, 0},
	}

	for _, tc := range testCases {
		result := utils.GetIntValue(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %v", tc.input)
	}
}

// TestFormatUptime 测试格式化运行时间
func TestFormatUptime(t *testing.T) {
	testCases := []struct {
		seconds  int
		expected string
	}{
		{0, "已停止"},
		{30, "不到1分钟"},
		{60, "1分钟"},
		{120, "2分钟"},
		{3600, "1小时0分"},
		{3660, "1小时1分"},
		{7200, "2小时0分"},
		{86400, "1天0小时0分"},
		{90000, "1天1小时0分"},
		{90060, "1天1小时1分"},
		{172800, "2天0小时0分"},
	}

	for _, tc := range testCases {
		result := utils.FormatUptime(tc.seconds)
		assert.Equal(t, tc.expected, result, "Seconds: %d", tc.seconds)
	}
}

// TestGetColorByState 测试根据状态获取颜色
func TestGetColorByState(t *testing.T) {
	testCases := []struct {
		state    int
		expected string
	}{
		{20, "\x1b[32m"},  // RUNNING - 绿色
		{10, "\x1b[33m"},  // STARTING - 黄色
		{30, "\x1b[33m"},  // STOPPING - 黄色
		{100, "\x1b[31m"}, // FATAL - 红色
		{0, "\x1b[37m"},   // STOPPED - 白色
		{200, "\x1b[37m"}, // BACKOFF - 白色(默认)
		{999, "\x1b[37m"}, // 未知状态 - 白色(默认)
	}

	for _, tc := range testCases {
		result := utils.GetColorByState(tc.state)
		assert.Equal(t, tc.expected, result, "State: %d", tc.state)
	}
}

// TestGetStateIcon 测试获取状态图标
func TestGetStateIcon(t *testing.T) {
	testCases := []struct {
		state    int
		expected string
	}{
		{20, "✅ 运行中"},   // RUNNING
		{10, "🚀 启动中"},   // STARTING
		{30, "⏹️ 停止中"},   // STOPPING
		{0, "⏸️ 已停止"},    // STOPPED
		{100, "❌ 致命错误"},  // FATAL
		{200, "⚠️ 重试中"},   // BACKOFF
		{999, "❓ 未知"},     // 未知状态
	}

	for _, tc := range testCases {
		result := utils.GetStateIcon(tc.state)
		assert.Equal(t, tc.expected, result, "State: %d", tc.state)
	}
}

// TestGetActionIcon 测试获取操作图标
func TestGetActionIcon(t *testing.T) {
	testCases := []struct {
		action   string
		expected string
	}{
		{"start", "🚀 启动"},
		{"stop", "⏹️ 停止"},
		{"restart", "🔄 重启"},
		{"reload", "⚙️ 操作"},  // 未知操作
		{"", "⚙️ 操作"},       // 空操作
	}

	for _, tc := range testCases {
		result := utils.GetActionIcon(tc.action)
		assert.Equal(t, tc.expected, result, "Action: %s", tc.action)
	}
}

// TestParseSupervisorctlOutput 测试解析supervisorctl输出
func TestParseSupervisorctlOutput(t *testing.T) {
	// 测试数据
	output := `agent:agent_00                   RUNNING   pid 988995, uptime 30 days, 16:17:38
agent:agent_01                   RUNNING   pid 988996, uptime 30 days, 16:17:38
agent:agent_02                   STOPPED   Not started
web:web_00                       STARTING  
web:web_01                       FATAL     Exited too quickly (process log may have details)
database:db_00                   BACKOFF   Exited too quickly (process log may have details)`

	processes := utils.ParseSupervisorctlOutput(output)

	assert.Len(t, processes, 6)

	// 测试第一个进程
	agent1 := processes[0]
	assert.Equal(t, 1, agent1.Index)
	assert.Equal(t, "agent:agent_00", agent1.Name)
	assert.Equal(t, "RUNNING", agent1.StateName)
	assert.Equal(t, 20, agent1.State) // RUNNING状态码
	assert.Equal(t, 988995, agent1.PID)
	assert.Equal(t, "30 days, 16:17:38", agent1.Uptime)
	assert.Equal(t, "✅ 运行中", agent1.Description)

	// 测试停止的进程
	agent2 := processes[2]
	assert.Equal(t, "STOPPED", agent2.StateName)
	assert.Equal(t, 0, agent2.State) // STOPPED状态码
	assert.Equal(t, 0, agent2.PID)
	assert.Equal(t, "Not started", agent2.Uptime)
	assert.Equal(t, "⏸️ 已停止", agent2.Description)

	// 测试启动中的进程
	web0 := processes[3]
	assert.Equal(t, "STARTING", web0.StateName)
	assert.Equal(t, 10, web0.State) // STARTING状态码
	assert.Equal(t, "🚀 启动中", web0.Description)

	// 测试致命错误进程
	web1 := processes[4]
	assert.Equal(t, "FATAL", web1.StateName)
	assert.Equal(t, 100, web1.State) // FATAL状态码
	assert.Equal(t, "❌ 致命错误", web1.Description)

	// 测试重试中的进程
	db0 := processes[5]
	assert.Equal(t, "BACKOFF", db0.StateName)
	assert.Equal(t, 200, db0.State) // BACKOFF状态码
	assert.Equal(t, "⚠️ 重试中", db0.Description)
}

// TestParseSupervisorctlOutput_Empty 测试解析空输出
func TestParseSupervisorctlOutput_Empty(t *testing.T) {
	processes := utils.ParseSupervisorctlOutput("")
	assert.Len(t, processes, 0)
}

// TestParseSupervisorctlOutput_InvalidLines 测试解析包含无效行的输出
func TestParseSupervisorctlOutput_InvalidLines(t *testing.T) {
	output := `nginx                          RUNNING   pid 1234, uptime 1h
invalid line without proper format
redis                          RUNNING   pid 5678, uptime 2d

another invalid line`

	processes := utils.ParseSupervisorctlOutput(output)

	// 应该只解析有效的行
	assert.Len(t, processes, 2)
	assert.Equal(t, "nginx", processes[0].Name)
	assert.Equal(t, "redis", processes[1].Name)
}

// TestParseProcessIndices 测试解析进程索引
func TestParseProcessIndices(t *testing.T) {
	// 创建模拟进程列表
	processes := []utils.ProcessInfo{
		{Name: "process1", Index: 1},
		{Name: "process2", Index: 2},
		{Name: "process3", Index: 3},
		{Name: "process4", Index: 4},
		{Name: "process5", Index: 5},
	}

	testCases := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "单个数字",
			args:     []string{"1"},
			expected: []string{"process1"},
			hasError: false,
		},
		{
			name:     "多个数字",
			args:     []string{"1", "3", "5"},
			expected: []string{"process1", "process3", "process5"},
			hasError: false,
		},
		{
			name:     "范围",
			args:     []string{"2-4"},
			expected: []string{"process2", "process3", "process4"},
			hasError: false,
		},
		{
			name:     "进程名称",
			args:     []string{"nginx"},
			expected: []string{"nginx"},
			hasError: false,
		},
		{
			name:     "混合",
			args:     []string{"1", "nginx", "3-4"},
			expected: []string{"process1", "nginx", "process3", "process4"},
			hasError: false,
		},
		{
			name:     "无效范围格式",
			args:     []string{"1-2-3"},
			expected: nil,
			hasError: true,
		},
		{
			name:     "范围超出",
			args:     []string{"1-10"},
			expected: nil,
			hasError: true,
		},
		{
			name:     "无效数字",
			args:     []string{"0"},
			expected: nil,
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := utils.ParseProcessIndices(tc.args, processes)

			if tc.hasError {
				assert.Error(t, err, "Test case: %s", tc.name)
				assert.Nil(t, result, "Test case: %s", tc.name)
			} else {
				assert.NoError(t, err, "Test case: %s", tc.name)
				assert.Equal(t, tc.expected, result, "Test case: %s", tc.name)
			}
		})
	}
}

// TestReadSupervisorConfig 测试读取Supervisor配置
func TestReadSupervisorConfig(t *testing.T) {
	// 测试默认配置
	host, username, password := readSupervisorConfig()
	assert.Equal(t, "http://localhost:9001/RPC2", host)
	assert.Equal(t, "", username)
	assert.Equal(t, "", password)
}