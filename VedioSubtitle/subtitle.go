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
	Output   string  `json:"output"`   // 实时输出日志
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

	// 准备路径 - 保存到视频所在目录
	videoDir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	jsonPath := filepath.Join(videoDir, baseName+".json")
	srtPath := filepath.Join(videoDir, baseName+".srt")

	// 获取 conda 路径，使用 python -m whisper 运行
	condaPath := getCondaPath()
	if condaPath == "" {
		return SubtitleResult{
			Success: false,
			Message: "未找到 conda",
		}
	}

	// 创建临时 Python 脚本来运行 whisper 并实时打印进度
	pythonScript := fmt.Sprintf(`
import whisper
import sys
import os
import json

# 加载模型
print("[INFO] 正在加载 Whisper 模型: %s", file=sys.stderr)
model = whisper.load_model("%s")

# 转录音频
print("[INFO] 开始转录音频...", file=sys.stderr)
result = model.transcribe("%s", language=%s, verbose=True)

# 实时打印每个片段
for segment in result["segments"]:
    start = segment["start"]
    end = segment["end"]
    text = segment["text"]
    print(f"[{start:.3f} --> {end:.3f}] {text}", flush=True)

# 保存 JSON 文件
json_path = os.path.join("%s", os.path.splitext(os.path.basename("%s"))[0] + ".json")
with open(json_path, "w", encoding="utf-8") as f:
    json.dump(result, f, ensure_ascii=False, indent=2)
print(f"[INFO] JSON 已保存: {json_path}", file=sys.stderr)

# 保存 SRT 文件
srt_path = os.path.join("%s", os.path.splitext(os.path.basename("%s"))[0] + ".srt")
with open(srt_path, "w", encoding="utf-8") as f:
    for i, segment in enumerate(result["segments"], 1):
        start = segment["start"]
        end = segment["end"]
        text = segment["text"].strip()
        # 转换为 SRT 时间格式
        def sec_to_srt(t):
            h = int(t // 3600)
            m = int((t %% 3600) // 60)
            s = int(t %% 60)
            ms = int((t - int(t)) * 1000)
            return f"{h:02d}:{m:02d}:{s:02d},{ms:03d}"
        f.write(f"{i}\\n")
        f.write(f"{sec_to_srt(start)} --> {sec_to_srt(end)}\\n")
        f.write(f"{text}\\n\\n")
print(f"[INFO] SRT 已保存: {srt_path}", file=sys.stderr)

print("[INFO] 转录完成", file=sys.stderr)
`, model, model, videoPath, fmt.Sprintf("\"%s\"", language), videoDir, videoPath, videoDir, videoPath)

	if language == "" || language == "auto" {
		pythonScript = fmt.Sprintf(`
import whisper
import sys
import os
import json

# 加载模型
print("[INFO] 正在加载 Whisper 模型: %s", file=sys.stderr)
model = whisper.load_model("%s")

# 转录音频（自动检测语言）
print("[INFO] 开始转录音频...", file=sys.stderr)
result = model.transcribe("%s", verbose=True)

# 实时打印每个片段
for segment in result["segments"]:
    start = segment["start"]
    end = segment["end"]
    text = segment["text"]
    print(f"[{start:.3f} --> {end:.3f}] {text}", flush=True)

# 保存 JSON 文件
json_path = os.path.join("%s", os.path.splitext(os.path.basename("%s"))[0] + ".json")
with open(json_path, "w", encoding="utf-8") as f:
    json.dump(result, f, ensure_ascii=False, indent=2)
print(f"[INFO] JSON 已保存: {json_path}", file=sys.stderr)

# 保存 SRT 文件
srt_path = os.path.join("%s", os.path.splitext(os.path.basename("%s"))[0] + ".srt")
with open(srt_path, "w", encoding="utf-8") as f:
    for i, segment in enumerate(result["segments"], 1):
        start = segment["start"]
        end = segment["end"]
        text = segment["text"].strip()
        # 转换为 SRT 时间格式
        def sec_to_srt(t):
            h = int(t // 3600)
            m = int((t %% 3600) // 60)
            s = int(t %% 60)
            ms = int((t - int(t)) * 1000)
            return f"{h:02d}:{m:02d}:{s:02d},{ms:03d}"
        f.write(f"{i}\\n")
        f.write(f"{sec_to_srt(start)} --> {sec_to_srt(end)}\\n")
        f.write(f"{text}\\n\\n")
print(f"[INFO] SRT 已保存: {srt_path}", file=sys.stderr)

print("[INFO] 转录完成", file=sys.stderr)
`, model, model, videoPath, videoDir, videoPath, videoDir, videoPath)
	}

	// 写入临时脚本
	tmpScript := filepath.Join(videoDir, ".whisper_run.py")
	if err := os.WriteFile(tmpScript, []byte(pythonScript), 0644); err != nil {
		return SubtitleResult{
			Success: false,
			Message: fmt.Sprintf("创建临时脚本失败: %v", err),
		}
	}
	defer os.Remove(tmpScript)

	// 构建 whisper 命令参数
	whisperArgs := []string{
		"run", "-n", "whisper",
		"python", tmpScript,
	}

	// 发送开始事件
	runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
		Status:   "processing",
		Progress: 0,
		Message:  "正在加载 Whisper 模型...",
		Output:   "",
	})

	// 执行 whisper 命令（通过 conda run）
	cmd := exec.Command(condaPath, whisperArgs...)

	// 获取 stdout 和 stderr 用于解析进度
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SubtitleResult{
			Success: false,
			Message: fmt.Sprintf("创建 stdout 管道失败: %v", err),
		}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return SubtitleResult{
			Success: false,
			Message: fmt.Sprintf("创建 stderr 管道失败: %v", err),
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

	// 解析进度 - 从时间戳估算进度
	var progressMutex = make(chan float64, 1)
	progressMutex <- 0

	// 处理输出的函数
	processOutput := func(scanner *bufio.Scanner, source string) {
		// 匹配时间戳格式 [0.000 --> 7.000] 或 [00:00.000 --> 00:07.000]
		timestampRegex := regexp.MustCompile(`\[(\d+\.?\d*)\s+-->`)

		for scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("Whisper [%s]: %s\n", source, line)

			// 获取当前进度
			currentProgress := <-progressMutex

			// 发送实时输出到前端
			runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
				Status:   "processing",
				Progress: currentProgress,
				Message:  "正在转录...",
				Output:   line,
			})

			// 尝试解析时间戳来估算进度
			matches := timestampRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				seconds, _ := strconv.ParseFloat(matches[1], 64)
				// 估算进度（假设视频约 5 分钟 = 300 秒）
				// 使用非线性进度，前期快后期慢
				var progress float64
				if seconds < 60 {
					progress = seconds * 0.5 // 前1分钟快速到30%
				} else if seconds < 180 {
					progress = 30 + (seconds-60)*0.4 // 1-3分钟到70%
				} else {
					progress = 70 + (seconds-180)*0.1 // 之后缓慢增长
				}
				if progress > 95 {
					progress = 95 // 留5%给最后处理
				}
				if progress > currentProgress {
					currentProgress = progress
					runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
						Status:   "processing",
						Progress: progress,
						Message:  fmt.Sprintf("正在转录... %.1fs", seconds),
						Output:   "",
					})
				}
			}

			// 更新进度
			progressMutex <- currentProgress
		}
	}

	// 同时处理 stdout 和 stderr
	go func() {
		stdoutScanner := bufio.NewScanner(stdout)
		processOutput(stdoutScanner, "stdout")
	}()
	go func() {
		stderrScanner := bufio.NewScanner(stderr)
		processOutput(stderrScanner, "stderr")
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		runtime.EventsEmit(a.ctx, "subtitle:progress", ProgressInfo{
			Status:   "error",
			Progress: 0,
			Message:  fmt.Sprintf("转录失败: %v", err),
			Output:   fmt.Sprintf("错误: %v", err),
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
		Output:   "",
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
