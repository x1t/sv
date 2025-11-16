#!/bin/bash

# 创建dist目录
mkdir -p dist

# 编译为当前系统架构，使用 -w -s 标志减小二进制文件大小
echo "正在编译 sv..."
go build -ldflags="-w -s" -o dist/sv main.go

echo "编译完成，输出文件: dist/sv"

# 显示文件信息
if [ -f "dist/sv" ]; then
    echo "文件信息:"
    ls -lh dist/sv
else
    echo "编译失败"
    exit 1
fi