package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/x1t/sv/pkg/cli"
)

func TestCLIHelpIncludesListAliases(t *testing.T) {
	var output bytes.Buffer
	app := cli.NewCLIAppWithWriters(&output, &output)

	require.NoError(t, app.RunArgs([]string{"help"}))
	for _, expected := range []string{"sv list", "sv ls", "sv init", "sv configure rpc"} {
		assert.Contains(t, output.String(), expected)
	}
}

func TestCLIRejectsRemovedServiceCommands(t *testing.T) {
	var output bytes.Buffer
	err := cli.NewCLIAppWithWriters(&output, &output).RunArgs([]string{"service", "install"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未知命令")
}

func TestCLIInitRejectsArgumentsBeforeSystemAccess(t *testing.T) {
	var output bytes.Buffer
	err := cli.NewCLIAppWithWriters(&output, &output).RunArgs([]string{"init", "unexpected"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init不接受额外参数")
}

func TestCLINoArgumentsPrintsUsage(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, cli.NewCLIAppWithWriters(&output, &output).RunArgs(nil))
	assert.Contains(t, output.String(), "sv - Supervisor进程管理工具")
}

func TestCLIRejectsUnknownCommand(t *testing.T) {
	var output bytes.Buffer
	err := cli.NewCLIAppWithWriters(&output, &output).RunArgs([]string{"unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未知命令")
}

func TestCLIRequiresProcessArgument(t *testing.T) {
	var output bytes.Buffer
	err := cli.NewCLIAppWithWriters(&output, &output).RunArgs([]string{"restart"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用法")
}

func TestCLIListAliasesRejectArgumentsBeforeNetworkAccess(t *testing.T) {
	for _, command := range []string{"list", "ls", "status"} {
		var output bytes.Buffer
		err := cli.NewCLIAppWithWriters(&output, &output).RunArgs([]string{command, "unexpected"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "不接受额外参数")
	}
}
