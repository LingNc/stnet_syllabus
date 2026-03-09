// Package config 负责加载和管理配置
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 全局配置
type Config struct {
	Semester   SemesterConfig   `yaml:"semester"`
	TimeSlots  []TimeSlotConfig `yaml:"time_slots"`
	AI         AIConfig         `yaml:"ai"`
	Paths      PathsConfig      `yaml:"paths"`
	Parser     ParserConfig     `yaml:"parser"`
}

// SemesterConfig 学期配置
type SemesterConfig struct {
	Code            string `yaml:"code"`
	StartDate       string `yaml:"start_date"`
	TotalWeeks      int    `yaml:"total_weeks"`
	ExamReviewWeeks []int  `yaml:"exam_review_weeks"`
}

// TimeSlotConfig 时间段配置
type TimeSlotConfig struct {
	Period    string `yaml:"period"`
	Start     string `yaml:"start"`
	End       string `yaml:"end"`
}

// AIConfig AI 接口配置
type AIConfig struct {
	BaseURL          string `yaml:"base_url"`
	Model            string `yaml:"model"`
	Concurrency      int    `yaml:"concurrency"`
	MaxRetries       int    `yaml:"max_retries"`
	RequestInterval  int    `yaml:"request_interval"`
}

// PathsConfig 路径配置
type PathsConfig struct {
	Input           string `yaml:"input"`
	Output          string `yaml:"output"`
	TempRaw         string `yaml:"temp_raw"`
	TempSimplified  string `yaml:"temp_simplified"`
	TempSplit2D     string `yaml:"temp_split_2d"`
	TempSplitList   string `yaml:"temp_split_list"`
	CSVNormalized   string `yaml:"csv_normalized"`
	Final           string `yaml:"final"`
	ErrorLog        string `yaml:"error_log"`
}

// ParserConfig 解析配置
type ParserConfig struct {
	Type1FullOccupy bool   `yaml:"type1_full_occupy"`
	CSVEncoding     string `yaml:"csv_encoding"`
}

// GlobalConfig 全局配置实例
var GlobalConfig *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.AI.Concurrency == 0 {
		cfg.AI.Concurrency = 5
	}
	if cfg.AI.MaxRetries == 0 {
		cfg.AI.MaxRetries = 3
	}
	if cfg.AI.RequestInterval == 0 {
		cfg.AI.RequestInterval = 500
	}

	GlobalConfig = &cfg
	return &cfg, nil
}

// GetAPIKey 读取 API 密钥
func GetAPIKey(configDir string) (string, error) {
	keyPath := filepath.Join(configDir, "api.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("读取 API 密钥失败: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// EnsureDirs 确保所有必要的目录存在
func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.Paths.TempRaw,
		c.Paths.TempSimplified,
		c.Paths.TempSplit2D,
		c.Paths.TempSplitList,
		c.Paths.CSVNormalized,
		c.Paths.Final,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	return nil
}
