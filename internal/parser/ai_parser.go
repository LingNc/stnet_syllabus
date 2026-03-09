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
	InputDir     string
	OutputDir    string
	PromptFile   string
	Client       AIClient
	Concurrency  int
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
func NewAI2DParser(inputDir, outputDir, promptFile string, client AIClient, concurrency int) *AI2DParser {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &AI2DParser{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		PromptFile:  promptFile,
		Client:      client,
		Concurrency: concurrency,
	}
}

// Process 处理所有文件
func (p *AI2DParser) Process() ([]AIResult, error) {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
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

	// 精简 HTML（保留核心表格数据）
	simplifiedHTML := simplify2DForAI(htmlContent)

	// 调用 AI
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	courseCSV, activityCSV, err := p.Client.Parse2DTable(ctx, simplifiedHTML, prompt)
	if err != nil {
		result.Error = fmt.Sprintf("AI 解析失败: %v", err)
		return result
	}

	// 生成输出文件名
	fileName := filepath.Base(filePath)
	baseName := strings.TrimSuffix(fileName, ".xls")

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

	// 提取学生信息
	doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
		result.WriteString("<tr>")
		s.Find("td").Each(func(j int, td *goquery.Selection) {
			text := strings.TrimSpace(td.Text())
			if text != "" {
				result.WriteString(fmt.Sprintf("<td>%s</td>", text))
			}
		})
		result.WriteString("</tr>\n")
	})

	// 提取课表主体
	doc.Find("table#mytable, table[name='mytable'], table.schedule").Find("tr").Each(func(i int, row *goquery.Selection) {
		result.WriteString("<tr>")
		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			courseNode := cell.Find("div.div1")
			if courseNode.Length() > 0 {
				text := strings.TrimSpace(courseNode.Text())
				result.WriteString(fmt.Sprintf("<td>%s</td>", text))
			} else if cell.Find("div.div_nokb").Length() > 0 {
				result.WriteString("<td></td>")
			} else {
				text := strings.TrimSpace(cell.Text())
				if text != "" {
					result.WriteString(fmt.Sprintf("<td>%s</td>", text))
				}
			}
		})
		result.WriteString("</tr>\n")
	})

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
