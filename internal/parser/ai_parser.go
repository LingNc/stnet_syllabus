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
		courseCSV = matches[0][1]
	}
	if len(matches) >= 2 {
		activityCSV = matches[1][1]
	}

	return courseCSV, activityCSV
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
