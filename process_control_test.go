package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ProcessControlTestSuite 进程控制测试套件
type ProcessControlTestSuite struct {
	suite.Suite
}

func TestProcessControlTestSuite(t *testing.T) {
	suite.Run(t, new(ProcessControlTestSuite))
}

// TestGetAllProcesses_RealSupervisor 测试获取真实Supervisor进程
func (suite *ProcessControlTestSuite) TestGetAllProcesses_RealSupervisor() {
	client := NewSupervisorClient("http://localhost:9001/RPC2", "", "")

	// 跳过如果supervisorctl不可用
	if _, err := exec.LookPath("supervisorctl"); err != nil {
		suite.T().Skip("supervisorctl not available, skipping real test")
	}

	processes, err := client.GetAllProcesses()

	// 可能成功或失败，取决于是否运行supervisor
	if err != nil {
		assert.Empty(suite.T(), processes)
		assert.Contains(suite.T(), err.Error(), "无法获取进程信息")
	} else {
		assert.NotEmpty(suite.T(), processes)
	}
}

// TestParseSupervisorctlOutput 测试解析supervisorctl输出
func (suite *ProcessControlTestSuite) TestParseSupervisorctlOutput() {
	// 测试数据
	output := `agent:agent_00                   RUNNING   pid 988995, uptime 30 days, 16:17:38
agent:agent_01                   RUNNING   pid 988996, uptime 30 days, 16:17:38
agent:agent_02                   STOPPED   Not started
web:web_00                       STARTING  
web:web_01                       FATAL     Exited too quickly (process log may have details)
database:db_00                   BACKOFF   Exited too quickly (process log may have details)`

	processes := parseSupervisorctlOutput(output)

	assert.Len(suite.T(), processes, 6)

	// 测试第一个进程
	agent1 := processes[0]
	assert.Equal(suite.T(), 1, agent1.Index)
	assert.Equal(suite.T(), "agent:agent_00", agent1.Name)
	assert.Equal(suite.T(), "RUNNING", agent1.StateName)
	assert.Equal(suite.T(), 20, agent1.State) // RUNNING状态码
	assert.Equal(suite.T(), 988995, agent1.PID)
	assert.Equal(suite.T(), "30 days, 16:17:38", agent1.Uptime)
	assert.Equal(suite.T(), "✅ 运行中", agent1.Description)

	// 测试停止的进程
	agent2 := processes[2]
	assert.Equal(suite.T(), "STOPPED", agent2.StateName)
	assert.Equal(suite.T(), 0, agent2.State) // STOPPED状态码
	assert.Equal(suite.T(), 0, agent2.PID)
	assert.Equal(suite.T(), "Not started", agent2.Uptime)
	assert.Equal(suite.T(), "⏸️ 已停止", agent2.Description)

	// 测试启动中的进程
	web0 := processes[3]
	assert.Equal(suite.T(), "STARTING", web0.StateName)
	assert.Equal(suite.T(), 10, web0.State) // STARTING状态码
	assert.Equal(suite.T(), "🚀 启动中", web0.Description)

	// 测试致命错误进程
	web1 := processes[4]
	assert.Equal(suite.T(), "FATAL", web1.StateName)
	assert.Equal(suite.T(), 100, web1.State) // FATAL状态码
	assert.Equal(suite.T(), "❌ 致命错误", web1.Description)

	// 测试重试中的进程
	db0 := processes[5]
	assert.Equal(suite.T(), "BACKOFF", db0.StateName)
	assert.Equal(suite.T(), 200, db0.State) // BACKOFF状态码
	assert.Equal(suite.T(), "⚠️ 重试中", db0.Description)
}

// TestParseSupervisorctlOutput_Empty 测试解析空输出
func (suite *ProcessControlTestSuite) TestParseSupervisorctlOutput_Empty() {
	processes := parseSupervisorctlOutput("")
	assert.Len(suite.T(), processes, 0)
}

// TestParseSupervisorctlOutput_InvalidLines 测试解析包含无效行的输出
func (suite *ProcessControlTestSuite) TestParseSupervisorctlOutput_InvalidLines() {
	output := `nginx                          RUNNING   pid 1234, uptime 1h
invalid line without proper format
redis                          RUNNING   pid 5678, uptime 2d

another invalid line`

	processes := parseSupervisorctlOutput(output)

	// 应该只解析有效的行
	assert.Len(suite.T(), processes, 2)
	assert.Equal(suite.T(), "nginx", processes[0].Name)
	assert.Equal(suite.T(), "redis", processes[1].Name)
}

// TestControlProcess_Start 测试启动进程
func (suite *ProcessControlTestSuite) TestControlProcess_Start() {
	client := NewSupervisorClient("http://localhost:9001/RPC2", "", "")

	// 测试启动不存在的进程 - 应该失败
	err := client.ControlProcess("start", "nonexistent-process")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "start进程失败")
}

// TestControlProcess_Restart 测试重启进程
func (suite *ProcessControlTestSuite) TestControlProcess_Restart() {
	client := NewSupervisorClient("http://localhost:9001/RPC2", "", "")

	// 测试重启不存在的进程 - 应该失败
	err := client.ControlProcess("restart", "nonexistent-process")
	assert.Error(suite.T(), err)
}

// TestControlProcess_InvalidAction 测试无效操作
func (suite *ProcessControlTestSuite) TestControlProcess_InvalidAction() {
	client := NewSupervisorClient("http://localhost:9001/RPC2", "", "")

	err := client.ControlProcess("invalid", "test-process")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "不支持的操作: invalid")
}

// TestParseProcessIndices 测试解析进程索引
func (suite *ProcessControlTestSuite) TestParseProcessIndices() {
	// 创建模拟进程列表
	processes := []ProcessInfo{
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
		suite.Run(tc.name, func() {
			result, err := ParseProcessIndices(tc.args, processes)

			if tc.hasError {
				assert.Error(suite.T(), err, "Test case: %s", tc.name)
				assert.Nil(suite.T(), result, "Test case: %s", tc.name)
			} else {
				assert.NoError(suite.T(), err, "Test case: %s", tc.name)
				assert.Equal(suite.T(), tc.expected, result, "Test case: %s", tc.name)
			}
		})
	}
}

// TestParseProcessIndices_EmptyArgs 测试空参数
func (suite *ProcessControlTestSuite) TestParseProcessIndices_EmptyArgs() {
	processes := []ProcessInfo{{Name: "process1", Index: 1}}
	result, err := ParseProcessIndices([]string{}, processes)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
}

// TestReadSupervisorConfig 测试读取Supervisor配置
func (suite *ProcessControlTestSuite) TestReadSupervisorConfig() {
	// 测试默认配置
	host, username, password := readSupervisorConfig()
	assert.Equal(suite.T(), "http://localhost:9001/RPC2", host)
	assert.Equal(suite.T(), "", username)
	assert.Equal(suite.T(), "", password)

	// 测试环境变量覆盖
	os.Setenv("SUPERVISOR_HOST", "http://custom:9002/RPC2")
	os.Setenv("SUPERVISOR_USER", "customuser")
	os.Setenv("SUPERVISOR_PASSWORD", "custompass")
	defer func() {
		os.Unsetenv("SUPERVISOR_HOST")
		os.Unsetenv("SUPERVISOR_USER")
		os.Unsetenv("SUPERVISOR_PASSWORD")
	}()

	host, username, password = readSupervisorConfig()
	assert.Equal(suite.T(), "http://custom:9002/RPC2", host)
	assert.Equal(suite.T(), "customuser", username)
	assert.Equal(suite.T(), "custompass", password)
}