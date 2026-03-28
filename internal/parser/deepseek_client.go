// Package parser 提供 AI 客户端实现
package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// AIClientFactory AI客户端工厂
type AIClientFactory struct {
	APIMode         string
	APIKey          string
	BaseURL         string
	Model           string
	MaxRetries      int
	RequestInterval int
}

// NewAIClient 根据API模式创建对应的AI客户端
func (f *AIClientFactory) NewAIClient() AIClient {
	switch f.APIMode {
	case "claude":
		return NewClaudeClient(f.APIKey, f.BaseURL, f.Model, f.MaxRetries, f.RequestInterval)
	case "openai":
		fallthrough
	default:
		return NewDeepSeekClient(f.APIKey, f.BaseURL, f.Model, f.MaxRetries, f.RequestInterval)
	}
}

// DeepSeekClient DeepSeek API 客户端 (OpenAI兼容模式)
type DeepSeekClient struct {
	APIKey          string
	BaseURL         string
	Model           string
	MaxRetries      int
	RequestInterval int
}

// NewDeepSeekClient 创建 DeepSeek 客户端
func NewDeepSeekClient(apiKey, baseURL, model string, maxRetries, interval int) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	// 自动拼接 OpenAI 兼容路径
	baseURL = ensureOpenAIPath(baseURL)

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

// ensureOpenAIPath 确保 baseURL 包含 OpenAI 兼容的完整路径
// 如果用户只输入了基础路径（如 https://api.deepseek.com），自动拼接 /v1/chat/completions
func ensureOpenAIPath(baseURL string) string {
	// 如果已经包含完整路径，直接返回
	if strings.Contains(baseURL, "/chat/completions") {
		return baseURL
	}

	// 解析 URL
	u, err := url.Parse(baseURL)
	if err != nil {
		// 解析失败，返回原值并附加默认路径
		return baseURL + "/v1/chat/completions"
	}

	// 检查是否已有 /v1 前缀
	if strings.HasPrefix(u.Path, "/v1/") || u.Path == "/v1" {
		u.Path = path.Join(u.Path, "chat/completions")
	} else {
		// 添加 /v1/chat/completions
		u.Path = path.Join(u.Path, "v1/chat/completions")
	}

	return u.String()
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

// ClaudeClient Anthropic Claude API 客户端
type ClaudeClient struct {
	APIKey          string
	BaseURL         string
	Model           string
	MaxRetries      int
	RequestInterval int
}

// NewClaudeClient 创建 Claude 客户端
func NewClaudeClient(apiKey, baseURL, model string, maxRetries, interval int) *ClaudeClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	// 自动拼接 Claude API 路径
	baseURL = ensureClaudePath(baseURL)

	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &ClaudeClient{
		APIKey:          apiKey,
		BaseURL:         baseURL,
		Model:           model,
		MaxRetries:      maxRetries,
		RequestInterval: interval,
	}
}

// ensureClaudePath 确保 baseURL 包含 Claude 的完整路径
// 如果用户只输入了基础路径（如 https://api.anthropic.com），自动拼接 /v1/messages
func ensureClaudePath(baseURL string) string {
	// 如果已经包含完整路径，直接返回
	if strings.Contains(baseURL, "/messages") {
		return baseURL
	}

	// 解析 URL
	u, err := url.Parse(baseURL)
	if err != nil {
		// 解析失败，返回原值并附加默认路径
		return baseURL + "/v1/messages"
	}

	// 检查是否已有 /v1 前缀
	if strings.HasPrefix(u.Path, "/v1/") || u.Path == "/v1" {
		u.Path = path.Join(u.Path, "messages")
	} else {
		// 添加 /v1/messages
		u.Path = path.Join(u.Path, "v1/messages")
	}

	return u.String()
}

// ClaudeMessage Claude消息
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeRequest Claude请求
type ClaudeRequest struct {
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	Messages    []ClaudeMessage   `json:"messages"`
	System      string            `json:"system,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
}

// ClaudeResponse Claude响应
type ClaudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Parse2DTable 调用 Claude API 解析二维表
func (c *ClaudeClient) Parse2DTable(ctx context.Context, htmlContent string, prompt string) (courseCSV, activityCSV string, err error) {
	// 构建请求消息 (Claude使用user/assistant角色，system通过system字段传递)
	messages := []ClaudeMessage{
		{
			Role:    "user",
			Content: fmt.Sprintf("请将以下二维课表转换为标准 CSV 格式:\n\n%s", htmlContent),
		},
	}

	reqBody := ClaudeRequest{
		Model:       c.Model,
		MaxTokens:   4096,
		Messages:    messages,
		System:      prompt,
		Temperature: 0.1,
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

// callAPI 调用 Claude API
func (c *ClaudeClient) callAPI(ctx context.Context, jsonData []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if claudeResp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("API 返回空结果")
	}

	return claudeResp.Content[0].Text, nil
}
