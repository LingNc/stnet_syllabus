// Package excel 处理 Excel 报表生成
package excel

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
	"stnet_syllabus/internal/config"
)

// Generator Excel 生成器
type Generator struct {
	CSVDir      string
	OutputDir   string
	TotalWeeks  int
	ExcelConfig config.ExcelConfig
}

// NewGenerator 创建 Excel 生成器
func NewGenerator(csvDir, outputDir string, totalWeeks int, excelConfig ...config.ExcelConfig) *Generator {
	g := &Generator{
		CSVDir:     csvDir,
		OutputDir:  outputDir,
		TotalWeeks: totalWeeks,
	}
	if len(excelConfig) > 0 {
		g.ExcelConfig = excelConfig[0]
	} else {
		// 默认配置
		g.ExcelConfig = config.ExcelConfig{
			Header: config.ExcelHeaderConfig{
				FontSize:     12,
				Bold:         true,
				BgColor:      "none",
				FontColor:    "#000000",
				BorderBottom: true,
				RowHeight:    25,
			},
			Data: config.ExcelDataConfig{
				RowHeight: 40,
				WrapText:  true,
			},
			Column: config.ExcelColumnConfig{
				MinWidth:        8,
				MaxWidth:        40,
				CharWidthFactor: 1.2,
			},
			Table: config.ExcelTableConfig{
				MaxPeriods: 4,
			},
		}
	}
	return g
}

// Generate 生成 Excel 报表
func (g *Generator) Generate() error {
	if err := os.MkdirAll(g.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建新工作簿
	f := excelize.NewFile()

	// 添加汇总表
	if err := g.addSummarySheet(f); err != nil {
		return fmt.Errorf("添加汇总表失败: %w", err)
	}

	// 添加每周表
	weeklyDir := filepath.Join(g.CSVDir, "weekly")
	for week := 1; week <= g.TotalWeeks; week++ {
		weekFile := filepath.Join(weeklyDir, fmt.Sprintf("free_week_%d.csv", week))
		if _, err := os.Stat(weekFile); os.IsNotExist(err) {
			continue
		}

		if err := g.addWeekSheet(f, week, weekFile); err != nil {
			fmt.Printf("警告: 添加第 %d 周表失败: %v\n", week, err)
			continue
		}
	}

	// 删除默认 Sheet
	f.DeleteSheet("Sheet1")

	// 保存文件
	outputFile := filepath.Join(g.OutputDir, "free_time_schedule.xlsx")
	if err := f.SaveAs(outputFile); err != nil {
		return fmt.Errorf("保存 Excel 文件失败: %w", err)
	}

	fmt.Printf("✓ 已生成 Excel 报表: %s\n", outputFile)
	return nil
}

// addSummarySheet 添加汇总表
func (g *Generator) addSummarySheet(f *excelize.File) error {
	// 创建汇总表
	_, err := f.NewSheet("汇总表")
	if err != nil {
		return err
	}

	// 读取汇总 CSV
	summaryFile := filepath.Join(g.CSVDir, "free_time_summary.csv")
	if _, err := os.Stat(summaryFile); os.IsNotExist(err) {
		return fmt.Errorf("汇总文件不存在")
	}

	records, err := g.readCSV(summaryFile)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		return fmt.Errorf("汇总文件为空")
	}

	// 写入数据
	for rowIdx, row := range records {
		for colIdx, cell := range row {
			cellRef, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			f.SetCellValue("汇总表", cellRef, cell)
		}
	}

	// 设置样式
	g.setSheetStyle(f, "汇总表", len(records[0]), len(records))

	return nil
}

// addWeekSheet 添加周表
func (g *Generator) addWeekSheet(f *excelize.File, week int, csvFile string) error {
	sheetName := fmt.Sprintf("第%d周", week)
	_, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}

	// 读取 CSV
	records, err := g.readCSV(csvFile)
	if err != nil {
		return err
	}

	// 写入数据
	for rowIdx, row := range records {
		for colIdx, cell := range row {
			cellRef, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			f.SetCellValue(sheetName, cellRef, cell)
		}
	}

	// 设置样式
	g.setSheetStyle(f, sheetName, len(records[0]), len(records))

	return nil
}

// readCSV 读取 CSV 文件
func (g *Generator) readCSV(filePath string) ([][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // 允许变长字段
	return reader.ReadAll()
}

// setSheetStyle 设置工作表样式
func (g *Generator) setSheetStyle(f *excelize.File, sheetName string, maxCol, maxRow int) {
	cfg := g.ExcelConfig

	// 构建表头样式
	headerStyle := &excelize.Style{
		Font: &excelize.Font{
			Bold:   cfg.Header.Bold,
			Size:   float64(cfg.Header.FontSize),
			Color:  cfg.Header.FontColor,
			Family: "Arial",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	}

	// 背景色
	if cfg.Header.BgColor != "" && cfg.Header.BgColor != "none" {
		headerStyle.Fill = excelize.Fill{
			Type:    "pattern",
			Color:   []string{cfg.Header.BgColor},
			Pattern: 1,
		}
	}

	// 底部边框
	if cfg.Header.BorderBottom {
		headerStyle.Border = []excelize.Border{
			{Type: "bottom", Color: "#000000", Style: 2},
		}
	}

	headerStyleID, _ := f.NewStyle(headerStyle)

	// 设置数据样式
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "top",
			WrapText:   cfg.Data.WrapText,
		},
	})

	// 应用表头样式
	for col := 1; col <= maxCol; col++ {
		cellRef, _ := excelize.CoordinatesToCellName(col, 1)
		f.SetCellStyle(sheetName, cellRef, cellRef, headerStyleID)
	}

	// 应用数据样式
	if maxRow > 1 {
		startCell, _ := excelize.CoordinatesToCellName(1, 2)
		endCell, _ := excelize.CoordinatesToCellName(maxCol, maxRow)
		f.SetCellStyle(sheetName, startCell, endCell, dataStyle)
	}

	// 设置列宽（使用配置）
	minWidth := cfg.Column.MinWidth
	if minWidth == 0 {
		minWidth = 8
	}
	maxWidth := cfg.Column.MaxWidth
	if maxWidth == 0 {
		maxWidth = 40
	}
	charFactor := cfg.Column.CharWidthFactor
	if charFactor == 0 {
		charFactor = 1.2
	}

	for col := 1; col <= maxCol; col++ {
		colLetter, _ := excelize.ColumnNumberToName(col)

		// 计算最大宽度
		colMaxWidth := minWidth
		for row := 1; row <= maxRow; row++ {
			cellRef, _ := excelize.CoordinatesToCellName(col, row)
			val, _ := f.GetCellValue(sheetName, cellRef)
			width := float64(len(val)) * charFactor
			if width > colMaxWidth {
				colMaxWidth = width
			}
		}

		// 限制最大宽度
		if colMaxWidth > maxWidth {
			colMaxWidth = maxWidth
		}
		if colMaxWidth < minWidth {
			colMaxWidth = minWidth
		}

		f.SetColWidth(sheetName, colLetter, colLetter, colMaxWidth)
	}

	// 设置行高（使用配置）
	f.SetRowHeight(sheetName, 1, cfg.Header.RowHeight)
	for row := 2; row <= maxRow; row++ {
		f.SetRowHeight(sheetName, row, cfg.Data.RowHeight)
	}

	// 冻结首行
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		XSplit:      1,
		YSplit:      1,
		TopLeftCell: "B2",
	})
}

// ConvertCSVToExcel 将单个 CSV 转换为 Excel
func (g *Generator) ConvertCSVToExcel(csvFile string) error {
	records, err := g.readCSV(csvFile)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		return fmt.Errorf("CSV 文件为空")
	}

	f := excelize.NewFile()
	sheetName := "Sheet1"

	// 写入数据
	for rowIdx, row := range records {
		for colIdx, cell := range row {
			cellRef, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			f.SetCellValue(sheetName, cellRef, cell)
		}
	}

	// 设置样式
	g.setSheetStyle(f, sheetName, len(records[0]), len(records))

	// 保存文件
	baseName := strings.TrimSuffix(filepath.Base(csvFile), ".csv")
	outputFile := filepath.Join(g.OutputDir, baseName+".xlsx")

	if err := f.SaveAs(outputFile); err != nil {
		return fmt.Errorf("保存 Excel 文件失败: %w", err)
	}

	fmt.Printf("✓ 已生成: %s\n", outputFile)
	return nil
}
