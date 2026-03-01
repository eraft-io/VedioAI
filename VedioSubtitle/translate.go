package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// TranslateResult 翻译结果
type TranslateResult struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message"`
	Subtitles []SubtitleItem `json:"subtitles"`
}

// TranslateProgress 翻译进度
type TranslateProgress struct {
	Status   string  `json:"status"`   // processing, completed, error
	Progress float64 `json:"progress"` // 0-100
	Message  string  `json:"message"`  // 提示信息
}

const (
	llamaCppVersion = "b4402"
	modelFileName   = "qwen2.5-3b-instruct-q4_k_m.gguf"
)

// getModelURLs 返回模型下载地址列表（按优先级排序）
func getModelURLs() []string {
	return []string{
		// 国内镜像源（优先）
		"https://hf-mirror.com/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf",
		// 官方源
		"https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf",
	}
}

// getLlamaCppPythonCmd 获取 llama-cpp-python 命令
func getLlamaCppPythonCmd() *exec.Cmd {
	condaPath, _ := exec.LookPath("conda")
	return exec.Command(condaPath, "run", "-n", "whisper", "python", "-c")
}

// getModelPath 获取模型文件路径
func getModelPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cache", "video-subtitle-translator", "models", modelFileName)
}

// getCondaPath 获取 conda 路径（translate.go 中使用）
func getCondaPathTranslate() string {
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

// isLlamaCppInstalled 检查 llama.cpp 是否已安装
func isLlamaCppInstalled() bool {
	// 检查是否可以通过 Python 导入 llama_cpp
	condaPath := getCondaPathTranslate()
	if condaPath == "" {
		return false
	}
	cmd := exec.Command(condaPath, "run", "-n", "whisper", "python", "-c", "import llama_cpp")
	err := cmd.Run()
	return err == nil
}

// isModelDownloaded 检查模型是否已下载
func isModelDownloaded() bool {
	modelPath := getModelPath()
	_, err := os.Stat(modelPath)
	return err == nil
}

// installLlamaCpp 安装 llama.cpp (通过 conda)
func installLlamaCpp() error {
	// 使用 conda 安装 llama.cpp
	fmt.Println("正在通过 conda 安装 llama-cpp-python...")

	// 检查 conda 是否可用
	condaPath := getCondaPathTranslate()
	if condaPath == "" {
		return fmt.Errorf("未找到 conda，请先安装 Anaconda 或 Miniconda")
	}

	fmt.Printf("找到 conda: %s\n", condaPath)

	// 检测操作系统和硬件
	homeDir, _ := os.UserHomeDir()

	// macOS 启用 Metal 支持
	installCmd := exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install",
		"llama-cpp-python", "--no-cache-dir", "--force-reinstall")

	// 设置环境变量启用 Metal (macOS)
	env := os.Environ()
	env = append(env, "CMAKE_ARGS=-DGGML_METAL=ON")
	env = append(env, "FORCE_CMAKE=1")
	installCmd.Env = env

	output, err := installCmd.CombinedOutput()

	fmt.Printf("安装输出: %s\n", string(output))

	if err != nil {
		return fmt.Errorf("安装 llama-cpp-python 失败: %v", err)
	}

	fmt.Println("llama-cpp-python 安装成功")

	// 创建 Metal 缓存目录（如果不存在）
	metalCacheDir := filepath.Join(homeDir, ".cache", "llama.cpp")
	os.MkdirAll(metalCacheDir, 0755)

	return nil
}

// downloadModel 下载翻译模型
func downloadModel(ctx context.Context) error {
	modelPath := getModelPath()
	modelDir := filepath.Dir(modelPath)

	// 创建目录
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 发送进度事件
	runtime.EventsEmit(ctx, "translate:progress", TranslateProgress{
		Status:   "processing",
		Progress: 0,
		Message:  "正在下载翻译模型...",
	})

	// 首先尝试使用 modelscope 命令行工具下载（国内最快）
	if err := downloadWithModelScope(ctx, modelPath); err == nil {
		runtime.EventsEmit(ctx, "translate:progress", TranslateProgress{
			Status:   "processing",
			Progress: 100,
			Message:  "模型下载完成",
		})
		return nil
	}

	// 如果 modelscope 失败，尝试 HTTP 下载
	var lastErr error
	urls := getModelURLs()
	for i, url := range urls {
		if i > 0 {
			runtime.EventsEmit(ctx, "translate:progress", TranslateProgress{
				Status:   "processing",
				Progress: 0,
				Message:  fmt.Sprintf("镜像源 %d 失败，尝试备用源...", i),
			})
		}
		lastErr = downloadFileWithProgress(ctx, url, modelPath)
		if lastErr == nil {
			break
		}
		fmt.Printf("下载源 %d 失败: %v\n", i+1, lastErr)
	}

	if lastErr != nil {
		return fmt.Errorf("下载模型失败: %v", lastErr)
	}

	runtime.EventsEmit(ctx, "translate:progress", TranslateProgress{
		Status:   "processing",
		Progress: 100,
		Message:  "模型下载完成",
	})

	return nil
}

// downloadWithModelScope 使用 modelscope 命令行工具下载模型
func downloadWithModelScope(ctx context.Context, modelPath string) error {
	modelDir := filepath.Dir(modelPath)

	// 检查 modelscope 是否已安装
	cmd := exec.Command("modelscope", "--version")
	if err := cmd.Run(); err != nil {
		fmt.Println("modelscope 命令未找到，尝试安装...")
		// 尝试安装 modelscope
		installCmd := exec.Command("pip", "install", "modelscope")
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("安装 modelscope 失败: %v", err)
		}
	}

	fmt.Println("使用 ModelScope 下载模型...")
	runtime.EventsEmit(ctx, "translate:progress", TranslateProgress{
		Status:   "processing",
		Progress: 10,
		Message:  "使用 ModelScope 下载模型...",
	})

	// 使用 modelscope 下载模型
	// modelscope download --model Qwen/Qwen2.5-3B-Instruct-GGUF --include qwen2.5-3b-instruct-q4_k_m.gguf
	downloadCmd := exec.Command("modelscope", "download",
		"--model", "Qwen/Qwen2.5-3B-Instruct-GGUF",
		"--include", "qwen2.5-3b-instruct-q4_k_m.gguf",
		"--local_dir", modelDir)

	// 获取 stdout 和 stderr
	stdout, _ := downloadCmd.StdoutPipe()
	stderr, _ := downloadCmd.StderrPipe()

	if err := downloadCmd.Start(); err != nil {
		return fmt.Errorf("启动 modelscope 下载失败: %v", err)
	}

	// 实时读取输出
	go func() {
		if stdout != nil {
			buf := make([]byte, 1024)
			for {
				n, err := stdout.Read(buf)
				if n > 0 {
					fmt.Printf("[modelscope] %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}
	}()

	go func() {
		if stderr != nil {
			buf := make([]byte, 1024)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					fmt.Printf("[modelscope] %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}
	}()

	if err := downloadCmd.Wait(); err != nil {
		return fmt.Errorf("modelscope 下载失败: %v", err)
	}

	// 检查文件是否下载成功
	// modelscope 会下载到 local_dir/Qwen2.5-3B-Instruct-GGUF/qwen2.5-3b-instruct-q4_k_m.gguf
	downloadedPath := filepath.Join(modelDir, "Qwen2.5-3B-Instruct-GGUF", "qwen2.5-3b-instruct-q4_k_m.gguf")
	if _, err := os.Stat(downloadedPath); err == nil {
		// 移动到目标路径
		if err := os.Rename(downloadedPath, modelPath); err != nil {
			return fmt.Errorf("移动模型文件失败: %v", err)
		}
		// 清理空目录
		os.RemoveAll(filepath.Join(modelDir, "Qwen2.5-3B-Instruct-GGUF"))
		fmt.Println("ModelScope 下载成功")
		return nil
	}

	// 检查是否直接下载到了目标路径
	if _, err := os.Stat(modelPath); err == nil {
		fmt.Println("ModelScope 下载成功")
		return nil
	}

	return fmt.Errorf("模型文件未找到")
}

// downloadFile 下载文件（带重试）
func downloadFile(url, filepath string) error {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			fmt.Printf("下载失败，第 %d 次重试...\n", i)
			time.Sleep(time.Second * 2)
		}

		err := downloadFileOnce(url, filepath)
		if err == nil {
			return nil
		}
		lastErr = err

		// 删除不完整的文件
		os.Remove(filepath)
	}

	return fmt.Errorf("下载失败（已重试 %d 次）: %v", maxRetries, lastErr)
}

// downloadFileOnce 单次下载文件
func downloadFileOnce(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// downloadFileWithProgress 带进度下载文件（带重试）
func downloadFileWithProgress(ctx context.Context, url, filepath string) error {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			runtime.EventsEmit(ctx, "translate:progress", TranslateProgress{
				Status:   "processing",
				Progress: 0,
				Message:  fmt.Sprintf("下载失败，第 %d 次重试...", i),
			})
			time.Sleep(time.Second * 2)
		}

		err := downloadFileWithProgressOnce(ctx, url, filepath)
		if err == nil {
			return nil
		}
		lastErr = err

		// 删除不完整的文件
		os.Remove(filepath)
	}

	return fmt.Errorf("下载失败（已重试 %d 次）: %v", maxRetries, lastErr)
}

// downloadFileWithProgressOnce 单次带进度下载文件
func downloadFileWithProgressOnce(ctx context.Context, url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}

	totalSize := resp.ContentLength
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 创建进度读取器
	reader := &progressReader{
		reader:     resp.Body,
		total:      totalSize,
		downloaded: 0,
		ctx:        ctx,
		lastEmit:   time.Now(),
	}

	_, err = io.Copy(out, reader)
	return err
}

// progressReader 带进度报告的读取器
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	ctx        context.Context
	lastEmit   time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)

	// 每 500ms 报告一次进度
	if time.Since(pr.lastEmit) > 500*time.Millisecond {
		progress := float64(pr.downloaded) / float64(pr.total) * 100
		runtime.EventsEmit(pr.ctx, "translate:progress", TranslateProgress{
			Status:   "processing",
			Progress: progress,
			Message:  fmt.Sprintf("正在下载翻译模型... %.1f%%", progress),
		})
		pr.lastEmit = time.Now()
	}

	return n, err
}

// unzip 解压 zip 文件
func unzip(src, dest string) error {
	cmd := exec.Command("unzip", "-o", src, "-d", dest)
	return cmd.Run()
}

// TranslateSubtitles 翻译字幕
func (a *App) TranslateSubtitles(subtitles []SubtitleItem) TranslateResult {
	if len(subtitles) == 0 {
		return TranslateResult{
			Success: false,
			Message: "没有需要翻译的字幕",
		}
	}

	// 检查 llama.cpp 是否已安装
	if !isLlamaCppInstalled() {
		runtime.EventsEmit(a.ctx, "translate:progress", TranslateProgress{
			Status:   "processing",
			Progress: 0,
			Message:  "正在安装 llama.cpp...",
		})

		if err := installLlamaCpp(); err != nil {
			return TranslateResult{
				Success: false,
				Message: fmt.Sprintf("安装 llama.cpp 失败: %v", err),
			}
		}
	}

	// 检查模型是否已下载
	if !isModelDownloaded() {
		if err := downloadModel(a.ctx); err != nil {
			return TranslateResult{
				Success: false,
				Message: fmt.Sprintf("下载模型失败: %v", err),
			}
		}
	}

	modelPath := getModelPath()

	// 翻译每个字幕
	translatedSubtitles := make([]SubtitleItem, len(subtitles))
	for i, subtitle := range subtitles {
		progress := float64(i) / float64(len(subtitles)) * 100
		runtime.EventsEmit(a.ctx, "translate:progress", TranslateProgress{
			Status:   "processing",
			Progress: progress,
			Message:  fmt.Sprintf("正在翻译字幕 %d/%d...", i+1, len(subtitles)),
		})

		translatedText, err := translateItem(subtitle.Text, modelPath)
		if err != nil {
			// 翻译失败，保留原文
			translatedText = subtitle.Text
			fmt.Printf("[翻译失败] 原文: %s -> 错误: %v\n", subtitle.Text, err)
		} else {
			fmt.Printf("[翻译成功] 原文: %s -> 译文: %s\n", subtitle.Text, translatedText)
		}

		translatedSubtitles[i] = SubtitleItem{
			ID:             subtitle.ID,
			StartTime:      subtitle.StartTime,
			EndTime:        subtitle.EndTime,
			Text:           subtitle.Text,
			TranslatedText: translatedText,
		}
	}

	runtime.EventsEmit(a.ctx, "translate:progress", TranslateProgress{
		Status:   "completed",
		Progress: 100,
		Message:  "翻译完成！",
	})

	return TranslateResult{
		Success:   true,
		Message:   fmt.Sprintf("翻译完成！共 %d 条字幕", len(translatedSubtitles)),
		Subtitles: translatedSubtitles,
	}
}

// translateItem 翻译单个文本
func translateItem(text string, modelPath string) (string, error) {
	// 创建临时文件存储结果
	tmpFile, err := os.CreateTemp("", "translate_*.txt")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 构建 Python 脚本 - 将结果写入文件
	pythonScript := fmt.Sprintf(`
import sys
import os
import warnings
warnings.filterwarnings('ignore')

# 检查模型文件是否存在
model_path = "%s"
if not os.path.exists(model_path):
    with open("%s", "w", encoding="utf-8") as f:
        f.write(f"ERROR: Model file not found: {model_path}")
    sys.exit(1)

# 检查模型文件大小
file_size = os.path.getsize(model_path)
if file_size < 1000000:  # 小于1MB可能下载不完整
    with open("%s", "w", encoding="utf-8") as f:
        f.write(f"ERROR: Model file too small ({file_size} bytes), may be corrupted")
    sys.exit(1)

try:
    from llama_cpp import Llama
    
    # 加载模型，启用 GPU 加速
    # n_gpu_layers: 将尽可能多的层加载到 GPU
    # n_ctx: 上下文长度
    # n_batch: 批处理大小
    llm = Llama(
        model_path=model_path, 
        n_ctx=256, 
        verbose=False,
        n_gpu_layers=-1,  # 自动检测并加载所有层到 GPU
        n_batch=512       # 增大批处理大小提高吞吐量
    )
    
    prompt = "Translate the following English text to Chinese. Only return the Chinese translation, no explanation.\n\nEnglish: %s\n\nChinese:"
    
    output = llm(prompt, max_tokens=64, temperature=0.1, stop=["\n", "English:"])
    result = output['choices'][0]['text'].strip()
    
    # 将结果写入文件
    with open("%s", "w", encoding="utf-8") as f:
        f.write(result)
except Exception as e:
    import traceback
    # 错误写入文件
    with open("%s", "w", encoding="utf-8") as f:
        f.write(f"ERROR: {e}\n{traceback.format_exc()}")
    sys.exit(1)
`, modelPath, tmpFile.Name(), tmpFile.Name(), text, tmpFile.Name(), tmpFile.Name())

	// 执行 Python 脚本
	condaPath := getCondaPathTranslate()
	if condaPath == "" {
		return "", fmt.Errorf("未找到 conda")
	}
	cmd := exec.Command(condaPath, "run", "-n", "whisper", "python", "-c", pythonScript)

	output, err := cmd.CombinedOutput()

	// 从文件读取结果
	resultBytes, readErr := os.ReadFile(tmpFile.Name())
	if readErr != nil {
		return "", fmt.Errorf("读取结果文件失败: %v", readErr)
	}

	result := strings.TrimSpace(string(resultBytes))

	// 打印调试信息
	fmt.Printf("\n=== 翻译调试 ===\n")
	fmt.Printf("翻译输入: %s\n", text)
	fmt.Printf("文件结果: '%s'\n", result)
	if len(output) > 0 {
		fmt.Printf("命令输出: %s\n", string(output))
	}
	fmt.Printf("================\n")

	// 清理结果
	result = cleanTranslationResult(result, "")

	return result, nil
}

// cleanTranslationResult 清理翻译结果
func cleanTranslationResult(result, prompt string) string {
	// 移除提示词残留
	if prompt != "" {
		result = strings.ReplaceAll(result, prompt, "")
	}

	// 移除常见的模型输出前缀
	prefixes := []string{
		"Translation:", "翻译：", "中文：", "Chinese:",
		"Chinese translation:", "译文：", "翻译结果：",
		"English:", "原文：",
	}
	for _, prefix := range prefixes {
		result = strings.TrimPrefix(result, prefix)
	}

	// 按行处理，过滤掉空行和指令行
	lines := strings.Split(result, "\n")
	var filteredLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 跳过指令相关行
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "translate") ||
			strings.HasPrefix(lower, "english:") ||
			strings.HasPrefix(lower, "chinese:") ||
			strings.HasPrefix(lower, "note:") ||
			strings.HasPrefix(lower, "explanation:") {
			continue
		}
		filteredLines = append(filteredLines, trimmed)
	}

	// 如果有多行，取第一行（通常是翻译结果）
	if len(filteredLines) > 0 {
		result = filteredLines[0]
	} else {
		result = ""
	}

	// 清理空白字符
	result = strings.TrimSpace(result)

	// 移除可能的引号
	result = strings.Trim(result, `"'`)

	return result
}

// CheckTranslateStatus 检查翻译环境状态
func (a *App) CheckTranslateStatus() map[string]interface{} {
	status := map[string]interface{}{
		"llamaInstalled":  isLlamaCppInstalled(),
		"modelDownloaded": isModelDownloaded(),
		"ready":           isLlamaCppInstalled() && isModelDownloaded(),
	}
	return status
}

// translateTextRealtime 实时翻译单个文本（供 OCR 使用）
func translateTextRealtime(text string) (string, error) {
	if text == "" {
		return "", nil
	}

	// 检查模型是否已下载
	if !isModelDownloaded() {
		return "", fmt.Errorf("翻译模型未下载")
	}

	modelPath := getModelPath()
	return translateItem(text, modelPath)
}

// TranslateText 翻译文本（Wails 绑定）
func (a *App) TranslateText(text string) map[string]interface{} {
	result := map[string]interface{}{
		"success":     false,
		"message":     "",
		"translation": "",
	}

	if text == "" {
		result["message"] = "文本为空"
		return result
	}

	translation, err := translateTextRealtime(text)
	if err != nil {
		result["message"] = err.Error()
		return result
	}

	result["success"] = true
	result["translation"] = translation
	return result
}

// SummarizeResult 总结结果
type SummarizeResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	OutputPath string `json:"outputPath"`
}

// SummarizeSubtitles 导出双语字幕对照 HTML 页面
func (a *App) SummarizeSubtitles(subtitles []SubtitleItem, videoPath string) SummarizeResult {
	result := SummarizeResult{
		Success: false,
	}

	if len(subtitles) == 0 {
		result.Message = "没有字幕可以导出"
		return result
	}

	runtime.EventsEmit(a.ctx, "summarize:progress", map[string]interface{}{
		"status":   "processing",
		"progress": 30,
		"message":  "正在生成 HTML 页面...",
	})

	// 确定输出路径
	var outputPath string
	var videoDir string
	if videoPath != "" {
		videoDir = filepath.Dir(videoPath)
		baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
		outputPath = filepath.Join(videoDir, baseName+"_bilingual.html")
	} else {
		homeDir, _ := os.UserHomeDir()
		videoDir = homeDir
		outputPath = filepath.Join(homeDir, "video_bilingual.html")
	}

	// 读取 intelligent_ppt 目录下的图片
	pptDir := filepath.Join(videoDir, "intelligent_ppt")
	var pptImages []PPTImageInfo
	if _, err := os.Stat(pptDir); err == nil {
		// 目录存在，读取图片
		files, err := os.ReadDir(pptDir)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && (strings.HasSuffix(strings.ToLower(file.Name()), ".png") ||
					strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") ||
					strings.HasSuffix(strings.ToLower(file.Name()), ".jpeg")) {
					// 从文件名提取时间戳
					timestamp := extractTimestampFromFilename(file.Name())
					pptImages = append(pptImages, PPTImageInfo{
						Filename:  file.Name(),
						Path:      filepath.Join(pptDir, file.Name()),
						Timestamp: timestamp,
					})
				}
			}
			// 按时间戳排序
			sort.Slice(pptImages, func(i, j int) bool {
				return pptImages[i].Timestamp < pptImages[j].Timestamp
			})
		}
	}

	runtime.EventsEmit(a.ctx, "summarize:progress", map[string]interface{}{
		"status":   "processing",
		"progress": 60,
		"message":  "正在写入文件...",
	})

	// 生成 HTML 内容
	htmlContent := generateBilingualHTML(videoPath, subtitles, pptImages)

	// 写入文件
	if err := os.WriteFile(outputPath, []byte(htmlContent), 0644); err != nil {
		result.Message = fmt.Sprintf("写入文件失败: %v", err)
		return result
	}

	runtime.EventsEmit(a.ctx, "summarize:progress", map[string]interface{}{
		"status":   "completed",
		"progress": 100,
		"message":  "导出完成！",
	})

	result.Success = true
	result.Message = fmt.Sprintf("双语字幕已保存到: %s", outputPath)
	result.OutputPath = outputPath

	return result
}

// PPTImageInfo PPT图片信息
type PPTImageInfo struct {
	Filename  string
	Path      string
	Timestamp float64
}

// extractTimestampFromFilename 从文件名提取时间戳
func extractTimestampFromFilename(filename string) float64 {
	// 文件名格式: ppt_HH_MM_SS_mmm_xxx.png
	re := regexp.MustCompile(`ppt_(\d+)_(\d+)_(\d+)_(\d+)`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) >= 5 {
		hours := 0
		minutes := 0
		seconds := 0
		milliseconds := 0
		fmt.Sscanf(matches[1], "%d", &hours)
		fmt.Sscanf(matches[2], "%d", &minutes)
		fmt.Sscanf(matches[3], "%d", &seconds)
		fmt.Sscanf(matches[4], "%d", &milliseconds)
		return float64(hours*3600+minutes*60+seconds) + float64(milliseconds)/1000.0
	}
	return 0
}

// generateBilingualHTML 生成双语字幕对照 HTML 页面
func generateBilingualHTML(videoPath string, subtitles []SubtitleItem, pptImages []PPTImageInfo) string {
	videoName := "Video Bilingual Subtitles"
	if videoPath != "" {
		videoName = strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	}

	// 计算时长
	var duration string
	if len(subtitles) > 0 {
		totalSeconds := subtitles[len(subtitles)-1].EndTime
		minutes := int(totalSeconds) / 60
		seconds := int(totalSeconds) % 60
		duration = fmt.Sprintf("%d:%02d", minutes, seconds)
	}

	var sb strings.Builder

	// HTML 头部 - Stanford CS336 风格
	sb.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + videoName + ` - 双语字幕</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #ffffff;
            color: #333;
            line-height: 1.6;
            min-height: 100vh;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            padding: 40px 20px;
        }
        .header {
            margin-bottom: 40px;
            padding-bottom: 20px;
            border-bottom: 2px solid #e8e8e8;
        }
        .header h1 {
            color: #2c3e50;
            font-size: 32px;
            font-weight: 600;
            margin-bottom: 16px;
            letter-spacing: -0.5px;
        }
        .stats {
            display: flex;
            gap: 24px;
            color: #666;
            font-size: 14px;
            flex-wrap: wrap;
        }
        .stats span {
            display: flex;
            align-items: center;
            gap: 6px;
        }
        .section-title {
            font-size: 20px;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 1px solid #e8e8e8;
        }
        .subtitle-list {
            display: flex;
            flex-direction: column;
            gap: 16px;
        }
        .subtitle-item {
            padding: 20px;
            background: #fafafa;
            border-radius: 8px;
            border-left: 4px solid #0066cc;
            transition: all 0.2s ease;
        }
        .subtitle-item:hover {
            background: #f5f5f5;
            box-shadow: 0 2px 8px rgba(0,0,0,0.05);
        }
        .time-badge {
            display: inline-block;
            color: #0066cc;
            font-size: 13px;
            font-weight: 600;
            margin-bottom: 12px;
            font-family: 'SF Mono', Monaco, monospace;
        }
        .english-text {
            color: #2c3e50;
            font-size: 16px;
            line-height: 1.7;
            margin-bottom: 10px;
        }
        .chinese-text {
            color: #555;
            font-size: 15px;
            line-height: 1.7;
            padding-left: 16px;
            border-left: 2px solid #ddd;
        }
        .no-translation {
            color: #999;
            font-style: italic;
        }
        .ppt-image {
            margin-top: 16px;
        }
        .ppt-image img {
            max-width: 100%;
            border-radius: 6px;
            box-shadow: 0 2px 12px rgba(0,0,0,0.1);
        }
        .footer {
            margin-top: 60px;
            padding-top: 20px;
            border-top: 1px solid #e8e8e8;
            text-align: center;
            color: #999;
            font-size: 13px;
        }
        @media (max-width: 600px) {
            .container {
                padding: 20px 16px;
            }
            .header h1 {
                font-size: 24px;
            }
            .stats {
                flex-direction: column;
                gap: 8px;
            }
            .subtitle-item {
                padding: 16px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + videoName + `</h1>
            <div class="stats">
                <span>📝 ` + fmt.Sprintf("%d 条字幕", len(subtitles)) + `</span>
                <span>⏱️ ` + duration + `</span>
                <span>📅 ` + time.Now().Format("2006-01-02") + `</span>
            </div>
        </div>
        <div class="section-title">双语字幕对照</div>
        <div class="subtitle-list">
`)

	// 字幕内容
	pptImageIndex := 0
	for i, sub := range subtitles {
		startMin := int(sub.StartTime) / 60
		startSec := int(sub.StartTime) % 60
		endMin := int(sub.EndTime) / 60
		endSec := int(sub.EndTime) % 60
		timeStr := fmt.Sprintf("%02d:%02d - %02d:%02d", startMin, startSec, endMin, endSec)

		sb.WriteString(fmt.Sprintf(`            <div class="subtitle-item">
                <div class="time-badge">#%d | %s</div>
                <div class="english-text">%s</div>
`, i+1, timeStr, sub.Text))

		if sub.TranslatedText != "" {
			sb.WriteString(fmt.Sprintf(`                <div class="chinese-text">%s</div>
`, sub.TranslatedText))
		} else {
			sb.WriteString(`                <div class="chinese-text no-translation">（未翻译）</div>
`)
		}

		// 检查是否有对应的 PPT 图片需要插入（在字幕时间范围内）
		for pptImageIndex < len(pptImages) {
			pptImg := pptImages[pptImageIndex]
			// 如果图片时间戳在当前字幕的时间范围内，插入图片
			if pptImg.Timestamp >= sub.StartTime && pptImg.Timestamp <= sub.EndTime {
				// 将图片转为 base64 或直接使用相对路径
				imgData, err := os.ReadFile(pptImg.Path)
				if err == nil {
					// 使用 base64 编码图片
					base64Img := base64.StdEncoding.EncodeToString(imgData)
					sb.WriteString(fmt.Sprintf(`                <div class="ppt-image">
                    <img src="data:image/png;base64,%s" alt="PPT Slide" style="max-width: 100%%; border-radius: 8px; margin-top: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1);">
                </div>
`, base64Img))
				}
				pptImageIndex++
			} else if pptImg.Timestamp < sub.StartTime {
				// 图片时间戳早于当前字幕，跳过
				pptImageIndex++
			} else {
				// 图片时间戳晚于当前字幕，等待下一个字幕
				break
			}
		}

		sb.WriteString(`            </div>
`)
	}

	// HTML 尾部
	sb.WriteString(`        </div>
        <div class="footer">
            Generated by VideoSubtitle | Powered by Whisper & Qwen2.5
        </div>
    </div>
</body>
</html>
`)

	return sb.String()
}
