// Package preprocess 处理数据预处理和映射
package preprocess

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// MappingEntry 映射表条目
type MappingEntry struct {
	Name      string // 姓名
	StudentID string // 学号
	FileName  string // 原始文件名
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
	for i, row := range rows {
		if i == 0 {
			continue // 跳过表头
		}
		if len(row) >= 3 {
			entry := MappingEntry{
				Name:      strings.TrimSpace(row[0]),
				StudentID: strings.TrimSpace(row[1]),
				FileName:  strings.TrimSpace(row[2]),
			}
			if entry.Name != "" && entry.StudentID != "" {
				entries = append(entries, entry)
			}
		}
	}

	return entries, nil
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

		// 只处理 .xls 文件
		if !strings.HasSuffix(strings.ToLower(file.Name), ".xls") {
			continue
		}

		// 获取原始文件名（不含路径和扩展名）
		baseName := strings.TrimSuffix(filepath.Base(file.Name), ".xls")

		// 查找映射
		entry, found := fileMap[baseName]
		if !found {
			fmt.Printf("警告: 文件 %s 在映射表中未找到，跳过\n", baseName)
			skipped++
			continue
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
