package main

import (
	// "crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// PPTFrame PPT帧信息
type PPTFrame struct {
	Index     int     `json:"index"`
	Timestamp float64 `json:"timestamp"`
	Filename  string  `json:"filename"`
	Path      string  `json:"path"`
}

// PPTProgressInfo PPT提取进度信息
type PPTProgressInfo struct {
	Status   string `json:"status"`   // processing, completed, error
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
	Current  int    `json:"current"`
	Total    int    `json:"total"`
}

// IntelligentPPTResultPPT提取结果
type IntelligentPPTResult struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Frames  []PPTFrame `json:"frames"`
	Dir     string     `json:"dir"`
}

// KeyFrameInfo 关键帧信息
type KeyFrameInfo struct {
	Timestamp  float64 `json:"timestamp"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
	IsKeyPoint bool    `json:"isKeyPoint"`
}

// AnalyzeSubtitlesByContent基于字幕内容分析关键帧
func (a *App) AnalyzeSubtitlesByContent(subtitles []SubtitleItem, videoPath string, imageDir string) IntelligentPPTResult {
	result := IntelligentPPTResult{
		Success: false,
		Message: "",
		Frames:  []PPTFrame{},
		Dir:     "",
	}

	if len(subtitles) == 0 {
		result.Message = "没有字幕数据，请先生成字幕"
		return result
	}

	if videoPath == "" {
		result.Message = "请选择视频文件"
		return result
	}

	// 默认图片目录
	if imageDir == "" {
		imageDir = "intelligent_ppt"
	}

	// 创建输出目录
	videoDir := filepath.Dir(videoPath)
	pptDir := filepath.Join(videoDir, imageDir)
	if err := os.MkdirAll(pptDir, 0755); err != nil {
		result.Message = fmt.Sprintf("创建目录失败: %v", err)
		return result
	}
	result.Dir = pptDir

	runtime.EventsEmit(a.ctx, "intelligent:progress", PPTProgressInfo{
		Status:   "processing",
		Progress: 0,
		Message:  "正在分析字幕内容...",
		Current:  0,
		Total:    len(subtitles),
	})

	// 1. 识别关键内容段落
	keyPoints := a.identifyKeyContentPoints(subtitles)
	fmt.Printf("[智能PPT] 识别到 %d 个关键内容点\n", len(keyPoints))

	// 2. 为每个关键点提取帧
	frames := []PPTFrame{}
	for i, point := range keyPoints {
		progress := int(float64(i+1) / float64(len(keyPoints)) * 80)
		runtime.EventsEmit(a.ctx, "intelligent:progress", PPTProgressInfo{
			Status:   "processing",
			Progress: progress,
			Message:  fmt.Sprintf("正在提取关键帧 %d/%d...", i+1, len(keyPoints)),
			Current:  i + 1,
			Total:    len(keyPoints),
		})

		frame := a.extractKeyFrameAtTime(point.Timestamp, videoPath, pptDir, i+1, point.Content)
		if frame != nil {
			frames = append(frames, *frame)
		}
	}

	// 3. 完成
	runtime.EventsEmit(a.ctx, "intelligent:progress", PPTProgressInfo{
		Status:   "completed",
		Progress: 100,
		Message:  fmt.Sprintf("成功提取 %d 个智能关键帧", len(frames)),
		Current:  len(frames),
		Total:    len(frames),
	})

	result.Success = true
	result.Message = fmt.Sprintf("成功提取 %d 个智能关键帧到: %s", len(frames), pptDir)
	result.Frames = frames

	return result
}

// identifyKeyContentPoints 识别关键内容点
func (a *App) identifyKeyContentPoints(subtitles []SubtitleItem) []KeyFrameInfo {
	var keyPoints []KeyFrameInfo

	// 关键词模式（中英文）
	keyPatterns := []struct {
		patterns []string
		weight   float64
		timeBias float64 // 时间偏移（秒）
	}{
		{
			patterns: []string{
				"首先", "第一", "开始", "介绍", "概述", "总览",
				"first", "begin", "introduction", "overview", "start",
			},
			weight:   0.8,
			timeBias: 2.0,
		},
		{
			patterns: []string{
				"其次", "然后", "接下来", "步骤", "流程", "方法",
				"second", "next", "then", "step", "process", "method",
			},
			weight:   0.7,
			timeBias: 1.0,
		},
		{
			patterns: []string{
				"最后", "总结", "要点", "重点", "关键", "核心",
				"finally", "last", "summary", "key", "important", "core",
			},
			weight:   0.9,
			timeBias: -1.0,
		},
		{
			patterns: []string{
				"图", "图表", "示意图", "图示", "图片", "图像",
				"figure", "chart", "diagram", "image", "graph", "picture",
			},
			weight:   0.85,
			timeBias: 0.0,
		},
		{
			patterns: []string{
				"注意", "重要", "特别", "强调", "关键点",
				"note", "important", "special", "emphasize", "key point",
			},
			weight:   0.95,
			timeBias: 0.0,
		},
	}

	// 分析每个字幕项
	for _, sub := range subtitles {
		text := strings.ToLower(sub.Text + " " + sub.TranslatedText)

		for _, patternGroup := range keyPatterns {
			for _, pattern := range patternGroup.patterns {
				if strings.Contains(text, pattern) {
					//计算时间戳（考虑偏移）
					timestamp := sub.StartTime + patternGroup.timeBias
					if timestamp < 0 {
						timestamp = 0
					}
					if timestamp > sub.EndTime {
						timestamp = sub.EndTime
					}

					keyPoints = append(keyPoints, KeyFrameInfo{
						Timestamp:  timestamp,
						Content:    sub.Text,
						Confidence: patternGroup.weight,
						IsKeyPoint: true,
					})
					break
				}
			}
		}
	}

	//和排序
	keyPoints = a.deduplicateKeyPoints(keyPoints)

	// 如果没有识别到关键点，按时间均匀采样
	if len(keyPoints) == 0 {
		keyPoints = a.uniformSampling(subtitles, 8) // 默认采样8帧
	}

	return keyPoints
}

// deduplicateKeyPoints去关键点
func (a *App) deduplicateKeyPoints(points []KeyFrameInfo) []KeyFrameInfo {
	if len(points) <= 1 {
		return points
	}

	//按时间戳排序
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp < points[j].Timestamp
	})

	//去（时间间隔小于3秒的合并）
	result := []KeyFrameInfo{points[0]}
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp-result[len(result)-1].Timestamp > 3.0 {
			result = append(result, points[i])
		} else {
			// 保留置信度更高的
			if points[i].Confidence > result[len(result)-1].Confidence {
				result[len(result)-1] = points[i]
			}
		}
	}

	return result
}

// uniformSampling时间采样
func (a *App) uniformSampling(subtitles []SubtitleItem, count int) []KeyFrameInfo {
	if len(subtitles) == 0 || count <= 0 {
		return []KeyFrameInfo{}
	}

	var keyPoints []KeyFrameInfo
	step := len(subtitles) / count
	if step == 0 {
		step = 1
	}

	for i := 0; i < len(subtitles) && len(keyPoints) < count; i += step {
		sub := subtitles[i]
		keyPoints = append(keyPoints, KeyFrameInfo{
			Timestamp:  sub.StartTime,
			Content:    sub.Text,
			Confidence: 0.5, //采样置信度较低
			IsKeyPoint: false,
		})
	}

	return keyPoints
}

// extractKeyFrameAtTime 在指定时间提取关键帧
func (a *App) extractKeyFrameAtTime(timestamp float64, videoPath, outputDir string, index int, content string) *PPTFrame {
	//格化时间戳为文件名
	hours := int(timestamp / 3600)
	minutes := int((timestamp - float64(hours*3600)) / 60)
	seconds := int(timestamp - float64(hours*3600) - float64(minutes*60))
	milliseconds := int((timestamp - float64(int(timestamp))) * 1000)

	//清理内容作为文件名的一部分
	cleanContent := regexp.MustCompile(`[^\w\x{4e00}-\x{9fa5}]`).ReplaceAllString(content, "_")
	if len(cleanContent) > 20 {
		cleanContent = cleanContent[:20]
	}

	// 获取 conda路径
	condaPath := getCondaPath()
	if condaPath == "" {
		fmt.Printf("[智能PPT] 未找到 conda\n")
		return nil
	}

	// 使用临时文件名先提取帧
	tempFilename := fmt.Sprintf("temp_%02d_%02d_%02d_%03d_%d.png",
		hours, minutes, seconds, milliseconds, index)
	tempPath := filepath.Join(outputDir, tempFilename)

	// 使用 ffmpeg 提取帧
	cmd := exec.Command(
		condaPath, "run", "-n", "whisper",
		"ffmpeg",
		"-ss", fmt.Sprintf("%.3f", timestamp),
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2", //高质量
		"-y",
		tempPath,
	)

	//执行
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		fmt.Printf("[智能PPT] 提取帧失败 (%.3fs): %v\n", timestamp, err)
		return nil
	}

	// // 计算文件MD5值
	// fileData, err := os.ReadFile(tempPath)
	// if err != nil {
	// 	fmt.Printf("[智能PPT] 读取文件失败: %v\n", err)
	// 	os.Remove(tempPath)
	// 	return nil
	// }
	// md5Hash := fmt.Sprintf("%x", md5.Sum(fileData))
	// // 取MD5前8位作为短标识
	// shortMD5 := md5Hash[:8]

	// 生成最终文件名（包含MD5）
	filename := fmt.Sprintf("ppt_%02d_%02d_%02d_%03d_%s.png",
		hours, minutes, seconds, milliseconds, cleanContent)
	outputPath := filepath.Join(outputDir, filename)

	// 重命名文件
	if err := os.Rename(tempPath, outputPath); err != nil {
		fmt.Printf("[智能PPT] 重命名文件失败: %v\n", err)
		os.Remove(tempPath)
		return nil
	}

	frame := &PPTFrame{
		Index:     index,
		Timestamp: timestamp,
		Filename:  filename,
		Path:      outputPath,
	}

	fmt.Printf("[智能PPT] 成功提取帧: %s (时间: %s, 内容: %s)\n",
		filename,
		time.Unix(int64(timestamp), 0).Format("15:04:05"),
		content)

	return frame
}

// ExportIntelligentPPTResult导出智能PPT结果
func (a *App) ExportIntelligentPPTResult(result IntelligentPPTResult) string {
	if !result.Success || len(result.Frames) == 0 {
		return "没有可导出的数据"
	}

	// 创建JSON文件
	exportData := map[string]interface{}{
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"frames":    result.Frames,
		"total":     len(result.Frames),
		"directory": result.Dir,
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Sprintf("序列化失败: %v", err)
	}

	outputPath := filepath.Join(result.Dir, "intelligent_ppt_result.json")
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Sprintf("写入文件失败: %v", err)
	}

	return fmt.Sprintf("结果已导出到: %s", outputPath)
}
