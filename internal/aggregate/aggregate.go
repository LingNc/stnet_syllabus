// Package aggregate 处理无课表统计和聚合
package aggregate

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

// Aggregator 聚合器
type Aggregator struct {
	InputDir      string
	OutputDir     string
	TotalWeeks    int
	ReviewWeeks   []int
}

// StudentSchedule 学生课表
type StudentSchedule struct {
	Name      string
	StudentID string
	BusySlots map[SlotKey]map[int]bool // 忙碌时间段 -> 周次集合
}

// SlotKey 时间段键
type SlotKey struct {
	Day   int // 0=周一, 4=周五
	Slot  int // 0=1-2节, 4=9-10节
}

// NewAggregator 创建聚合器
func NewAggregator(inputDir, outputDir string, totalWeeks int, reviewWeeks []int) *Aggregator {
	return &Aggregator{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		TotalWeeks:  totalWeeks,
		ReviewWeeks: reviewWeeks,
	}
}

// Process 处理所有 CSV 文件
func (a *Aggregator) Process() error {
	if err := os.MkdirAll(a.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 读取所有 CSV 文件
	files, err := a.findCSVFiles()
	if err != nil {
		return err
	}

	// 解析所有课表
	schedules := make(map[string]*StudentSchedule)
	for _, file := range files {
		if err := a.parseCSV(file, schedules); err != nil {
			fmt.Printf("警告: 解析文件 %s 失败: %v\n", filepath.Base(file), err)
		}
	}

	fmt.Printf("共解析 %d 位学生的课表\n", len(schedules))

	// 计算空闲时间
	freeTimes := a.calculateFreeTime(schedules)

	// 生成人类可读的汇总表
	if err := a.generateSummary(freeTimes); err != nil {
		return fmt.Errorf("生成汇总表失败: %w", err)
	}

	// 生成机器可读表
	if err := a.generateMachineReadable(freeTimes); err != nil {
		return fmt.Errorf("生成机器可读表失败: %w", err)
	}

	return nil
}

// findCSVFiles 查找所有 CSV 文件
func (a *Aggregator) findCSVFiles() ([]string, error) {
	entries, err := os.ReadDir(a.InputDir)
	if err != nil {
		return nil, fmt.Errorf("读取输入目录失败: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
			files = append(files, filepath.Join(a.InputDir, entry.Name()))
		}
	}

	return files, nil
}

// parseCSV 解析单个 CSV 文件
func (a *Aggregator) parseCSV(filePath string, schedules map[string]*StudentSchedule) error {
	// 从文件名解析学生信息
	fileName := filepath.Base(filePath)
	name, studentID, semester, fileType := parseFileName(fileName)
	_ = semester // 暂时未使用

	key := name + "_" + studentID
	schedule, exists := schedules[key]
	if !exists {
		schedule = &StudentSchedule{
			Name:      name,
			StudentID: studentID,
			BusySlots: make(map[SlotKey]map[int]bool),
		}
		schedules[key] = schedule
	}

	// 读取 CSV
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	if len(records) < 2 {
		return nil // 空文件
	}

	// 解析表头
	headers := records[0]
	colMap := make(map[string]int)
	for i, h := range headers {
		colMap[strings.TrimSpace(h)] = i
	}

	// 判断是课程还是环节
	isActivity := fileType == "activity" || strings.Contains(fileName, "activity")

	// 解析每一行
	for _, record := range records[1:] {
		if isActivity {
			a.parseActivityRow(record, colMap, schedule)
		} else {
			a.parseCourseRow(record, colMap, schedule)
		}
	}

	return nil
}

// parseFileName 从文件名解析信息
// 格式: 姓名_学号_学期_course.csv 或 姓名_学号_学期_activity.csv
func parseFileName(fileName string) (name, studentID, semester, fileType string) {
	baseName := strings.TrimSuffix(fileName, ".csv")
	parts := strings.Split(baseName, "_")

	if len(parts) >= 3 {
		name = parts[0]
		studentID = parts[1]
		semester = parts[2]
		if len(parts) >= 4 {
			fileType = parts[3]
		}
	}

	return
}

// parseCourseRow 解析课程行
func (a *Aggregator) parseCourseRow(record []string, colMap map[string]int, schedule *StudentSchedule) {
	// 获取周次和节次
	weekCol := colMap["周次"]
	sessionCol := colMap["节次"]

	if weekCol >= len(record) || sessionCol >= len(record) {
		return
	}

	weekStr := strings.TrimSpace(record[weekCol])
	sessionStr := strings.TrimSpace(record[sessionCol])

	if weekStr == "" || sessionStr == "" {
		return
	}

	// 解析周次
	weeks := parseWeeks(weekStr)

	// 解析节次
	day, slots, _ := parseSession(sessionStr)

	// 标记忙碌时间
	for _, slot := range slots {
		key := SlotKey{Day: day, Slot: slot}
		if _, exists := schedule.BusySlots[key]; !exists {
			schedule.BusySlots[key] = make(map[int]bool)
		}
		for _, w := range weeks {
			schedule.BusySlots[key][w] = true
		}
	}
}

// parseActivityRow 解析环节行（环节占用全部时间）
func (a *Aggregator) parseActivityRow(record []string, colMap map[string]int, schedule *StudentSchedule) {
	weekCol := colMap["周次"]
	if weekCol >= len(record) {
		return
	}

	weekStr := strings.TrimSpace(record[weekCol])
	if weekStr == "" {
		return
	}

	weeks := parseWeeks(weekStr)

	// 环节占用所有时间段
	for day := 0; day < 5; day++ {
		for slot := 0; slot < 5; slot++ {
			key := SlotKey{Day: day, Slot: slot}
			if _, exists := schedule.BusySlots[key]; !exists {
				schedule.BusySlots[key] = make(map[int]bool)
			}
			for _, w := range weeks {
				schedule.BusySlots[key][w] = true
			}
		}
	}
}

// calculateFreeTime 计算空闲时间
func (a *Aggregator) calculateFreeTime(schedules map[string]*StudentSchedule) map[SlotKey][]FreeEntry {
	result := make(map[SlotKey][]FreeEntry)

	allWeeks := make(map[int]bool)
	for w := 1; w <= a.TotalWeeks; w++ {
		allWeeks[w] = true
	}
	for _, w := range a.ReviewWeeks {
		delete(allWeeks, w)
	}

	for day := 0; day < 5; day++ {
		for slot := 0; slot < 5; slot++ {
			key := SlotKey{Day: day, Slot: slot}
			var entries []FreeEntry

			for _, schedule := range schedules {
				busyWeeks := schedule.BusySlots[key]
				freeWeeks := make([]int, 0)

				for w := range allWeeks {
					if !busyWeeks[w] {
						freeWeeks = append(freeWeeks, w)
					}
				}

				sort.Ints(freeWeeks)

				if len(freeWeeks) > 0 {
					entries = append(entries, FreeEntry{
						Name:  schedule.Name,
						Weeks: freeWeeks,
					})
				}
			}

			result[key] = entries
		}
	}

	return result
}

// FreeEntry 空闲时间条目
type FreeEntry struct {
	Name  string
	Weeks []int
}

// generateSummary 生成人类可读的汇总表
func (a *Aggregator) generateSummary(freeTimes map[SlotKey][]FreeEntry) error {
	outputFile := filepath.Join(a.OutputDir, "free_time_summary.csv")
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入 BOM
	file.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 表头
	days := []string{"周一", "周二", "周三", "周四", "周五"}
	slots := []string{"1-2节", "3-4节", "5-6节", "7-8节", "9-10节"}

	writer.Write(append([]string{"节次"}, days...))

	// 数据行
	for slotIdx, slotName := range slots {
		row := []string{slotName}

		for dayIdx := range days {
			key := SlotKey{Day: dayIdx, Slot: slotIdx}
			entries := freeTimes[key]

			// 按周次分组
			weeksToNames := make(map[string][]string)
			for _, entry := range entries {
				weekStr := formatWeeks(entry.Weeks, a.TotalWeeks, a.ReviewWeeks)
				weeksToNames[weekStr] = append(weeksToNames[weekStr], entry.Name)
			}

			// 构建单元格内容
			var parts []string
			for weekStr, names := range weeksToNames {
				sort.Strings(names)
				if weekStr == "" {
					parts = append(parts, strings.Join(names, "、"))
				} else {
					parts = append(parts, fmt.Sprintf("%s(%s)", strings.Join(names, "、"), weekStr))
				}
			}

			row = append(row, strings.Join(parts, " "))
		}

		writer.Write(row)
	}

	fmt.Printf("已生成人类可读汇总表: %s\n", outputFile)
	return nil
}

// generateMachineReadable 生成机器可读表
func (a *Aggregator) generateMachineReadable(freeTimes map[SlotKey][]FreeEntry) error {
	outputFile := filepath.Join(a.OutputDir, "free_time_machine.csv")
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入 BOM
	file.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 表头
	days := []string{"周一", "周二", "周三", "周四", "周五"}
	slots := []string{"1-2节", "3-4节", "5-6节", "7-8节", "9-10节"}

	writer.Write(append([]string{"节次"}, days...))

	// 数据行
	for slotIdx, slotName := range slots {
		row := []string{slotName}

		for dayIdx := range days {
			key := SlotKey{Day: dayIdx, Slot: slotIdx}
			entries := freeTimes[key]

			var parts []string
			for _, entry := range entries {
				weekStr := joinInts(entry.Weeks, ",")
				parts = append(parts, fmt.Sprintf("%s[%s]", entry.Name, weekStr))
			}

			row = append(row, strings.Join(parts, " "))
		}

		writer.Write(row)
	}

	fmt.Printf("已生成机器可读表: %s\n", outputFile)
	return nil
}

// parseWeeks 解析周次字符串
func parseWeeks(weekStr string) []int {
	var weeks []int
	weekStr = strings.TrimSpace(weekStr)

	if weekStr == "" {
		return weeks
	}

	// 提取单双周标记
	isOdd := strings.Contains(weekStr, "单")
	isEven := strings.Contains(weekStr, "双")
	weekStr = strings.ReplaceAll(weekStr, "单", "")
	weekStr = strings.ReplaceAll(weekStr, "双", "")

	// 解析范围
	re := regexp.MustCompile(`(\d+)(?:-(\d+))?`)
	matches := re.FindAllStringSubmatch(weekStr, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		start, _ := strconv.Atoi(match[1])
		end := start
		if match[2] != "" {
			end, _ = strconv.Atoi(match[2])
		}

		for w := start; w <= end; w++ {
			if isOdd && w%2 == 0 {
				continue
			}
			if isEven && w%2 == 1 {
				continue
			}
			weeks = append(weeks, w)
		}
	}

	return weeks
}

// parseSession 解析节次
func parseSession(sessionStr string) (day int, slots []int, parity string) {
	// 解析星期
	dayMap := map[string]int{
		"一": 0, "二": 1, "三": 2, "四": 3, "五": 4,
		"1": 0, "2": 1, "3": 2, "4": 3, "5": 4,
	}

	// 提取单双周标记
	if strings.Contains(sessionStr, "单") {
		parity = "单"
		sessionStr = strings.ReplaceAll(sessionStr, "单", "")
	} else if strings.Contains(sessionStr, "双") {
		parity = "双"
		sessionStr = strings.ReplaceAll(sessionStr, "双", "")
	}

	// 匹配星期
	re := regexp.MustCompile(`([一二三四五12345])`)
	match := re.FindString(sessionStr)
	if match != "" {
		day = dayMap[match]
	}

	// 匹配节次
	re = regexp.MustCompile(`(\d+)(?:-(\d+))?`)
	matches := re.FindAllStringSubmatch(sessionStr, -1)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		end := start
		if m[2] != "" {
			end, _ = strconv.Atoi(m[2])
		}

		// 转换为 slot (0-4)
		for s := start; s <= end; s++ {
			slot := (s - 1) / 2
			if slot >= 0 && slot < 5 {
				slots = append(slots, slot)
			}
		}
	}

	// 去重
	slotMap := make(map[int]bool)
	var uniqueSlots []int
	for _, s := range slots {
		if !slotMap[s] {
			slotMap[s] = true
			uniqueSlots = append(uniqueSlots, s)
		}
	}
	sort.Ints(uniqueSlots)

	return day, uniqueSlots, parity
}

// formatWeeks 格式化周次为可读字符串
func formatWeeks(weeks []int, totalWeeks int, reviewWeeks []int) string {
	if len(weeks) == 0 {
		return "(无空闲)"
	}

	// 检查是否全覆盖
	scheduleWeeks := make(map[int]bool)
	for w := 1; w <= totalWeeks; w++ {
		isReview := false
		for _, rw := range reviewWeeks {
			if w == rw {
				isReview = true
				break
			}
		}
		if !isReview {
			scheduleWeeks[w] = true
		}
	}

	allCovered := true
	for w := range scheduleWeeks {
		found := false
		for _, week := range weeks {
			if week == w {
				found = true
				break
			}
		}
		if !found {
			allCovered = false
			break
		}
	}

	if allCovered {
		return "" // 全部空闲
	}

	// 合并连续周次
	var segments []string
	start := weeks[0]
	prev := weeks[0]

	for i := 1; i < len(weeks); i++ {
		if weeks[i] == prev+1 {
			prev = weeks[i]
		} else {
			if start == prev {
				segments = append(segments, fmt.Sprintf("%d", start))
			} else {
				segments = append(segments, fmt.Sprintf("%d-%d", start, prev))
			}
			start = weeks[i]
			prev = weeks[i]
		}
	}

	if start == prev {
		segments = append(segments, fmt.Sprintf("%d", start))
	} else {
		segments = append(segments, fmt.Sprintf("%d-%d", start, prev))
	}

	return strings.Join(segments, ",") + "周"
}

// joinInts 将整数数组连接为字符串
func joinInts(ints []int, sep string) string {
	var parts []string
	for _, i := range ints {
		parts = append(parts, strconv.Itoa(i))
	}
	return strings.Join(parts, sep)
}
