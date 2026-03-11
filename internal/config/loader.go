// Package config 负责加载和管理配置
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CLIOverride 存储命令行参数覆盖值
type CLIOverride struct {
	InputPath       string
	OutputPath      string
	AIKey           string
	PromptFilePath  string
	APIKeyFilePath  string
	SemesterStart   string
}

// GlobalOverride 全局 CLI 覆盖值
var GlobalOverride *CLIOverride

// Config 全局配置
type Config struct {
	Semester   SemesterConfig   `yaml:"semester"`
	TimeSlots  []TimeSlotConfig `yaml:"time_slots"`
	AI         AIConfig         `yaml:"ai"`
	Paths      PathsConfig      `yaml:"paths"`
	Parser     ParserConfig     `yaml:"parser"`
	Excel      ExcelConfig      `yaml:"excel"`
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

// ExcelConfig Excel样式配置
type ExcelConfig struct {
	Header       ExcelHeaderConfig       `yaml:"header"`
	Data         ExcelDataConfig         `yaml:"data"`
	Column       ExcelColumnConfig       `yaml:"column"`
	Table        ExcelTableConfig        `yaml:"table"`
	FirstColumn  ExcelFirstColumnConfig  `yaml:"first_column"`
}

// ExcelHeaderConfig 表头样式配置
type ExcelHeaderConfig struct {
	FontSize     int    `yaml:"font_size"`
	Bold         bool   `yaml:"bold"`
	BgColor      string `yaml:"bg_color"`
	FontColor    string `yaml:"font_color"`
	BorderBottom bool   `yaml:"border_bottom"`
	RowHeight    float64 `yaml:"row_height"`
}

// ExcelDataConfig 数据行样式配置
type ExcelDataConfig struct {
	RowHeight float64 `yaml:"row_height"`
	WrapText  bool    `yaml:"wrap_text"`
}

// ExcelColumnConfig 列宽配置
type ExcelColumnConfig struct {
	MinWidth          float64 `yaml:"min_width"`
	MaxWidth          float64 `yaml:"max_width"`
	CharWidthFactor   float64 `yaml:"char_width_factor"`
}

// ExcelTableConfig 表格内容配置
type ExcelTableConfig struct {
	MaxPeriods int `yaml:"max_periods"`
}

// ExcelFirstColumnConfig 第一列（节次列）专用配置
type ExcelFirstColumnConfig struct {
	FontSize   int     `yaml:"font_size"`
	Bold       bool    `yaml:"bold"`
	BgColor    string  `yaml:"bg_color"`
	FontColor  string  `yaml:"font_color"`
	Align      string  `yaml:"align"`
	Width      float64 `yaml:"width"`
}

// GlobalConfig 全局配置实例
var GlobalConfig *Config

// Load 加载配置文件并应用 CLI 覆盖
func Load(configPath string, override ...*CLIOverride) (*Config, error) {
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

	// 应用 CLI 覆盖（CLI 参数优先级高于配置文件）
	if len(override) > 0 && override[0] != nil {
		ov := override[0]

		// 覆盖输入路径
		if ov.InputPath != "" {
			cfg.Paths.Input = ov.InputPath
		}

		// 覆盖输出路径
		if ov.OutputPath != "" {
			cfg.Paths.Output = ov.OutputPath
			// 同步更新派生路径
			cfg.Paths.TempRaw = filepath.Join(ov.OutputPath, "temp", "raw_xls")
			cfg.Paths.TempSimplified = filepath.Join(ov.OutputPath, "temp", "simplified_xls")
			cfg.Paths.TempSplit2D = filepath.Join(ov.OutputPath, "temp", "split", "2d_table")
			cfg.Paths.TempSplitList = filepath.Join(ov.OutputPath, "temp", "split", "list")
			cfg.Paths.CSVNormalized = filepath.Join(ov.OutputPath, "csv_normalized")
			cfg.Paths.Final = filepath.Join(ov.OutputPath, "final")
			cfg.Paths.ErrorLog = filepath.Join(ov.OutputPath, "error.log")
		}

		// 覆盖学期开始日期
		if ov.SemesterStart != "" {
			cfg.Semester.StartDate = ov.SemesterStart
		}
	}

	GlobalConfig = &cfg
	return &cfg, nil
}

// GetAPIKey 读取 API 密钥
// 优先使用 CLI 传入的密钥，其次从文件读取
func GetAPIKey(configDir string, cliKey ...string) (string, error) {
	// 优先使用 CLI 传入的密钥
	if len(cliKey) > 0 && cliKey[0] != "" {
		return cliKey[0], nil
	}

	// 检查是否有 CLI 指定的密钥文件路径
	if GlobalOverride != nil && GlobalOverride.APIKeyFilePath != "" {
		data, err := os.ReadFile(GlobalOverride.APIKeyFilePath)
		if err != nil {
			return "", fmt.Errorf("读取 API 密钥文件失败: %w", err)
		}
		return extractKeyFromData(string(data)), nil
	}

	keyPath := filepath.Join(configDir, "api.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("读取 API 密钥失败: %w", err)
	}

	return extractKeyFromData(string(data)), nil
}

// extractKeyFromData 从文件内容中提取密钥
func extractKeyFromData(data string) string {
	// 逐行读取，找到非注释的非空行
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 返回第一个非注释行
		return line
	}
	return ""
}

// GetPromptFilePath 获取 Prompt 文件路径
// 优先使用 CLI 指定的路径，其次使用默认路径
func GetPromptFilePath(configDir string) string {
	if GlobalOverride != nil && GlobalOverride.PromptFilePath != "" {
		return GlobalOverride.PromptFilePath
	}
	return filepath.Join(configDir, "二维表.prompt")
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
