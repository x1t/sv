#!/usr/bin/env bash

set -euo pipefail

# 创建 dist 目录
dist_dir="dist"
mkdir -p "$dist_dir"

# 编译为当前系统架构，使用 -w -s 标志减小二进制文件大小
echo "正在编译 sv..."
go build -trimpath -ldflags="-w -s" -o "$dist_dir/sv" .

echo "编译完成，输出文件: $dist_dir/sv"

# 显示文件信息
if [ -f "$dist_dir/sv" ]; then
    echo "文件信息:"
    ls -lh "$dist_dir/sv"
else
    echo "编译失败"
    exit 1
fi
