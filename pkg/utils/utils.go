// Package utils 提供通用工具函数
package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	dstDir := filepath.Dir(dst)
	if err := EnsureDir(dstDir); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// ExtractZip 解压 zip 文件到指定目录
func ExtractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := EnsureDir(destDir); err != nil {
		return err
	}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// 跳过 macOS 系统文件
		if strings.Contains(f.Name, "__MACOSX") || strings.HasPrefix(f.Name, ".") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		path := filepath.Join(destDir, filepath.Base(f.Name))
		outFile, err := os.Create(path)
		if err != nil {
			return err
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}
	}

	return nil
}

// DetectTableFormat 检测 HTML 课表格式
func DetectTableFormat(htmlContent string) int {
	// 列表格式特征
	hasPageTitle := strings.Contains(htmlContent, `pagetitle="pagetitle"`)
	hasClassCode := strings.Contains(htmlContent, "上课班级代码")
	if hasPageTitle || hasClassCode {
		return 2 // FormatList
	}

	// 二维表特征
	hasMyTable := strings.Contains(htmlContent, `id='mytable'`) || strings.Contains(htmlContent, `id="mytable"`)
	hasNoKb := strings.Contains(htmlContent, "div_nokb")
	if hasMyTable || hasNoKb {
		return 1 // Format2D
	}

	return 0 // FormatUnknown
}

// ParseSemesterCode 解析学期代码
// 如 "2025-2026第二学期" -> "20251"
func ParseSemesterCode(semesterStr string) string {
	// 提取年份
	yearRe := regexp.MustCompile(`(\d{4})-(\d{4})`)
	matches := yearRe.FindStringSubmatch(semesterStr)
	if len(matches) >= 2 {
		startYear := matches[1]
		// 判断学期
		semesterNum := "0"
		if strings.Contains(semesterStr, "第二") || strings.Contains(semesterStr, "2") {
			semesterNum = "1"
		}
		return startYear + semesterNum
	}
	return ""
}

// CleanCourseName 清理课程名，去除编号
// 如 "[1707292]项目管理A" -> "项目管理A"
func CleanCourseName(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.Index(name, "]")
	if idx != -1 {
		return strings.TrimSpace(name[idx+1:])
	}
	return name
}

// ParseWeeks 解析周次字符串
// 输入: "1-11周(单)" 输出: "1-11单"
func ParseWeeks(weekStr string) string {
	weekStr = strings.TrimSpace(weekStr)
	weekStr = strings.ReplaceAll(weekStr, "周", "")

	// 提取单双周标记
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

// CleanLocation 清理地点，去除末尾括号内容
// 如 "三教楼106(172)" -> "三教楼106"
func CleanLocation(loc string) string {
	loc = strings.TrimSpace(loc)
	idx := strings.Index(loc, "(")
	if idx == -1 {
		idx = strings.Index(loc, "（")
	}
	if idx != -1 {
		return strings.TrimSpace(loc[:idx])
	}
	return loc
}

// ParseSession 解析节次字符串
// 输入格式: "五[3-4节]单" 或 "三[7-8节]"
func ParseSession(sessionStr string) (dayOfWeek string, sections string, parity string) {
	// 匹配模式: 五[3-4节]单
	re := regexp.MustCompile(`([一二三四五六日天])\[(\d+)(?:-(\d+))?节?\]\s*(单|双)?`)
	matches := re.FindStringSubmatch(sessionStr)

	if len(matches) >= 2 {
		dayOfWeek = matches[1]
		if matches[3] != "" {
			sections = matches[2] + "-" + matches[3]
		} else {
			sections = matches[2]
		}
		if len(matches) >= 5 {
			parity = matches[4]
		}
	}

	return dayOfWeek, sections, parity
}

// WriteErrorLog 写入错误日志
func WriteErrorLog(logPath string, errors []string) error {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range errors {
		fmt.Fprintln(f, e)
	}
	return nil
}

// NormalizeSession 标准化节次格式
// 如: "五", "3-4", "单" -> "五[3-4]单"
func NormalizeSession(day, sections, parity string) string {
	result := day + "[" + sections + "]"
	if parity != "" {
		result += parity
	}
	return result
}
