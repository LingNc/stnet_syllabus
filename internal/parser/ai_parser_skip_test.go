package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHasExistingResult 验证“已有解析结果则跳过”的判定逻辑
func TestHasExistingResult(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "csv")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "张三_123_20260.xls")
	if err := os.WriteFile(input, []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &AI2DParser{OutputDir: outDir, SkipExisting: true}

	// 未生成过 CSV：不跳过
	if p.hasExistingResult(input) {
		t.Fatal("不存在课程 CSV 时不应跳过")
	}

	// 课程 CSV 存在：跳过
	course := filepath.Join(outDir, "张三_123_20260_course.csv")
	if err := os.WriteFile(course, []byte("课程,教师,周次,节次,地点\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !p.hasExistingResult(input) {
		t.Fatal("存在非空课程 CSV 时应跳过")
	}

	// 空 CSV（上次失败残留）：不跳过
	if err := os.WriteFile(course, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if p.hasExistingResult(input) {
		t.Fatal("空课程 CSV 不应视为已有结果")
	}

	// 关闭 SkipExisting：永不跳过
	if err := os.WriteFile(course, []byte("课程,教师,周次,节次,地点\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p.SkipExisting = false
	if p.hasExistingResult(input) {
		t.Fatal("SkipExisting=false 时不应跳过")
	}
}
