// 学生网管课程表解析和预排班系统
// 适配教务系统导出的课表文件（.xls，实质为 HTML 格式）
// 转化为标准化的 CSV 和 Excel 文件用于排班

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stnet_syllabus/internal/aggregate"
	"stnet_syllabus/internal/config"
	"stnet_syllabus/internal/excel"
	"stnet_syllabus/internal/ics"
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
		// 基本参数
		configFile = flag.String("config", "", "配置文件路径（可选，默认使用嵌入配置或当前目录config/）")
		step       = flag.String("step", "all", "执行步骤: all|preprocess|simplify|validate|split|parse|aggregate|weekly|excel|ics")
		skipAI     = flag.Bool("skip-ai", false, "跳过 AI 解析（仅处理列表格式）")

		// 路径覆盖参数
		inputPath  = flag.String("input", "", "输入目录路径（覆盖配置文件）")
		outputPath = flag.String("output", "", "输出目录路径（覆盖配置文件）")

		// AI 相关参数
		aiKey          = flag.String("aikey", "", "AI API 密钥（覆盖配置文件和 api.key 文件）")
		promptFilePath = flag.String("prompt", "", "AI Prompt 文件路径（覆盖默认路径）")
		apiKeyFilePath = flag.String("apikey-file", "", "API 密钥文件路径（覆盖默认路径）")

		// 学期相关参数
		semesterStart = flag.String("semester-start", "", "学期开始日期（格式: YYYY-MM-DD，覆盖配置文件）")

		// ICS 导出参数
		icsEnabled    = flag.Bool("ics", false, "启用 ICS 日历批量导出（在正常流程后生成所有ics）")
		icsOutputDir  = flag.String("ics-dir", "", "ICS 输出目录（覆盖配置文件中的路径）")
		icsInputFile  = flag.String("ics-input", "", "输入的 .xls 课表文件路径（个人模式：直接从xls生成ics）")
		icsOutputFile = flag.String("ics-output", "", "输出 ICS 文件路径（个人模式使用）")

		// 初始化参数
		initFlag    = flag.Bool("init", false, "初始化配置目录（在当前目录创建 config/ 并释放默认配置）")
		initForce   = flag.Bool("init-force", false, "强制覆盖已存在的配置文件")
	)
	flag.Parse()

	// 处理 -init 参数
	if *initFlag {
		if err := InitConfig(".", *initForce); err != nil {
			fmt.Fprintf(os.Stderr, "初始化配置失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// 如果没有指定配置文件，检查默认路径
	if *configFile == "" {
		defaultConfig := "config/config.yaml"
		if _, err := os.Stat(defaultConfig); err == nil {
			*configFile = defaultConfig
		} else {
			fmt.Fprintf(os.Stderr, "错误: 未找到默认配置文件 %s\n", defaultConfig)
			fmt.Fprintf(os.Stderr, "请使用 -config 指定配置文件路径，或使用 -init 初始化默认配置\n")
			fmt.Fprintf(os.Stderr, "\n示例:\n")
			fmt.Fprintf(os.Stderr, "  %s -init                    # 初始化配置\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  %s -config /path/to/config.yaml  # 指定配置文件\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  %s -input ./data -output ./out   # 使用命令行参数运行（无需配置文件）\n", os.Args[0])
			os.Exit(1)
		}
	}

	// 设置 CLI 覆盖值
	config.GlobalOverride = &config.CLIOverride{
		InputPath:       *inputPath,
		OutputPath:      *outputPath,
		AIKey:           *aiKey,
		PromptFilePath:  *promptFilePath,
		APIKeyFilePath:  *apiKeyFilePath,
		SemesterStart:   *semesterStart,
	}

	// 加载配置
	cfg, err := config.Load(*configFile, config.GlobalOverride)
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

	// 处理 ICS 单文件导出模式（直接从 .xls 到 .ics 的个人模式）
	if *icsInputFile != "" {
		runICSSingleFile(cfg, *icsInputFile, *icsOutputFile, *configFile, *skipAI)
		return
	}

	// 根据步骤执行
	switch *step {
	case "all":
		runAll(cfg, *skipAI)
		// 如果指定了 -ics 参数，在批量流程后生成 ICS
		if *icsEnabled {
			fmt.Println("\n" + strings.Repeat("=", 40))
			runICSExport(cfg, *icsOutputDir)
		}
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
		logError, // 传递日志函数
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
		logError,          // 传递日志函数
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
		// 删除验证失败的文件，防止进入后续步骤
		if !r.Valid {
			if r.Error != "" {
				logError("验证失败 [%s]: %s", r.FilePath, r.Error)
			}
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
		apiKey, err := config.GetAPIKey(filepath.Dir(cfg.Paths.Input)+"/config", config.GlobalOverride.AIKey)
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

		// 使用新的方法获取 Prompt 文件路径
		promptFilePath := config.GetPromptFilePath(filepath.Dir(cfg.Paths.Input) + "/config")

		aiParser := parser.NewAI2DParser(
			cfg.Paths.TempSplit2D,
			cfg.Paths.CSVNormalized,
			promptFilePath,
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
		cfg.Excel,
	)
	aggregator.SetLogFunc(logError)

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
		cfg.Excel,
	)

	// 生成汇总表（只包含汇总，放到final目录）
	if err := generator.GenerateSummaryOnly(); err != nil {
		fmt.Fprintf(os.Stderr, "生成汇总表失败: %v\n", err)
		logError("生成汇总表失败: %v", err)
		return
	}

	// 生成总表（包含汇总+所有周表），放到output根目录
	fullScheduleFile := generateFullScheduleFileName(cfg.Semester.Code)
	fullSchedulePath := filepath.Join(cfg.Paths.Output, fullScheduleFile)
	if err := generator.GenerateFullSchedule(fullSchedulePath); err != nil {
		fmt.Fprintf(os.Stderr, "生成总表失败: %v\n", err)
		logError("生成总表失败: %v", err)
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

// generateFullScheduleFileName 根据学期代码生成总表文件名
// 学期代码格式: 20251 (2025-2026学年第二学期)
// 文件名格式: 学生网管2025-2026学年第二学期无课表.xlsx
func generateFullScheduleFileName(semesterCode string) string {
	if len(semesterCode) < 5 {
		return "学生网管无课表.xlsx"
	}

	// 提取年份和学期
	year := semesterCode[:4]
	semesterNum := semesterCode[4:]

	// 计算下一学年
	yearInt, _ := strconv.Atoi(year)
	nextYear := yearInt + 1

	// 学期名称
	semesterName := "第一学期"
	if semesterNum == "1" {
		semesterName = "第二学期"
	}

	return fmt.Sprintf("学生网管%d-%d学年%s无课表.xlsx", yearInt, nextYear, semesterName)
}

// runICSExport 批量生成 ICS（从 csv_normalized 目录）
func runICSExport(cfg *config.Config, icsFilePath string) {
	fmt.Println("=== ICS 日历批量导出 ===\n")

	// 解析学期开始日期
	startDate, err := time.Parse("2006-01-02", cfg.Semester.StartDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析学期开始日期失败: %v\n", err)
		logError("解析学期开始日期失败: %v", err)
		return
	}

	// 验证开始日期是周一
	if startDate.Weekday() != time.Monday {
		fmt.Fprintf(os.Stderr, "错误: 学期开始日期 %s 不是周一\n", cfg.Semester.StartDate)
		logError("学期开始日期不是周一: %s", cfg.Semester.StartDate)
		return
	}

	// 构建时间段映射
	periodTimes := buildPeriodTimes(cfg.TimeSlots)

	// 查找输入的 CSV 文件
	csvFiles := findCSVCourseFiles(cfg.Paths.CSVNormalized)
	if len(csvFiles) == 0 {
		fmt.Fprintf(os.Stderr, "错误: 在 %s 中未找到课程 CSV 文件\n", cfg.Paths.CSVNormalized)
		logError("未找到课程 CSV 文件: %s", cfg.Paths.CSVNormalized)
		return
	}

	// 如果没有指定输出目录，使用默认
	outputDir := icsFilePath
	if outputDir == "" || outputDir == "true" {
		if cfg.Paths.ICS != "" {
			outputDir = cfg.Paths.ICS
		} else {
			outputDir = filepath.Join(cfg.Paths.Output, "ics")
		}
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建输出目录失败: %v\n", err)
		logError("创建输出目录失败: %v", err)
		return
	}

	// 为每个 CSV 生成 ICS
	for _, csvFile := range csvFiles {
		generator := ics.NewGenerator(startDate, periodTimes, 15)
		if err := generator.AddFromCSV(csvFile); err != nil {
			fmt.Fprintf(os.Stderr, "  警告: 处理失败 %s: %v\n", filepath.Base(csvFile), err)
			logError("ICS 处理 CSV 失败 [%s]: %v", csvFile, err)
			continue
		}

		// 生成文件名
		baseName := strings.TrimSuffix(filepath.Base(csvFile), ".csv")
		parts := strings.Split(baseName, "_")
		var icsFileName string
		if len(parts) >= 3 {
			icsFileName = fmt.Sprintf("%s_%s_%s_ics.ics", parts[0], parts[1], parts[2])
		} else {
			icsFileName = baseName + "_ics.ics"
		}

		icsPath := filepath.Join(outputDir, icsFileName)
		if err := generator.Save(icsPath); err != nil {
			fmt.Fprintf(os.Stderr, "  警告: 保存失败 %s: %v\n", icsFileName, err)
			logError("ICS 保存失败 [%s]: %v", icsPath, err)
			continue
		}

		fmt.Printf("✓ 已生成: %s (%d 个事件)\n", icsFileName, len(generator.Events))
	}

	fmt.Printf("\n✓ ICS 批量导出完成，输出目录: %s\n", outputDir)
}
// 流程: .xls -> 简化 -> 拆分 -> 解析(CSV) -> ICS
func runICSSingleFile(cfg *config.Config, inputFile, outputFile, configFilePath string, skipAI bool) {
	fmt.Println("=== ICS 日历导出模式（个人版）===\n")
	fmt.Printf("输入文件: %s\n", filepath.Base(inputFile))

	// 检查输入文件
	if _, err := os.Stat(inputFile); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 输入文件不存在: %s\n", inputFile)
		os.Exit(1)
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "stnet_ics_*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建临时目录失败: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	// 步骤1: 简化 HTML
	fmt.Println("[1/4] 简化 HTML...")
	baseName := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
	simplifiedFile := filepath.Join(tempDir, baseName+".html")
	simplifier := simplify.NewSimplifier("", "", logError)
	if err := simplifier.SimplifyFile(inputFile, simplifiedFile); err != nil {
		fmt.Fprintf(os.Stderr, "HTML 简化失败: %v\n", err)
		os.Exit(1)
	}

	// 步骤2: 拆分课程/环节
	fmt.Println("[2/4] 拆分课程和环节...")
	splitter := split.NewSplitter(tempDir, tempDir, tempDir, cfg.Semester.Code)
	result := splitter.SplitFile(simplifiedFile)
	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "数据拆分失败: %s\n", result.Error)
		os.Exit(1)
	}

	// 步骤3: 解析为 CSV
	fmt.Println("[3/4] 解析课表...")
	var courseCSVFile string

	if result.Format == "list" {
		// 列表格式直接解析
		fmt.Println("      检测到列表格式，直接解析...")
		parser := parser.NewListParser(tempDir, tempDir)
		parseResult := parser.ParseFile(result.CourseFile)
		if parseResult.Error != "" {
			fmt.Fprintf(os.Stderr, "列表解析失败: %s\n", parseResult.Error)
			os.Exit(1)
		}
		courseCSVFile = parseResult.CourseCSV
	} else {
		// 二维表需要 AI 解析
		if skipAI {
			fmt.Println("      检测到二维表格式，但已跳过 AI 解析")
			fmt.Println("      提示: 去掉 -skip-ai 参数以使用 AI 解析二维表")
			os.Exit(1)
		}

		fmt.Println("      检测到二维表格式，使用 AI 解析...")

		// 获取 API 密钥
		apiKey, err := config.GetAPIKey(filepath.Dir(configFilePath), config.GlobalOverride.AIKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 API 密钥失败: %v\n", err)
			fmt.Println("      提示: 使用 -aikey 参数指定密钥，或创建 config/api.key 文件")
			os.Exit(1)
		}

		client := parser.NewDeepSeekClient(
			apiKey,
			cfg.AI.BaseURL,
			cfg.AI.Model,
			cfg.AI.MaxRetries,
			cfg.AI.RequestInterval,
		)

		promptFilePath := config.GetPromptFilePath(filepath.Dir(configFilePath))
		prompt, err := os.ReadFile(promptFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 prompt 文件失败: %v\n", err)
			os.Exit(1)
		}

		aiParser := parser.NewAI2DParser(tempDir, tempDir, promptFilePath, client, 1)
		aiResult := aiParser.ParseFile(result.CourseFile, string(prompt))
		if !aiResult.Success {
			fmt.Fprintf(os.Stderr, "AI 解析失败: %s\n", aiResult.Error)
			os.Exit(1)
		}
		courseCSVFile = aiResult.CourseCSV
	}

	// 步骤4: 生成 ICS
	fmt.Println("[4/4] 生成 ICS 日历...")

	// 解析学期开始日期
	startDate, err := time.Parse("2006-01-02", cfg.Semester.StartDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析学期开始日期失败: %v\n", err)
		os.Exit(1)
	}

	// 验证开始日期是周一
	if startDate.Weekday() != time.Monday {
		fmt.Fprintf(os.Stderr, "错误: 学期开始日期 %s 不是周一\n", cfg.Semester.StartDate)
		os.Exit(1)
	}

	// 构建时间段映射
	periodTimes := buildPeriodTimes(cfg.TimeSlots)

	// 如果没有指定输出路径，自动生成
	if outputFile == "" {
		// 从输入文件名推断
		baseName := strings.TrimSuffix(filepath.Base(inputFile), ".xls")
		parts := strings.Split(baseName, "_")
		if len(parts) >= 3 {
			outputFile = fmt.Sprintf("%s_%s_%s_ics.ics", parts[0], parts[1], parts[2])
		} else {
			outputFile = baseName + "_ics.ics"
		}
	}

	// 生成 ICS
	generator := ics.NewGenerator(startDate, periodTimes, 15)
	if err := generator.AddFromCSV(courseCSVFile); err != nil {
		fmt.Fprintf(os.Stderr, "添加课程到 ICS 失败: %v\n", err)
		os.Exit(1)
	}

	if err := generator.Save(outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "保存 ICS 文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ ICS 日历已生成: %s\n", outputFile)
	fmt.Printf("  共导出 %d 个课程事件\n", len(generator.Events))
}

// buildPeriodTimes 构建时间段映射
func buildPeriodTimes(timeSlots []config.TimeSlotConfig) map[int]map[string]string {
	periodTimes := make(map[int]map[string]string)

	for _, slot := range timeSlots {
		// 解析节次（如 "1-2" -> 1, 3-4 -> 2 等）
		parts := strings.Split(slot.Period, "-")
		if len(parts) == 2 {
			startPeriod, _ := strconv.Atoi(parts[0])
			periodIdx := (startPeriod + 1) / 2 // 1-2->1, 3-4->2, 等等

			periodTimes[periodIdx] = map[string]string{
				"start": slot.Start,
				"end":   slot.End,
			}
		}
	}

	return periodTimes
}

// findCSVCourseFiles 查找所有课程 CSV 文件
func findCSVCourseFiles(dir string) []string {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".csv") &&
			!strings.Contains(name, "activity") &&
			!strings.Contains(name, "free_time") {
			files = append(files, filepath.Join(dir, name))
		}
	}

	return files
}

// parseCSVFileName 从 CSV 文件名解析信息
// 格式: 姓名_学号_学期_course.csv
func parseCSVFileName(fileName string) (name, studentID, semester string) {
	baseName := strings.TrimSuffix(fileName, ".csv")
	parts := strings.Split(baseName, "_")

	if len(parts) >= 3 {
		name = parts[0]
		studentID = parts[1]
		semester = parts[2]
	}

	return
}
