#!/bin/bash

# 视频字幕翻译器 macOS ARM64 (Apple Silicon) PKG 打包脚本

set -e

export PATH=$PATH:$(go env GOPATH)/bin

echo "================================"
echo "视频字幕翻译器 macOS ARM64 打包脚本"
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

# 获取版本号
VERSION=$(grep '"version"' wails.json | head -1 | sed 's/.*: "\(.*\)".*/\1/')
if [ -z "$VERSION" ]; then
    VERSION="1.0.0"
fi

echo ""
echo "版本号: $VERSION"
echo ""

# 清理旧的构建
echo "清理旧的构建..."
rm -rf build/bin/*.app
rm -rf build/darwin/arm64
rm -f build/pkg/VideoSubtitle-*-arm64.pkg

# 创建输出目录
mkdir -p build/pkg

echo ""
echo "================================"
echo "开始打包 ARM64..."
echo "================================"

# 打包 ARM64 (Apple Silicon)
echo ""
echo "[1/1] 打包 ARM64 (Apple Silicon)..."
wails build -platform darwin/arm64 -ldflags "-s -w" -trimpath

if [ -d "build/bin/VideoSubtitle.app" ]; then
    mkdir -p build/darwin/arm64
    cp -R build/bin/VideoSubtitle.app "build/darwin/arm64/VideoSubtitle.app"
    echo "✓ ARM64 应用打包成功"
    
    # 创建 ARM64 PKG
    echo "创建 ARM64 PKG 安装包..."
    pkgbuild \
        --root "build/darwin/arm64" \
        --identifier "com.videosubtitle.app.arm64" \
        --version "${VERSION}" \
        --install-location "/Applications" \
        "build/pkg/VideoSubtitle-${VERSION}-arm64.pkg"
    echo "✓ ARM64 PKG 创建成功"
else
    echo "✗ ARM64 打包失败"
    exit 1
fi

# 验证 PKG
echo ""
echo "================================"
echo "验证 PKG 安装包..."
echo "================================"
pkgutil --check-signature "build/pkg/VideoSubtitle-${VERSION}-arm64.pkg" 2>/dev/null || echo "注意: PKG 未签名"

# 输出打包结果
echo ""
echo "================================"
echo "打包完成！"
echo "================================"
echo ""
echo "输出文件:"
echo "  ARM64: build/pkg/VideoSubtitle-${VERSION}-arm64.pkg"
echo ""
ls -lh build/pkg/VideoSubtitle-${VERSION}-arm64.pkg
echo ""
echo "安装方法:"
echo "  1. 双击 .pkg 文件运行安装程序"
echo "  2. 或命令行安装:"
echo "     sudo installer -pkg build/pkg/VideoSubtitle-${VERSION}-arm64.pkg -target /"
echo ""
echo "注意: 此安装包仅适用于 Apple Silicon Mac (M1/M2/M3)"
echo ""
