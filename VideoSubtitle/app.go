package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 检查 whisper 是否已安装
	if !a.isWhisperInstalled() {
		fmt.Println("Whisper 未安装，将在首次使用时自动安装")
	} else {
		fmt.Println("Whisper 已安装")
	}
}

// isWhisperInstalled 检查 whisper 是否已安装
func (a *App) isWhisperInstalled() bool {
	whisperCmd := a.getWhisperCommandPath()
	if whisperCmd == "whisper" {
		// 没有找到特定路径，尝试直接执行
		_, err := exec.LookPath("whisper")
		return err == nil
	}

	// 检查文件是否存在
	_, err := os.Stat(whisperCmd)
	return err == nil
}

// isDockerAvailable 检查 Docker 是否可用
func (a *App) isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	err := cmd.Run()
	return err == nil
}

// isWhisperImageExists 检查 whisper 镜像是否存在
func (a *App) isWhisperImageExists() bool {
	cmd := exec.Command("docker", "images", "-q", "video-subtitle-whisper")
	output, err := cmd.Output()
	return err == nil && len(output) > 0
}

// buildWhisperImage 构建 Whisper Docker 镜像
func (a *App) buildWhisperImage() error {
	// 获取应用目录
	appDir := a.getAppDir()

	// 创建临时 Dockerfile，使用镜像加速
	dockerfileContent := `# 使用镜像加速拉取
FROM docker.mirrors.ustc.edu.cn/library/python:3.10-slim

# 设置环境变量优化性能
ENV OMP_NUM_THREADS=0
ENV MKL_NUM_THREADS=0
ENV OPENBLAS_NUM_THREADS=0
ENV VECLIB_MAXIMUM_THREADS=0
ENV NUMEXPR_NUM_THREADS=0

# 更换 apt 源为阿里云
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources 2>/dev/null || \
    sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list 2>/dev/null || true

# 安装系统依赖和性能优化库
RUN apt-get update && apt-get install -y \\
    ffmpeg \\
    git \\
    libopenblas-dev \\
    libomp-dev \\
    && rm -rf /var/lib/apt/lists/*

# 更换 pip 源为阿里云，并设置超时
RUN pip config set global.index-url https://mirrors.aliyun.com/pypi/simple/ && \\
    pip config set global.timeout 120 && \\
    pip config set global.retries 5

# 升级 pip
RUN pip install --upgrade pip

# 安装 Python 依赖（分步安装，避免超时）
RUN pip install --no-cache-dir "numpy<2" || \\
    pip install --no-cache-dir --timeout 120 "numpy<2"
    
RUN pip install --no-cache-dir numba || \\
    pip install --no-cache-dir --timeout 120 numba
    
RUN pip install --no-cache-dir openai-whisper || \\
    pip install --no-cache-dir --timeout 120 openai-whisper

# 创建工作目录
WORKDIR /workspace

# 默认命令
ENTRYPOINT ["whisper"]
CMD ["--help"]
`
	// 写入临时 Dockerfile
	tmpDockerfile := filepath.Join(appDir, "Dockerfile.tmp")
	if err := os.WriteFile(tmpDockerfile, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("创建临时 Dockerfile 失败: %v", err)
	}
	defer os.Remove(tmpDockerfile)

	// 设置国内镜像加速
	env := os.Environ()
	env = append(env, "DOCKER_BUILDKIT=1")

	// 使用临时 Dockerfile 构建
	args := []string{
		"build",
		"-f", tmpDockerfile,
		"-t", "video-subtitle-whisper",
		".",
	}

	cmd := exec.Command("docker", args...)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	fmt.Println("开始构建 Docker 镜像，使用中科大镜像源...")

	if err := cmd.Run(); err != nil {
		// 如果失败，尝试使用原始 Dockerfile
		fmt.Println("使用中科大镜像失败，尝试直接拉取...")
		args = []string{
			"build",
			"-t", "video-subtitle-whisper",
			".",
		}
		cmd = exec.Command("docker", args...)
		cmd.Dir = appDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = env

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("构建 Docker 镜像失败: %v", err)
		}
	}

	fmt.Println("Whisper Docker 镜像构建完成")
	return nil
}

// getAppDir 获取应用目录
func (a *App) getAppDir() string {
	// 尝试几种可能的位置
	possiblePaths := []string{
		"/Users/colin/Desktop/VedioAI/VideoSubtitle",
		".",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(filepath.Join(path, "Dockerfile")); err == nil {
			return path
		}
	}

	return "."
}

// CheckWhisperStatus 检查 Whisper 安装状态
func (a *App) CheckWhisperStatus() map[string]interface{} {
	status := map[string]interface{}{
		"installed":   false,
		"version":     "",
		"message":     "",
		"needInstall": false,
	}

	// 获取 whisper 命令路径（使用与生成字幕相同的逻辑）
	whisperCmd := a.getWhisperCommandPath()

	// 检查 whisper 命令
	cmd := exec.Command(whisperCmd, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		status["needInstall"] = true
		status["message"] = "Whisper 未安装"
		return status
	}

	// whisper 已安装
	status["installed"] = true
	status["version"] = strings.TrimSpace(string(output))
	status["message"] = "Whisper 已就绪"

	return status
}

// getWhisperCommandPath 获取 whisper 命令路径
func (a *App) getWhisperCommandPath() string {
	// 首先尝试直接使用 whisper（如果已在 PATH 中）
	if path, err := exec.LookPath("whisper"); err == nil {
		return path
	}

	// 尝试常见 conda 环境路径
	homeDir, _ := os.UserHomeDir()
	possiblePaths := []string{
		"/opt/miniconda3/envs/whisper/bin/whisper",
		filepath.Join(homeDir, "opt", "miniconda3", "envs", "whisper", "bin", "whisper"),
		filepath.Join(homeDir, "anaconda3", "envs", "whisper", "bin", "whisper"),
		filepath.Join(homeDir, "miniconda3", "envs", "whisper", "bin", "whisper"),
		filepath.Join(homeDir, "opt", "anaconda3", "envs", "whisper", "bin", "whisper"),
		"/usr/local/anaconda3/envs/whisper/bin/whisper",
		"/opt/anaconda3/envs/whisper/bin/whisper",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 如果都找不到，返回 whisper 让系统尝试查找
	return "whisper"
}

// InstallWhisper 自动安装 Whisper
func (a *App) InstallWhisper() map[string]interface{} {
	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	// 检查 conda 是否可用
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		result["message"] = "未找到 conda，请先安装 Anaconda 或 Miniconda"
		return result
	}

	fmt.Println("开始安装 Whisper...")

	// 先接受 conda Terms of Service
	fmt.Println("接受 conda Terms of Service...")
	exec.Command(condaPath, "tos", "accept", "--override-channels", "--channel", "https://repo.anaconda.com/pkgs/main").Run()
	exec.Command(condaPath, "tos", "accept", "--override-channels", "--channel", "https://repo.anaconda.com/pkgs/r").Run()

	fmt.Println("步骤 1/4: 创建 conda 环境 (Python 3.10)...")

	// 1. 删除旧环境（如果存在）并创建新环境
	exec.Command(condaPath, "remove", "-n", "whisper", "--all", "-y").Run()

	cmd := exec.Command(condaPath, "create", "-n", "whisper", "python=3.10", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		result["message"] = fmt.Sprintf("创建环境失败: %v", err)
		return result
	}

	fmt.Println("步骤 2/5: 安装 ffmpeg...")

	// 2. 安装 ffmpeg（强制重新安装确保正确）
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "conda-forge", "ffmpeg", "-y", "--force-reinstall")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("ffmpeg 安装警告: %v\n", err)
	}

	// 2.5 验证 ffmpeg 是否可用
	fmt.Println("验证 ffmpeg 安装...")
	ffmpegCheck := exec.Command(condaPath, "run", "-n", "whisper", "which", "ffmpeg")
	ffmpegPath, err := ffmpegCheck.Output()
	if err != nil {
		fmt.Printf("ffmpeg 未找到: %v\n", err)
	} else {
		fmt.Printf("ffmpeg 路径: %s\n", strings.TrimSpace(string(ffmpegPath)))
	}

	fmt.Println("步骤 3/5: 安装 llvmlite 和 numba...")

	// 3. 安装 llvmlite 和 numba
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "conda-forge", "llvmlite", "numba", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		result["message"] = fmt.Sprintf("安装 llvmlite/numba 失败: %v", err)
		return result
	}

	fmt.Println("步骤 4/6: 安装 numpy...")

	// 4. 安装 numpy
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "conda-forge", "numpy=1.26", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		result["message"] = fmt.Sprintf("安装 numpy 失败: %v", err)
		return result
	}

	fmt.Println("步骤 5/6: 安装 torch...")

	// 5. 安装 torch（CPU 版本）
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "pytorch", "pytorch", "cpuonly", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// 尝试用 pip 安装
		cmd = exec.Command(condaPath, "run", "-n", "whisper", "pip", "install", "torch", "-q")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}

	fmt.Println("步骤 6/6: 安装 whisper...")

	// 6. 安装 whisper（使用 --no-deps 避免重复安装依赖）
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install", "--no-deps", "openai-whisper")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		result["message"] = fmt.Sprintf("安装 whisper 失败: %v", err)
		return result
	}

	// 7. 安装 whisper 的其他依赖
	fmt.Println("安装其他依赖...")
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install", "tiktoken", "more-itertools", "tqdm", "-q")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// 8. 安装 llama-cpp-python（用于字幕翻译）
	fmt.Println("步骤 7/7: 安装 llama-cpp-python（用于字幕翻译）...")
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install", "llama-cpp-python", "--no-cache-dir")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("llama-cpp-python 安装可能有问题，翻译功能可能不可用...")
	} else {
		fmt.Println("llama-cpp-python 安装成功！")
	}

	fmt.Println("Whisper 安装完成！")
	result["success"] = true
	result["message"] = "Whisper 安装成功"
	return result
}

// BuildWhisperImage 前端调用的构建镜像方法
func (a *App) BuildWhisperImage() map[string]interface{} {
	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	if !a.isDockerAvailable() {
		result["message"] = "Docker 未运行"
		return result
	}

	if err := a.buildWhisperImage(); err != nil {
		result["message"] = err.Error()
		return result
	}

	result["success"] = true
	result["message"] = "Docker 镜像构建成功"
	return result
}

// SelectVideoFile 打开文件选择对话框选择视频文件
func (a *App) SelectVideoFile() string {
	cmd := exec.Command("osascript", "-e", `
		tell application "System Events"
			activate
			set videoFile to choose file with prompt "选择视频文件" of type {"public.movie"}
			return POSIX path of videoFile
		end tell
	`)

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
