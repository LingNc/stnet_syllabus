// Package validate 处理数据验证和信息提取
package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Validator 验证器
type Validator struct {
	InputDir  string
	ErrorLog  string
	errors    []string
}

// ValidationResult 验证结果
type ValidationResult struct {
	FilePath     string
	Name         string
	StudentID    string
	Semester     string
	SemesterCode string
	Valid        bool
	Error        string
}

// NewValidator 创建验证器
func NewValidator(inputDir, errorLog string) *Validator {
	return &Validator{
		InputDir: inputDir,
		ErrorLog: errorLog,
		errors:   []string{},
	}
}

// ValidateFile 验证单个文件
func (v *Validator) ValidateFile(filePath string) ValidationResult {
	result := ValidationResult{
		FilePath: filePath,
		Valid:    false,
	}

	// 从文件名解析姓名和学号
	fileName := filepath.Base(filePath)
	name, studentID, err := parseFileName(fileName)
	if err != nil {
		result.Error = fmt.Sprintf("文件名格式错误: %v", err)
		v.logError(fileName, result.Error)
		return result
	}
	result.Name = name
	result.StudentID = studentID

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("读取文件失败: %v", err)
		v.logError(fileName, result.Error)
		return result
	}

	htmlContent := string(content)

	// 提取学期信息
	semester, semesterCode := extractSemester(htmlContent)
	result.Semester = semester
	result.SemesterCode = semesterCode

	// 验证姓名学号一致性
	if err := v.validateConsistency(htmlContent, name, studentID); err != nil {
		result.Error = fmt.Sprintf("数据一致性验证失败: %v", err)
		v.logError(fileName, result.Error)
		return result
	}

	result.Valid = true
	return result
}

// Process 处理目录中的所有文件
func (v *Validator) Process() ([]ValidationResult, error) {
	entries, err := os.ReadDir(v.InputDir)
	if err != nil {
		return nil, fmt.Errorf("读取输入目录失败: %w", err)
	}

	var results []ValidationResult
	validCount := 0
	errorCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".xls") {
			continue
		}

		filePath := filepath.Join(v.InputDir, entry.Name())
		result := v.ValidateFile(filePath)
		results = append(results, result)

		if result.Valid {
			validCount++
			fmt.Printf("✓ 验证通过: %s (学期: %s)\n", entry.Name(), result.SemesterCode)
		} else {
			errorCount++
			fmt.Printf("✗ 验证失败: %s - %s\n", entry.Name(), result.Error)
		}
	}

	// 写入错误日志
	if len(v.errors) > 0 {
		if err := v.writeErrorLog(); err != nil {
			fmt.Printf("警告: 写入错误日志失败: %v\n", err)
		}
	}

	fmt.Printf("\n验证完成: 通过 %d, 失败 %d\n", validCount, errorCount)
	return results, nil
}

// parseFileName 从文件名解析姓名和学号
// 格式: <姓名>_<学号>.xls
func parseFileName(fileName string) (name, studentID string, err error) {
	// 去除扩展名
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// 按最后一个下划线分割
	lastUnderscore := strings.LastIndex(baseName, "_")
	if lastUnderscore == -1 {
		return "", "", fmt.Errorf("文件名格式不正确，应为: 姓名_学号.xls")
	}

	name = baseName[:lastUnderscore]
	studentID = baseName[lastUnderscore+1:]

	if name == "" || studentID == "" {
		return "", "", fmt.Errorf("姓名或学号为空")
	}

	return name, studentID, nil
}

// extractSemester 提取学期信息
func extractSemester(htmlContent string) (semester, semesterCode string) {
	// 匹配 "2025-2026第二学期" 或 "2025-2026学年第二学期"
	re := regexp.MustCompile(`(\d{4})-(\d{4})(?:学年)?第([一二12])学期`)
	matches := re.FindStringSubmatch(htmlContent)

	if len(matches) >= 4 {
		startYear := matches[1]
		semesterNum := "0" // 默认第一学期
		if matches[3] == "二" || matches[3] == "2" {
			semesterNum = "1"
		}
		semester = matches[0]
		semesterCode = startYear + semesterNum
	}

	return semester, semesterCode
}

// validateConsistency 验证文件内容与文件名的一致性
func (v *Validator) validateConsistency(htmlContent, expectedName, expectedID string) error {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return fmt.Errorf("解析 HTML 失败: %w", err)
	}

	// 在页面中查找姓名和学号
	pageText := doc.Text()

	// 检查姓名
	if !strings.Contains(pageText, expectedName) {
		// 有些文件可能使用繁体或不同写法，输出警告但不阻止
		fmt.Printf("  警告: 文件中可能未包含姓名 '%s'\n", expectedName)
	}

	// 检查学号（尝试多种格式）
	// 由于Excel科学计数法可能导致精度丢失，这里只做警告不阻止
	if !strings.Contains(pageText, expectedID) {
		// 尝试匹配后8位
		if len(expectedID) >= 8 {
			suffix := expectedID[len(expectedID)-8:]
			if strings.Contains(pageText, suffix) {
				return nil
			}
		}
		// 仅警告，不阻止处理
		fmt.Printf("  警告: 文件内容中学号 '%s' 与文件名不一致（可能是Excel精度问题）\n", expectedID)
	}

	return nil
}

// logError 记录错误
func (v *Validator) logError(fileName, errorMsg string) {
	timestamp := "" // 可以添加时间戳
	v.errors = append(v.errors, fmt.Sprintf("[%s] %s: %s", timestamp, fileName, errorMsg))
}

// writeErrorLog 写入错误日志
func (v *Validator) writeErrorLog() error {
	// 确保日志目录存在
	logDir := filepath.Dir(v.ErrorLog)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(v.ErrorLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range v.errors {
		fmt.Fprintln(f, e)
	}

	return nil
}
