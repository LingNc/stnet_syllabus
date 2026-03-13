// Package simplify 处理 HTML 内容精简
package simplify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Simplifier HTML 精简器
type Simplifier struct {
	InputDir  string
	OutputDir string
	LogFunc   func(string, ...interface{}) // 外部日志函数
}

// NewSimplifier 创建精简器
func NewSimplifier(inputDir, outputDir string, logFunc ...func(string, ...interface{})) *Simplifier {
	s := &Simplifier{
		InputDir:  inputDir,
		OutputDir: outputDir,
	}
	if len(logFunc) > 0 {
		s.LogFunc = logFunc[0]
	}
	return s
}

// SimplifyFile 精简单个 HTML 文件
func (s *Simplifier) SimplifyFile(inputPath, outputPath string) error {
	// 读取文件内容
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 检测是否是二进制格式（xlsx等）
	if isBinaryContent(content) {
		return fmt.Errorf("文件是二进制格式（可能是xlsx），不是HTML格式，请转换为HTML后再处理")
	}

	// 尝试将 GBK 转换为 UTF-8
	// 首先检测是否已经是 UTF-8
	var htmlContent string
	if isValidUTF8(content) {
		// 已经是 UTF-8，直接使用
		htmlContent = string(content)
	} else {
		// 尝试 GBK 解码
		decoded, err := decodeGBK(content)
		if err != nil {
			// 解码失败，尝试直接使用原内容
			htmlContent = string(content)
		} else {
			htmlContent = decoded
		}
	}

	// 检查是否是有效的HTML格式
	if !isValidHTML(htmlContent) {
		return fmt.Errorf("文件不是有效的HTML格式（缺少必要的HTML标签）")
	}

	// 判断文件格式
	format := detectFormat(htmlContent)

	var simplified string
	switch format {
	case "list":
		simplified = simplifyListHTML(htmlContent)
	case "2d":
		simplified = simplify2DHTML(htmlContent)
	default:
		// 尝试通用精简
		simplified = simplifyGeneric(htmlContent)
	}

	// 确保输出目录存在
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 写入精简后的文件
	if err := os.WriteFile(outputPath, []byte(simplified), 0644); err != nil {
		return fmt.Errorf("写入精简文件失败: %w", err)
	}

	return nil
}

// Process 处理目录中的所有文件
func (s *Simplifier) Process() error {
	// 确保输出目录存在
	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 读取输入目录
	entries, err := os.ReadDir(s.InputDir)
	if err != nil {
		return fmt.Errorf("读取输入目录失败: %w", err)
	}

	processed := 0
	errors := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理 .xls 文件
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".xls") {
			continue
		}

		inputPath := filepath.Join(s.InputDir, entry.Name())
		outputPath := filepath.Join(s.OutputDir, entry.Name())

		if err := s.SimplifyFile(inputPath, outputPath); err != nil {
			errMsg := fmt.Sprintf("精简文件 %s 失败: %v", entry.Name(), err)
			fmt.Printf("错误: %s\n", errMsg)
			if s.LogFunc != nil {
				s.LogFunc("%s", errMsg)
			}
			errors++
			continue
		}

		fmt.Printf("已精简: %s\n", entry.Name())
		processed++
	}

	fmt.Printf("\n精简完成: 成功 %d, 失败 %d\n", processed, errors)
	return nil
}

// detectFormat 检测 HTML 格式
func detectFormat(htmlContent string) string {
	if strings.Contains(htmlContent, `pagetitle="pagetitle"`) ||
		strings.Contains(htmlContent, "上课班级代码") {
		return "list"
	}
	if strings.Contains(htmlContent, `id='mytable'`) ||
		strings.Contains(htmlContent, `id="mytable"`) ||
		strings.Contains(htmlContent, "div_nokb") {
		return "2d"
	}
	return "unknown"
}

// simplifyListHTML 精简列表格式 HTML
func simplifyListHTML(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var result strings.Builder
	result.WriteString("<!DOCTYPE html>\n<html>\n<body>\n")

	// 提取学生基础信息和学期
	infoDiv := doc.Find("div[group='group']").First()
	pageTitleDiv := doc.Find("div[pagetitle='pagetitle']").First()

	if infoDiv.Length() > 0 {
		text := cleanWhitespace(infoDiv.Text())
		result.WriteString(fmt.Sprintf("<div class=\"info\">%s</div>\n\n", text))
	}

	// 提取学期信息
	if pageTitleDiv.Length() > 0 {
		text := cleanWhitespace(pageTitleDiv.Text())
		result.WriteString(fmt.Sprintf("<!-- SEMESTER: %s -->\n", text))
	}

	// 遍历所有数据表格
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// 跳过布局表格
		style := table.AttrOr("style", "")
		if strings.Contains(style, "border:0px") && strings.Contains(style, "clear:left") {
			return
		}

		result.WriteString("<table>\n")

		// 判断表头
		firstHeader := strings.TrimSpace(table.Find("thead td, th").First().Text())
		isCourseTable := strings.Contains(firstHeader, "上课班级代码")
		isActivityTable := strings.Contains(firstHeader, "环节")

		if isCourseTable {
			result.WriteString("<!-- TYPE: COURSE -->\n")
		} else if isActivityTable {
			result.WriteString("<!-- TYPE: ACTIVITY -->\n")
		}

		// 处理表格行
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			result.WriteString("<tr>\n")

			row.Find("td, th").Each(func(k int, cell *goquery.Selection) {
				text := cleanWhitespace(cell.Text())
				tag := "td"
				if cell.Is("th") || j == 0 {
					tag = "th"
				}

				if text != "" {
					result.WriteString(fmt.Sprintf("<%s>%s</%s>\n", tag, text, tag))
				}
			})

			result.WriteString("</tr>\n")
		})

		result.WriteString("</table>\n\n")
	})

	result.WriteString("</body>\n</html>")
	return result.String()
}

// simplify2DHTML 精简二维表格式 HTML
func simplify2DHTML(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var result strings.Builder
	result.WriteString("<!DOCTYPE html>\n<html>\n<body>\n")

	// 提取学生信息（第一个表格）
	result.WriteString("<table class=\"info\">\n")
	doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
		result.WriteString("<tr>\n")
		s.Find("td").Each(func(j int, td *goquery.Selection) {
			text := strings.TrimSpace(td.Text())
			if text != "" {
				cleanText := strings.ReplaceAll(text, "\u2002", " ") // 替换 &ensp;
				result.WriteString(fmt.Sprintf("<th>%s</th>\n", cleanWhitespace(cleanText)))
			}
		})
		result.WriteString("</tr>\n")
	})
	result.WriteString("</table>\n\n")

	// 提取学期信息（从隐藏字段）- xn是学年，xq_m是学期码(0或1)，直接拼接
	xn := doc.Find("input#xn").AttrOr("value", "")
	xq := doc.Find("input#xq_m").AttrOr("value", "")
	if xn != "" && xq != "" {
		// 直接拼接：如 2025 + 1 = 20251
		semesterCode := xn + xq
		semesterText := fmt.Sprintf("%s-%d学年第%d学期", xn, mustInt(xn)+1, mustInt(xq)+1)
		result.WriteString(fmt.Sprintf("<!-- SEMESTER: %s -->\n", semesterText))
		result.WriteString(fmt.Sprintf("<!-- SEMESTER_CODE: %s -->\n", semesterCode))
	}

	// 提取课表主体
	result.WriteString("<!-- TYPE: 2D_TABLE -->\n")
	result.WriteString("<table class=\"schedule\">\n")

	doc.Find("table#mytable, table[name='mytable']").Find("tr").Each(func(i int, row *goquery.Selection) {
		result.WriteString("<tr>\n")

		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			// 检查是否有课程内容
			courseNode := cell.Find("div.div1")
			if courseNode.Length() > 0 {
				rawText := strings.TrimSpace(courseNode.Text())
				cleanText := cleanWhitespace(rawText)
				result.WriteString(fmt.Sprintf("<td class=\"course\">%s</td>\n", cleanText))
			} else if cell.Find("div.div_nokb").Length() > 0 {
				// 无课
				result.WriteString("<td></td>\n")
			} else {
				// 表头或节次
				text := cleanWhitespace(cell.Text())
				if text != "" {
					result.WriteString(fmt.Sprintf("<td>%s</td>\n", text))
				}
			}
		})

		result.WriteString("</tr>\n")
	})

	result.WriteString("</table>\n")

	// 提取环节信息（从底部注释）
	// 环节数据通常在课表后面的 div 中，格式为：注1、[编号]环节名称 第X-X周 指导老师
	doc.Find("div").Each(func(i int, div *goquery.Selection) {
		text := strings.TrimSpace(div.Text())
		// 检查是否包含环节注释标记
		if strings.Contains(text, "注1") || strings.Contains(text, "注2") || strings.Contains(text, "注：") {
			cleanText := cleanWhitespace(text)
			if cleanText != "" {
				result.WriteString(fmt.Sprintf("<!-- ACTIVITIES: %s -->\n", cleanText))
			}
		}
	})

	result.WriteString("</body>\n</html>")
	return result.String()
}

// simplifyGeneric 通用精简
func simplifyGeneric(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	var result strings.Builder
	result.WriteString("<!DOCTYPE html>\n<html>\n<body>\n")

	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		result.WriteString("<table>\n")
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			result.WriteString("<tr>\n")
			row.Find("td, th").Each(func(k int, cell *goquery.Selection) {
				text := cleanWhitespace(cell.Text())
				if text != "" {
					result.WriteString(fmt.Sprintf("<td>%s</td>\n", text))
				}
			})
			result.WriteString("</tr>\n")
		})
		result.WriteString("</table>\n\n")
	})

	result.WriteString("</body>\n</html>")
	return result.String()
}

// cleanWhitespace 清理空白字符
func cleanWhitespace(str string) string {
	str = strings.ReplaceAll(str, "\n", " ")
	str = strings.ReplaceAll(str, "\t", " ")
	str = strings.ReplaceAll(str, "\u2002", " ") // &ensp;
	str = strings.ReplaceAll(str, "\u00A0", " ") // &nbsp;
	// 合并多个空格
	fields := strings.Fields(str)
	return strings.Join(fields, " ")
}

// mustInt 将字符串转换为整数，失败返回 0
func mustInt(s string) int {
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

// decodeGBK 将 GBK 编码的字节转换为 UTF-8 字符串
func decodeGBK(data []byte) (string, error) {
	decoder := simplifiedchinese.GBK.NewDecoder()
	result, _, err := transform.Bytes(decoder, data)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// isValidUTF8 检测内容是否是有效的 UTF-8 编码
// 使用标准库的 utf8.Valid 进行严格检测
func isValidUTF8(data []byte) bool {
	return utf8.Valid(data)
}

// isBinaryContent 检测内容是否是二进制格式（如xlsx）
func isBinaryContent(data []byte) bool {
	// 检查常见的二进制文件魔数
	// xlsx 文件实际上是 ZIP 格式，以 PK 开头
	if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
		return true
	}
	// 检查是否包含大量 null 字节（二进制文件特征）
	nullCount := 0
	checkLen := len(data)
	if checkLen > 1024 {
		checkLen = 1024
	}
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			nullCount++
		}
	}
	// 如果前1KB中有超过10个null字节，认为是二进制
	return nullCount > 10
}

// isValidHTML 检查内容是否是有效的HTML格式
func isValidHTML(content string) bool {
	contentLower := strings.ToLower(content)
	// 必须包含基本的HTML标签
	hasHTML := strings.Contains(contentLower, "<html") ||
		strings.Contains(contentLower, "<!doctype html")
	hasBody := strings.Contains(contentLower, "<body")
	// 必须包含表格（课表应该有表格）
	hasTable := strings.Contains(contentLower, "<table")

	return hasHTML && hasBody && hasTable
}
