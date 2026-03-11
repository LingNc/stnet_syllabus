// Package weekly 处理周次切片
package weekly

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Slicer 周次切片器
type Slicer struct {
	MachineCSV string
	OutputDir  string
	TotalWeeks int
}

// WeekResult 周结果
type WeekResult struct {
	Week    int
	File    string
	Success bool
	Error   string
}

// NewSlicer 创建切片器
func NewSlicer(machineCSV, outputDir string, totalWeeks int) *Slicer {
	return &Slicer{
		MachineCSV: machineCSV,
		OutputDir:  outputDir,
		TotalWeeks: totalWeeks,
	}
}

// Process 处理所有周次
func (s *Slicer) Process() ([]WeekResult, error) {
	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 读取机器可读表
	data, err := s.readMachineCSV()
	if err != nil {
		return nil, fmt.Errorf("读取机器可读表失败: %w", err)
	}

	var results []WeekResult

	// 为每周生成切片
	for week := 1; week <= s.TotalWeeks; week++ {
		result := s.generateWeekSlice(week, data)
		results = append(results, result)

		if result.Success {
			fmt.Printf("✓ 生成第 %d 周切片: %s\n", week, filepath.Base(result.File))
		} else {
			fmt.Printf("✗ 第 %d 周切片失败: %s\n", week, result.Error)
		}
	}

	return results, nil
}

// MachineData 机器可读数据结构
type MachineData struct {
	Headers []string
	Rows    [][]string
}

// readMachineCSV 读取机器可读 CSV
func (s *Slicer) readMachineCSV() (*MachineData, error) {
	file, err := os.Open(s.MachineCSV)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("数据为空")
	}

	return &MachineData{
		Headers: records[0],
		Rows:    records[1:],
	}, nil
}

// generateWeekSlice 生成单周切片
func (s *Slicer) generateWeekSlice(week int, data *MachineData) WeekResult {
	result := WeekResult{
		Week: week,
	}

	// 生成输出文件路径
	outputFile := filepath.Join(s.OutputDir, fmt.Sprintf("free_week_%d.csv", week))
	result.File = outputFile

	// 创建文件
	file, err := os.Create(outputFile)
	if err != nil {
		result.Error = fmt.Sprintf("创建文件失败: %v", err)
		return result
	}
	defer file.Close()

	// 写入 BOM
	file.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	writer.Write(data.Headers)

	// 处理每一行
	for _, row := range data.Rows {
		newRow := []string{row[0]} // 节次列保持不变

		for colIdx := 1; colIdx < len(row); colIdx++ {
			cell := row[colIdx]
			// 提取该周有空闲的人
			names := extractNamesForWeek(cell, week)
			newRow = append(newRow, strings.Join(names, "、"))
		}

		writer.Write(newRow)
	}

	result.Success = true
	return result
}

// extractNamesForWeek 从单元格提取指定周有空闲的人
// 格式: "姓名[1,2,3] 姓名2[2,4]"
func extractNamesForWeek(cell string, week int) []string {
	if cell == "" {
		return []string{}
	}

	var names []string

	// 正则匹配: 姓名[周次列表]
	re := regexp.MustCompile(`([^\s\[]+)\[([^\]]+)\]`)
	matches := re.FindAllStringSubmatch(cell, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		name := strings.TrimSpace(match[1])
		weeksStr := match[2]

		// 解析周次
		weeks := parseWeekList(weeksStr)
		for _, w := range weeks {
			if w == week {
				names = append(names, name)
				break
			}
		}
	}

	sort.Strings(names)
	return names
}

// parseWeekList 解析周次列表
// 格式: "1,2,3" 或 "1-5,7,9-11"
func parseWeekList(weeksStr string) []int {
	var weeks []int
	parts := strings.Split(weeksStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 检查是否为范围
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, _ := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, _ := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				for w := start; w <= end; w++ {
					weeks = append(weeks, w)
				}
			}
		} else {
			w, _ := strconv.Atoi(part)
			if w > 0 {
				weeks = append(weeks, w)
			}
		}
	}

	return weeks
}

// GenerateWeekSummary 生成每周人员汇总
func (s *Slicer) GenerateWeekSummary() error {
	// 统计每周有空闲的人员
	data, err := s.readMachineCSV()
	if err != nil {
		return err
	}

	summary := make(map[int]map[string]bool)
	for week := 1; week <= s.TotalWeeks; week++ {
		summary[week] = make(map[string]bool)
	}

	// 遍历所有单元格
	for _, row := range data.Rows {
		for colIdx := 1; colIdx < len(row); colIdx++ {
			cell := row[colIdx]
			re := regexp.MustCompile(`([^\s\[]+)\[([^\]]+)\]`)
			matches := re.FindAllStringSubmatch(cell, -1)

			for _, match := range matches {
				if len(match) < 3 {
					continue
				}
				name := strings.TrimSpace(match[1])
				weeks := parseWeekList(match[2])

				for _, w := range weeks {
					if w >= 1 && w <= s.TotalWeeks {
						summary[w][name] = true
					}
				}
			}
		}
	}

	// 生成汇总 CSV
	summaryFile := filepath.Join(s.OutputDir, "week_summary.csv")
	file, err := os.Create(summaryFile)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("\xef\xbb\xbf")
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 表头
	writer.Write([]string{"周次", "人数", "人员"})

	// 数据行
	for week := 1; week <= s.TotalWeeks; week++ {
		names := make([]string, 0, len(summary[week]))
		for name := range summary[week] {
			names = append(names, name)
		}
		sort.Strings(names)

		writer.Write([]string{
			strconv.Itoa(week),
			strconv.Itoa(len(names)),
			strings.Join(names, "、"),
		})
	}

	fmt.Printf("已生成周汇总: %s\n", summaryFile)
	return nil
}
