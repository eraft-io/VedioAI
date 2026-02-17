package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	modelURL        = "https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf"
	modelFileName   = "qwen2.5-3b-instruct-q4_k_m.gguf"
)

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

// isLlamaCppInstalled 检查 llama.cpp 是否已安装
func isLlamaCppInstalled() bool {
	// 检查是否可以通过 Python 导入 llama_cpp
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return false
	}
	cmd := exec.Command(condaPath, "run", "-n", "whisper", "python", "-c", "import llama_cpp")
	err = cmd.Run()
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
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return fmt.Errorf("未找到 conda，请先安装 Anaconda 或 Miniconda")
	}

	// 使用 python -m pip 安装
	installCmd := exec.Command(condaPath, "run", "-n", "whisper", "python", "-m", "pip", "install",
		"llama-cpp-python", "--no-cache-dir")
	installCmd.Env = os.Environ()
	output, err := installCmd.CombinedOutput()

	fmt.Printf("安装输出: %s\n", string(output))

	if err != nil {
		return fmt.Errorf("安装 llama-cpp-python 失败: %v", err)
	}

	fmt.Println("llama-cpp-python 安装成功")
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

	// 下载模型
	if err := downloadFileWithProgress(ctx, modelURL, modelPath); err != nil {
		return fmt.Errorf("下载模型失败: %v", err)
	}

	runtime.EventsEmit(ctx, "translate:progress", TranslateProgress{
		Status:   "processing",
		Progress: 100,
		Message:  "模型下载完成",
	})

	return nil
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
    
    # 加载模型，使用更小的上下文
    llm = Llama(model_path=model_path, n_ctx=256, verbose=False)
    
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
	condaPath, _ := exec.LookPath("conda")
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
