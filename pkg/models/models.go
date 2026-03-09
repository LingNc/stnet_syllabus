// Package models 定义系统核心数据模型
package models

// Student 学生信息
type Student struct {
	Name      string // 姓名
	StudentID string // 学号
	FileName  string // 原始文件名
}

// Course 课程信息
type Course struct {
	Name     string // 课程名
	Teacher  string // 教师
	Weeks    string // 周次（如 "1-11单"）
	Session  string // 节次（如 "五[3-4]"）
	Location string // 地点
}

// Activity 环节信息（实验/实习等）
type Activity struct {
	Name    string // 环节名
	Weeks   string // 周次
	Teacher string // 指导老师
}

// Schedule 单个学生的完整课表
type Schedule struct {
	Student   Student
	Semester  string    // 学期代码
	Courses   []Course
	Activities []Activity
}

// TimeSlot 时间段
type TimeSlot struct {
	Period    string // 如 "1-2", "3-4"
	StartTime string // 开始时间
	EndTime   string // 结束时间
}

// WeekDay 星期几
type WeekDay int

const (
	Monday WeekDay = 0
	Tuesday WeekDay = 1
	Wednesday WeekDay = 2
	Thursday WeekDay = 3
	Friday WeekDay = 4
	Saturday WeekDay = 5
	Sunday WeekDay = 6
)

// FreeTime 空闲时间
type FreeTime struct {
	Name  string
	Weeks []int // 空闲周次列表
}

// TableFormat 课表格式类型
type TableFormat int

const (
	FormatUnknown TableFormat = 0
	Format2D      TableFormat = 1 // 二维表
	FormatList    TableFormat = 2 // 列表
)

// ParseResult 解析结果
type ParseResult struct {
	Student    Student
	Semester   string
	Format     TableFormat
	CourseFile string // 课程文件路径
	ActivityFile string // 环节文件路径
	Error      error
}
