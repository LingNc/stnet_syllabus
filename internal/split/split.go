// Package split 处理格式检测和数据拆分
package split

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Splitter 拆分器
type Splitter struct {
	InputDir      string
	OutputDir2D   string
	OutputDirList string
	SemesterCode  string // 从 config 传入的学期代码
}

// SplitResult 拆分结果
type SplitResult struct {
	FilePath     string
	Name         string
	StudentID    string
	SemesterCode string
	Format       string // "2d" 或 "list"
	CourseFile   string
	ActivityFile string
	Success      bool
	Error        string
}

// NewSplitter 创建拆分器
func NewSplitter(inputDir, outputDir2D, outputDirList, semesterCode string) *Splitter {
	return &Splitter{
		InputDir:      inputDir,
		OutputDir2D:   outputDir2D,
		OutputDirList: outputDirList,
		SemesterCode:  semesterCode,
	}
}

// Process 处理所有文件
func (s *Splitter) Process() ([]SplitResult, error) {
	// 确保输出目录存在
	if err := os.MkdirAll(s.OutputDir2D, 0755); err != nil {
		return nil, fmt.Errorf("创建 2D 输出目录失败: %w", err)
	}
	if err := os.MkdirAll(s.OutputDirList, 0755); err != nil {
		return nil, fmt.Errorf("创建 List 输出目录失败: %w", err)
	}

	// 读取输入目录
	entries, err := os.ReadDir(s.InputDir)
	if err != nil {
		return nil, fmt.Errorf("读取输入目录失败: %w", err)
	}

	var results []SplitResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".xls") {
			continue
		}

		filePath := filepath.Join(s.InputDir, entry.Name())
		result := s.SplitFile(filePath)
		results = append(results, result)

		if result.Success {
			fmt.Printf("✓ 拆分完成: %s -> 格式: %s\n", entry.Name(), result.Format)
		} else {
			fmt.Printf("✗ 拆分失败: %s - %s\n", entry.Name(), result.Error)
		}
	}

	return results, nil
}

// SplitFile 拆分单个文件
func (s *Splitter) SplitFile(filePath string) SplitResult {
	return s.SplitFileWithOptions(filePath, false)
}

// SplitFileWithOptions 拆分单个文件（支持选项）
func (s *Splitter) SplitFileWithOptions(filePath string, relaxed bool) SplitResult {
	result := SplitResult{
		FilePath: filePath,
		Success:  false,
	}

	// 解析文件名
	fileName := filepath.Base(filePath)
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	name, studentID, semesterCode, err := s.parseFileName(fileName)
	if err != nil {
		if relaxed {
			// 宽松模式：使用默认值
			name = baseName
			studentID = "unknown"
			semesterCode = "unknown"
		} else {
			result.Error = err.Error()
			return result
		}
	}
	result.Name = name
	result.StudentID = studentID
	result.SemesterCode = semesterCode

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("读取文件失败: %v", err)
		return result
	}

	htmlContent := string(content)

	// 如果学期代码是 unknown，从 HTML 中提取
	if semesterCode == "unknown" {
		semesterCode = extractSemesterCode(htmlContent)
		if semesterCode == "" {
			semesterCode = "unknown"
		}
		result.SemesterCode = semesterCode
	}

	// 宽松模式下，如果姓名或学号是默认值，尝试从 HTML 中提取
	if relaxed && (name == baseName || studentID == "unknown") {
		extractedName, extractedID := extractStudentInfo(htmlContent)
		if extractedName != "" {
			name = extractedName
		}
		if extractedID != "" {
			studentID = extractedID
		}
		result.Name = name
		result.StudentID = studentID
	}

	// 检测格式
	format := detectFormat(htmlContent)
	result.Format = format

	// 对于 2D 表格，直接使用 config 中的学期代码
	if format == "2d" && s.SemesterCode != "" {
		result.SemesterCode = s.SemesterCode
	}

	// 根据格式进行拆分
	switch format {
	case "list":
		return s.splitList(filePath, htmlContent, result)
	case "2d":
		return s.split2D(filePath, htmlContent, result)
	default:
		result.Error = "无法识别的文件格式"
		return result
	}
}

// parseFileName 解析文件名
func (s *Splitter) parseFileName(fileName string) (name, studentID, semesterCode string, err error) {
	// 去除扩展名
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// 格式: 姓名_学号_学期码
	parts := strings.Split(baseName, "_")
	if len(parts) >= 3 {
		name = parts[0]
		studentID = parts[1]
		semesterCode = parts[2]
	} else if len(parts) == 2 {
		// 兼容: 姓名_学号
		name = parts[0]
		studentID = parts[1]
		// 从内容中提取学期码
		semesterCode = "unknown"
	} else {
		return "", "", "", fmt.Errorf("文件名格式不正确")
	}

	return name, studentID, semesterCode, nil
}

// detectFormat 检测格式
func detectFormat(htmlContent string) string {
	// 优先检查明确的 TYPE 标记（简化后的文件）
	if strings.Contains(htmlContent, "<!-- TYPE: 2D_TABLE -->") {
		return "2d"
	}
	if strings.Contains(htmlContent, "<!-- TYPE: COURSE -->") ||
		strings.Contains(htmlContent, "<!-- TYPE: ACTIVITY -->") {
		return "list"
	}
	// 检查原始标记（未简化的文件）
	if strings.Contains(htmlContent, `pagetitle="pagetitle"`) ||
		strings.Contains(htmlContent, "上课班级代码") {
		return "list"
	}
	// 检查简化后的其他标记
	if strings.Contains(htmlContent, "<!-- SEMESTER:") {
		// 有 SEMESTER 注释但没有 TYPE 标记，可能是 list 格式
		return "list"
	}
	// 检查 2D 表原始标记
	if strings.Contains(htmlContent, `id='mytable'`) ||
		strings.Contains(htmlContent, `id="mytable"`) ||
		strings.Contains(htmlContent, "div_nokb") {
		return "2d"
	}
	return "unknown"
}

// extractSemesterCode 从 HTML 中提取学期代码
func extractSemesterCode(htmlContent string) string {
	// 首先尝试从 SEMESTER_CODE 注释中提取（2D表）
	re := regexp.MustCompile(`<!-- SEMESTER_CODE: (\d{5}) -->`)
	matches := re.FindStringSubmatch(htmlContent)
	if len(matches) >= 2 {
		return matches[1]
	}

	// 从 SEMESTER 注释中提取（list表）
	re = regexp.MustCompile(`<!-- SEMESTER: .*?(\d{4})-(\d{4})(?:学年)?第([一二12])学期.*?-->`)
	matches = re.FindStringSubmatch(htmlContent)

	if len(matches) >= 4 {
		startYear := matches[1]
		semesterNum := "0"
		if matches[3] == "二" || matches[3] == "2" {
			semesterNum = "1"
		}
		return startYear + semesterNum
	}

	// 尝试从其他位置提取
	re = regexp.MustCompile(`(\d{4})-(\d{4})(?:学年)?第([一二12])学期`)
	matches = re.FindStringSubmatch(htmlContent)

	if len(matches) >= 4 {
		startYear := matches[1]
		semesterNum := "0"
		if matches[3] == "二" || matches[3] == "2" {
			semesterNum = "1"
		}
		return startYear + semesterNum
	}

	return ""
}

// extractStudentInfo 从 HTML 中提取学生姓名和学号
func extractStudentInfo(htmlContent string) (name, studentID string) {
	// 尝试提取学号（12位数字）
	re := regexp.MustCompile(`学号[：:]\s*(\d{12})`)
	matches := re.FindStringSubmatch(htmlContent)
	if len(matches) >= 2 {
		studentID = matches[1]
	}

	// 尝试提取姓名
	re = regexp.MustCompile(`姓名[：:]\s*([^\s<]+)`)
	matches = re.FindStringSubmatch(htmlContent)
	if len(matches) >= 2 {
		name = matches[1]
	}

	return name, studentID
}

// splitList 拆分列表格式
func (s *Splitter) splitList(filePath string, htmlContent string, result SplitResult) SplitResult {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = fmt.Sprintf("解析 HTML 失败: %v", err)
		return result
	}

	var courseHTML strings.Builder
	var activityHTML strings.Builder

	// 提取学生信息
	infoDiv := doc.Find("div[group='group']").First()
	infoHTML := ""
	if infoDiv.Length() > 0 {
		infoHTML = fmt.Sprintf("<div group=\"group\">%s</div>\n", cleanWhitespace(infoDiv.Text()))
	}

	courseHTML.WriteString("<!DOCTYPE html>\n<html>\n<body>\n")
	activityHTML.WriteString("<!DOCTYPE html>\n<html>\n<body>\n")

	if infoHTML != "" {
		courseHTML.WriteString(infoHTML)
		activityHTML.WriteString(infoHTML)
	}

	hasCourse := false
	hasActivity := false

	// 遍历所有表格
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// 跳过布局表格
		style := table.AttrOr("style", "")
		if strings.Contains(style, "border:0px") && strings.Contains(style, "clear:left") {
			return
		}

		// 判断表格类型
		firstHeader := strings.TrimSpace(table.Find("thead td, th").First().Text())
		isCourseTable := strings.Contains(firstHeader, "上课班级代码")
		isActivityTable := strings.Contains(firstHeader, "环节")

		// 提取表格 HTML
		tableHTML := extractTableHTML(table)

		if isCourseTable {
			courseHTML.WriteString(tableHTML)
			hasCourse = true
		} else if isActivityTable {
			activityHTML.WriteString(tableHTML)
			hasActivity = true
		}
	})

	courseHTML.WriteString("</body>\n</html>")
	activityHTML.WriteString("</body>\n</html>")

	// 保存文件
	baseName := fmt.Sprintf("%s_%s_%s", result.Name, result.StudentID, result.SemesterCode)

	if hasCourse {
		courseFile := filepath.Join(s.OutputDirList, baseName+"_course.xls")
		if err := os.WriteFile(courseFile, []byte(courseHTML.String()), 0644); err != nil {
			result.Error = fmt.Sprintf("保存课程文件失败: %v", err)
			return result
		}
		result.CourseFile = courseFile
	}

	if hasActivity {
		activityFile := filepath.Join(s.OutputDirList, baseName+"_activity.xls")
		if err := os.WriteFile(activityFile, []byte(activityHTML.String()), 0644); err != nil {
			result.Error = fmt.Sprintf("保存环节文件失败: %v", err)
			return result
		}
		result.ActivityFile = activityFile
	}

	if !hasCourse && !hasActivity {
		result.Error = "未找到课程或环节数据"
		return result
	}

	result.Success = true
	return result
}

// split2D 拆分二维表格式
func (s *Splitter) split2D(filePath string, htmlContent string, result SplitResult) SplitResult {
	// 二维表不实际拆分，只是复制到目标目录
	// 实际解析由 AI 完成

	// 提取学期代码并与配置对比
	extractedCode := extractSemesterCode(htmlContent)
	if extractedCode != "" && s.SemesterCode != "" && extractedCode != s.SemesterCode {
		// 记录警告但不阻止处理
		fmt.Printf("  ⚠ 警告: 2D表文件 %s 中的学期代码 %s 与配置 %s 不一致\n",
			filepath.Base(filePath), extractedCode, s.SemesterCode)
	}

	// 如果提取到学期代码，使用提取的；否则使用配置的
	if extractedCode != "" {
		result.SemesterCode = extractedCode
	}

	baseName := fmt.Sprintf("%s_%s_%s", result.Name, result.StudentID, result.SemesterCode)
	targetFile := filepath.Join(s.OutputDir2D, baseName+".xls")

	// 复制文件
	if err := os.WriteFile(targetFile, []byte(htmlContent), 0644); err != nil {
		result.Error = fmt.Sprintf("复制文件失败: %v", err)
		return result
	}

	result.CourseFile = targetFile
	result.Success = true
	return result
}

// extractTableHTML 提取表格的 HTML
func extractTableHTML(table *goquery.Selection) string {
	var result strings.Builder
	result.WriteString("<table>\n")

	table.Find("tr").Each(func(i int, row *goquery.Selection) {
		result.WriteString("<tr>\n")
		row.Find("td, th").Each(func(j int, cell *goquery.Selection) {
			tag := "td"
			if cell.Is("th") {
				tag = "th"
			}
			text := cleanWhitespace(cell.Text())
			if text != "" {
				result.WriteString(fmt.Sprintf("<%s>%s</%s>\n", tag, text, tag))
			}
		})
		result.WriteString("</tr>\n")
	})

	result.WriteString("</table>\n")
	return result.String()
}

// cleanWhitespace 清理空白字符
func cleanWhitespace(str string) string {
	str = strings.ReplaceAll(str, "\n", " ")
	str = strings.ReplaceAll(str, "\t", " ")
	str = strings.ReplaceAll(str, "\u2002", " ")
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(str, " "))
}
