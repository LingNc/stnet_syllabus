// Package ics 处理 ICS 日历文件生成
package ics

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Generator ICS 日历生成器
type Generator struct {
	StartDate   time.Time
	PeriodTimes map[int]map[string]string
	AlarmTime   int
	Events      []Event
}

// Event 日历事件
type Event struct {
	UID         string
	Name        string
	Teacher     string
	Location    string
	StartTime   time.Time
	EndTime     time.Time
	Description string
	RRULE       string
	Duration    time.Duration
}

// NewGenerator 创建 ICS 生成器
func NewGenerator(startDate time.Time, periodTimes map[int]map[string]string, alarmTime int) *Generator {
	if alarmTime <= 0 {
		alarmTime = 15
	}
	return &Generator{
		StartDate:   startDate,
		PeriodTimes: periodTimes,
		AlarmTime:   alarmTime,
		Events:      []Event{},
	}
}

// AddFromCSV 从 CSV 文件添加课程事件
func (g *Generator) AddFromCSV(csvFile string) error {
	file, err := os.Open(csvFile)
	if err != nil {
		return fmt.Errorf("打开 CSV 文件失败: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // 允许变长字段

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("读取 CSV 失败: %w", err)
	}

	if len(records) < 2 {
		return nil // 空文件或只有表头
	}

	// 解析表头
	headers := records[0]
	colMap := make(map[string]int)
	for i, h := range headers {
		colMap[strings.TrimSpace(h)] = i
	}

	// 解析每一行
	for _, record := range records[1:] {
		if err := g.parseCourseRow(record, colMap); err != nil {
			// 记录警告但继续处理
			fmt.Printf("  警告: 解析行失败: %v\n", err)
		}
	}

	return nil
}

// parseCourseRow 解析课程行
func (g *Generator) parseCourseRow(record []string, colMap map[string]int) error {
	courseCol := colMap["课程"]
	teacherCol := colMap["任课老师"]
	weekCol := colMap["周次"]
	sessionCol := colMap["节次"]
	locationCol := colMap["地点"]

	// 检查必要的列
	if courseCol >= len(record) || sessionCol >= len(record) {
		return nil
	}

	courseName := strings.TrimSpace(record[courseCol])
	if courseName == "" {
		return nil
	}

	// 清理课程名（去除前缀编号）
	courseName = cleanCourseName(courseName)

	var teacher, location string
	if teacherCol < len(record) {
		teacher = strings.TrimSpace(record[teacherCol])
	}
	if locationCol < len(record) {
		location = strings.TrimSpace(record[locationCol])
	}

	weekStr := ""
	if weekCol < len(record) {
		weekStr = strings.TrimSpace(record[weekCol])
	}

	sessionStr := strings.TrimSpace(record[sessionCol])

	// 跳过未排课的课程（周次为*未排课*或节次为空）
	if weekStr == "*未排课*" || sessionStr == "" {
		return nil
	}

	// 解析节次
	dayOfWeek, startPeriod, _, parity := parseTimeSlot(sessionStr)
	if dayOfWeek < 0 {
		return fmt.Errorf("无法解析节次: %s", sessionStr)
	}

	// 获取时间段
	periodIdx := (startPeriod + 1) / 2 // 1-2->1, 3-4->2, etc.
	periodInfo, ok := g.PeriodTimes[periodIdx]
	if !ok {
		return fmt.Errorf("未配置第 %d 节课的时间", periodIdx)
	}

	// 解析时间
	startTime, err := time.Parse("15:04", periodInfo["start"])
	if err != nil {
		// 尝试其他格式 (如 "1930")
		startTime, _ = time.Parse("Hi", periodInfo["start"])
	}
	endTime, _ := time.Parse("15:04", periodInfo["end"])
	if endTime.IsZero() {
		endTime, _ = time.Parse("Hi", periodInfo["end"])
	}

	// 解析周次
	recurrenceGroups := getRecurringWeekGroups(weekStr, parity)
	if len(recurrenceGroups) == 0 {
		return fmt.Errorf("无法解析周次: %s", weekStr)
	}

	// 为每个循环组创建事件
	for _, group := range recurrenceGroups {
		startWeek, count, interval := group[0], group[1], group[2]

		// 计算开始日期
		daysToAdd := (startWeek - 1) * 7 + dayOfWeek
		eventDate := g.StartDate.AddDate(0, 0, daysToAdd)

		// 组合日期和时间
		startDateTime := combineDateTime(eventDate, startTime)
		endDateTime := combineDateTime(eventDate, endTime)

		duration := endDateTime.Sub(startDateTime)

		// 构建 RRULE
		dayNames := []string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"}
		byday := dayNames[dayOfWeek]

		var rrule string
		if interval == 1 {
			rrule = fmt.Sprintf("FREQ=WEEKLY;WKST=MO;COUNT=%d;BYDAY=%s", count, byday)
		} else {
			rrule = fmt.Sprintf("FREQ=WEEKLY;WKST=MO;COUNT=%d;INTERVAL=%d;BYDAY=%s", count, interval, byday)
		}

		event := Event{
			UID:         generateUID(),
			Name:        courseName,
			Teacher:     teacher,
			Location:    location,
			StartTime:   startDateTime,
			EndTime:     endDateTime,
			Description: teacher,
			RRULE:       rrule,
			Duration:    duration,
		}

		g.Events = append(g.Events, event)
	}

	return nil
}

// Save 保存 ICS 文件
func (g *Generator) Save(outputPath string) error {
	var builder strings.Builder

	// 写入 ICS 头部
	builder.WriteString("BEGIN:VCALENDAR\r\n")
	builder.WriteString("VERSION:2.0\r\n")
	builder.WriteString("PRODID:-//学生网管课程表系统//CN\r\n")
	builder.WriteString("CALSCALE:GREGORIAN\r\n")
	builder.WriteString("METHOD:PUBLISH\r\n")
	builder.WriteString("X-WR-TIMEZONE:Asia/Shanghai\r\n")

	// 写入所有事件
	for _, event := range g.Events {
		g.writeEvent(&builder, event)
	}

	// 写入 ICS 尾部
	builder.WriteString("END:VCALENDAR\r\n")

	// 确保输出目录存在
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建输出目录失败: %w", err)
		}
	}

	// 写入文件
	if err := os.WriteFile(outputPath, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("写入 ICS 文件失败: %w", err)
	}

	return nil
}

// writeEvent 写入单个事件
func (g *Generator) writeEvent(builder *strings.Builder, event Event) {
	// 开始事件
	builder.WriteString("BEGIN:VEVENT\r\n")

	// UID
	builder.WriteString(fmt.Sprintf("UID:%s\r\n", event.UID))

	// 时间戳（创建时间）
	now := time.Now().UTC().Format("20060102T150405Z")
	builder.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))

	// 开始时间（本地时间格式）
	startStr := event.StartTime.Format("20060102T150405")
	builder.WriteString(fmt.Sprintf("DTSTART;TZID=Asia/Shanghai:%s\r\n", startStr))

	// 持续时间（替代 DTEND）
	durationStr := formatDuration(event.Duration)
	builder.WriteString(fmt.Sprintf("DURATION:%s\r\n", durationStr))

	// 摘要（课程名）
	summary := escapeICS(event.Name)
	builder.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", summary))

	// 地点
	if event.Location != "" {
		location := escapeICS(event.Location)
		builder.WriteString(fmt.Sprintf("LOCATION:%s\r\n", location))
	}

	// 描述
	if event.Description != "" {
		desc := escapeICS(event.Description)
		builder.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", desc))
	}

	// 重复规则
	if event.RRULE != "" {
		builder.WriteString(fmt.Sprintf("RRULE:%s\r\n", event.RRULE))
	}

	// 透明度
	builder.WriteString("TRANSP:OPAQUE\r\n")

	// 状态
	builder.WriteString("STATUS:CONFIRMED\r\n")

	// 提醒
	if g.AlarmTime > 0 {
		builder.WriteString("BEGIN:VALARM\r\n")
		builder.WriteString("ACTION:DISPLAY\r\n")
		builder.WriteString(fmt.Sprintf("TRIGGER:-PT%dM\r\n", g.AlarmTime))
		builder.WriteString(fmt.Sprintf("DESCRIPTION:%s即将开始\r\n", summary))
		builder.WriteString("END:VALARM\r\n")
	}

	// 结束事件
	builder.WriteString("END:VEVENT\r\n")
}

// cleanCourseName 清理课程名（去除前缀编号）
func cleanCourseName(name string) string {
	// 去除 [编号] 前缀
	re := regexp.MustCompile(`^\[.*?\]`)
	name = re.ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}

// parseTimeSlot 解析节次
// 返回: 星期(0-6), 开始节次, 结束节次, 单双周标记
func parseTimeSlot(sessionStr string) (dayOfWeek, startPeriod, endPeriod int, parity string) {
	dayOfWeek = -1

	// 星期映射
	dayMap := map[string]int{
		"一": 0, "二": 1, "三": 2, "四": 3, "五": 4, "六": 5, "日": 6,
		"1": 0, "2": 1, "3": 2, "4": 3, "5": 4, "6": 5, "7": 6,
	}

	// 检查单双周
	if strings.Contains(sessionStr, "单") {
		parity = "odd"
		sessionStr = strings.ReplaceAll(sessionStr, "单", "")
	} else if strings.Contains(sessionStr, "双") {
		parity = "even"
		sessionStr = strings.ReplaceAll(sessionStr, "双", "")
	}

	// 解析星期
	re := regexp.MustCompile(`([一二三四五六日1234567])`)
	match := re.FindString(sessionStr)
	if match != "" {
		dayOfWeek = dayMap[match]
	}

	// 解析节次
	re = regexp.MustCompile(`(\d+)(?:-(\d+))?`)
	matches := re.FindAllStringSubmatch(sessionStr, -1)

	if len(matches) > 0 {
		startPeriod, _ = strconv.Atoi(matches[0][1])
		if matches[0][2] != "" {
			endPeriod, _ = strconv.Atoi(matches[0][2])
		} else {
			endPeriod = startPeriod
		}
	}

	return dayOfWeek, startPeriod, endPeriod, parity
}

// getRecurringWeekGroups 获取重复周组
// 返回: [(开始周, 次数, 间隔), ...]
func getRecurringWeekGroups(weekStr string, parity string) [][]int {
	var groups [][]int

	activeWeeks := make(map[int]bool)

	// 解析周次范围
	re := regexp.MustCompile(`(\d+)(?:-(\d+))?`)
	matches := re.FindAllStringSubmatch(weekStr, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		startWeek, _ := strconv.Atoi(match[1])
		endWeek := startWeek
		if match[2] != "" {
			endWeek, _ = strconv.Atoi(match[2])
		}

		for w := startWeek; w <= endWeek; w++ {
			if parity == "odd" && w%2 == 0 {
				continue
			}
			if parity == "even" && w%2 == 1 {
				continue
			}
			activeWeeks[w] = true
		}
	}

	if len(activeWeeks) == 0 {
		return groups
	}

	// 排序周次
	var sortedWeeks []int
	for w := range activeWeeks {
		sortedWeeks = append(sortedWeeks, w)
	}
	sort.Ints(sortedWeeks)

	// 计算间隔
	interval := 1
	if parity == "odd" || parity == "even" {
		interval = 2
	}

	// 分组
	if len(sortedWeeks) == 0 {
		return groups
	}

	currentStart := sortedWeeks[0]
	currentCount := 1

	for i := 1; i < len(sortedWeeks); i++ {
		expectedNext := sortedWeeks[i-1] + interval
		if sortedWeeks[i] == expectedNext {
			currentCount++
		} else {
			groups = append(groups, []int{currentStart, currentCount, interval})
			currentStart = sortedWeeks[i]
			currentCount = 1
		}
	}

	groups = append(groups, []int{currentStart, currentCount, interval})

	return groups
}

// combineDateTime 组合日期和时间
func combineDateTime(date time.Time, t time.Time) time.Time {
	return time.Date(
		date.Year(), date.Month(), date.Day(),
		t.Hour(), t.Minute(), t.Second(),
		0, time.Local,
	)
}

// formatDuration 格式化为 ICS DURATION 格式
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("PT%dH%dM%dS", hours, minutes, seconds)
		}
		return fmt.Sprintf("PT%dH", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("PT%dM%dS", minutes, seconds)
	}
	return fmt.Sprintf("PT%dS", seconds)
}

// generateUID 生成唯一标识符
func generateUID() string {
	return fmt.Sprintf("stnet-%d-%d@syllabus", time.Now().UnixNano(), time.Now().Nanosecond())
}

// escapeICS 转义 ICS 特殊字符
func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
