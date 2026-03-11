# 学生网管课程表解析系统 - Agent 状态记录

## 项目概述

本项目旨在适配教务系统（如郑州轻工业大学教务系统）导出的课表文件（扩展名为 `.xls`，实质为 HTML 格式），并将其转化为标准化、可用于排班系统的 CSV 和 Excel 文件。由于导出的课表包含"二维表"和"列表"两种不同结构，系统将采用程序正则解析（针对列表）与 AI 大模型智能转换（针对二维表）相结合的方式进行处理。

**作者**: 绫袅
**协议**: MIT
**技术栈**: Go 语言
**AI 接口**: DeepSeek API (deepseek-chat)

---

## 详细流程

### Step 1: 数据预处理与映射
- 读取 `input/` 文件夹中的压缩包及汇总了姓名、学号和文件名的 `.xlsx` 表格。
- 解压文件，通过比对 `.xlsx` 中的映射关系，将解压出的源 `.xls` 文件重命名为 `<姓名>_<学号>.xls`。
- 将重命名后的文件统一移入 `temp/raw_xls/` 待处理文件夹。
- **关键修复**: 通过解析 xlsx XML 直接读取原始字符串值，避免 Excel 科学计数法导致学号精度丢失（如 `542307250111` 被截断为 `542307250100`）。

### Step 2: HTML 内容精简
- 读取 `.xls`（HTML）源码，通过 DOM 树解析（goquery）过滤掉无用的 CSS 样式、冗余标签和空白符，仅保留核心的 `<table>` 数据。
- 提取学期信息并添加 SEMESTER 注释，供后续步骤使用。
- 将 GBK 编码转换为 UTF-8。
- 精简后的文件输出至 `temp/simplified_xls/`，以提高后续 AI 调用的 Token 利用率和程序解析的准确度。

### Step 3: 数据验证与信息提取
- 校验文件内容中的姓名、学号是否与文件名 `<姓名>_<学号>` 一致。如不一致，记录至 `error.log` 并跳过该文件。
- 提取学期代码（如将"2025-2026第二学期"转换为 `20251`）。验证通过的文件进入下一环节。
- **学期代码规则**: YYYYS 格式，YYYY 为学年开始年份，S 为学期（0=第一学期，1=第二学期）。

### Step 4: 格式辨别与数据拆分
- 分析精简后的内容特征，判定其属于"二维表"还是"列表"。
  - 列表格式特征: `pagetitle="pagetitle"` 或包含 `上课班级代码`
  - 二维表特征: `id='mytable'` 或 `div_nokb` 或 `TYPE: 2D_TABLE`
- 针对每位学生的课表，将其拆分为"课程"和"环节"两个独立部分。
- 将拆分后的文件按照 `<姓名>_<学号>_<学期代码>_<课程|环节>.xls` 规范命名，并分别存入 `temp/split/2d_table/` 和 `temp/split/list/` 目录。

### Step 5: 核心解析与 CSV 标准化转换
输出至 `output/csv_normalized/`。

**情况 A（二维表）**: 利用 Go 的 `goroutine` 并发调用 DeepSeek AI API。
- 读取 `config/二维表.prompt` 作为系统提示词。
- 要求 LLM 严格返回两个 CSV 代码块（第一块为课程，第二块为环节）。
- 程序捕获并解析保存。
- 加入 API 重试（最多 3 次）和限流控制机制。
- **API 配置**: 从 `config/api.key` 读取密钥，支持注释行（以 `#` 开头的行将被跳过）。

**情况 B（列表）**: 使用 Go 程序直接解析。
- 过滤列：课程保留"课程,教师,周次节次地点"；环节保留"环节,周次,指导老师"。
- 分离多课时：按分号（`;`）拆分"周次节次地点"列，将单行数据克隆为多行。
- 正则提取：剔除课程名前的编号；通过正则解析周次为 `<开始周>-<结束周>|<某一周>`，节次为 `<星期>[<开始节次>-<结束节次>]<单|双|空>`，地点剔除末尾括号内容保留 `<楼名><房间号>`。

### Step 6: 无课表统计与总表生成
- 读取所有标准化后的 CSV 文件，在内存中构建所有学生的全局时间矩阵。
- 生成机器可读的总无课表（`free_time_machine.csv`）和人类可读的总无课表（`free_time_summary.csv`）。

### Step 7: 周次维度的视图切片
- 遍历该学期的所有教学周，动态过滤并生成每一周独立的无课表 CSV 文件（如 `free_week_1.csv` 到 `free_week_20.csv`）。

### Step 8: Excel 最终报表构建
- 使用 Go 的 Excel 处理库（`excelize`），将上述生成的各类 CSV 汇总输出为一个美观的 `.xlsx` 工作簿。
- `Sheet1` 放置人类可读的全部无课表，后续 Sheet 分别放置按周划分的无课表。

---

## 统一 CSV 格式规范

### 课程 CSV 格式
```csv
课程,教师,周次,节次,地点
<无编号课程名>,<教师名>,<开始周>-<结束周>|<某一周>,<星期>[<开始节次>-<结束节次>]<不分单双周留空|如果单双周上课填写'单'or'双'>,<楼名><房间号>
```

### 环节 CSV 格式
```csv
环节,周次,指导老师
<无编号环节名>,<开始周>-<结束周>|<某一周>,<教师名>
```

---

## 开发进度

### 已完成
- [x] 项目初始化
  - [x] Go 模块初始化
  - [x] 目录结构创建
  - [x] 配置文件 (config.yaml)
  - [x] AI 提示词 (二维表.prompt)
- [x] Step 1: 数据预处理与映射 (internal/preprocess/)
- [x] Step 2: HTML 内容精简 (internal/simplify/)
- [x] Step 3: 数据验证与信息提取 (internal/validate/)
- [x] Step 4: 格式辨别与数据拆分 (internal/split/)
- [x] Step 5: 核心解析与 CSV 标准化转换
  - [x] 5A: 二维表 AI 解析 (internal/parser/ai_parser.go, deepseek_client.go)
  - [x] 5B: 列表格式直接解析 (internal/parser/list_parser.go)
- [x] Step 6: 无课表统计与总表生成 (internal/aggregate/)
- [x] Step 7: 周次维度的视图切片 (internal/weekly/)
- [x] Step 8: Excel 最终报表构建 (internal/excel/)
- [x] 主程序入口 (cmd/main.go)
- [x] **PLAN_1 新增功能**
  - [x] CLI 参数优先级覆盖 (input, output, aikey, prompt, apikey-file, semester-start)
  - [x] `-init` 零配置启动 (go:embed 嵌入默认配置)
  - [x] ICS 日历导出 (internal/ics/)
  - [x] 增强的配置加载 (支持 CLI 覆盖)

### 待测试
- [ ] AI 解析准确性测试
- [ ] 完整流程集成测试
- [ ] Excel 格式优化

---

## 架构设计

### 目录结构
```
stnet_syllabus/
├── cmd/                 # 命令入口
│   ├── main.go          # 主程序
│   ├── embed.go         # 配置嵌入与初始化
│   └── config/          # 嵌入的默认配置（go:embed）
│       ├── config.yaml  # 全局配置模板
│       ├── api.key      # API 密钥模板（示例）
│       ├── 二维表.prompt # AI 提示词模板
│       └── README.md    # 配置说明
├── input/               # 输入数据（运行时创建）
├── output/              # 输出数据（运行时创建）
│   ├── ics/             # ICS 日历文件
│   ├── temp/            # 临时文件
│   │   ├── raw_xls/     # 重命名后的原始文件
│   │   ├── simplified_xls/  # 精简后的 HTML
│   │   └── split/       # 拆分后的文件
│   │       ├── 2d_table/
│   │       └── list/
│   ├── csv_normalized/  # 标准化 CSV
│   ├── final/           # 最终 Excel 报表
│   └── error.log        # 错误日志
├── config/              # 运行时配置（-init生成，gitignore）
│   ├── config.yaml      # 用户自定义配置
│   ├── api.key          # 用户API密钥
│   ├── 二维表.prompt     # AI提示词（可自定义）
│   └── README.md        # 配置说明
├── internal/            # 内部包
│   ├── preprocess/      # 数据预处理
│   ├── simplify/        # HTML 精简
│   ├── validate/        # 数据验证
│   ├── split/           # 数据拆分
│   ├── parser/          # 解析器（含 AI 调用）
│   ├── aggregate/       # 无课表聚合
│   ├── weekly/          # 周次切片
│   ├── excel/           # Excel 生成
│   └── ics/             # ICS 日历生成
│   └── ics/             # ICS 日历生成 (PLAN_1新增)
├── pkg/                 # 公共包
│   ├── models/          # 数据模型
│   └── utils/           # 工具函数
└── cmd/                 # 命令入口
    └── main.go          # 主程序
```

### 核心数据流
1. input/ (压缩包 + Excel 映射表)
   ↓
2. temp/raw_xls/ (解压重命名后的 .xls)
   ↓
3. temp/simplified_xls/ (精简 HTML)
   ↓
4. temp/split/ (按格式拆分)
   ↓
5. csv_normalized/ (标准化 CSV)
   ↓
6. output/final/ (Excel 报表)
   ↓
7. **ICS 导出模式**: csv_normalized/ → .ics 日历文件 (PLAN_1新增)

---

## 技术规范

### 1. 格式检测策略
- 列表格式特征: `pagetitle="pagetitle"` 或 `上课班级代码`
- 二维表特征: `id='mytable'` 或 `div_nokb`

### 2. 并发控制
- AI 调用使用 goroutine 池，限制并发数为 5
- 使用 channel 进行限流

### 3. 错误处理
- 所有错误记录到 error.log
- 验证失败文件跳过但继续处理其他文件
- API 调用失败时自动重试（最多 3 次）

### 4. 全局配置文件 (config/config.yaml)
需包含以下核心参数：
- 学期代码（如 `20251`）
- 学期开始日期（如 `2026-03-02`）
- 学期总周数（如 `20`）
- 每节课的开始与结束时间映射表（为后续对接日历/iCal功能铺垫）
- 输入/输出文件夹路径定义
- AI 配置：base_url, model, concurrency, max_retries, request_interval

### 5. AI 接口配置
- 采用 DeepSeek 免费 API (`https://api.deepseek.com/chat/completions`)
- 指定模型 `deepseek-chat`
- API 密钥必须与代码解耦，读取自 `config/api.key`
- 支持注释行（以 `#` 开头的行将被跳过）
- 支持 CLI 参数 `-aikey` 覆盖，支持 `-apikey-file` 指定自定义密钥文件

### 6. CLI 参数优先级
参数优先级从高到低：
1. 命令行参数（如 `-input`, `-output`, `-aikey`）
2. 配置文件（config.yaml）
3. 内置默认值

### 7. ICS 日历导出
- 支持从标准化 CSV 生成标准 iCalendar (.ics) 格式
- 使用 RRULE 重复规则实现循环课程事件
- 时区：Asia/Shanghai
- 课前提醒：默认 15 分钟
- 兼容小米日历及其他标准日历应用

---

## 变更日志

### 2026-03-09
- 项目初始化
- 创建目录结构和配置文件
- 编写 AGENTS.md

### 2026-03-09 (续)
- 完成所有核心模块开发
- 实现数据预处理、HTML 精简、数据验证、格式拆分
- 实现列表格式直接解析和二维表 AI 解析
- 实现空闲时间聚合、周次切片、Excel 生成
- 主程序入口完成，支持分步执行和完整流程
- 项目编译通过，依赖库配置完成

### 2026-03-09 (修复)
- **修复学号精度丢失问题**: 通过解析 xlsx XML 直接读取原始字符串值，避免 Excel 科学计数法导致 12 位学号后两位变为 00
- **修复 API 密钥读取问题**: 修改配置读取逻辑，跳过 `api.key` 中的注释行（以 `#` 开头）
- **修复学期代码提取**: 在 simplify 步骤添加 SEMESTER 注释，split 步骤从 HTML 中提取学期代码

### 2026-03-09 (修复2)
- **修复 2D 表学期代码为 unknown**: `simplify2DHTML` 未从隐藏字段提取学期，已添加 `xn` 和 `xq_m` 字段提取
- **修复 2D 表 AI 解析课程为空**: `simplify2DForAI` 未匹配 `table.schedule` 类表格，已添加选择器
- **修复 AI 返回 CSV 格式问题**: 周次包含逗号时未用引号包围，已添加 `fixCSVQuotes` 后处理函数自动修复
- **更新 AI 提示词**: 明确指示 AI 周次包含逗号时必须用双引号包围

### 2026-03-09 (修复3 - 根据 ANALYSIS_REPORT.md)
- **修复教师字段清理不彻底**: 新增 `cleanTeacherName` 函数，使用正则 `\s*\[\d+\]\s*` 清除教师姓名后的方括号编号
- **添加 error.log 错误日志**: 在 `cmd/main.go` 中添加全局错误日志记录功能，所有步骤的错误都会记录到 `output/error.log`

### 2026-03-09 (修复4 - 根据用户反馈)
- **修复 2D 表学期代码问题**:
  - 恢复 `simplify2DHTML` 中从隐藏字段提取学期的逻辑
  - 正确理解 `xq_m` 字段：`0` 是第一学期（20250），`1` 是第二学期（20251），直接拼接 `xn` + `xq_m` 得学期代码
  - 添加 SEMESTER_CODE 注释到简化后的 HTML
  - Split 步骤对 2D 表提取学期代码并与 config 对比，不一致时报告警告
- **修复学期校验**: 在 `validate` 和 `split` 中对 list 和 2D 表都进行学期代码对比，不一致时报告警告
- **统一控制台输出格式**: 修改 AI 解析输出，使其与列表解析格式一致（显示课程/环节文件）
- **修复数据完整性问题**: 对于映射表中姓名为空的记录，尝试从文件名推断姓名，避免跳过有效记录（如李紫颖）

1. ✅ ~~教师字段清理不彻底~~ - 已修复
2. ✅ ~~缺少 error.log~~ - 已添加
3. 需要测试 AI 解析的准确性和稳定性
4. 需要优化 Excel 输出格式的美观性
5. 考虑添加进度显示功能

### 2026-03-10 (修复 - 数据校验流程控制)
- **修复二进制 xlsx 文件进入后续步骤的问题**:
  - 在 `simplify` 步骤添加 `isBinaryContent()` 检测二进制文件（xlsx格式以PK开头或含大量null字节）
  - 在 `simplify` 步骤添加 `isValidHTML()` 检测是否包含基本HTML标签和表格
  - 二进制文件在 simplify 阶段被拦截，不会进入 simplified_xls 目录
  - 错误信息通过日志函数记录到 error.log

- **统一日志系统**:
  - `simplify` 和 `validate` 模块添加 `LogFunc` 回调函数参数
  - 所有错误和警告统一使用 `main.go` 的全局 `logError()` 记录
  - 确保时间戳格式一致，避免日志混乱

- **验证失败文件阻止进入后续步骤**:
  - 二进制文件在 simplify 阶段被拦截，不会生成到 simplified_xls
  - 验证失败的文件在 validate 阶段被删除
  - 后续步骤只处理有效文件

- **修复预处理文件名匹配问题**:
  - 支持 .xlsx 扩展名文件（李紫颖的"早睡.xlsx"）
  - 修复全角空格文件名匹配（白昊杰的文件名是多个全角空格）
  - 处理结果从 34 个成功恢复到 36 个成功

### 2026-03-10 (修复 - Excel格式和CSV分隔符)
- **修复Excel总文件生成问题**:
  - 修复 weekly 文件路径错误，现在正确从 `output/weekly/` 目录读取
  - 生成的 `free_time_schedule.xlsx` 现在包含汇总表和20个周表

- **修复Excel样式问题**:
  - 去掉9-10节（只保留1-2节、3-4节、5-6节、7-8节）
  - 去掉蓝色背景，改为干净的黑白样式
  - 表头行加粗，添加底部边框
  - 调整列宽（更紧凑）
  - 调整行高（表头25，数据40）
  - 代码位置：`internal/excel/excel.go:setSheetStyle()`

- **修复CSV人名分隔符**:
  - 人类可读的无课表（free_time_summary.csv）中人名分隔从顿号"、"改为空格" "
  - 代码位置：`internal/aggregate/aggregate.go:generateSummary()`

### 2026-03-10 (feat - 第一列独立样式配置)
- **添加第一列（节次列）独立样式配置**:
  - 新增 `excel.first_column` 配置项
  - 可独立设置字体大小、加粗、背景色、文字颜色、对齐方式、列宽
  - 与其他列（周一到周五）的样式分离
  - 代码位置：`internal/config/loader.go`, `internal/excel/excel.go`

### 2026-03-10 (feat - 无环节数据检测)
- **增加无环节数据检测功能**:
  - 在 aggregate 步骤检测哪些学生没有环节（activity）数据
  - 同时适用于列表格式和AI解析的2D表格式
  - 警告信息输出到控制台和 error.log
  - 代码位置：`internal/aggregate/aggregate.go:checkMissingActivities()`

### 2026-03-11 (feat - 分离汇总表和总表文件)
- **分离汇总表和总表文件**:
  - `output/final/free_time_schedule.xlsx`: 只包含汇总表（1个sheet）
  - `output/学生网管xxxx-xxxx学年第x学期无课表.xlsx`: 包含汇总表+所有周表
  - 总表文件名根据学期代码自动生成（如20251→2025-2026学年第二学期）
  - 代码位置：`cmd/main.go:generateFullScheduleFileName()`, `internal/excel/excel.go`

### 2026-03-11 (feat - PLAN_1 实现)
- **任务1: 构建灵活的命令行接口 (CLI) 与参数优先级覆盖**:
  - 新增 `-input` 参数：指定输入目录路径（覆盖配置文件）
  - 新增 `-output` 参数：指定输出目录路径（覆盖配置文件）
  - 新增 `-aikey` 参数：指定 AI API 密钥（覆盖配置文件和 api.key）
  - 新增 `-prompt` 参数：指定 AI Prompt 文件路径
  - 新增 `-apikey-file` 参数：指定 API 密钥文件路径
  - 新增 `-semester-start` 参数：指定学期开始日期
  - 命令行参数优先级：CLI > 配置文件 > 默认值
  - 代码位置：`cmd/main.go`, `internal/config/loader.go:CLIOverride`

- **任务2: 实现 `-init` 零配置启动与脚手架生成**:
  - 新增 `-init` 参数：在当前目录创建 config/ 并释放默认配置
  - 新增 `-init-force` 参数：强制覆盖已存在的配置文件
  - 使用 `go:embed` 嵌入默认配置文件到二进制
  - 默认配置包括：config.yaml、二维表.prompt、README.md
  - 代码位置：`cmd/embed.go`, `cmd/main.go`

- **任务3: 支持标准 iCalendar (.ics) 日历文件导出**:
  - **个人模式**: 新增 `-ics-input <file.xls>` 参数，直接从单个 xls 文件生成 ics
  - **批量模式**: 新增 `-ics <dir>` 参数，在正常流程后批量生成所有 ics
  - 支持从标准化 CSV 生成 .ics 日历文件
  - 实现 RRULE 重复规则，兼容小米日历等标准日历应用
  - 支持课前提醒（默认15分钟）
  - 自动从 CSV 文件名推断学生信息生成默认 ICS 文件名
  - 代码位置：`internal/ics/ics.go`, `cmd/main.go:runICSExport()`, `cmd/main.go:runICSSingleFile()`

- **新增 ICS 输出路径配置**:
  - 在 `config.yaml` 的 `paths` 中添加 `ics` 配置项
  - 默认输出到 `./output/ics`
  - 代码位置：`internal/config/loader.go`, `cmd/config/config.yaml`

- **配置加载增强**:
  - 新增 `config.CLIOverride` 结构体管理 CLI 参数覆盖
  - 修改 `config.Load()` 函数支持 CLI 覆盖
  - 新增 `config.GetPromptFilePath()` 和增强的 `config.GetAPIKey()` 支持自定义路径
  - 当未指定配置文件时，自动检查默认路径或提示用户使用 `-init`

### 2026-03-11 (chore - 配置目录重构)
- **重构配置目录结构**:
  - 移除根目录 `config/`，改为 `cmd/config/` 用于 `go:embed` 嵌入
  - 根目录 `config/` 是运行时 `-init` 生成的，不应提交到版本控制
  - 添加示例 `api.key` 文件到 `cmd/config/`，方便用户 init 后直接填入密钥
  - 更新 `.gitignore` 排除根目录 `config/`
  - 更新 `cmd/config/README.md` 说明配置文件的两种位置

### 2026-03-11 (fix - UTF-8编码检测)
- **修复 simplify 模块编码检测**:
  - 新增 `isValidUTF8()` 函数检测文件是否已经是 UTF-8 编码
  - 修复对 UTF-8 文件错误进行 GBK 解码导致乱码的问题
  - 优先检测 UTF-8，如果不是则尝试 GBK 解码
  - 代码位置：`internal/simplify/simplify.go`

### 开发进度更新
- [x] 任务1: CLI 参数覆盖与优先级
- [x] 任务2: `-init` 零配置启动
- [x] 任务3: ICS 日历导出

## 可用技能

### analysis-report
**路径**: `.agents/skills/analysis-report/SKILL.md`

**用途**: 根据计划要求分析项目执行成果，检查输出一致性，验证数据完整性，并生成全面的ANALYSIS_REPORT.md。

**适用场景**:
- 验证项目输出是否符合预期
- 排查不一致问题
- 对照需求核实完整性
- 审核执行结果

**使用方法**:
1. 确保已运行项目生成输出
2. 使用 `/analysis-report` 或调用analysis-report技能
3. 指定要比对的Plan文件（如 `plan/PLAN_0.md`）
4. 技能会自动检查输出文件、日志，生成分析报告

- 示例代码位于 `example/` 目录
- Python 原型代码供逻辑参考
- 目标输出格式参考 `example/` 中的 CSV 文件

---

## Agent 自举说明

**本文档作为项目全局状态机与综述，能够防止代码库膨胀后上下文丢失，确保后续 AI 辅助开发时拥有完整的记忆和设计初衷。**

### 实时同步要求
1. 当 PLAN.md 中的流程描述更新时，必须同步更新 AGENTS.md 中的对应章节
2. 当修复关键 Bug 时，必须在变更日志中记录问题和解决方案
3. 当完成开发任务时，必须更新开发进度清单
4. 当添加新的技术规范时，必须在技术规范章节补充

### 维护责任
- 每次会话开始前，AI 工具应优先读取 AGENTS.md 了解项目状态
- 每次重要变更后，AI 应更新 AGENTS.md 以反映最新状态
- 保持 AGENTS.md 与 PLAN.md 的描述一致性
