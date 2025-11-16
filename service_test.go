package main

import (
	"os"
	"testing"
)

func TestServiceCommand(t *testing.T) {
	// 保存原始命令行参数
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// 测试service命令帮助
	os.Args = []string{"sv", "service"}
	
	// 这里我们不能直接调用main()，因为它会调用os.Exit
	// 而是应该测试handleServiceCommand函数
	t.Log("测试service命令需要重构main函数以支持测试")
}