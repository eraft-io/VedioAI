#!/bin/bash

# VideoSubtitle 构建脚本
# 仅构建应用，不打包

set -e

export PATH=$PATH:$(go env GOPATH)/bin

echo "================================"
echo "VideoSubtitle 构建脚本"
echo "================================"

# 检查 wails 是否安装
if ! command -v wails &> /dev/null; then
    echo "错误: 未找到 wails 命令"
    echo "请先安装 Wails: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

# 检查是否在项目目录
if [ ! -f "wails.json" ]; then
    echo "错误: 请在 VideoSubtitle 项目目录下运行此脚本"
    exit 1
fi

echo ""
echo "开始构建..."
echo ""

# 构建应用
wails build

if [ $? -eq 0 ]; then
    echo ""
    echo "================================"
    echo "构建成功！"
    echo "================================"
    echo ""
    echo "输出文件:"
    ls -lh build/bin/VideoSubtitle
    echo ""
else
    echo ""
    echo "================================"
    echo "构建失败"
    echo "================================"
    exit 1
fi
