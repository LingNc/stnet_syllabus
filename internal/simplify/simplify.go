// Package simplify 处理 HTML 内容精简
package simplify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Simplifier HTML 精简器
type Simplifier struct {
	InputDir  string
	OutputDir string
}

// NewSimplifier 创建精简器
func NewSimplifier(inputDir, outputDir string) *Simplifier {
	return &Simplifier{
		InputDir:  inputDir,
		OutputDir: outputDir,
	}
}

// SimplifyFile 精简单个 HTML 文件
func (s *Simplifier) SimplifyFile(inputPath, outputPath string) error {
	// 读取文件内容
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 尝试将 GBK 转换为 UTF-8
	htmlContent, err := decodeGBK(content)
	if err != nil {
		// 如果转换失败，尝试直接使用原内容
		htmlContent = string(content)
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
			fmt.Printf("错误: 精简文件 %s 失败: %v\n", entry.Name(), err)
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

// decodeGBK 将 GBK 编码的字节转换为 UTF-8 字符串
func decodeGBK(data []byte) (string, error) {
	decoder := simplifiedchinese.GBK.NewDecoder()
	result, _, err := transform.Bytes(decoder, data)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
