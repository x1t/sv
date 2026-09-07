package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/x1t/sv/pkg/supervisor"
)

func TestServiceCommandRequiresAction(t *testing.T) {
	var output bytes.Buffer
	err := supervisor.NewServiceManagerWithWriter(&output).HandleServiceCommand(nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "缺少服务操作")
	assert.Contains(t, output.String(), "sv service <action>")
}

func TestServiceCommandRejectsUnknownAction(t *testing.T) {
	var output bytes.Buffer
	err := supervisor.NewServiceManagerWithWriter(&output).HandleServiceCommand([]string{"unknown"})
	assert.ErrorContains(t, err, "未知服务操作")
}
