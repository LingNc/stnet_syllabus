这确实是一个非常经典的爬虫/解析器开发场景。在 Go 语言中，处理这类“伪 XLS”（实质为嵌套 HTML）的最佳实践是使用类似 jQuery 的 DOM 解析库，比如大名鼎鼎的 `github.com/PuerkitoBio/goquery`。

下面我为你编写了一段 Go 代码。这段代码的核心逻辑是剥离所有冗余的 CSS 和隐藏标签，精准定位到包含排课数据的 DOM 节点（比如 `<div class="div1">` 和 `<tbody>` 里的 `<tr>` ），并输出精简后的纯净 HTML 结构，完美复刻刚刚我为你演示的精简效果。

你可以先通过 `go get github.com/PuerkitoBio/goquery` 安装依赖。

```go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ParseGridHTML 解析二维网格课表并精简
func ParseGridHTML(htmlContent string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("")
	fmt.Println("<table>")

	// 1. 提取头部学生信息
	doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
		fmt.Println("\t<tr>")
		s.Find("td").Each(func(j int, td *goquery.Selection) {
			text := strings.TrimSpace(td.Text())
			if text != "" {
				// 替换掉HTML中的不间断空格 &ensp;
				cleanText := strings.ReplaceAll(text, " ", " ")
				fmt.Printf("\t\t<th>%s</th>\n", cleanText)
			}
		})
		fmt.Println("\t</tr>")
	})
	fmt.Println("</table>\n")

	fmt.Println("<table>")
	// 2. 解析课表主体 (带有 id='mytable' 的 table)
	doc.Find("table#mytable").Find("tr").Each(func(i int, row *goquery.Selection) {
		fmt.Println("\t<tr>")
		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			// 检查是否有实际课程内容 (class='div1')
			courseNode := cell.Find("div.div1")
			if courseNode.Length() > 0 {
				// 获取文本并清理多余的换行符
				rawText := strings.TrimSpace(courseNode.Text())
				cleanText := cleanWhitespace(rawText)
				fmt.Printf("\t\t<td>%s</td>\n", cleanText)
			} else if cell.Find("div.div_nokb").Length() > 0 {
				// 无课时间段
				fmt.Println("\t\t<td></td>")
			} else {
				// 表头或侧边栏节次
				text := cleanWhitespace(cell.Text())
				if text != "" {
					fmt.Printf("\t\t<td>%s</td>\n", text)
				}
			}
		})
		fmt.Println("\t</tr>")
	})
	fmt.Println("</table>")
}

// ParseListHTML 解析列表形课表并精简
func ParseListHTML(htmlContent string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n")

	// 1. 提取学生基础信息
	infoDiv := doc.Find("div[group='group']").First()
	fmt.Printf("<div>%s</div>\n\n", cleanWhitespace(infoDiv.Text()))

	// 2. 遍历页面中的所有数据表格 (常规课表和环节课表)
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// 跳过作为布局的无边框表格
		if table.AttrOr("style", "") == "clear:left;width:256mm;font-size:12px;border:0px" {
			return
		}

		fmt.Println("<table>")
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			fmt.Println("\t<tr>")
			row.Find("td").Each(func(k int, cell *goquery.Selection) {
				text := cleanWhitespace(cell.Text())
				if j == 0 {
					// 表头部分
					fmt.Printf("\t\t<th>%s</th>\n", text)
				} else {
					// 数据行
					if text != "" {
						fmt.Printf("\t\t<td>%s</td>\n", text)
					}
				}
			})
			fmt.Println("\t</tr>")
		})
		fmt.Println("</table>\n")
	})
}

// 辅助函数：清理节点中提取出来的冗余空格和换行
func cleanWhitespace(str string) string {
	str = strings.ReplaceAll(str, "\n", " ")
	str = strings.ReplaceAll(str, "\t", "")
	str = strings.ReplaceAll(str, " ", " ") // 清理 &ensp;
	// 将多个连续空格替换为分隔符或者单空格
	return strings.Join(strings.Fields(str), " | ")
}

```

### 代码原理解析

1.
**`goquery.NewDocumentFromReader`**：这是核心入口，它会帮你把凌乱、不规范的教务 HTML 代码  自动构建成一棵标准的 DOM 树，忽略那些没有闭合的标签。


2. **选择器过滤**：
* 在处理二维表时，我们直接锁定 `<table id="mytable">` 来避开外围的布局表格，然后通过判断单元格内部是否含有 `<div class="div1">`  (有课) 还是 `<div class="div_nokb">`  (无课) 来决定提取的数据。


* 在处理列表时，通过过滤特定的行内样式 (`border:0px` ) 剔除掉布局用的外层包裹 table，直捣黄龙提取真实的课程清单 。




3.
**`cleanWhitespace` 辅助函数**：原文件大量使用了 `<br>` 和 `&ensp;` 进行排版 。通过 `strings.Fields` 我们把那些被打碎的文本块重新拼装，并用 `|` 作为分隔符，这就为接下来转换为 CSV 铺平了道路。



这个精简版的 HTML 生成后，由于其高度规范化的结构，再往 CSV 转换就只是一层简单的循环组装逻辑了。需要我接着把这段代码补充完整，直接生成可以导入 Excel 的标准 CSV 文件吗？



在数据处理和清洗的过程中，在进入较重的 DOM 解析（如使用 `goquery` 构建整个节点树）之前，加一层快速的“路由”或“筛选”机制是非常好的工程习惯。

针对这两种教务导出的 HTML 文件，我们完全不需要深度解析就能区分它们。通过观察你提供的原始 HTML，我们可以抓取它们各自独有的**强特征字符串**来进行快速判别：

*
**列表格式的独有特征**：带有 `pagetitle="pagetitle"` 属性的 `<div>` 标签 ，以及 `上课班级代码` 这样的专属表头 。


*
**二维表格式的独有特征**：带有 `id='mytable'` 的 `<table>` 标签 ，以及用于表示无课时间段的独有类名 `div_nokb` 。



我们可以使用 Go 标准库中的 `strings.Contains` 来做一个极其轻量且快速的判定函数。

### 快速判别与筛选代码

下面是一段精简的 Go 代码，利用特征匹配来筛出列表文件，并直接丢弃二维表文件：

```go
package main

import (
	"fmt"
	"strings"
)

// IsListFormat 通过特征字符串快速判断是否为“列表”格式
func IsListFormat(htmlContent string) bool {
	// 列表格式的强特征：带有 pagetitle 属性或包含特定的表头
	hasPageTitle := strings.Contains(htmlContent, `pagetitle="pagetitle"`)
	hasSpecificHeader := strings.Contains(htmlContent, "上课班级代码")

	return hasPageTitle || hasSpecificHeader
}

// IsGridFormat 判断是否为“二维表”格式（可选，用于双重校验）
func IsGridFormat(htmlContent string) bool {
	// 二维表格式的强特征：包含特定的 id 或 class
	hasMyTable := strings.Contains(htmlContent, `id='mytable'`) || strings.Contains(htmlContent, `id="mytable"`)
	hasNoKb := strings.Contains(htmlContent, "div_nokb")

	return hasMyTable || hasNoKb
}

func main() {
	// 假设这是你从目录中批量读取到的两个文件内容
	gridHTML := `<!DOCTYPE HTML... <table class='table' id='mytable' name='mytable'> ...`
	listHTML := `<!DOCTYPE HTML... <div pagetitle="pagetitle" ...>上课班级代码...`

	// 模拟文件遍历过程
	files := map[string]string{
		"file1_grid.txt": gridHTML,
		"file2_list.txt": listHTML,
	}

	for filename, content := range files {
		if IsListFormat(content) {
			fmt.Printf("[保留] %s 是列表格式，准备进入清洗和提取流程。\n", filename)
			// TODO: 在这里调用将列表 HTML 转换为 CSV 的解析函数
		} else {
			fmt.Printf("[丢弃] %s 是二维表（或其他）格式，跳过。\n", filename)
		}
	}
}

```

### 为什么这样写更好？

1. **极低的性能开销**：`strings.Contains` 只是在内存中进行底层的字节比对，速度远快于正则匹配，更是秒杀将其载入 `goquery` 构建 DOM 树的耗时。
2. **高容错性**：即使教务系统的页面样式发生了微调，只要核心的数据表头（如“上课班级代码”）或页面标题标识（`pagetitle`）没有彻底重构，这个判别逻辑就依然坚挺。

既然现在我们已经能够精准地将“列表”格式的文件单独拎出来了，需要我接着为你编写将这个筛选出来的列表 HTML 直接无损导出为标准 `.csv` 文件的 Go 代码吗？


这个数据清洗和转换逻辑非常清晰！你观察得非常准确，教务系统导出的数据里，同一个课程一周上多次确实是用中文分号 `；` 分隔的 ，而且包含了很多像课程编号、校区代码这样的冗余后缀。

下面是为你量身定制的 Go 语言代码。这段代码会精准提取“列表”格式的课表，完成列的过滤、课程编号的去除，并对“上课时间地点”进行按你要求的高级拆分与格式化。最后，它会将处理好的数据生成标准的 CSV 结构。

### Go 解析与清洗代码

```go
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// 去除中括号包裹的编号，例如 "[1707292]项目管理A" -> "项目管理A"
func cleanName(s string) string {
	s = strings.TrimSpace(s)
	idx := strings.Index(s, "]")
	if idx != -1 {
		return strings.TrimSpace(s[idx+1:])
	}
	return s
}

// 解析并格式化时间地点字符串
// 输入: "1-11周(单) 五[3-4] 三教楼106(172)"
// 输出周次: "1-11", 节次: "五[3-4]单", 地点: "三教楼106"
func parseTimeLocation(raw string) (week, session, location string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ""
	}

	// 按照空格拆分，通常会分为 2 到 3 个部分
	parts := strings.Fields(raw)
	if len(parts) >= 1 {
		w := parts[0]
		parity := ""
		// 提取单双周标识
		if strings.Contains(w, "(单)") {
			parity = "单"
			w = strings.Replace(w, "(单)", "", 1)
		} else if strings.Contains(w, "(双)") {
			parity = "双"
			w = strings.Replace(w, "(双)", "", 1)
		}
		// 去除“周”字
		week = strings.Replace(w, "周", "", 1)

		if len(parts) >= 2 {
			// 拼接节次和单双周（例如：五[3-4] + 单）
			session = parts[1] + parity
		}

		if len(parts) >= 3 {
			locPart := parts[2]
			// 去除地点后面的括号及里面的内容 (例如：三教楼106(172) -> 三教楼106)
			idx := strings.Index(locPart, "(")
			if idx == -1 {
				idx = strings.Index(locPart, "（") // 兼容中文括号
			}
			if idx != -1 {
				locPart = locPart[:idx]
			}
			location = locPart
		}
	}
	return
}

func ProcessListToCSV(htmlContent string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		log.Fatal(err)
	}

	var courseRows [][]string
	// 添加课程表头
	courseRows = append(courseRows, []string{"课程", "教师", "周次", "节次", "地点"})

	var activityRows [][]string
	// 添加环节表头
	activityRows = append(activityRows, []string{"环节", "周次", "指导老师"})

	// 遍历所有数据表格
	doc.Find("table").Each(func(tableIdx int, table *goquery.Selection) {
		// 跳过作为布局的无边框表格
		if table.AttrOr("style", "") == "clear:left;width:256mm;font-size:12px;border:0px" {
			return
		}

		// 判断是课程表还是环节表：通过检查第一个 th 的文本
		firstHeader := strings.TrimSpace(table.Find("thead td").First().Text())

		isCourseTable := strings.Contains(firstHeader, "上课班级代码")
		isActivityTable := strings.Contains(firstHeader, "环节")

		table.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() == 0 {
				return
			}

			if isCourseTable && cells.Length() >= 11 {
				// 提取课程表所需的列 (索引 2: 课程, 6: 教师, 10: 时间地点)
				courseName := cleanName(cells.Eq(2).Text())
				teacher := cleanName(cells.Eq(6).Text()) // 教师名字通常也有 [编号]
				timeLocStr := strings.TrimSpace(cells.Eq(10).Text())

				// 如果没有时间地点（比如网络课），直接存一条空记录
				if timeLocStr == "" {
					courseRows = append(courseRows, []string{courseName, teacher, "", "", ""})
					return
				}

				// 按照中文分号拆分一周内的多次课
				sessions := strings.Split(timeLocStr, "；")
				for _, sessionStr := range sessions {
					week, session, location := parseTimeLocation(sessionStr)
					courseRows = append(courseRows, []string{courseName, teacher, week, session, location})
				}
			} else if isActivityTable && cells.Length() >= 8 {
				// 提取环节表所需的列 (由于“环节”列使用了 colspan=2，goquery eq 索引按实际出现的 td 算)
				// 索引 0: 环节(跨两列), 5: 周次, 7: 指导教师
				activityName := cleanName(cells.Eq(0).Text())
				week := strings.TrimSpace(cells.Eq(5).Text())
				teacher := cleanName(cells.Eq(7).Text())

				activityRows = append(activityRows, []string{activityName, week, teacher})
			}
		})
	})

	// 打印输出 CSV 结果
	fmt.Println("=== 课程表 (Courses.csv) ===")
	writeCSVToConsole(courseRows)

	fmt.Println("\n=== 环节表 (Activities.csv) ===")
	writeCSVToConsole(activityRows)
}

// 辅助函数：将二维数组打印为标准的 CSV 格式
func writeCSVToConsole(data [][]string) {
	writer := csv.NewWriter(os.Stdout)
	for _, record := range data {
		if err := writer.Write(record); err != nil {
			log.Fatalln("error writing record to csv:", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatal(err)
	}
}

func main() {
    // 假设 htmlData 是你读取到的列表页面的 HTML 字符串
	// ProcessListToCSV(htmlData)
}

```

### 代码处理细节说明：

1.
**课程编号清洗 (`cleanName`)**：使用 `strings.Index` 寻找 `]` 符号，如果存在，直接截取后面的文本。无论是 `[1707292]项目管理A` 还是教师列的 `[2020001]王曼曼` ，它都能干净地提取出纯文本。


2.
**多节课拆分**：在解析课程行时，拿到“时间地点”文本后，第一时间使用 `strings.Split(timeLocStr, "；")` 切割 。然后循环遍历切出来的数组，有几次课就向 `courseRows` 里 `append` 几行，完美实现了你的“行复制拆分”需求。


3. **时间地点精确切片 (`parseTimeLocation`)**：
* **周次与单双周**：先利用 `strings.Fields` 按空格分块。第一块里，如果检测到 `(单)`，就在变量中记录“单”，然后把它从源字符串里剔除。接着剔除“周”字，剩下的就是纯粹的 `<开始周>-<结束周>`。
* **节次组合**：直接拿第二块（例如 `五[3-4]`）加上刚刚识别出的单双周状态（例如 `单` 或留空），拼接成 `五[3-4]单`。
*
**地点剥离**：拿第三块（例如 `三教楼106(172)` ），寻找左括号 `(`（同时兼容了中文括号 `（` 以防万一），切掉从括号开始到末尾的所有字符，只保留干净的楼宇房间号。





需要我帮你把这段代码加上文件读写的逻辑，让你能直接读取本地的 `xls内容.txt` 文件并生成两个 `.csv` 文件吗？