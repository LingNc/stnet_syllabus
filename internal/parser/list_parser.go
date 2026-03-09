// Package parser 处理课表解析（列表格式直接解析）
package parser

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ListParser 列表格式解析器
type ListParser struct {
	InputDir  string
	OutputDir string
}

// ParseResult 解析结果
type ParseResult struct {
	InputFile    string
	CourseCSV    string
	ActivityCSV  string
	Success      bool
	Error        string
}

// NewListParser 创建列表格式解析器
func NewListParser(inputDir, outputDir string) *ListParser {
	return &ListParser{
		InputDir:  inputDir,
		OutputDir: outputDir,
	}
}

// Process 处理所有文件
func (p *ListParser) Process() ([]ParseResult, error) {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	entries, err := os.ReadDir(p.InputDir)
	if err != nil {
		return nil, fmt.Errorf("读取输入目录失败: %w", err)
	}

	// 按文件名分组（course 和 activity）
	fileGroups := make(map[string][]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".xls") {
			continue
		}

		baseName := strings.TrimSuffix(entry.Name(), ".xls")
		// 移除 _course 或 _activity 后缀
		key := baseName
		if strings.HasSuffix(baseName, "_course") {
			key = strings.TrimSuffix(baseName, "_course")
		} else if strings.HasSuffix(baseName, "_activity") {
			key = strings.TrimSuffix(baseName, "_activity")
		}

		filePath := filepath.Join(p.InputDir, entry.Name())
		fileGroups[key] = append(fileGroups[key], filePath)
	}

	var results []ParseResult
	for _, files := range fileGroups {
		for _, file := range files {
			result := p.ParseFile(file)
			results = append(results, result)

			if result.Success {
				fmt.Printf("✓ 解析完成: %s\n", filepath.Base(file))
				if result.CourseCSV != "" {
					fmt.Printf("  课程: %s\n", filepath.Base(result.CourseCSV))
				}
				if result.ActivityCSV != "" {
					fmt.Printf("  环节: %s\n", filepath.Base(result.ActivityCSV))
				}
			} else {
				fmt.Printf("✗ 解析失败: %s - %s\n", filepath.Base(file), result.Error)
			}
		}
	}

	return results, nil
}

// ParseFile 解析单个文件
func (p *ListParser) ParseFile(filePath string) ParseResult {
	result := ParseResult{
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

	// 判断是课程还是环节文件
	fileName := filepath.Base(filePath)
	isActivity := strings.Contains(fileName, "_activity")
	isCourse := strings.Contains(fileName, "_course")

	// 如果没有明确标记，从内容判断
	if !isActivity && !isCourse {
		if strings.Contains(htmlContent, "环节") && !strings.Contains(htmlContent, "上课班级代码") {
			isActivity = true
		} else {
			isCourse = true
		}
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = fmt.Sprintf("解析 HTML 失败: %v", err)
		return result
	}

	// 生成输出文件名
	baseName := strings.TrimSuffix(fileName, ".xls")

	if isCourse {
		courses := p.parseCourses(doc)
		if len(courses) > 0 {
			outputFile := filepath.Join(p.OutputDir, baseName+".csv")
			if err := p.writeCourseCSV(outputFile, courses); err != nil {
				result.Error = fmt.Sprintf("写入 CSV 失败: %v", err)
				return result
			}
			result.CourseCSV = outputFile
		}
	}

	if isActivity {
		activities := p.parseActivities(doc)
		if len(activities) > 0 {
			outputFile := filepath.Join(p.OutputDir, baseName+".csv")
			if err := p.writeActivityCSV(outputFile, activities); err != nil {
				result.Error = fmt.Sprintf("写入 CSV 失败: %v", err)
				return result
			}
			result.ActivityCSV = outputFile
		}
	}

	result.Success = true
	return result
}

// Course 课程
type Course struct {
	Name     string
	Teacher  string
	Weeks    string
	Session  string
	Location string
}

// parseCourses 解析课程表
func (p *ListParser) parseCourses(doc *goquery.Document) []Course {
	var courses []Course

	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// 跳过布局表格
		style := table.AttrOr("style", "")
		if strings.Contains(style, "border:0px") && strings.Contains(style, "clear:left") {
			return
		}

		// 确认是课程表 - 检查表头或内容
		firstRowText := strings.TrimSpace(table.Find("tr").First().Text())
		if !strings.Contains(firstRowText, "上课班级代码") && !strings.Contains(firstRowText, "课程") {
			return
		}

		// 解析每一行（跳过表头）
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			// 跳过表头行
			if j == 0 {
				return
			}

			cells := row.Find("td")
			if cells.Length() < 10 {
				return
			}

			courseName := cleanCourseName(cells.Eq(1).Text())
			teacher := cleanCourseName(cells.Eq(5).Text())
			timeLocStr := strings.TrimSpace(cells.Eq(9).Text())

			if timeLocStr == "" {
				// 无时间地点（如网络课）
				courses = append(courses, Course{
					Name:    courseName,
					Teacher: teacher,
					Weeks:   "",
					Session: "",
					Location: "",
				})
				return
			}

			// 按中文分号拆分多节课
			sessions := strings.Split(timeLocStr, "；")
			for _, sessionStr := range sessions {
				week, session, location := parseTimeLocation(sessionStr)
				courses = append(courses, Course{
					Name:     courseName,
					Teacher:  teacher,
					Weeks:    week,
					Session:  session,
					Location: location,
				})
			}
		})
	})

	return courses
}

// Activity 环节
type Activity struct {
	Name    string
	Weeks   string
	Teacher string
}

// parseActivities 解析环节表
func (p *ListParser) parseActivities(doc *goquery.Document) []Activity {
	var activities []Activity

	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// 跳过布局表格
		style := table.AttrOr("style", "")
		if strings.Contains(style, "border:0px") && strings.Contains(style, "clear:left") {
			return
		}

		// 确认是环节表 - 检查表头或内容
		firstRowText := strings.TrimSpace(table.Find("tr").First().Text())
		if !strings.Contains(firstRowText, "环节") {
			return
		}

		// 解析每一行（跳过表头）
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			// 跳过表头行
			if j == 0 {
				return
			}

			cells := row.Find("td")
			if cells.Length() < 6 {
				return
			}

			activityName := cleanCourseName(cells.Eq(0).Text())
			week := strings.TrimSpace(cells.Eq(5).Text())
			week = parseWeeks(week)
			// 指导教师在第6列（最后一列），索引为6或cells.Length()-1
			teacherCol := cells.Length() - 1
			if teacherCol < 6 {
				teacherCol = 6
			}
			teacher := cleanCourseName(cells.Eq(teacherCol).Text())

			activities = append(activities, Activity{
				Name:    activityName,
				Weeks:   week,
				Teacher: teacher,
			})
		})
	})

	return activities
}

// writeCourseCSV 写入课程 CSV
func (p *ListParser) writeCourseCSV(filePath string, courses []Course) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入 BOM
	file.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	writer.Write([]string{"课程", "教师", "周次", "节次", "地点"})

	// 写入数据
	for _, c := range courses {
		writer.Write([]string{c.Name, c.Teacher, c.Weeks, c.Session, c.Location})
	}

	return writer.Error()
}

// writeActivityCSV 写入环节 CSV
func (p *ListParser) writeActivityCSV(filePath string, activities []Activity) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入 BOM
	file.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	writer.Write([]string{"环节", "周次", "指导老师"})

	// 写入数据
	for _, a := range activities {
		writer.Write([]string{a.Name, a.Weeks, a.Teacher})
	}

	return writer.Error()
}

// cleanCourseName 清理课程名
func cleanCourseName(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.Index(name, "]")
	if idx != -1 {
		return strings.TrimSpace(name[idx+1:])
	}
	return name
}

// parseWeeks 解析周次
func parseWeeks(weekStr string) string {
	weekStr = strings.TrimSpace(weekStr)
	weekStr = strings.ReplaceAll(weekStr, "周", "")

	parity := ""
	if strings.Contains(weekStr, "(单)") || strings.Contains(weekStr, "（单）") {
		parity = "单"
		weekStr = strings.ReplaceAll(weekStr, "(单)", "")
		weekStr = strings.ReplaceAll(weekStr, "（单）", "")
	} else if strings.Contains(weekStr, "(双)") || strings.Contains(weekStr, "（双）") {
		parity = "双"
		weekStr = strings.ReplaceAll(weekStr, "(双)", "")
		weekStr = strings.ReplaceAll(weekStr, "（双）", "")
	}

	weekStr = strings.TrimSpace(weekStr)
	return weekStr + parity
}

// parseTimeLocation 解析时间地点字符串
// 输入: "1-11周(单) 五[3-4] 三教楼106(172)"
func parseTimeLocation(raw string) (week, session, location string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ""
	}

	parts := strings.Fields(raw)
	if len(parts) >= 1 {
		// 解析周次
		w := parts[0]
		parity := ""
		if strings.Contains(w, "(单)") || strings.Contains(w, "（单）") {
			parity = "单"
			w = strings.ReplaceAll(w, "(单)", "")
			w = strings.ReplaceAll(w, "（单）", "")
		} else if strings.Contains(w, "(双)") || strings.Contains(w, "（双）") {
			parity = "双"
			w = strings.ReplaceAll(w, "(双)", "")
			w = strings.ReplaceAll(w, "（双）", "")
		}
		w = strings.ReplaceAll(w, "周", "")
		week = w + parity

		// 解析节次
		if len(parts) >= 2 {
			session = normalizeSession(parts[1], parity)
		}

		// 解析地点
		if len(parts) >= 3 {
			loc := parts[2]
			idx := strings.Index(loc, "(")
			if idx == -1 {
				idx = strings.Index(loc, "（")
			}
			if idx != -1 {
				loc = loc[:idx]
			}
			location = loc
		}
	}

	return
}

// normalizeSession 标准化节次格式
// 如: "五[3-4]" -> "五[3-4]"
func normalizeSession(session, parity string) string {
	// 移除已有的"节"字
	session = strings.ReplaceAll(session, "节", "")
	if parity != "" {
		return session + parity
	}
	return session
}
