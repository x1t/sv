package main

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/x1t/sv/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// UtilsTestSuite 工具函数测试套件
type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

// TestGetStringValue 测试从interface{}获取string值
func (suite *UtilsTestSuite) TestGetStringValue() {
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
		assert.Equal(suite.T(), tc.expected, result, "Input: %v", tc.input)
	}
}

// TestGetIntValue 测试从interface{}获取int值
func (suite *UtilsTestSuite) TestGetIntValue() {
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
		assert.Equal(suite.T(), tc.expected, result, "Input: %v", tc.input)
	}
}

// TestFormatUptime 测试格式化运行时间
func (suite *UtilsTestSuite) TestFormatUptime() {
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
		assert.Equal(suite.T(), tc.expected, result, "Seconds: %d", tc.seconds)
	}
}

// TestGetColorByState 测试根据状态获取颜色
func (suite *UtilsTestSuite) TestGetColorByState() {
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
		assert.Equal(suite.T(), tc.expected, result, "State: %d", tc.state)
	}
}

// TestGetStateIcon 测试获取状态图标
func (suite *UtilsTestSuite) TestGetStateIcon() {
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
		assert.Equal(suite.T(), tc.expected, result, "State: %d", tc.state)
	}
}

// TestGetActionIcon 测试获取操作图标
func (suite *UtilsTestSuite) TestGetActionIcon() {
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
		assert.Equal(suite.T(), tc.expected, result, "Action: %s", tc.action)
	}
}

// TestDisplayStatus 测试显示进程状态
func (suite *UtilsTestSuite) TestDisplayStatus() {
	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 创建测试进程数据
	processes := []utils.ProcessInfo{
		{
			Index:       1,
			Name:        "nginx",
			State:       20, // RUNNING
			StateName:   "RUNNING",
			PID:         1234,
			Uptime:      "1小时30分",
			Description: "✅ 运行中",
		},
		{
			Index:       2,
			Name:        "mysql",
			State:       0, // STOPPED
			StateName:   "STOPPED",
			PID:         0,
			Uptime:      "已停止",
			Description: "⏸️ 已停止",
		},
	}

	// 调用显示函数
	utils.DisplayStatus(processes)

	// 恢复标准输出并读取捕获的内容
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 验证输出内容
	assert.Contains(suite.T(), output, "序号")
	assert.Contains(suite.T(), output, "名称")
	assert.Contains(suite.T(), output, "状态")
	assert.Contains(suite.T(), output, "PID")
	assert.Contains(suite.T(), output, "运行时间")
	assert.Contains(suite.T(), output, "描述")
	assert.Contains(suite.T(), output, "nginx")
	assert.Contains(suite.T(), output, "mysql")
	assert.Contains(suite.T(), output, "RUNNING")
	assert.Contains(suite.T(), output, "STOPPED")
	assert.Contains(suite.T(), output, "1234")
	assert.Contains(suite.T(), output, "1小时30分")
}

// TestDisplayStatus_Empty 测试显示空进程列表
func (suite *UtilsTestSuite) TestDisplayStatus_Empty() {
	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 调用显示函数
	utils.DisplayStatus([]utils.ProcessInfo{})

	// 恢复标准输出并读取捕获的内容
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(suite.T(), output, "没有找到任何进程")
}

// TestDisplayStatus_SingleProcess 测试显示单个进程
func (suite *UtilsTestSuite) TestDisplayStatus_SingleProcess() {
	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	processes := []utils.ProcessInfo{
		{
			Index:       1,
			Name:        "single-process",
			State:       20,
			StateName:   "RUNNING",
			PID:         9999,
			Uptime:      "5分钟",
			Description: "✅ 运行中",
		},
	}

	utils.DisplayStatus(processes)

	// 恢复标准输出并读取捕获的内容
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(suite.T(), output, "single-process")
	assert.Contains(suite.T(), output, "9999")
	assert.Contains(suite.T(), output, "5分钟")
	assert.Contains(suite.T(), output, "✅ 运行中")
}

// TestParseSupervisorctlOutput_EdgeCases 测试解析supervisorctl输出的边界情况
func (suite *UtilsTestSuite) TestParseSupervisorctlOutput_EdgeCases() {
	testCases := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "只有换行符",
			output:   "\n\n\n",
			expected: 0,
		},
		{
			name:     "只有空格和换行",
			output:   "   \n  \n \t \n",
			expected: 0,
		},
		{
			name:     "单行有效数据",
			output:   "nginx                    RUNNING   pid 1234, uptime 1h",
			expected: 1,
		},
		{
			name:     "混合有效无效行",
			output:   "nginx                    RUNNING   pid 1234, uptime 1h\ninvalid line\nredis                    RUNNING   pid 5678, uptime 2h",
			expected: 2,
		},
		{
			name: "复杂格式",
			output: `long_process_name_very_long    RUNNING   pid 9999, uptime 30 days, 5:23:45
short                          STOPPED   Not started
another:process_with_colon     STARTING`,
			expected: 3,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			processes := utils.ParseSupervisorctlOutput(tc.output)
			assert.Len(suite.T(), processes, tc.expected)
		})
	}
}

// BenchmarkParseSupervisorctlOutput 性能基准测试
func BenchmarkParseSupervisorctlOutput(b *testing.B) {
	output := `nginx                    RUNNING   pid 1234, uptime 1h
redis                    RUNNING   pid 5678, uptime 2h
mysql                    STOPPED   Not started
postgresql               STARTING
elasticsearch            BACKOFF   Exited too quickly
mongodb                  FATAL     Killed`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.ParseSupervisorctlOutput(output)
	}
}

// BenchmarkFormatUptime 性能基准测试
func BenchmarkFormatUptime(b *testing.B) {
	testCases := []int{0, 30, 60, 3600, 86400, 90061}

	for _, seconds := range testCases {
		b.Run(fmt.Sprintf("seconds_%d", seconds), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				utils.FormatUptime(seconds)
			}
		})
	}
}