// Package parser 提供 AI 客户端实现
package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeepSeekClient DeepSeek API 客户端
type DeepSeekClient struct {
	APIKey   string
	BaseURL  string
	Model    string
	MaxRetries int
	RequestInterval int
}

// NewDeepSeekClient 创建 DeepSeek 客户端
func NewDeepSeekClient(apiKey, baseURL, model string, maxRetries, interval int) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/chat/completions"
	}
	if model == "" {
		model = "deepseek-chat"
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &DeepSeekClient{
		APIKey:          apiKey,
		BaseURL:         baseURL,
		Model:           model,
		MaxRetries:      maxRetries,
		RequestInterval: interval,
	}
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Parse2DTable 调用 AI 解析二维表
func (c *DeepSeekClient) Parse2DTable(ctx context.Context, htmlContent string, prompt string) (courseCSV, activityCSV string, err error) {
	// 构建请求消息
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: string(prompt),
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("请将以下二维课表转换为标准 CSV 格式:\n\n%s", htmlContent),
		},
	}

	reqBody := ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0.1, // 低温度以获得更确定的结果
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 重试逻辑
	var lastErr error
	for attempt := 0; attempt < c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		resp, err := c.callAPI(ctx, jsonData)
		if err != nil {
			lastErr = err
			continue
		}

		// 解析响应
		courseCSV, activityCSV = ParseCSVFromResponse(resp)
		if courseCSV == "" {
			lastErr = fmt.Errorf("AI 响应中未找到课程 CSV")
			continue
		}

		return courseCSV, activityCSV, nil
	}

	return "", "", fmt.Errorf("API 调用失败 (重试 %d 次): %w", c.MaxRetries, lastErr)
}

// callAPI 调用 API
func (c *DeepSeekClient) callAPI(ctx context.Context, jsonData []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("API 返回空结果")
	}

	return chatResp.Choices[0].Message.Content, nil
}
