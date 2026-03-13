# 学生网管课程表解析和预排班系统

本项目旨在适配教务系统（如郑州轻工业大学青果教务系统）导出的课表文件（扩展名为 `.xls`，实质为 HTML 格式），并将其转化为标准化、可用于排班系统的 CSV 和 Excel 文件。

## 功能特性

- **数据预处理**: 解压压缩包，根据映射表重命名文件
- **HTML 精简**: 提取核心表格数据，去除冗余样式
- **数据验证**: 校验姓名学号一致性，提取学期信息
- **格式检测**: 自动识别二维表和列表格式
- **智能解析**: 列表格式直接解析，二维表使用 DeepSeek AI 解析
- **空闲时间统计**: 生成机器可读和人类可读的无课表
- **周次切片**: 生成每周独立的无课表
- **Excel 报表**: 美观的 Excel 工作簿输出
- **ICS 日历导出**: 支持批量和个人模式导出 iCalendar 格式

## 快速开始

### 安装依赖

```bash
# 克隆仓库
git clone https://github.com/LingNc/stnet_syllabus
cd stnet_syllabus

# 安装 Go 依赖
go mod tidy
```

### 配置

1. 运行`./stnet_syllabus -init` 初始化配置目录 `config/`
2. 将 API 密钥写入 `config/api.key` 文件
3. 根据需要修改 `config/config.yaml` 中的配置
4. 将课表压缩包和映射表放入 `input/` 目录 或者 直接放入 xls 文件（程序会自动检测）

### 运行

```bash
# 编译程序
go build -o stnet_syllabus ./cmd

# 初始化配置（首次运行），然后配置api密钥
./stnet_syllabus -init

# 个人模式
# 从单个 xls 生成 ics（-ics-output可以省略，默认当前文件夹）
./stnet_syllabus -ics-input input/张三_202401010101_20251.xls

# 排班模式

# 执行完整流程
# input放入"所有收集到的xls"或者放入"腾讯文档收集的附件zip和包含姓名、学号和上传文件名映射的xlsx"文件
./stnet_syllabus

# 执行完整流程并生成 ICS 日历
./stnet_syllabus -ics

# 单步执行示例；`-step` 还可用：
# simplify(HTML精简)、validate(数据验证)、split(数据拆分)
# parse(课表解析)、aggregate(空闲时间聚合)、weekly(周次切片)
# excel(Excel生成)、ics(ICS批量导出)
./stnet_syllabus -step preprocess

# 跳过 AI 解析（仅处理列表格式）
./stnet_syllabus -skip-ai

# CLI 参数覆盖配置
./stnet_syllabus -input ./data -output ./out
./stnet_syllabus -aikey YOUR_API_KEY
./stnet_syllabus -semester-start 2026-03-02
```

## 配置说明

### config.yaml

```yaml
semester:
  code: "20251"                    # 学期代码（表示2025-2026第二学期）
  start_date: "2026-03-02"         # 学期开始日期
  total_weeks: 21                # 学期总周数
  exam_review_weeks: [20,21]          # 复习周（不排班）

ai:
  base_url: "https://api.deepseek.com/chat/completions"
  model: "deepseek-chat"
  concurrency: 5                   # 并发数
  max_retries: 3                   # 重试次数
  request_interval: 500            # 请求间隔（毫秒）

paths:
  input: "./input"
  output: "./output"
  ics: "./output/ics"              # ICS 输出目录
  # ... 其他路径配置
```

## 项目结构

```
stnet_syllabus/
├── cmd/                 # 命令入口
├── config/              # 运行时配置（-init 生成，gitignore）
│   ├── config.yaml
│   ├── 二维表.prompt
│   └── api.key
├── input/               # 输入数据（腾讯文档收集表导出）
│   ├── *.xls            # 方法一：自动检测如果有xls文件直接读（而不是读zip和xlsx）
│   ├── *.zip            # 方法二：收集的所有人的青果导出的xls课程表（可以是二维表也可以是列表）
│   └── *.xlsx           # 方法二：收集的表格（每行姓名、学号和对应的导出课程表文件名）
├── output/              # 输出数据
│   ├── ics/             # ICS 日历文件
│   ├── temp/            # 临时文件
│   ├── csv_normalized/  # 标准化 CSV
│   ├── final/           # 最终 Excel 报表
│   ├── weekly/          # 每周无课表
│   └── error.log        # 错误日志
├── internal/            # 内部包
│   ├── preprocess/      # 数据预处理
│   ├── simplify/        # HTML 精简
│   ├── validate/        # 数据验证
│   ├── split/           # 数据拆分
│   ├── parser/          # 解析器（含 AI 调用）
│   ├── aggregate/       # 无课表聚合
│   ├── weekly/          # 周次切片
│   ├── excel/           # Excel 生成
│   ├── ics/             # ICS 日历生成
│   └── config/          # 配置加载
├── pkg/                 # 公共包
└── plan/                # 开发计划文档
```

## CLI 参数

支持命令行参数覆盖配置文件：

| 参数 | 说明 |
|------|------|
| `-config <path>` | 指定配置文件路径 |
| `-input <path>` | 输入目录（覆盖配置） |
| `-output <path>` | 输出目录（覆盖配置） |
| `-aikey <key>` | API 密钥（覆盖配置） |
| `-semester-start <date>` | 学期开始日期（YYYY-MM-DD） |
| `-skip-ai` | 跳过 AI 解析 |
| `-ics` | 启用 ICS 导出（批量模式） |
| `-ics-input <file>` | 个人模式：输入 xls 文件 |
| `-ics-output <file>` | 个人模式：输出 ics 文件 |
| `-init` | 初始化配置目录 |
| `-init-force` | 强制覆盖已有配置 |

## CSV 格式规范

### 课程表
```csv
课程,教师,周次,节次,地点
项目管理A,王老师,1-11单,五[3-4]单,三教楼106
```

### 环节表
```csv
环节,周次,指导老师
毕业设计,1-16,张老师
```

## 技术栈

- **语言**: Go 1.21+
- **HTML 解析**: goquery
- **Excel 处理**: excelize
- **YAML 解析**: yaml.v3
- **AI 接口**: DeepSeek API

## 开发计划

- [ ] 更多的可调节配置
- [ ] 微服务模式
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能优化

## 作者

绫袅

## 协议

MIT License
