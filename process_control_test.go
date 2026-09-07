package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/x1t/sv/pkg/supervisor"
)

func TestProcessControllerValidatesBeforeNetworkAccess(t *testing.T) {
	controller := supervisor.NewProcessController(supervisor.NewRPCClient("http://127.0.0.1:1/RPC2", "", ""))

	assert.Error(t, controller.ControlProcess("pause", "worker"))
	assert.Error(t, controller.ControlProcess("start", ""))
	assert.Error(t, controller.ControlProcess("start", " worker"))
	assert.Error(t, controller.ControlProcess("start", "-worker"))
	assert.ErrorContains(t, controller.ControlProcess("start", "worker\nname"), "控制字符")
}
