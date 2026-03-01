package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	gos "runtime"
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

	// 根据操作系统选择不同的路径
	homeDir, _ := os.UserHomeDir()
	var possiblePaths []string

	if gos.GOOS == "windows" {
		// Windows 路径
		possiblePaths = []string{
			filepath.Join(homeDir, "miniconda3", "Scripts", "conda.exe"),
			filepath.Join(homeDir, "anaconda3", "Scripts", "conda.exe"),
			filepath.Join(homeDir, "miniconda3", "condabin", "conda.bat"),
			filepath.Join(homeDir, "anaconda3", "condabin", "conda.bat"),
			`C:\ProgramData\miniconda3\Scripts\conda.exe`,
			`C:\ProgramData\anaconda3\Scripts\conda.exe`,
			`C:\ProgramData\miniconda3\condabin\conda.bat`,
			`C:\ProgramData\anaconda3\condabin\conda.bat`,
		}
	} else {
		// macOS/Linux 路径
		possiblePaths = []string{
			"/opt/miniconda3/bin/conda",
			"/opt/anaconda3/bin/conda",
			"/usr/local/miniconda3/bin/conda",
			"/usr/local/anaconda3/bin/conda",
			filepath.Join(homeDir, "miniconda3", "bin", "conda"),
			filepath.Join(homeDir, "anaconda3", "bin", "conda"),
			filepath.Join(homeDir, "opt", "miniconda3", "bin", "conda"),
			filepath.Join(homeDir, "opt", "anaconda3", "bin", "conda"),
		}
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

	// Windows 下如果没有找到 conda，尝试自动下载安装 Miniconda
	if condaPath == "" && gos.GOOS == "windows" {
		runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
			"status":   "running",
			"message":  "未找到 conda，正在自动下载安装 Miniconda...",
			"progress": 2,
		})

		minicondaPath, err := a.installMinicondaWindows()
		if err != nil {
			result["message"] = fmt.Sprintf("自动安装 Miniconda 失败: %v", err)
			runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
				"status":  "error",
				"message": result["message"],
			})
			return result
		}
		condaPath = minicondaPath
	}

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

	// 先接受 conda Terms of Service（Windows 下需要接受所有可能用到的 channel）
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "接受 conda Terms of Service...",
		"progress": 10,
	})
	tosChannels := []string{
		"https://repo.anaconda.com/pkgs/main",
		"https://repo.anaconda.com/pkgs/r",
		"https://repo.anaconda.com/pkgs/msys2",
		"https://conda.anaconda.org/conda-forge",
		"https://conda.anaconda.org/pytorch",
		"https://conda.anaconda.org/nvidia",
		"https://conda.anaconda.org/huggingface",
	}
	for _, channel := range tosChannels {
		exec.Command(condaPath, "tos", "accept", "--override-channels", "--channel", channel).Run()
	}

	// 1. 删除旧环境（如果存在）并创建新环境
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "步骤 1/7: 创建 conda 环境 (Python 3.10)...",
		"progress": 15,
	})
	exec.Command(condaPath, "remove", "-n", "whisper", "--all", "-y").Run()

	// 如果目录仍然存在（可能是损坏的环境），手动删除
	condaInfo, _ := exec.Command(condaPath, "info", "--json").Output()
	var condaInfoData map[string]interface{}
	if json.Unmarshal(condaInfo, &condaInfoData) == nil {
		if envsDirs, ok := condaInfoData["envs_dirs"].([]interface{}); ok && len(envsDirs) > 0 {
			for _, envsDir := range envsDirs {
				whisperEnvPath := filepath.Join(envsDir.(string), "whisper")
				if _, err := os.Stat(whisperEnvPath); err == nil {
					fmt.Printf("[InstallWhisper] 删除损坏的环境目录: %s\n", whisperEnvPath)
					os.RemoveAll(whisperEnvPath)
				}
			}
		}
	}

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
	// Windows 上 conda-forge 的 ffmpeg 有编码问题，使用默认 channel
	if gos.GOOS == "windows" {
		cmd = exec.Command(condaPath, "install", "-n", "whisper", "ffmpeg", "-y")
	} else {
		cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "conda-forge", "ffmpeg", "-y", "--force-reinstall")
	}
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
	// Windows 上使用 pip 从 PyTorch 官方源安装，避免 DLL 依赖问题
	if gos.GOOS == "windows" {
		cmd = exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install",
			"torch", "--index-url", "https://download.pytorch.org/whl/cpu")
	} else {
		cmd = exec.Command(condaPath, "install", "-n", "whisper", "-c", "pytorch", "pytorch", "cpuonly", "-y")
	}
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
	// 使用 Wails 跨平台文件对话框
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择视频文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "视频文件",
				Pattern:     "*.mp4;*.mov;*.avi;*.mkv;*.webm;*.flv;*.wmv;*.m4v",
			},
			{
				DisplayName: "所有文件",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		fmt.Printf("[SelectVideoFile] 错误: %v\n", err)
		return ""
	}
	return selection
}

// SelectSubtitleFile 打开文件选择对话框选择字幕文件
func (a *App) SelectSubtitleFile() string {
	// 使用 Wails 跨平台文件对话框
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择字幕 JSON 文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "JSON 文件",
				Pattern:     "*.json",
			},
			{
				DisplayName: "所有文件",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		fmt.Printf("[SelectSubtitleFile] 错误: %v\n", err)
		return ""
	}
	return selection
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

// installMinicondaWindows 在 Windows 上自动下载和安装 Miniconda
func (a *App) installMinicondaWindows() (string, error) {
	homeDir, _ := os.UserHomeDir()
	minicondaInstaller := filepath.Join(homeDir, "miniconda.exe")

	// Windows 下 Miniconda 可能的安装路径
	possiblePaths := []string{
		filepath.Join(homeDir, "miniconda3", "Scripts", "conda.exe"),
		filepath.Join(homeDir, "AppData", "Local", "miniconda3", "Scripts", "conda.exe"),
		filepath.Join(homeDir, "anaconda3", "Scripts", "conda.exe"),
		filepath.Join(homeDir, "AppData", "Local", "anaconda3", "Scripts", "conda.exe"),
		`C:ProgramDataminiconda3Scriptsconda.exe`,
		`C:ProgramDataanaconda3Scriptsconda.exe`,
	}

	// 检查是否已安装
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("[Miniconda] 已安装在: %s\n", path)
			return path, nil
		}
	}

	fmt.Printf("[Miniconda] 开始下载安装程序...\n")
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "正在下载 Miniconda 安装程序...",
		"progress": 2,
	})

	// 使用 PowerShell 下载 Miniconda 安装程序
	psDownloadCmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("Invoke-WebRequest -Uri \"%s\" -OutFile \"%s\"",
			"https://repo.anaconda.com/miniconda/Miniconda3-latest-Windows-x86_64.exe",
			minicondaInstaller))
	psDownloadCmd.Stdout = os.Stdout
	psDownloadCmd.Stderr = os.Stderr

	if err := psDownloadCmd.Run(); err != nil {
		return "", fmt.Errorf("下载 Miniconda 失败: %v", err)
	}

	// 检查下载是否成功
	if _, err := os.Stat(minicondaInstaller); err != nil {
		return "", fmt.Errorf("下载文件不存在: %v", err)
	}

	fmt.Printf("[Miniconda] 下载完成，开始安装...\n")
	runtime.EventsEmit(a.ctx, "install:progress", map[string]interface{}{
		"status":   "running",
		"message":  "正在安装 Miniconda（可能需要几分钟）...",
		"progress": 3,
	})

	// 使用 PowerShell 静默安装 Miniconda
	psInstallCmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("Start-Process -FilePath \"%s\" -ArgumentList '/S' -Wait",
			minicondaInstaller))
	psInstallCmd.Stdout = os.Stdout
	psInstallCmd.Stderr = os.Stderr

	if err := psInstallCmd.Run(); err != nil {
		return "", fmt.Errorf("安装 Miniconda 失败: %v", err)
	}

	// 清理安装程序
	os.Remove(minicondaInstaller)

	// 验证安装（重新检查所有可能路径）
	fmt.Printf("[Miniconda] 正在查找 conda.exe...\n")
	var installedPath string
	for _, path := range possiblePaths {
		fmt.Printf("[Miniconda] 检查路径: %s\n", path)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("[Miniconda] 找到: %s\n", path)
			installedPath = path
			break
		} else {
			fmt.Printf("[Miniconda] 未找到: %v\n", err)
		}
	}

	// 如果还是没找到，尝试搜索 homeDir 下的所有可能位置
	if installedPath == "" {
		fmt.Printf("[Miniconda] 尝试搜索 homeDir 下的 conda.exe...\n")
		filepath.Walk(homeDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && info.Name() == "conda.exe" {
				fmt.Printf("[Miniconda] 搜索发现: %s\n", path)
				if installedPath == "" {
					installedPath = path
				}
			}
			return nil
		})
	}

	if installedPath == "" {
		return "", fmt.Errorf("安装后未找到 conda，请检查安装是否成功")
	}

	fmt.Printf("[Miniconda] 安装成功: %s\n", installedPath)

	// 接受 Conda Terms of Service（Windows 下需要接受所有可能用到的 channel）
	fmt.Printf("[Miniconda] 接受 Terms of Service...\n")
	channels := []string{
		"https://repo.anaconda.com/pkgs/main",
		"https://repo.anaconda.com/pkgs/r",
		"https://repo.anaconda.com/pkgs/msys2",
		"https://conda.anaconda.org/conda-forge",
		"https://conda.anaconda.org/pytorch",
		"https://conda.anaconda.org/nvidia",
		"https://conda.anaconda.org/huggingface",
	}
	for _, channel := range channels {
		exec.Command(installedPath, "tos", "accept", "--override-channels", "--channel", channel).Run()
	}

	return installedPath, nil
}
