package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SubtitleItem 表示一个字幕条目
type SubtitleItem struct {
	ID             int     `json:"id"`
	StartTime      float64 `json:"startTime"`      // 开始时间（秒）
	EndTime        float64 `json:"endTime"`        // 结束时间（秒）
	Text           string  `json:"text"`           // 字幕文本（原文）
	TranslatedText string  `json:"translatedText"` // 翻译文本（中文）
}

// SubtitleResult 表示字幕生成结果
type SubtitleResult struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message"`
	Subtitles []SubtitleItem `json:"subtitles"`
}

// ProgressInfo 进度信息
type ProgressInfo struct {
	Status   string  `json:"status"`   // 状态: processing, completed, error
	Progress float64 `json:"progress"` // 进度 0-100
	Message  string  `json:"message"`  // 提示信息
}

// GenerateSubtitle 使用本地 Whisper 生成字幕
func (a *App) GenerateSubtitle(videoPath string, model string, language string) SubtitleResult {
	if videoPath == "" {
		return SubtitleResult{
			Success: false,
			Message: "请选择视频文件",
		}
	}

	// 检查视频文件是否存在
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return SubtitleResult{
			Success: false,
			Message: "视频文件不存在",
		}
	}

	// 设置默认模型
	if model == "" {
		model = "base"
	}

	// 获取 whisper 命令路径（在 conda 环境中）
	whisperCmd := getWhisperCommand()

	// 准备路径 - 保存到视频所在目录
	videoDir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	jsonPath := filepath.Join(videoDir, baseName+".json")
	srtPath := filepath.Join(videoDir, baseName+".srt")

	// 构建本地 whisper 命令参数 - 使用 all 生成所有格式
	whisperArgs := []string{
		videoPath,
		"--model", model,
		"--output_format", "all",
		"--output_dir", videoDir,
	}

	if language != "" && language != "auto" {
		whisperArgs = append(whisperArgs, "--language", language)
	}

	// 发送开始事件
	runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
		Status:   "processing",
		Progress: 0,
		Message:  "正在加载 Whisper 模型...",
	})

	// 执行 whisper 命令（使用完整路径）
	cmd := exec.Command(whisperCmd, whisperArgs...)

	// 获取 stderr 用于解析进度
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return SubtitleResult{
			Success: false,
			Message: fmt.Sprintf("创建管道失败: %v", err),
		}
	}

	fmt.Println("执行命令:", cmd.String())

	// 启动命令
	if err := cmd.Start(); err != nil {
		return SubtitleResult{
			Success: false,
			Message: fmt.Sprintf("启动 Whisper 失败: %v", err),
		}
	}

	// 解析进度
	go func() {
		scanner := bufio.NewScanner(stderr)
		progressRegex := regexp.MustCompile(`(\d+)%`)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("Whisper:", line)

			// 尝试解析进度百分比
			matches := progressRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				if percent, err := strconv.Atoi(matches[1]); err == nil {
					runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
						Status:   "processing",
						Progress: float64(percent),
						Message:  fmt.Sprintf("正在转录... %d%%", percent),
					})
				}
			}
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
			Status:   "error",
			Progress: 0,
			Message:  fmt.Sprintf("转录失败: %v", err),
		})
		return SubtitleResult{
			Success: false,
			Message: fmt.Sprintf("Whisper Docker 执行失败: %v", err),
		}
	}

	// 读取生成的 JSON 文件
	subtitles, err := parseWhisperJSON(jsonPath)
	if err != nil {
		return SubtitleResult{
			Success: false,
			Message: fmt.Sprintf("解析字幕文件失败: %v", err),
		}
	}

	// 发送完成事件
	runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
		Status:   "completed",
		Progress: 100,
		Message:  fmt.Sprintf("字幕生成成功！已保存到: %s", srtPath),
	})

	return SubtitleResult{
		Success:   true,
		Message:   fmt.Sprintf("字幕生成成功！\nSRT: %s\nJSON: %s", srtPath, jsonPath),
		Subtitles: subtitles,
	}
}

// parseWhisperJSON 解析 Whisper 生成的 JSON 文件
func parseWhisperJSON(jsonPath string) ([]SubtitleItem, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}

	var result struct {
		Segments []struct {
			ID    int     `json:"id"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"segments"`
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	subtitles := make([]SubtitleItem, 0, len(result.Segments))
	for _, seg := range result.Segments {
		subtitles = append(subtitles, SubtitleItem{
			ID:        seg.ID,
			StartTime: seg.Start,
			EndTime:   seg.End,
			Text:      strings.TrimSpace(seg.Text),
		})
	}

	return subtitles, nil
}

// GetCurrentSubtitle 根据当前时间获取应该显示的字幕
func (a *App) GetCurrentSubtitle(subtitles []SubtitleItem, currentTime float64) *SubtitleItem {
	for i := range subtitles {
		if currentTime >= subtitles[i].StartTime && currentTime <= subtitles[i].EndTime {
			return &subtitles[i]
		}
	}
	return nil
}

// FormatTime 将秒数格式化为时间字符串 (MM:SS 或 HH:MM:SS)
func (a *App) FormatTime(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// ParseTime 将时间字符串解析为秒数
func (a *App) ParseTime(timeStr string) float64 {
	parts := strings.Split(timeStr, ":")
	if len(parts) == 2 {
		// MM:SS 格式
		m, _ := strconv.Atoi(parts[0])
		s, _ := strconv.Atoi(parts[1])
		return float64(m*60 + s)
	} else if len(parts) == 3 {
		// HH:MM:SS 格式
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		s, _ := strconv.Atoi(parts[2])
		return float64(h*3600 + m*60 + s)
	}
	return 0
}

// getWhisperCommand 获取 whisper 命令的完整路径（与 App.getWhisperCommandPath 保持一致）
func getWhisperCommand() string {
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
