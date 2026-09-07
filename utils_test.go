package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/x1t/sv/pkg/utils"
)

func TestProcessUptimeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "30 days, 16:17:38", want: "30天16小时17分38秒"},
		{input: "1 day, 00:00:03", want: "1天0小时00分03秒"},
		{input: "1:59:48", want: "1小时59分48秒"},
		{input: "12:34", want: "12分34秒"},
		{input: "00:05", want: "5秒"},
		{input: "Not started", want: "Not started"},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, utils.ProcessUptimeString(test.input), test.input)
	}
}

func TestParseSupervisorctlOutput(t *testing.T) {
	output := "app RUNNING pid 1234, uptime 0:01:05\n" +
		"worker\tSTOPPED\tNot started\n" +
		"broken UNKNOWN no useful state\n"
	processes := utils.ParseSupervisorctlOutput(output)
	require.Len(t, processes, 2)
	assert.Equal(t, 1, processes[0].Index)
	assert.Equal(t, "app", processes[0].Name)
	assert.Equal(t, utils.StateRunning, processes[0].State)
	assert.Equal(t, 1234, processes[0].PID)
	assert.Equal(t, "1分05秒", processes[0].Uptime)
	assert.Equal(t, 2, processes[1].Index)
	assert.Equal(t, utils.StateStopped, processes[1].State)
	assert.Equal(t, "已停止", processes[1].Uptime)
}

func TestParseProcessIndices(t *testing.T) {
	processes := []utils.ProcessInfo{
		{Index: 1, Name: "web:api"},
		{Index: 2, Name: "web:worker"},
		{Index: 3, Name: "batch"},
		{Index: 4, Name: "my-app"},
	}

	got, err := utils.ParseProcessIndices([]string{"1-2", "batch", "api", "my-app", "1"}, processes)
	require.NoError(t, err)
	want := []string{"web:api", "web:worker", "batch", "my-app"}
	assert.Equal(t, want, got)

	assert.Error(t, func() error {
		_, err := utils.ParseProcessIndices([]string{"5"}, processes)
		return err
	}())
	assert.Error(t, func() error {
		_, err := utils.ParseProcessIndices([]string{"api"}, []utils.ProcessInfo{{Name: "web:api"}, {Name: "batch:api"}})
		return err
	}())
	assert.Error(t, func() error {
		_, err := utils.ParseProcessIndices([]string{"3-1"}, processes)
		return err
	}())
}

func TestRenderStatusDoesNotColorNonTerminalWriter(t *testing.T) {
	var output bytes.Buffer
	processes := []utils.ProcessInfo{{
		Index:     1,
		Name:      "app",
		State:     utils.StateRunning,
		StateName: "RUNNING",
		PID:       1234,
		Uptime:    "1分05秒",
	}}
	require.NoError(t, utils.RenderStatus(&output, processes))
	text := output.String()
	for _, expected := range []string{"序号", "名称", "状态", "PID", "运行时间", "RUNNING", "1234"} {
		assert.Contains(t, text, expected)
	}
	assert.NotContains(t, text, "\x1b[")
}

func TestFormatUptime(t *testing.T) {
	for _, test := range []struct {
		seconds int
		want    string
	}{
		{seconds: 0, want: "已停止"},
		{seconds: 65, want: "1分05秒"},
		{seconds: 3661, want: "1小时01分01秒"},
		{seconds: -1, want: "无效时长"},
	} {
		assert.Equal(t, test.want, utils.FormatUptime(test.seconds), test.seconds)
	}
}
