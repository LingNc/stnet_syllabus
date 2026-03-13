// Package preprocess 处理数据预处理和映射
package preprocess

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"stnet_syllabus/internal/simplify"
)

// MappingEntry 映射表条目
type MappingEntry struct {
	Name      string // 姓名
	StudentID string // 学号
	FileName  string // 原始文件名
}

// XlsxCell XML 单元格定义
type XlsxCell struct {
	R string `xml:"r,attr"`
	V string `xml:"v"`
}

// XlsxRow XML 行定义
type XlsxRow struct {
	Cells []XlsxCell `xml:"c"`
}

// XlsxSheetData XML 表数据
type XlsxSheetData struct {
	Rows []XlsxRow `xml:"row"`
}

// XlsxWorksheet XML 工作表
type XlsxWorksheet struct {
	SheetData XlsxSheetData `xml:"sheetData"`
}

// formatStudentID 格式化学号，处理科学计数法
// 如 "5.42311010415E+11" -> "542311010415"
func formatStudentID(id string) string {
	id = strings.TrimSpace(id)

	// 检查是否是科学计数法
	if strings.Contains(id, "E") || strings.Contains(id, "e") {
		// 使用高精度计算
		id = strings.ToUpper(id)

		// 解析尾数和指数
		var mantissa float64
		var exp int
		_, err := fmt.Sscanf(id, "%lE", &mantissa)
		if err != nil {
			return id
		}

		// 手动提取指数部分
		parts := strings.Split(id, "E")
		if len(parts) == 2 {
			exp, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}

		// 获取尾数字符串（去掉小数点）
		mantissaStr := strings.ReplaceAll(parts[0], ".", "")
		mantissaStr = strings.TrimLeft(mantissaStr, "0")

		// 根据指数调整
		// 5.42311010415E+11 表示 542311010415
		// 尾数整数部分1位，指数是11，所以结果应该是12位
		decimalIdx := strings.Index(parts[0], ".")
		if decimalIdx > 0 {
			// 小数点后的位数
			fracDigits := len(parts[0]) - decimalIdx - 1

			// 重新精确计算：将尾数作为整数，然后乘以10^(exp-小数位数)
			multiplier := exp - fracDigits

			// 构造结果
			result := mantissaStr
			for i := 0; i < multiplier; i++ {
				result += "0"
			}
			return result
		}
	}

	return id
}

// Processor 预处理器
type Processor struct {
	InputDir   string
	OutputDir  string
	MappingFile string
}

// NewProcessor 创建预处理器
func NewProcessor(inputDir, outputDir, mappingFile string) *Processor {
	return &Processor{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		MappingFile: mappingFile,
	}
}

// LoadMapping 从 Excel 文件加载映射表
func (p *Processor) LoadMapping() ([]MappingEntry, error) {
	// 首先从 XML 中读取原始学号
	studentIDs, err := p.loadStudentIDsFromXML()
	if err != nil {
		fmt.Printf("警告: 从 XML 读取学号失败: %v，将使用 Excel API\n", err)
	}

	f, err := excelize.OpenFile(p.MappingFile)
	if err != nil {
		return nil, fmt.Errorf("打开映射表失败: %w", err)
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取映射表失败: %w", err)
	}

	var entries []MappingEntry
	// 跳过表头，从第二行开始
	// Excel列: 提交者, 提交时间, 姓名, 学号, 文件名
	for i, row := range rows {
		if i == 0 {
			continue // 跳过表头
		}
		if len(row) >= 5 {
			studentID := formatStudentID(row[3])

			// 如果 XML 中有原始值，使用 XML 的值（第 i 行对应 D{i+1} 单元格）
			if studentIDs != nil {
				if xmlID, ok := studentIDs[i+1]; ok && xmlID != "" {
					studentID = xmlID
				}
			}

			name := strings.TrimSpace(row[2])
			fileName := strings.TrimSpace(row[4])

			// 如果名字为空，尝试从文件名中提取
			if name == "" && fileName != "" {
				// 从文件名提取（去除扩展名）
				baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
				// 如果文件名包含下划线，取第一部分作为名字
				if idx := strings.Index(baseName, "_"); idx > 0 {
					name = baseName[:idx]
				} else {
					name = baseName
				}
				fmt.Printf("警告: 第 %d 行姓名为空，从文件名推断为: %s\n", i+1, name)
			}

			entry := MappingEntry{
				Name:      name,
				StudentID: studentID,
				FileName:  fileName,
			}
			if entry.Name != "" && entry.StudentID != "" {
				entries = append(entries, entry)
			} else {
				fmt.Printf("警告: 第 %d 行数据不完整（姓名: %s, 学号: %s），跳过\n", i+1, entry.Name, entry.StudentID)
			}
		}
	}

	return entries, nil
}

// loadStudentIDsFromXML 直接从 xlsx 的 XML 中读取学号
func (p *Processor) loadStudentIDsFromXML() (map[int]string, error) {
	// 打开 xlsx 作为 zip
	r, err := zip.OpenReader(p.MappingFile)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// 找到 sheet1.xml
	var sheetFile *zip.File
	for _, f := range r.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			sheetFile = f
			break
		}
	}
	if sheetFile == nil {
		return nil, fmt.Errorf("未找到 sheet1.xml")
	}

	// 读取 XML
	rc, err := sheetFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	// 解析 XML
	var worksheet XlsxWorksheet
	if err := xml.Unmarshal(data, &worksheet); err != nil {
		return nil, err
	}

	// 提取 D 列的学号
	studentIDs := make(map[int]string)
	for _, row := range worksheet.SheetData.Rows {
		for _, cell := range row.Cells {
			// D 列的单元格，如 D2, D3 等
			if len(cell.R) >= 2 && cell.R[0] == 'D' {
				rowNum, _ := strconv.Atoi(cell.R[1:])
				if rowNum > 0 && cell.V != "" {
					studentIDs[rowNum] = cell.V
				}
			}
		}
	}

	return studentIDs, nil
}

// ExtractAndRename 解压 zip 文件并重命名 xls 文件
func (p *Processor) ExtractAndRename(zipPath string, mapping []MappingEntry) error {
	// 确保输出目录存在
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建文件名到映射的查找表
	fileMap := make(map[string]MappingEntry)
	for _, entry := range mapping {
		// 存储不带扩展名的文件名作为键
		baseName := strings.TrimSuffix(entry.FileName, filepath.Ext(entry.FileName))
		fileMap[baseName] = entry
	}

	// 打开 zip 文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 zip 文件失败: %w", err)
	}
	defer r.Close()

	processed := 0
	skipped := 0

	for _, file := range r.File {
		// 跳过目录和系统文件
		if file.FileInfo().IsDir() || strings.HasPrefix(file.Name, "__MACOSX") || strings.HasPrefix(file.Name, ".") {
			continue
		}

		// 只处理 .xls 和 .xlsx 文件
		lowerName := strings.ToLower(file.Name)
		if !strings.HasSuffix(lowerName, ".xls") && !strings.HasSuffix(lowerName, ".xlsx") {
			continue
		}

		// 获取原始文件名（不含路径和扩展名）
		baseName := filepath.Base(file.Name)
		// 去除 .xls 或 .xlsx 扩展名
		baseName = strings.TrimSuffix(baseName, ".xls")
		baseName = strings.TrimSuffix(baseName, ".xlsx")
		baseName = strings.TrimSuffix(baseName, ".XLS")
		baseName = strings.TrimSuffix(baseName, ".XLSX")

		// 查找映射（支持全角空格文件名匹配）
		entry, found := fileMap[baseName]
		if !found {
			// 尝试用原始字符串匹配（处理特殊字符）
			for key, val := range fileMap {
				if strings.TrimSpace(key) == strings.TrimSpace(baseName) ||
					strings.Contains(key, baseName) ||
					strings.Contains(baseName, key) {
					entry = val
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("警告: 文件 %s (base=%s) 在映射表中未找到，跳过\n", file.Name, baseName)
				skipped++
				continue
			}
		}

		// 打开 zip 中的文件
		rc, err := file.Open()
		if err != nil {
			fmt.Printf("错误: 无法打开文件 %s: %v\n", file.Name, err)
			skipped++
			continue
		}

		// 创建新文件名
		newFileName := fmt.Sprintf("%s_%s.xls", entry.Name, entry.StudentID)
		outputPath := filepath.Join(p.OutputDir, newFileName)

		// 创建输出文件
		outFile, err := os.Create(outputPath)
		if err != nil {
			rc.Close()
			fmt.Printf("错误: 无法创建文件 %s: %v\n", outputPath, err)
			skipped++
			continue
		}

		// 复制内容
		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			fmt.Printf("错误: 复制文件内容失败 %s: %v\n", file.Name, err)
			skipped++
			continue
		}

		fmt.Printf("已处理: %s -> %s\n", file.Name, newFileName)
		processed++
	}

	fmt.Printf("\n预处理完成: 成功 %d, 跳过 %d\n", processed, skipped)
	return nil
}

// Process 执行完整的预处理流程
func (p *Processor) Process() error {
	// 查找映射表文件
	if p.MappingFile == "" {
		// 自动查找 input 目录下的 xlsx 文件
		entries, err := os.ReadDir(p.InputDir)
		if err != nil {
			return fmt.Errorf("读取输入目录失败: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") {
				p.MappingFile = filepath.Join(p.InputDir, entry.Name())
				break
			}
		}
	}

	if p.MappingFile == "" {
		return fmt.Errorf("未找到映射表文件")
	}

	fmt.Printf("使用映射表: %s\n", p.MappingFile)

	// 加载映射表
	mapping, err := p.LoadMapping()
	if err != nil {
		return err
	}
	fmt.Printf("加载了 %d 条映射记录\n", len(mapping))

	// 查找 zip 文件
	entries, err := os.ReadDir(p.InputDir)
	if err != nil {
		return fmt.Errorf("读取输入目录失败: %w", err)
	}

	var zipFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			zipFiles = append(zipFiles, filepath.Join(p.InputDir, entry.Name()))
		}
	}

	if len(zipFiles) == 0 {
		return fmt.Errorf("未找到 zip 文件")
	}

	// 处理所有 zip 文件
	for _, zipPath := range zipFiles {
		fmt.Printf("\n处理压缩包: %s\n", zipPath)
		if err := p.ExtractAndRename(zipPath, mapping); err != nil {
			return err
		}
	}

	return nil
}

// ProcessDirectXLS 直接处理 input 目录中的 xls 文件（无 zip/映射表模式）
// 从 xls 文件中提取学生信息并重命名
func (p *Processor) ProcessDirectXLS() error {
	// 确保输出目录存在
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 读取 input 目录
	entries, err := os.ReadDir(p.InputDir)
	if err != nil {
		return fmt.Errorf("读取输入目录失败: %w", err)
	}

	var xlsFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lowerName := strings.ToLower(entry.Name())
		if strings.HasSuffix(lowerName, ".xls") && !strings.HasSuffix(lowerName, ".xlsx") {
			xlsFiles = append(xlsFiles, filepath.Join(p.InputDir, entry.Name()))
		}
	}

	if len(xlsFiles) == 0 {
		return fmt.Errorf("未找到 xls 文件")
	}

	fmt.Printf("发现 %d 个 xls 文件，开始直接处理...\n\n", len(xlsFiles))

	processed := 0
	skipped := 0

	for _, xlsPath := range xlsFiles {
		fileName := filepath.Base(xlsPath)
		fmt.Printf("处理: %s\n", fileName)

		// 从文件中提取学生信息
		info, err := simplify.ExtractStudentInfoFromFile(xlsPath)
		if err != nil {
			fmt.Printf("  错误: 提取学生信息失败: %v，跳过\n", err)
			skipped++
			continue
		}

		// 构建新文件名
		newFileName := fmt.Sprintf("%s_%s_%s.xls", info.Name, info.StudentID, info.SemesterCode)
		outputPath := filepath.Join(p.OutputDir, newFileName)

		// 复制文件
		content, err := os.ReadFile(xlsPath)
		if err != nil {
			fmt.Printf("  错误: 读取文件失败: %v，跳过\n", err)
			skipped++
			continue
		}

		if err := os.WriteFile(outputPath, content, 0644); err != nil {
			fmt.Printf("  错误: 写入文件失败: %v，跳过\n", err)
			skipped++
			continue
		}

		fmt.Printf("  成功: %s -> %s\n", fileName, newFileName)
		if info.SemesterCode == "" {
			fmt.Printf("  警告: 未能提取学期代码，请检查配置文件\n")
		}
		processed++
	}

	fmt.Printf("\n直接处理完成: 成功 %d, 跳过 %d\n", processed, skipped)
	return nil
}
