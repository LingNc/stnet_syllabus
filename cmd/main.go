// 学生网管课程表解析和预排班系统
// 适配教务系统导出的课表文件（.xls，实质为 HTML 格式）
// 转化为标准化的 CSV 和 Excel 文件用于排班

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stnet_syllabus/internal/aggregate"
	"stnet_syllabus/internal/config"
	"stnet_syllabus/internal/excel"
	"stnet_syllabus/internal/parser"
	"stnet_syllabus/internal/preprocess"
	"stnet_syllabus/internal/simplify"
	"stnet_syllabus/internal/split"
	"stnet_syllabus/internal/validate"
	"stnet_syllabus/internal/weekly"
)

// errorLog 全局错误日志文件
var errorLog *os.File

// initErrorLog 初始化错误日志
func initErrorLog(cfg *config.Config) error {
	errorLogPath := filepath.Join(cfg.Paths.Output, "error.log")
	f, err := os.OpenFile(errorLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	errorLog = f
	logError("=== 错误日志开始 ===")
	logError("时间: %s", time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

// closeErrorLog 关闭错误日志
func closeErrorLog() {
	if errorLog != nil {
		logError("=== 错误日志结束 ===")
		errorLog.Close()
	}
}

// logError 记录错误到日志
func logError(format string, args ...interface{}) {
	if errorLog != nil {
		timestamp := time.Now().Format("15:04:05")
		fmt.Fprintf(errorLog, "[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	}
}

func main() {
	// 解析命令行参数
	var (
		configFile = flag.String("config", "config/config.yaml", "配置文件路径")
		step       = flag.String("step", "all", "执行步骤: all|preprocess|simplify|validate|split|parse|aggregate|weekly|excel")
		skipAI     = flag.Bool("skip-ai", false, "跳过 AI 解析（仅处理列表格式）")
	)
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 确保目录存在
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化错误日志
	if err := initErrorLog(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "初始化错误日志失败: %v\n", err)
	}
	defer closeErrorLog()

	// 根据步骤执行
	switch *step {
	case "all":
		runAll(cfg, *skipAI)
	case "preprocess":
		runPreprocess(cfg)
	case "simplify":
		runSimplify(cfg)
	case "validate":
		runValidate(cfg)
	case "split":
		runSplit(cfg)
	case "parse":
		runParse(cfg, *skipAI)
	case "aggregate":
		runAggregate(cfg)
	case "weekly":
		runWeekly(cfg)
	case "excel":
		runExcel(cfg)
	default:
		fmt.Fprintf(os.Stderr, "未知步骤: %s\n", *step)
		fmt.Fprintf(os.Stderr, "可用步骤: all, preprocess, simplify, validate, split, parse, aggregate, weekly, excel\n")
		os.Exit(1)
	}
}

// runAll 执行完整流程
func runAll(cfg *config.Config, skipAI bool) {
	fmt.Println("=== 开始完整处理流程 ===\n")

	steps := []struct {
		name string
		fn   func()
	}{
		{"Step 1: 数据预处理", func() { runPreprocess(cfg) }},
		{"Step 2: HTML 精简", func() { runSimplify(cfg) }},
		{"Step 3: 数据验证", func() { runValidate(cfg) }},
		{"Step 4: 数据拆分", func() { runSplit(cfg) }},
		{"Step 5: 课表解析", func() { runParse(cfg, skipAI) }},
		{"Step 6: 空闲时间聚合", func() { runAggregate(cfg) }},
		{"Step 7: 周次切片", func() { runWeekly(cfg) }},
		{"Step 8: Excel 生成", func() { runExcel(cfg) }},
	}

	for i, s := range steps {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(steps), s.name)
		fmt.Println(strings.Repeat("-", 40))
		s.fn()
	}

	fmt.Println("\n=== 所有步骤完成 ===")
}

// runPreprocess 执行数据预处理
func runPreprocess(cfg *config.Config) {
	// 查找映射表文件
	mappingFile := ""
	entries, err := os.ReadDir(cfg.Paths.Input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取输入目录失败: %v\n", err)
		logError("读取输入目录失败: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") {
			mappingFile = filepath.Join(cfg.Paths.Input, entry.Name())
			break
		}
	}

	if mappingFile == "" {
		fmt.Println("警告: 未找到映射表文件，跳过预处理")
		logError("未找到映射表文件，跳过预处理")
		return
	}

	processor := preprocess.NewProcessor(
		cfg.Paths.Input,
		cfg.Paths.TempRaw,
		mappingFile,
	)

	if err := processor.Process(); err != nil {
		fmt.Fprintf(os.Stderr, "预处理失败: %v\n", err)
		logError("预处理失败: %v", err)
	}
}

// runSimplify 执行 HTML 精简
func runSimplify(cfg *config.Config) {
	simplifier := simplify.NewSimplifier(
		cfg.Paths.TempRaw,
		cfg.Paths.TempSimplified,
	)

	if err := simplifier.Process(); err != nil {
		fmt.Fprintf(os.Stderr, "HTML 精简失败: %v\n", err)
		logError("HTML 精简失败: %v", err)
	}
}

// runValidate 执行数据验证
func runValidate(cfg *config.Config) {
	validator := validate.NewValidator(
		cfg.Paths.TempSimplified,
		cfg.Paths.ErrorLog,
		cfg.Semester.Code, // 传入配置的学期代码
	)

	results, err := validator.Process()
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据验证失败: %v\n", err)
		logError("数据验证失败: %v", err)
		return
	}

	// 记录验证错误并删除无效文件
	invalidCount := 0
	for _, r := range results {
		if r.Error != "" {
			logError("验证失败 [%s]: %s", r.FilePath, r.Error)
		}
		// 删除验证失败的文件，防止进入后续步骤
		if !r.Valid {
			if err := os.Remove(r.FilePath); err != nil {
				fmt.Fprintf(os.Stderr, "警告: 删除无效文件失败 %s: %v\n", r.FilePath, err)
				logError("删除无效文件失败 [%s]: %v", r.FilePath, err)
			} else {
				fmt.Printf("  已删除无效文件: %s\n", filepath.Base(r.FilePath))
				invalidCount++
			}
		}
	}

	if invalidCount > 0 {
		fmt.Printf("\n已清理 %d 个无效文件，这些文件将不会进入后续处理步骤\n", invalidCount)
	}
}

// runSplit 执行数据拆分
func runSplit(cfg *config.Config) {
	splitter := split.NewSplitter(
		cfg.Paths.TempSimplified,
		cfg.Paths.TempSplit2D,
		cfg.Paths.TempSplitList,
		cfg.Semester.Code, // 传入配置的学期代码
	)

	results, err := splitter.Process()
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据拆分失败: %v\n", err)
		logError("数据拆分失败: %v", err)
		return
	}

	// 记录拆分错误
	for _, r := range results {
		if r.Error != "" {
			logError("拆分失败 [%s]: %s", r.FilePath, r.Error)
		}
	}
}

// runParse 执行课表解析
func runParse(cfg *config.Config, skipAI bool) {
	// 解析列表格式
	listParser := parser.NewListParser(
		cfg.Paths.TempSplitList,
		cfg.Paths.CSVNormalized,
	)

	results, err := listParser.Process()
	if err != nil {
		fmt.Fprintf(os.Stderr, "列表格式解析失败: %v\n", err)
		logError("列表格式解析失败: %v", err)
	} else {
		// 记录解析错误
		for _, r := range results {
			if r.Error != "" {
				logError("列表解析失败 [%s]: %s", r.InputFile, r.Error)
			}
		}
	}

	// 解析二维表（AI）
	if !skipAI {
		apiKey, err := config.GetAPIKey(filepath.Dir(cfg.Paths.Input) + "/config")
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 API 密钥失败: %v\n", err)
			logError("读取 API 密钥失败: %v", err)
			fmt.Println("跳过 AI 解析，仅使用列表格式结果")
			return
		}

		client := parser.NewDeepSeekClient(
			apiKey,
			cfg.AI.BaseURL,
			cfg.AI.Model,
			cfg.AI.MaxRetries,
			cfg.AI.RequestInterval,
		)

		aiParser := parser.NewAI2DParser(
			cfg.Paths.TempSplit2D,
			cfg.Paths.CSVNormalized,
			filepath.Join(filepath.Dir(cfg.Paths.Input), "config", "二维表.prompt"),
			client,
			cfg.AI.Concurrency,
		)

		aiResults, err := aiParser.Process()
		if err != nil {
			fmt.Fprintf(os.Stderr, "AI 解析失败: %v\n", err)
			logError("AI 解析失败: %v", err)
		} else {
			// 记录 AI 解析错误
			for _, r := range aiResults {
				if r.Error != "" {
					logError("AI 解析失败 [%s]: %s", r.InputFile, r.Error)
				}
			}
		}
	}
}

// runAggregate 执行空闲时间聚合
func runAggregate(cfg *config.Config) {
	aggregator := aggregate.NewAggregator(
		cfg.Paths.CSVNormalized,
		cfg.Paths.Output,
		cfg.Semester.TotalWeeks,
		cfg.Semester.ExamReviewWeeks,
	)

	if err := aggregator.Process(); err != nil {
		fmt.Fprintf(os.Stderr, "空闲时间聚合失败: %v\n", err)
		logError("空闲时间聚合失败: %v", err)
	}
}

// runWeekly 执行周次切片
func runWeekly(cfg *config.Config) {
	machineCSV := filepath.Join(cfg.Paths.Output, "free_time_machine.csv")

	weeklyDir := filepath.Join(cfg.Paths.Output, "weekly")
	if err := os.MkdirAll(weeklyDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建周切片目录失败: %v\n", err)
		logError("创建周切片目录失败: %v", err)
		return
	}

	slicer := weekly.NewSlicer(machineCSV, weeklyDir, cfg.Semester.TotalWeeks)

	results, err := slicer.Process()
	if err != nil {
		fmt.Fprintf(os.Stderr, "周次切片失败: %v\n", err)
		logError("周次切片失败: %v", err)
		return
	}

	// 记录切片错误
	for _, r := range results {
		if r.Error != "" {
			logError("周次切片失败 [第%d周]: %s", r.Week, r.Error)
		}
	}
}

// runExcel 执行 Excel 生成
func runExcel(cfg *config.Config) {
	weeklyDir := filepath.Join(cfg.Paths.Output, "weekly")

	generator := excel.NewGenerator(
		cfg.Paths.Output,
		cfg.Paths.Final,
		cfg.Semester.TotalWeeks,
	)

	// 生成主报表（包含汇总和每周）
	if err := generator.Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "生成主 Excel 报表失败: %v\n", err)
		logError("生成主 Excel 报表失败: %v", err)
		return
	}

	// 单独转换每周 CSV 为 Excel
	entries, err := os.ReadDir(weeklyDir)
	if err != nil {
		logError("读取周切片目录失败: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csv") {
			csvFile := filepath.Join(weeklyDir, entry.Name())
			if err := generator.ConvertCSVToExcel(csvFile); err != nil {
				logError("转换 CSV 为 Excel 失败 [%s]: %v", entry.Name(), err)
			}
		}
	}
}
