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

### 待测试
- [ ] AI 解析准确性测试
- [ ] 完整流程集成测试
- [ ] Excel 格式优化

---

## 架构设计

### 目录结构
```
stnet_syllabus/
├── config/              # 配置文件
│   ├── config.yaml      # 全局配置
│   ├── api.key          # API 密钥（gitignore）
│   └── 二维表.prompt     # AI 提示词
├── input/               # 输入数据
├── output/              # 输出数据
│   ├── temp/            # 临时文件
│   │   ├── raw_xls/     # 重命名后的原始文件
│   │   ├── simplified_xls/  # 精简后的 HTML
│   │   └── split/       # 拆分后的文件
│   │       ├── 2d_table/
│   │       └── list/
│   ├── csv_normalized/  # 标准化 CSV
│   ├── final/           # 最终 Excel 报表
│   └── error.log        # 错误日志
├── internal/            # 内部包
│   ├── preprocess/      # 数据预处理
│   ├── simplify/        # HTML 精简
│   ├── validate/        # 数据验证
│   ├── split/           # 数据拆分
│   ├── parser/          # 解析器（含 AI 调用）
│   ├── aggregate/       # 无课表聚合
│   ├── weekly/          # 周次切片
│   └── excel/           # Excel 生成
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

---

## 待解决问题

1. 需要测试 AI 解析的准确性和稳定性
2. 需要优化 Excel 输出格式的美观性
3. 考虑添加进度显示功能

---

## 参考资源

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
