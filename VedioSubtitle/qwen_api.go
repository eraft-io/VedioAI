package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// QwenAPIConfig 阿里云百炼配置
type QwenAPIConfig struct {
	APIKey string `json:"api_key"`
}

// QwenRequest 请求体
type QwenRequest struct {
	Model       string          `json:"model"`
	Input       QwenInput       `json:"input"`
	Parameters  QwenParameters  `json:"parameters,omitempty"`
}

type QwenInput struct {
	Prompt string `json:"prompt"`
}

type QwenParameters struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type QwenResponse struct {
	Output struct {
		Text string `json:"text"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// GetQwenAPIKey 从配置文件读取 API Key
func GetQwenAPIKey() (string, error) {
	configPath := getQwenConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("未找到 API Key 配置")
	}

	var config QwenAPIConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("解析配置文件失败：%v", err)
	}

	if config.APIKey == "" {
		return "", fmt.Errorf("API Key 为空")
	}

	return config.APIKey, nil
}

// SaveQwenAPIKey 保存 API Key 到配置文件
func SaveQwenAPIKey(apiKey string) error {
	configPath := getQwenConfigPath()
	
	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败：%v", err)
	}

	config := QwenAPIConfig{
		APIKey: strings.TrimSpace(apiKey),
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("保存配置文件失败：%v", err)
	}

	return nil
}

// getQwenConfigPath 获取配置文件路径
func getQwenConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".video-subtitle-translator", "qwen_api_key.json")
}

// TranslateWithQwen 使用阿里云百炼千问模型翻译
func TranslateWithQwen(ctx context.Context, text string) (string, error) {
	apiKey, err := GetQwenAPIKey()
	if err != nil {
		return "", err
	}

	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"

	prompt := fmt.Sprintf(`请将以下英文文本翻译成中文，只返回翻译结果，不要任何解释：

%s

中文翻译：`, text)

	reqBody := QwenRequest{
		Model: "qwen-turbo",
		Input: QwenInput{
			Prompt: prompt,
		},
		Parameters: QwenParameters{
			Temperature: 0.1,
			MaxTokens:   512,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败：%v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 错误 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(body, &qwenResp); err != nil {
		return "", fmt.Errorf("解析响应失败：%v", err)
	}

	if qwenResp.Output.Text == "" {
		return "", fmt.Errorf("API 返回空结果")
	}

	return strings.TrimSpace(qwenResp.Output.Text), nil
}
