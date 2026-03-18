// Package parser 处理 AI 解析（二维表）
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// AIClient AI 客户端接口
type AIClient interface {
	Parse2DTable(ctx context.Context, htmlContent string, prompt string) (courseCSV, activityCSV string, err error)
}

// AI2DParser 二维表 AI 解析器
type AI2DParser struct {
	InputDir       string
	OutputDir      string
	PreprocessDir  string // AI预处理后的HTML输出目录
	PromptFile     string
	Client         AIClient
	Concurrency    int
}

// AIResult AI 解析结果
type AIResult struct {
	InputFile    string
	CourseCSV    string
	ActivityCSV  string
	Success      bool
	Error        string
}

// NewAI2DParser 创建 AI 解析器
func NewAI2DParser(inputDir, outputDir, preprocessDir, promptFile string, client AIClient, concurrency int) *AI2DParser {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &AI2DParser{
		InputDir:      inputDir,
		OutputDir:     outputDir,
		PreprocessDir: preprocessDir,
		PromptFile:    promptFile,
		Client:        client,
		Concurrency:   concurrency,
	}
}

// Process 处理所有文件
func (p *AI2DParser) Process() ([]AIResult, error) {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := os.MkdirAll(p.PreprocessDir, 0755); err != nil {
		return nil, fmt.Errorf("创建预处理输出目录失败: %w", err)
	}

	// 读取 prompt
	prompt, err := os.ReadFile(p.PromptFile)
	if err != nil {
		return nil, fmt.Errorf("读取 prompt 文件失败: %w", err)
	}

	// 读取输入目录
	entries, err := os.ReadDir(p.InputDir)
	if err != nil {
		return nil, fmt.Errorf("读取输入目录失败: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".xls") {
			continue
		}
		files = append(files, filepath.Join(p.InputDir, entry.Name()))
	}

	// 使用 worker pool 并发处理
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, p.Concurrency)
	resultsChan := make(chan AIResult, len(files))

	for _, file := range files {
		wg.Add(1)
		semaphore <- struct{}{} // 获取令牌

		go func(filePath string) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放令牌

			result := p.ParseFile(filePath, string(prompt))
			resultsChan <- result

			if result.Success {
				fmt.Printf("✓ AI 解析完成: %s\n", filepath.Base(filePath))
				if result.CourseCSV != "" {
					fmt.Printf("  课程: %s\n", filepath.Base(result.CourseCSV))
				}
				if result.ActivityCSV != "" {
					fmt.Printf("  环节: %s\n", filepath.Base(result.ActivityCSV))
				}
			} else {
				fmt.Printf("✗ AI 解析失败: %s - %s\n", filepath.Base(filePath), result.Error)
			}
		}(file)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集结果
	var results []AIResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return results, nil
}

// ParseFile 解析单个文件
func (p *AI2DParser) ParseFile(filePath string, prompt string) AIResult {
	result := AIResult{
		InputFile: filePath,
		Success:   false,
	}

	// 读取文件
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("读取文件失败: %v", err)
		return result
	}

	htmlContent := string(content)

	// 生成输出文件名（提前定义 baseName 用于后续保存）
	fileName := filepath.Base(filePath)
	baseName := strings.TrimSuffix(fileName, ".xls")

	// 精简 HTML（保留核心表格数据）
	simplifiedHTML := simplify2DForAI(htmlContent)

	// 保存预处理后的 HTML 到 split/2d_ai_pre 目录
	if p.PreprocessDir != "" {
		preprocessFile := filepath.Join(p.PreprocessDir, baseName+"_preprocessed.xls")
		if err := os.WriteFile(preprocessFile, []byte(simplifiedHTML), 0644); err != nil {
			// 保存失败不影响主流程，仅记录警告
			fmt.Printf("  警告: 保存预处理 HTML 失败: %v\n", err)
		}
	}

	// 调用 AI
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	courseCSV, activityCSV, err := p.Client.Parse2DTable(ctx, simplifiedHTML, prompt)
	if err != nil {
		result.Error = fmt.Sprintf("AI 解析失败: %v", err)
		return result
	}

	// 保存课程 CSV
	if courseCSV != "" {
		courseFile := filepath.Join(p.OutputDir, baseName+"_course.csv")
		if err := os.WriteFile(courseFile, []byte(courseCSV), 0644); err != nil {
			result.Error = fmt.Sprintf("保存课程 CSV 失败: %v", err)
			return result
		}
		result.CourseCSV = courseFile
	}

	// 保存环节 CSV
	if activityCSV != "" {
		activityFile := filepath.Join(p.OutputDir, baseName+"_activity.csv")
		if err := os.WriteFile(activityFile, []byte(activityCSV), 0644); err != nil {
			result.Error = fmt.Sprintf("保存环节 CSV 失败: %v", err)
			return result
		}
		result.ActivityCSV = activityFile
	}

	result.Success = true
	return result
}

// simplify2DForAI 为 AI 精简 2D HTML
func simplify2DForAI(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	var result strings.Builder
	result.WriteString("<!DOCTYPE html>\n<html>\n<body>\n")

	// 提取学生信息（第一行通常是表头，保留原有结构）
	doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
		result.WriteString("<tr>")
		s.Find("td").Each(func(j int, td *goquery.Selection) {
			text := strings.TrimSpace(td.Text())
			// 保留所有单元格，包括空的
			result.WriteString(fmt.Sprintf("<td>%s</td>", text))
		})
		result.WriteString("</tr>\n")
	})

	// 提取课表主体
	doc.Find("table#mytable, table[name='mytable'], table.schedule").Find("tr").Each(func(i int, row *goquery.Selection) {
		result.WriteString("<tr>")
		cells := row.Find("td")
		cellCount := cells.Length()

		// 检测是否是第一行（星期行）：通常是7列（周一到周日）
		isWeekHeader := false
		if i == 0 || cellCount == 7 {
			// 检查是否包含星期信息
			firstCell := cells.First().Text()
			if strings.Contains(firstCell, "星期") || strings.Contains(firstCell, "周一") {
				isWeekHeader = true
			}
		}

		// 如果是星期行，在前面添加"节次\\星期"列
		if isWeekHeader {
			result.WriteString("<td>节次\\星期</td>")
		}

		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			text := strings.TrimSpace(cell.Text())

			// 剔除"上午"、"下午"、"晚上"等时间分段标签
			if text == "上午" || text == "下午" || text == "晚上" {
				return // 跳过此单元格
			}

			courseNode := cell.Find("div.div1")
			if courseNode.Length() > 0 {
				result.WriteString(fmt.Sprintf("<td>%s</td>", text))
			} else if cell.Find("div.div_nokb").Length() > 0 {
				result.WriteString("<td></td>")
			} else {
				// 保留所有单元格，包括空的
				result.WriteString(fmt.Sprintf("<td>%s</td>", text))
			}
		})
		result.WriteString("</tr>\n")
	})

	// 提取环节信息
	// 优先从 <!-- ACTIVITIES: ... --> 注释中提取（简化后的格式）
	activityCommentRe := regexp.MustCompile(`<!-- ACTIVITIES:\s*(.+?)\s*-->`)
	matches := activityCommentRe.FindStringSubmatch(htmlContent)
	if len(matches) >= 2 {
		// 从注释中提取环节数据
		activityText := matches[1]
		// 分割多个环节（注1、注2...）
		activityRe := regexp.MustCompile(`注\d+、[^注]+`)
		activities := activityRe.FindAllString(activityText, -1)
		for _, activity := range activities {
			activity = strings.TrimSpace(activity)
			if activity != "" {
				result.WriteString(fmt.Sprintf("<div class=\"activity\">%s</div>\n", activity))
			}
		}
	} else {
		// 从原始 HTML 的 div 中提取（未简化格式）
		// 环节数据通常在课表后面的 div 中，格式为：注1、[编号]环节名称 第X-X周 指导老师
		doc.Find("div").Each(func(i int, div *goquery.Selection) {
			text := strings.TrimSpace(div.Text())
			// 检查是否包含环节注释标记
			if strings.Contains(text, "注1") || strings.Contains(text, "注2") || strings.Contains(text, "注：") {
				// 清理并输出环节信息
				lines := strings.Split(text, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && (strings.HasPrefix(line, "注") || strings.Contains(line, "课程设计") || strings.Contains(line, "实习")) {
						result.WriteString(fmt.Sprintf("<div class=\"activity\">%s</div>\n", line))
					}
				}
			}
		})
	}

	result.WriteString("</body>\n</html>")
	return result.String()
}

// ParseCSVFromResponse 从 AI 响应中解析 CSV
func ParseCSVFromResponse(response string) (courseCSV, activityCSV string) {
	// 使用正则表达式提取 CSV 代码块
	// 修改：使用 (?s) 开启多行模式，\n? 使结尾换行符可选
	courseRe := regexp.MustCompile("(?s)```csv\\s*\\n(.*?)\\n?```")
	matches := courseRe.FindAllStringSubmatch(response, -1)

	if len(matches) >= 1 {
		courseCSV = fixCSVQuotes(matches[0][1])
	}
	if len(matches) >= 2 {
		activityCSV = fixCSVQuotes(matches[1][1])
	}

	return courseCSV, activityCSV
}

// fixCSVQuotes 修复 CSV 中的引号问题
// 如果某行列数超过预期，尝试将多余的部分合并到周次字段并添加引号
func fixCSVQuotes(csvContent string) string {
	lines := strings.Split(csvContent, "\n")
	if len(lines) == 0 {
		return csvContent
	}

	var result strings.Builder
	// 写入表头
	result.WriteString(lines[0] + "\n")

	// 检测表头的列数
	headerCols := strings.Split(lines[0], ",")
	expectedCols := len(headerCols)

	// 处理数据行
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// 简单解析：按逗号分割，但考虑引号
		cols := parseCSVLine(line)

		if len(cols) > expectedCols && expectedCols >= 3 {
			// 列数过多，需要合并到周次字段（第3列，索引2）
			// 合并从索引2开始到倒数第2列的所有内容（假设最后两列是节次和地点）
			extraCols := len(cols) - expectedCols
			weekField := cols[2]
			for j := 0; j < extraCols; j++ {
				weekField += "," + cols[3+j]
			}
			// 重新构建行
			newCols := make([]string, expectedCols)
			newCols[0] = cols[0] // 课程
			newCols[1] = cols[1] // 教师
			newCols[2] = weekField
			// 复制剩余列
			for j := 3; j < expectedCols; j++ {
				idx := len(cols) - expectedCols + j
				if idx < len(cols) {
					newCols[j] = cols[idx]
				}
			}
			cols = newCols
		}

		// 确保包含逗号的字段有引号
		for j := range cols {
			if strings.Contains(cols[j], ",") && !strings.HasPrefix(cols[j], "\"") {
				cols[j] = fmt.Sprintf("\"%s\"", cols[j])
			}
		}

		result.WriteString(strings.Join(cols, ",") + "\n")
	}

	return result.String()
}

// parseCSVLine 简单解析 CSV 行，处理引号
func parseCSVLine(line string) []string {
	var cols []string
	var current strings.Builder
	inQuotes := false

	for _, ch := range line {
		if ch == '"' {
			inQuotes = !inQuotes
			current.WriteRune(ch)
		} else if ch == ',' && !inQuotes {
			cols = append(cols, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}
	// 添加最后一列
	if current.Len() > 0 || len(line) == 0 || line[len(line)-1] == ',' {
		cols = append(cols, current.String())
	}

	return cols
}

// MergeCSVs 合并所有 CSV 到一个目录
func MergeCSVs(inputDir, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
			continue
		}

		srcPath := filepath.Join(inputDir, entry.Name())
		dstPath := filepath.Join(outputDir, entry.Name())

		content, err := os.ReadFile(srcPath)
		if err != nil {
			fmt.Printf("警告: 读取文件失败 %s: %v\n", entry.Name(), err)
			continue
		}

		if err := os.WriteFile(dstPath, content, 0644); err != nil {
			fmt.Printf("警告: 写入文件失败 %s: %v\n", entry.Name(), err)
			continue
		}
	}

	return nil
}
