#!/bin/bash

# 视频字幕翻译器 Windows 打包脚本
# 在 macOS 上交叉编译 Windows 版本

set -e

export PATH=$PATH:$(go env GOPATH)/bin

# 获取版本号
VERSION=$(grep '"version"' wails.json | head -1 | sed 's/.*: "\(.*\)".*/\1/')
if [ -z "$VERSION" ]; then
    VERSION="1.0.0"
fi

echo "================================"
echo "视频字幕翻译器 Windows 打包脚本"
echo "================================"
echo "版本号: $VERSION"
echo ""

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

# 检查是否安装了 Windows 交叉编译工具链
if ! command -v x86_64-w64-mingw32-gcc &> /dev/null; then
    echo "警告: 未找到 Windows 交叉编译工具链 (mingw-w64)"
    echo "尝试通过 Homebrew 安装..."
    if command -v brew &> /dev/null; then
        brew install mingw-w64
    else
        echo "错误: 请先安装 mingw-w64 工具链"
        echo "  brew install mingw-w64"
        exit 1
    fi
fi

# 清理旧的构建
echo "清理旧的构建..."
rm -rf build/bin/VideoSubtitle.exe
rm -rf build/windows
rm -f build/pkg/VideoSubtitle-*-windows-*.exe
rm -f build/pkg/VideoSubtitle-*-windows-*.zip

# 创建输出目录
mkdir -p build/pkg

echo ""
echo "================================"
echo "开始打包 Windows AMD64..."
echo "================================"

# 设置交叉编译环境变量
export CGO_ENABLED=1
export GOOS=windows
export GOARCH=amd64
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++

echo ""
echo "[1/2] 构建 Windows AMD64 可执行文件..."
wails build -platform windows/amd64 -ldflags "-s -w" -trimpath

if [ -f "build/bin/VideoSubtitle.exe" ]; then
    echo "✓ Windows AMD64 构建成功"
    
    # 创建 Windows 发布目录
    mkdir -p build/windows/VideoSubtitle
    cp build/bin/VideoSubtitle.exe build/windows/VideoSubtitle/
    
    # 创建 README
    cat > build/windows/VideoSubtitle/README.txt << 'EOF'
视频字幕翻译器 VideoSubtitle
========================

版本: VERSION_PLACEHOLDER

系统要求:
- Windows 10/11 64位
- 需要安装 Whisper 环境（首次使用时会自动安装）

安装说明:
1. 解压 VideoSubtitle-windows-amd64.zip 到任意目录
2. 运行 VideoSubtitle.exe
3. 首次使用需要安装 Whisper 环境，点击"安装 Whisper"按钮

使用说明:
1. 选择视频文件
2. 选择 Whisper 模型（推荐 base）
3. 点击"生成字幕"
4. 等待字幕生成完成
5. 点击"翻译字幕"获取中文翻译
6. 点击"导出双语"生成双语对照 HTML

注意事项:
- 首次使用需要下载翻译模型（约 2GB），请确保网络连接
- 字幕生成需要一定时间，请耐心等待
- 支持的视频格式: mp4, mov, avi, mkv 等

技术支持:
如有问题，请查看应用内的安装指南或联系开发者。
EOF
    
    sed -i '' "s/VERSION_PLACEHOLDER/${VERSION}/g" build/windows/VideoSubtitle/README.txt
    
    # 创建 ZIP 压缩包
    echo ""
    echo "[2/2] 创建 ZIP 安装包..."
    cd build/windows
    zip -r "../pkg/VideoSubtitle-${VERSION}-windows-amd64.zip" VideoSubtitle/
    cd ../..
    
    if [ -f "build/pkg/VideoSubtitle-${VERSION}-windows-amd64.zip" ]; then
        echo "✓ ZIP 安装包创建成功"
        
        # 输出打包结果
        echo ""
        echo "================================"
        echo "打包完成！"
        echo "================================"
        echo ""
        echo "输出文件:"
        echo "  ZIP: build/pkg/VideoSubtitle-${VERSION}-windows-amd64.zip"
        echo ""
        ls -lh build/pkg/VideoSubtitle-${VERSION}-windows-amd64.zip
        echo ""
        echo "安装方法:"
        echo "  1. 解压 VideoSubtitle-${VERSION}-windows-amd64.zip"
        echo "  2. 进入 VideoSubtitle 目录"
        echo "  3. 运行 VideoSubtitle.exe"
        echo ""
        echo "注意: 此安装包仅适用于 Windows 10/11 64位系统"
        echo ""
    else
        echo "✗ ZIP 创建失败"
        exit 1
    fi
else
    echo "✗ Windows AMD64 构建失败"
    exit 1
fi
