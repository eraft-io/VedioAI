package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	condaPath := getCondaPath()
	if condaPath == "" {
		return false
	}
	return a.checkWhisperEnvInstalled(condaPath)
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

	// 检查 conda 是否可用
	condaPath := getCondaPath()
	if condaPath == "" {
		status["needInstall"] = true
		status["message"] = "未找到 conda，请先安装 Anaconda 或 Miniconda"
		return status
	}

	// 使用统一的检测逻辑
	if a.checkWhisperEnvInstalled(condaPath) {
		// 获取版本信息
		cmd := exec.Command(condaPath, "run", "-n", "whisper", "whisper", "--version")
		output, err := cmd.CombinedOutput()
		if err == nil {
			status["installed"] = true
			status["version"] = strings.TrimSpace(string(output))
			status["message"] = "Whisper 已就绪"
		} else {
			status["installed"] = true
			status["message"] = "Whisper 已安装"
		}
	} else {
		status["needInstall"] = true
		// status["message"] = "Whisper 未安装"
	}

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

// getCondaPath 获取 conda 路径
func getCondaPath() string {
	// 首先尝试 LookPath
	if path, err := exec.LookPath("conda"); err == nil {
		return path
	}

	// 尝试常见的 conda 安装路径
	homeDir, _ := os.UserHomeDir()
	possiblePaths := []string{
		"/opt/miniconda3/bin/conda",
		"/opt/anaconda3/bin/conda",
		"/usr/local/miniconda3/bin/conda",
		"/usr/local/anaconda3/bin/conda",
		filepath.Join(homeDir, "miniconda3", "bin", "conda"),
		filepath.Join(homeDir, "anaconda3", "bin", "conda"),
		filepath.Join(homeDir, "opt", "miniconda3", "bin", "conda"),
		filepath.Join(homeDir, "opt", "anaconda3", "bin", "conda"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// checkWhisperEnvInstalled 检查 Whisper 环境是否已完全安装
func (a *App) checkWhisperEnvInstalled(condaPath string) bool {
	fmt.Printf("[调试] 开始检测 Whisper 环境，conda 路径: %s\n", condaPath)

	// 检查 whisper 环境是否存在
	cmd := exec.Command(condaPath, "env", "list")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("[调试] conda env list 失败: %v\n", err)
		return false
	}
	if !strings.Contains(string(output), "whisper") {
		fmt.Printf("[调试] 未找到 whisper 环境\n")
		return false
	}
	fmt.Printf("[调试] whisper 环境存在\n")

	// 检查 whisper 是否可通过 python -m 运行
	fmt.Printf("[调试] 检测 whisper (通过 python -m whisper)...\n")
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "whisper", "--version")
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[调试] whisper 检测失败: %v, 输出: %s\n", err, string(output))
		return false
	}
	fmt.Printf("[调试] whisper 可用，版本: %s\n", strings.TrimSpace(string(output)))

	// 检查 ffmpeg 是否已安装
	fmt.Printf("[调试] 检测 ffmpeg...\n")
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "ffmpeg", "-version")
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[调试] ffmpeg 检测失败: %v, 输出: %s\n", err, string(output))
		return false
	}
	fmt.Printf("[调试] ffmpeg 已安装\n")

	// 检查 llama-cpp-python 是否已安装
	fmt.Printf("[调试] 检测 llama-cpp-python...\n")
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-c", "import llama_cpp")
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[调试] llama-cpp-python 检测失败: %v, 输出: %s\n", err, string(output))
		return false
	}
	fmt.Printf("[调试] llama-cpp-python 已安装\n")

	fmt.Printf("[调试] 所有检测通过！\n")
	return true
}

// InstallWhisper 自动安装 Whisper
func (a *App) InstallWhisper() map[string]interface{} {
	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	// 发送开始事件
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "started",
		"message":  "开始检查环境...",
		"progress": 0,
	})

	// 检查 conda 是否可用
	condaPath := getCondaPath()
	if condaPath == "" {
		result["message"] = "未找到 conda，请先安装 Anaconda 或 Miniconda"
		runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"status":  "error",
			"message": result["message"],
		})
		return result
	}

	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  fmt.Sprintf("找到 conda: %s", condaPath),
		"progress": 5,
	})

	// 检查是否已安装
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "检测现有环境...",
		"progress": 8,
	})

	if a.checkWhisperEnvInstalled(condaPath) {
		runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"status":   "completed",
			"message":  "Whisper 环境已安装，跳过安装步骤",
			"progress": 100,
		})
		result["success"] = true
		result["message"] = "Whisper 环境已就绪"
		return result
	}

	// 先接受 conda Terms of Service
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "接受 conda Terms of Service...",
		"progress": 10,
	})
	exec.Command(condaPath, "tos", "accept", "--override-channels", "--channel", "https://repo.anaconda.com/pkgs/main").Run()
	exec.Command(condaPath, "tos", "accept", "--override-channels", "--channel", "https://repo.anaconda.com/pkgs/r").Run()

	// 1. 删除旧环境（如果存在）并创建新环境
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 1/7: 创建 conda 环境 (Python 3.10)...",
		"progress": 15,
	})
	exec.Command(condaPath, "remove", "-n", "whisper", "--all", "-y").Run()

	cmd := exec.Command(condaPath, "create", "-n", "whisper", "python=3.10", "-y")
	output, err := cmd.CombinedOutput()
	if err != nil {
		result["message"] = fmt.Sprintf("创建环境失败: %v", err)
		runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"status":  "error",
			"message": result["message"],
			"output":  string(output),
		})
		return result
	}
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "conda 环境创建成功",
		"progress": 25,
		"output":   string(output),
	})

	// 2. 安装 ffmpeg
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 2/7: 安装 ffmpeg...",
		"progress": 30,
	})
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "conda-forge", "ffmpeg", "-y", "--force-reinstall")
	output, _ = cmd.CombinedOutput()
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "ffmpeg 安装完成",
		"progress": 40,
		"output":   string(output),
	})

	// 3. 安装 llvmlite 和 numba
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 3/7: 安装 llvmlite 和 numba...",
		"progress": 45,
	})
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "conda-forge", "llvmlite", "numba", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		result["message"] = fmt.Sprintf("安装 llvmlite/numba 失败: %v", err)
		runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"status":  "error",
			"message": result["message"],
			"output":  string(output),
		})
		return result
	}
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "llvmlite 和 numba 安装完成",
		"progress": 55,
	})

	// 4. 安装 numpy
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 4/7: 安装 numpy...",
		"progress": 60,
	})
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "conda-forge", "numpy=1.26", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		result["message"] = fmt.Sprintf("安装 numpy 失败: %v", err)
		runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"status":  "error",
			"message": result["message"],
			"output":  string(output),
		})
		return result
	}
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "numpy 安装完成",
		"progress": 70,
	})

	// 5. 安装 torch
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 5/7: 安装 torch...",
		"progress": 75,
	})
	cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "pytorch", "pytorch", "cpuonly", "-y")
	output, _ = cmd.CombinedOutput()
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "torch 安装完成",
		"progress": 80,
	})

	// 6. 安装 whisper
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 6/7: 安装 whisper...",
		"progress": 85,
	})
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install", "--no-deps", "openai-whisper")
	output, err = cmd.CombinedOutput()
	if err != nil {
		result["message"] = fmt.Sprintf("安装 whisper 失败: %v", err)
		runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"status":  "error",
			"message": result["message"],
			"output":  string(output),
		})
		return result
	}
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "whisper 安装完成",
		"progress": 90,
	})

	// 7. 安装其他依赖
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "安装其他依赖 (tiktoken, more-itertools, tqdm)...",
		"progress": 92,
	})
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install", "tiktoken", "more-itertools", "tqdm", "-q")
	cmd.Run()

	// 8. 安装 llama-cpp-python
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 7/7: 安装 llama-cpp-python（用于字幕翻译）...",
		"progress": 93,
	})
	cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install", "llama-cpp-python", "--no-cache-dir")
	output, _ = cmd.CombinedOutput()
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "llama-cpp-python 安装完成",
		"progress": 95,
	})

	// 完成
	result["success"] = true
	result["message"] = "Whisper 安装成功"
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "completed",
		"message":  "Whisper 安装完成！",
		"progress": 100,
	})
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

// SelectSubtitleFile 打开文件选择对话框选择字幕文件
func (a *App) SelectSubtitleFile() string {
	cmd := exec.Command("osascript", "-e", `
		tell application "System Events"
			activate
			set subtitleFile to choose file with prompt "选择字幕 JSON 文件" of type {"public.json"}
			return POSIX path of subtitleFile
		end tell
	`)

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// ImportSubtitleResult 导入字幕结果
type ImportSubtitleResult struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message"`
	Subtitles []SubtitleItem `json:"subtitles"`
}

// ImportSubtitleFromJSON 从 JSON 文件导入字幕
func (a *App) ImportSubtitleFromJSON(jsonPath string) ImportSubtitleResult {
	result := ImportSubtitleResult{
		Success:   false,
		Message:   "",
		Subtitles: []SubtitleItem{},
	}

	if jsonPath == "" {
		result.Message = "请选择字幕文件"
		return result
	}

	// 检查文件是否存在
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		result.Message = "字幕文件不存在"
		return result
	}

	// 解析 JSON 文件
	subtitles, err := parseWhisperJSON(jsonPath)
	if err != nil {
		result.Message = fmt.Sprintf("解析字幕文件失败: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("成功导入 %d 条字幕", len(subtitles))
	result.Subtitles = subtitles

	return result
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// ExportSubtitleResult 导出字幕结果
type ExportSubtitleResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// ExportSubtitlesToJSON 将字幕导出为 JSON 文件
func (a *App) ExportSubtitlesToJSON(subtitles []SubtitleItem, videoPath string) ExportSubtitleResult {
	result := ExportSubtitleResult{
		Success: false,
		Message: "",
		Path:    "",
	}

	if len(subtitles) == 0 {
		result.Message = "没有可导出的字幕"
		return result
	}

	// 确定输出路径
	var outputPath string
	if videoPath != "" {
		videoDir := filepath.Dir(videoPath)
		baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
		outputPath = filepath.Join(videoDir, baseName+"_translated.json")
	} else {
		homeDir, _ := os.UserHomeDir()
		outputPath = filepath.Join(homeDir, "subtitles_translated.json")
	}

	// 构建 Whisper 格式的 JSON
	exportData := struct {
		Segments []struct {
			ID             int     `json:"id"`
			Start          float64 `json:"start"`
			End            float64 `json:"end"`
			Text           string  `json:"text"`
			TranslatedText string  `json:"translatedText,omitempty"`
		} `json:"segments"`
		Text string `json:"text"`
	}{
		Segments: make([]struct {
			ID             int     `json:"id"`
			Start          float64 `json:"start"`
			End            float64 `json:"end"`
			Text           string  `json:"text"`
			TranslatedText string  `json:"translatedText,omitempty"`
		}, len(subtitles)),
	}

	// 构建完整文本
	var fullText strings.Builder
	for i, sub := range subtitles {
		exportData.Segments[i].ID = sub.ID
		exportData.Segments[i].Start = sub.StartTime
		exportData.Segments[i].End = sub.EndTime
		exportData.Segments[i].Text = sub.Text
		exportData.Segments[i].TranslatedText = sub.TranslatedText
		fullText.WriteString(sub.Text)
		fullText.WriteString(" ")
	}
	exportData.Text = strings.TrimSpace(fullText.String())

	// 写入 JSON 文件
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		result.Message = fmt.Sprintf("序列化 JSON 失败: %v", err)
		return result
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		result.Message = fmt.Sprintf("写入文件失败: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("成功导出 %d 条字幕到: %s", len(subtitles), outputPath)
	result.Path = outputPath

	return result
}
