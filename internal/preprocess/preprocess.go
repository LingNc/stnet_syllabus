// Package preprocess 处理数据预处理和映射
package preprocess

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"stnet_syllabus/internal/simplify"
)

// attachmentDirName 附件文件夹名，收集表导出的压缩包解压后放在这里
const attachmentDirName = "附件"

// MappingEntry 映射表条目
type MappingEntry struct {
	Name      string // 姓名
	StudentID string // 学号
	FileName  string // 原始文件名
}

// XlsxCell XML 单元格定义
type XlsxCell struct {
	R string `xml:"r,attr"`
	V string `xml:"v"`
}

// XlsxRow XML 行定义
type XlsxRow struct {
	Cells []XlsxCell `xml:"c"`
}

// XlsxSheetData XML 表数据
type XlsxSheetData struct {
	Rows []XlsxRow `xml:"row"`
}

// XlsxWorksheet XML 工作表
type XlsxWorksheet struct {
	SheetData XlsxSheetData `xml:"sheetData"`
}

// formatStudentID 格式化学号，处理科学计数法
// 如 "5.42311010415E+11" -> "542311010415"
func formatStudentID(id string) string {
	id = strings.TrimSpace(id)

	// 检查是否是科学计数法
	if strings.Contains(id, "E") || strings.Contains(id, "e") {
		// 使用高精度计算
		id = strings.ToUpper(id)

		// 解析尾数和指数
		var mantissa float64
		var exp int
		_, err := fmt.Sscanf(id, "%lE", &mantissa)
		if err != nil {
			return id
		}

		// 手动提取指数部分
		parts := strings.Split(id, "E")
		if len(parts) == 2 {
			exp, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}

		// 获取尾数字符串（去掉小数点）
		mantissaStr := strings.ReplaceAll(parts[0], ".", "")
		mantissaStr = strings.TrimLeft(mantissaStr, "0")

		// 根据指数调整
		// 5.42311010415E+11 表示 542311010415
		// 尾数整数部分1位，指数是11，所以结果应该是12位
		decimalIdx := strings.Index(parts[0], ".")
		if decimalIdx > 0 {
			// 小数点后的位数
			fracDigits := len(parts[0]) - decimalIdx - 1

			// 重新精确计算：将尾数作为整数，然后乘以10^(exp-小数位数)
			multiplier := exp - fracDigits

			// 构造结果
			result := mantissaStr
			for i := 0; i < multiplier; i++ {
				result += "0"
			}
			return result
		}
	}

	return id
}

// columnLayout 映射表的列布局
// 教务系统的收集表列会变化（例如新增了一列自动填写的“姓名”），
// 因此不能写死下标，必须按表头文字定位
type columnLayout struct {
	NameIdx  int    // 姓名列下标
	IDIdx    int    // 学号列下标
	FileIdx  int    // 附件文件名列下标
	IDColumn string // 学号列字母，用于从 XML 中读取原始值
}

// legacyLayout 旧版收集表布局：提交者, 提交时间, 姓名, 学号, 教学安排表
var legacyLayout = columnLayout{NameIdx: 2, IDIdx: 3, FileIdx: 4, IDColumn: "D"}

// resolveColumns 按表头文字解析列布局，识别不到时退回旧版布局
func resolveColumns(header []string) columnLayout {
	layout := legacyLayout
	nameIdx, idIdx, fileIdx := -1, -1, -1

	for i, h := range header {
		h = strings.TrimSpace(h)
		switch {
		case strings.Contains(h, "姓名"):
			nameIdx = i
		case strings.Contains(h, "学号"):
			idIdx = i
		case strings.Contains(h, "教学安排表") || strings.Contains(h, "文件名"):
			fileIdx = i
		}
	}

	if nameIdx >= 0 {
		layout.NameIdx = nameIdx
	}
	if idIdx >= 0 {
		layout.IDIdx = idIdx
		layout.IDColumn = columnLetter(idIdx)
	}
	if fileIdx >= 0 {
		layout.FileIdx = fileIdx
	}
	return layout
}

// columnLetter 将 0 基列下标转换为 Excel 列字母（0 -> A, 25 -> Z, 26 -> AA）
func columnLetter(idx int) string {
	name := ""
	for idx >= 0 {
		name = string(rune('A'+idx%26)) + name
		idx = idx/26 - 1
	}
	return name
}

// leadingLetters 返回单元格引用中的列字母部分（"D12" -> "D"）
func leadingLetters(ref string) string {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	return ref[:i]
}

// ImportReport 预处理统计，用于汇总需要人工处理的附件
type ImportReport struct {
	Processed   int      // 成功导入
	Duplicates  int      // 重复附件（压缩包与已解压目录同时存在）
	Unmatched   []string // 映射表中找不到记录的附件
	Unsupported []string // 非 HTML 课表（二进制 Excel），需要本人重新导出
}

// InputLayout 输入目录的布局
// 支持两种结构：
//  1. 旧版：映射表 xlsx + 压缩包（都在输入根目录）
//  2. 新版：映射表 xlsx + 附件/ 文件夹（已解压的课表，可能仍带有原始压缩包）
type InputLayout struct {
	MappingFile string   // 映射表 xlsx 路径，未找到时为空
	CourseFiles []string // 课表附件 .xls/.xlsx（输入根目录 + 附件/），不含映射表
	Archives    []string // 压缩包 .zip（输入根目录 + 附件/）
}

// ScanInput 扫描输入目录，识别映射表和课表附件
func ScanInput(inputDir string) InputLayout {
	layout := InputLayout{MappingFile: findMappingFile(inputDir)}

	for _, dir := range []string{inputDir, filepath.Join(inputDir, attachmentDirName)} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		// 同一目录下如果已经有解压好的附件，就跳过该目录的压缩包，避免重复导入
		var looseFiles, archives []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			lowerName := strings.ToLower(entry.Name())
			switch {
			case strings.HasSuffix(lowerName, ".zip"):
				archives = append(archives, path)
			case strings.HasSuffix(lowerName, ".xls"), strings.HasSuffix(lowerName, ".xlsx"):
				if path != layout.MappingFile {
					looseFiles = append(looseFiles, path)
				}
			}
		}

		layout.CourseFiles = append(layout.CourseFiles, looseFiles...)
		if len(looseFiles) == 0 {
			layout.Archives = append(layout.Archives, archives...)
		}
	}

	return layout
}

// findMappingFile 在输入根目录中查找映射表
func findMappingFile(inputDir string) string {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return ""
	}

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") {
			continue
		}
		candidates = append(candidates, entry.Name())
	}

	// 优先选择名字里带“收集结果”的那一份
	for _, name := range candidates {
		if strings.Contains(name, "收集结果") {
			return filepath.Join(inputDir, name)
		}
	}
	if len(candidates) > 0 {
		return filepath.Join(inputDir, candidates[0])
	}
	return ""
}

// fileIndex 文件名到映射条目的索引
type fileIndex struct {
	exact map[string]MappingEntry   // 归一化文件名 -> 条目
	tails map[string][]MappingEntry // 去掉提交者前缀后的文件名 -> 条目
}

// normalizeKey 归一化文件名：全角空格转半角并合并空白
func normalizeKey(s string) string {
	s = strings.ReplaceAll(s, "　", " ")
	return strings.Join(strings.Fields(s), " ")
}

// newFileIndex 建立文件名索引
func newFileIndex(mapping []MappingEntry) *fileIndex {
	idx := &fileIndex{
		exact: make(map[string]MappingEntry),
		tails: make(map[string][]MappingEntry),
	}
	for _, entry := range mapping {
		key := normalizeKey(trimWorkbookExt(entry.FileName))
		idx.exact[key] = entry
		if tail := tailAfterSubmitter(key); tail != "" {
			idx.tails[tail] = append(idx.tails[tail], entry)
		}
	}
	return idx
}

// tailAfterSubmitter 返回文件名中提交者前缀之后的部分
// 上传系统有时会把提交者昵称换掉（例如全角空格变成“-”），前缀不可靠，只能比对后半段
func tailAfterSubmitter(key string) string {
	i := strings.Index(key, "_")
	if i < 0 {
		return ""
	}
	return key[i+1:]
}

// lookup 按附件文件名查找映射条目
func (idx *fileIndex) lookup(fileName string) (MappingEntry, bool) {
	key := normalizeKey(trimWorkbookExt(fileName))
	if entry, ok := idx.exact[key]; ok {
		return entry, true
	}
	// 提交者前缀不同时，用时间戳之后的文件名兜底（仅在唯一匹配时采用）
	if tail := tailAfterSubmitter(key); tail != "" {
		if list := idx.tails[tail]; len(list) == 1 {
			return list[0], true
		}
	}
	return MappingEntry{}, false
}

// trimWorkbookExt 去掉课表附件的扩展名，非 xls/xlsx 返回 false
func trimWorkbookExt(fileName string) string {
	lower := strings.ToLower(fileName)
	if strings.HasSuffix(lower, ".xls") || strings.HasSuffix(lower, ".xlsx") {
		return fileName[:len(fileName)-len(filepath.Ext(fileName))]
	}
	return ""
}

// decodeZipEntryName 还原压缩包内的文件名
// 部分收集系统生成的 zip 没有设置 UTF-8 标志位，文件名是 GBK 编码
func decodeZipEntryName(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	decoded, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), name)
	if err != nil {
		return name
	}
	return decoded
}

// isBinaryWorkbook 判断是否为二进制 Excel 文件（.xlsx 或 Excel 97-2003 的 .xls）
// 教务系统导出的 .xls 实质是 HTML，二进制文件无法按 HTML 流程解析
func isBinaryWorkbook(content []byte) bool {
	if len(content) >= 2 && content[0] == 'P' && content[1] == 'K' {
		return true
	}
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	return len(content) >= len(oleMagic) && bytes.Equal(content[:len(oleMagic)], oleMagic)
}

// Processor 预处理器
type Processor struct {
	InputDir    string
	OutputDir   string
	MappingFile string
	Report      ImportReport
}

// NewProcessor 创建预处理器
func NewProcessor(inputDir, outputDir, mappingFile string) *Processor {
	return &Processor{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		MappingFile: mappingFile,
	}
}

// LoadMapping 从 Excel 文件加载映射表
func (p *Processor) LoadMapping() ([]MappingEntry, error) {
	f, err := excelize.OpenFile(p.MappingFile)
	if err != nil {
		return nil, fmt.Errorf("打开映射表失败: %w", err)
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取映射表失败: %w", err)
	}

	// 按表头定位各列：收集表的列数会随问卷改版变化
	layout := legacyLayout
	if len(rows) > 0 {
		layout = resolveColumns(rows[0])
	}

	// 从 XML 中读取学号原始值（避免长数字被格式化成科学计数法）
	studentIDs, err := p.loadStudentIDsFromXML(layout.IDColumn)
	if err != nil {
		fmt.Printf("警告: 从 XML 读取学号失败: %v，将使用 Excel API\n", err)
	}

	var entries []MappingEntry
	// 跳过表头，从第二行开始
	// 新旧两种列布局：提交者, 提交时间, [姓名,] 姓名, 学号, 文件名
	for i, row := range rows {
		if i == 0 {
			continue // 跳过表头
		}
		if len(row) <= layout.NameIdx || len(row) <= layout.IDIdx || len(row) <= layout.FileIdx {
			continue
		}
		studentID := formatStudentID(row[layout.IDIdx])

		// 如果 XML 中有原始值，使用 XML 的值（第 i 行对应第 i+1 行单元格）
		if xmlID, ok := studentIDs[i+1]; ok && xmlID != "" {
			studentID = xmlID
		}

		name := strings.TrimSpace(row[layout.NameIdx])
		fileName := strings.TrimSpace(row[layout.FileIdx])

		// 如果名字为空，尝试从文件名中提取
		if name == "" && fileName != "" {
			// 从文件名提取（去除扩展名）
			baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			// 如果文件名包含下划线，取第一部分作为名字
			if idx := strings.Index(baseName, "_"); idx > 0 {
				name = baseName[:idx]
			} else {
				name = baseName
			}
			fmt.Printf("警告: 第 %d 行姓名为空，从文件名推断为: %s\n", i+1, name)
		}

		entry := MappingEntry{
			Name:      name,
			StudentID: studentID,
			FileName:  fileName,
		}
		if entry.Name != "" && entry.StudentID != "" {
			entries = append(entries, entry)
		} else {
			fmt.Printf("警告: 第 %d 行数据不完整（姓名: %s, 学号: %s），跳过\n", i+1, entry.Name, entry.StudentID)
		}
	}

	return entries, nil
}

// loadStudentIDsFromXML 直接从 xlsx 的 XML 中读取指定列的原始值
func (p *Processor) loadStudentIDsFromXML(column string) (map[int]string, error) {
	// 打开 xlsx 作为 zip
	r, err := zip.OpenReader(p.MappingFile)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// 找到 sheet1.xml
	var sheetFile *zip.File
	for _, f := range r.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			sheetFile = f
			break
		}
	}
	if sheetFile == nil {
		return nil, fmt.Errorf("未找到 sheet1.xml")
	}

	// 读取 XML
	rc, err := sheetFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	// 解析 XML
	var worksheet XlsxWorksheet
	if err := xml.Unmarshal(data, &worksheet); err != nil {
		return nil, err
	}

	// 提取学号列（单元格引用形如 D2、D3）
	studentIDs := make(map[int]string)
	for _, row := range worksheet.SheetData.Rows {
		for _, cell := range row.Cells {
			if leadingLetters(cell.R) != column {
				continue
			}
			rowNum, err := strconv.Atoi(cell.R[len(column):])
			if err == nil && rowNum > 0 && cell.V != "" {
				studentIDs[rowNum] = cell.V
			}
		}
	}

	return studentIDs, nil
}

// ExtractAndRename 解压 zip 文件并按映射表重命名其中的课表附件
func (p *Processor) ExtractAndRename(zipPath string, mapping []MappingEntry) error {
	// 打开 zip 文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 zip 文件失败: %w", err)
	}
	defer r.Close()

	index := newFileIndex(mapping)

	for _, file := range r.File {
		// 跳过目录和 macOS 元数据目录
		// 注意：不能跳过 "." 开头的条目，合法的课表文件名可能就以 "._" 开头
		if file.FileInfo().IsDir() || strings.HasPrefix(file.Name, "__MACOSX") {
			continue
		}

		origName := filepath.Base(decodeZipEntryName(file.Name))
		if trimWorkbookExt(origName) == "" {
			continue // 只处理 .xls/.xlsx
		}

		rc, err := file.Open()
		if err != nil {
			fmt.Printf("错误: 无法打开文件 %s: %v\n", origName, err)
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			fmt.Printf("错误: 读取文件失败 %s: %v\n", origName, err)
			continue
		}

		p.importAttachment(origName, content, index)
	}

	return nil
}

// importAttachment 按映射表重命名并写入一个课表附件
// origName 为附件的原始文件名（含扩展名）
func (p *Processor) importAttachment(origName string, content []byte, index *fileIndex) {
	entry, found := index.lookup(origName)
	if !found {
		fmt.Printf("警告: 附件 %s 在映射表中未找到，跳过\n", origName)
		p.Report.Unmatched = append(p.Report.Unmatched, origName)
		return
	}

	// 二进制 Excel（.xlsx / Excel 97-2003）不是教务系统导出的 HTML 课表
	if isBinaryWorkbook(content) {
		fmt.Printf("跳过: %s 不是网页格式课表（属于 %s/%s）\n", origName, entry.Name, entry.StudentID)
		p.Report.Unsupported = append(p.Report.Unsupported,
			fmt.Sprintf("%s(%s) %s", entry.Name, entry.StudentID, origName))
		return
	}

	outputPath := filepath.Join(p.OutputDir, fmt.Sprintf("%s_%s.xls", entry.Name, entry.StudentID))
	if _, err := os.Stat(outputPath); err == nil {
		// 压缩包和已解压目录里都有同一份附件时，保留先处理的那一份
		p.Report.Duplicates++
		return
	}

	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		fmt.Printf("错误: 写入文件失败 %s: %v\n", outputPath, err)
		return
	}

	fmt.Printf("已处理: %s -> %s\n", origName, filepath.Base(outputPath))
	p.Report.Processed++
}

// Process 执行完整的预处理流程
// 兼容两种输入结构：
//  1. 映射表 xlsx + 压缩包（放在输入根目录）
//  2. 映射表 xlsx + 附件/ 文件夹（已解压的课表，可能仍保留原始压缩包）
func (p *Processor) Process() error {
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	layout := ScanInput(p.InputDir)
	if p.MappingFile == "" {
		p.MappingFile = layout.MappingFile
	}
	if p.MappingFile == "" {
		return fmt.Errorf("未找到映射表文件（%s 目录下的 .xlsx）", p.InputDir)
	}

	fmt.Printf("使用映射表: %s\n", p.MappingFile)

	// 加载映射表
	mapping, err := p.LoadMapping()
	if err != nil {
		return err
	}
	fmt.Printf("加载了 %d 条映射记录\n", len(mapping))

	if len(layout.CourseFiles) == 0 && len(layout.Archives) == 0 {
		return fmt.Errorf("未找到课表附件（%s 或 %s/ 下的 .xls/.xlsx/.zip）", p.InputDir, attachmentDirName)
	}

	p.Report = ImportReport{}
	index := newFileIndex(mapping)

	// 先处理已解压的附件，再处理压缩包（重复的附件只保留先导入的那一份）
	if len(layout.CourseFiles) > 0 {
		fmt.Printf("发现 %d 个课表附件，开始导入...\n", len(layout.CourseFiles))
		for _, path := range layout.CourseFiles {
			origName := filepath.Base(path)
			content, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("错误: 读取文件失败 %s: %v\n", origName, err)
				continue
			}
			p.importAttachment(origName, content, index)
		}
	}

	// 处理所有压缩包
	for _, zipPath := range layout.Archives {
		fmt.Printf("\n处理压缩包: %s\n", zipPath)
		if err := p.ExtractAndRename(zipPath, mapping); err != nil {
			return err
		}
	}

	p.printSummary()
	return nil
}

// printSummary 输出预处理汇总，并列出需要人工处理的附件
func (p *Processor) printSummary() {
	fmt.Printf("\n预处理完成: 成功 %d", p.Report.Processed)
	if p.Report.Duplicates > 0 {
		fmt.Printf(", 重复跳过 %d", p.Report.Duplicates)
	}
	fmt.Printf(", 无法解析 %d, 未匹配 %d\n", len(p.Report.Unsupported), len(p.Report.Unmatched))

	if len(p.Report.Unmatched) > 0 {
		fmt.Println("\n以下附件在映射表中找不到对应记录，已跳过：")
		for _, name := range p.Report.Unmatched {
			fmt.Printf("  - %s\n", name)
		}
	}

	if len(p.Report.Unsupported) > 0 {
		fmt.Println("\n以下附件不是教务系统导出的网页格式课表（多为 Excel 二进制文件或截图），")
		fmt.Println("无法解析，需要让本人重新导出为 .xls 后提交：")
		for _, item := range p.Report.Unsupported {
			fmt.Printf("  - %s\n", item)
		}
	}
}

// ProcessDirectXLS 直接处理输入目录中的 xls 文件（无映射表模式）
// 从 xls 文件中提取学生信息并重命名
func (p *Processor) ProcessDirectXLS() error {
	// 确保输出目录存在
	if err := os.MkdirAll(p.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 同时扫描输入根目录和附件/
	layout := ScanInput(p.InputDir)
	var xlsFiles []string
	for _, path := range layout.CourseFiles {
		if !strings.HasSuffix(strings.ToLower(path), ".xlsx") {
			xlsFiles = append(xlsFiles, path)
		}
	}

	if len(xlsFiles) == 0 {
		return fmt.Errorf("未找到 xls 文件")
	}

	fmt.Printf("发现 %d 个 xls 文件，开始直接处理...\n\n", len(xlsFiles))

	p.Report = ImportReport{}
	processed := 0
	skipped := 0

	for _, xlsPath := range xlsFiles {
		fileName := filepath.Base(xlsPath)
		fmt.Printf("处理: %s\n", fileName)

		// 读取文件
		content, err := os.ReadFile(xlsPath)
		if err != nil {
			fmt.Printf("  错误: 读取文件失败: %v，跳过\n", err)
			skipped++
			continue
		}

		// 二进制 Excel 不是教务系统导出的 HTML 课表
		if isBinaryWorkbook(content) {
			fmt.Printf("  跳过: 不是网页格式课表（需要让本人重新导出提交）\n")
			p.Report.Unsupported = append(p.Report.Unsupported, fileName)
			skipped++
			continue
		}

		// 从文件中提取学生信息
		info, err := simplify.ExtractStudentInfoFromFile(xlsPath)
		if err != nil {
			fmt.Printf("  错误: 提取学生信息失败: %v，跳过\n", err)
			skipped++
			continue
		}

		// 构建新文件名
		newFileName := fmt.Sprintf("%s_%s_%s.xls", info.Name, info.StudentID, info.SemesterCode)
		outputPath := filepath.Join(p.OutputDir, newFileName)

		if err := os.WriteFile(outputPath, content, 0644); err != nil {
			fmt.Printf("  错误: 写入文件失败: %v，跳过\n", err)
			skipped++
			continue
		}

		fmt.Printf("  成功: %s -> %s\n", fileName, newFileName)
		if info.SemesterCode == "" {
			fmt.Printf("  警告: 未能提取学期代码，请检查配置文件\n")
		}
		processed++
	}

	fmt.Printf("\n直接处理完成: 成功 %d, 跳过 %d\n", processed, skipped)
	return nil
}
