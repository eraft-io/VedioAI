#!/bin/bash

# 视频字幕翻译器 macOS ARM64 (Apple Silicon) 打包脚本
# 支持生成 PKG 和 DMG 两种格式

set -e

export PATH=$PATH:$(go env GOPATH)/bin

# 默认生成 DMG，可通过参数指定: ./pkg-arm64.sh pkg
BUILD_TYPE="${1:-dmg}"

echo "================================"
echo "视频字幕翻译器 macOS ARM64 打包脚本"
echo "================================"
echo "构建类型: $BUILD_TYPE"

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

# 检查并关闭正在运行的应用
if pgrep -x "VideoSubtitle" > /dev/null; then
    echo "检测到 VideoSubtitle 正在运行，正在关闭..."
    killall "VideoSubtitle" 2>/dev/null || true
    sleep 2
fi

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
    
    if [ "$BUILD_TYPE" = "pkg" ]; then
        # 创建 ARM64 PKG
        echo "创建 ARM64 PKG 安装包..."
        pkgbuild \
            --root "build/darwin/arm64" \
            --identifier "com.wails.VideoSubtitle" \
            --version "${VERSION}" \
            --install-location "/Applications" \
            "build/pkg/VideoSubtitle-${VERSION}-arm64.pkg"
        echo "✓ ARM64 PKG 创建成功"
        
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
        echo "  PKG: build/pkg/VideoSubtitle-${VERSION}-arm64.pkg"
        echo ""
        ls -lh build/pkg/VideoSubtitle-${VERSION}-arm64.pkg
        echo ""
        echo "安装方法:"
        echo "  1. 双击 .pkg 文件运行安装程序"
        echo "  2. 或命令行安装:"
        echo "     sudo installer -pkg build/pkg/VideoSubtitle-${VERSION}-arm64.pkg -target /"
        
    else
        # 创建 DMG（拖动安装方式）
        echo "创建 ARM64 DMG 安装包（拖动安装）..."
        
        DMG_FILE="build/pkg/VideoSubtitle-${VERSION}-arm64.dmg"
        
        # 创建临时目录用于构建 DMG
        DMG_TEMP_DIR="build/dmg_temp"
        rm -rf "$DMG_TEMP_DIR"
        mkdir -p "$DMG_TEMP_DIR"
        
        # 复制应用到临时目录
        cp -R "build/darwin/arm64/VideoSubtitle.app" "$DMG_TEMP_DIR/"
        
        # 创建 Applications 文件夹快捷方式
        ln -s /Applications "$DMG_TEMP_DIR/Applications"
        
        # 使用 hdiutil 创建 DMG（macOS 原生支持，无需额外安装）
        echo "正在创建 DMG 文件..."
        
        # 先创建未压缩的 DMG
        TEMP_DMG="build/pkg/temp_${VERSION}.dmg"
        hdiutil create \
            -srcfolder "$DMG_TEMP_DIR" \
            -volname "VideoSubtitle" \
            -fs HFS+ \
            -format UDRW \
            -ov \
            "$TEMP_DMG"
        
        # 压缩 DMG
        echo "正在压缩 DMG..."
        hdiutil convert "$TEMP_DMG" -format UDZO -o "$DMG_FILE"
        
        # 清理临时文件
        rm -f "$TEMP_DMG"
        rm -rf "$DMG_TEMP_DIR"
        
        if [ -f "$DMG_FILE" ]; then
            echo "✓ ARM64 DMG 创建成功"
            
            # 输出打包结果
            echo ""
            echo "================================"
            echo "打包完成！"
            echo "================================"
            echo ""
            echo "输出文件:"
            echo "  DMG: $DMG_FILE"
            echo ""
            ls -lh "$DMG_FILE"
            echo ""
            echo "安装方法:"
            echo "  1. 双击 .dmg 文件打开"
            echo "  2. 将 VideoSubtitle 图标拖动到 Applications 文件夹"
            echo "  3. 从 Launchpad 或 Applications 启动应用"
        else
            echo "✗ DMG 创建失败"
            exit 1
        fi
    fi
else
    echo "✗ ARM64 打包失败"
    exit 1
fi

echo ""
echo "注意: 此安装包仅适用于 Apple Silicon Mac (M1/M2/M3)"
echo ""
