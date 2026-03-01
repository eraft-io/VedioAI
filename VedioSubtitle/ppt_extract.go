package main

import (
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

// PPTResult PPT提取结果
type PPTResult struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Frames  []PPTFrame `json:"frames"`
	Dir     string     `json:"dir"`
}

// ExtractPPTFrames 提取PPT关键帧
func (a *App) ExtractPPTFrames(videoPath string, threshold float64) PPTResult {
	result := PPTResult{
		Success: false,
		Message: "",
		Frames:  []PPTFrame{},
		Dir:     "",
	}

	if videoPath == "" {
		result.Message = "请选择视频文件"
		return result
	}

	// 创建输出目录
	videoDir := filepath.Dir(videoPath)
	pptDir := filepath.Join(videoDir, "ppt_frames")
	if err := os.MkdirAll(pptDir, 0755); err != nil {
		result.Message = fmt.Sprintf("创建目录失败: %v", err)
		return result
	}
	result.Dir = pptDir

	runtime.EventsEmit(a.ctx, "ppt:progress", PPTProgressInfo{
		Status:   "processing",
		Progress: 0,
		Message:  "正在分析视频场景变化...",
		Current:  0,
		Total:    0,
	})

	// 获取 conda路径
	condaPath := getCondaPath()
	if condaPath == "" {
		result.Message = "未找到 conda"
		return result
	}

	// 使用 ffmpeg 提取关键帧
	// ffmpeg -i input.mp4 -vf "select='gt(scene,0.3)',setpts=N/(25*TB)" -q:v 2 output_%04d.jpg
	cmd := exec.Command(
		condaPath, "run", "-n", "whisper",
		"ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("select='gt(scene,%f)',setpts=N/(25*TB)", threshold),
		"-q:v", "2",
		"-vsync", "vfr",
		filepath.Join(pptDir, "ppt_%04d.jpg"),
	)

	//执行命令
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		result.Message = fmt.Sprintf("提取失败: %v", err)
		runtime.EventsEmit(a.ctx, "ppt:progress", PPTProgressInfo{
			Status:   "error",
			Progress: 0,
			Message:  result.Message,
			Current:  0,
			Total:    0,
		})
		return result
	}

	// 获取提取的帧文件
	files, err := os.ReadDir(pptDir)
	if err != nil {
		result.Message = fmt.Sprintf("读取目录失败: %v", err)
		return result
	}

	//收帧信息
	var frames []PPTFrame
	frameIndex := 1

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") {
			framePath := filepath.Join(pptDir, file.Name())

			// 从文件名中提取时间戳信息（如果可能）
			timestamp := a.extractTimestampFromFile(file.Name(), videoPath)

			frames = append(frames, PPTFrame{
				Index:     frameIndex,
				Timestamp: timestamp,
				Filename:  file.Name(),
				Path:      framePath,
			})
			frameIndex++
		}
	}

	//按时间戳排序
	sort.Slice(frames, func(i, j int) bool {
		return frames[i].Timestamp < frames[j].Timestamp
	})

	// 更新帧索引
	for i := range frames {
		frames[i].Index = i + 1
	}

	result.Success = true
	result.Frames = frames
	result.Message = fmt.Sprintf("成功提取 %d 个PPT关键帧到: %s", len(frames), pptDir)

	runtime.EventsEmit(a.ctx, "ppt:progress", PPTProgressInfo{
		Status:   "completed",
		Progress: 100,
		Message:  result.Message,
		Current:  len(frames),
		Total:    len(frames),
	})

	return result
}

// extractTimestampFromFile 从文件名中提取时间戳
func (a *App) extractTimestampFromFile(filename, videoPath string) float64 {
	//尝从文件名中提取序号
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(filename, -1)

	if len(matches) > 0 {
		// 如果有数字，假设是帧序号，需要转换为时间戳
		// 这里简化处理，实际需要更复杂的逻辑
		return float64(len(matches)) * 0.1 //每帧间隔0.1秒
	}

	// 默认返回0
	return 0.0
}

// ExportPPTResult导出PPT提取结果
func (a *App) ExportPPTResult(result PPTResult) string {
	if !result.Success || len(result.Frames) == 0 {
		return "没有可导出的数据"
	}

	// 创建JSON文件
	exportData := map[string]interface{}{
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"frames":    result.Frames,
		"total":     len(result.Frames),
		"directory": result.Dir,
		"threshold": 0.3, // 默认阈值
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Sprintf("序列化失败: %v", err)
	}

	outputPath := filepath.Join(result.Dir, "ppt_result.json")
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Sprintf("写入文件失败: %v", err)
	}

	return fmt.Sprintf("结果已导出到: %s", outputPath)
}
