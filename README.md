# 学生网管课程表解析和预排班系统

本项目旨在适配教务系统（如郑州轻工业大学青果教务系统）导出的课表文件（扩展名为 `.xls`，实质为 HTML 格式），并将其转化为标准化、可用于排班系统的 CSV 和 Excel 文件。

## 功能特性

- **教务系统自动获取**: 扫码登录获取 cookies，调用官方 API 导出课表（支持郑州轻工业大学青果教务系统）
- **数据预处理**: 解压压缩包，根据映射表重命名文件
- **HTML 精简**: 提取核心表格数据，去除冗余样式
- **数据验证**: 校验姓名学号一致性，提取学期信息
- **格式检测**: 自动识别二维表和列表格式
- **智能解析**: 列表格式直接解析，二维表使用 DeepSeek AI 解析
- **空闲时间统计**: 生成机器可读和人类可读的无课表
- **周次切片**: 生成每周独立的无课表
- **Excel 报表**: 美观的 Excel 工作簿输出
- **ICS 日历导出**: 支持批量和个人模式导出 iCalendar 格式
- **版本信息**: 支持 `-version/-v` 查看版本号

## 快速开始

### 安装依赖

```bash
# 克隆仓库
git clone https://github.com/LingNc/stnet_syllabus
cd stnet_syllabus

# 安装 Go 依赖
go mod tidy

# 安装 Python 脚本依赖（如需自动获取课表）
pip3 install requests qrcode Pillow
```

### 方式一：自动获取课表（推荐，支持郑州轻工业大学）

使用 Python 脚本自动登录教务系统并导出课表：

```bash
# 1. 登录教务系统
cd scripts/
python3 login.py          # 扫码登录，保存 cookies.json

# 2. 导出课表
python3 get_schedule.py   # 导出 xls/<姓名>_<学号>_<学年学期>.xls

# 3. 生成 ICS 日历（可选）
cd ..
./stnet_syllabus -ics-input scripts/xls/<姓名>_<学号>_<学年学期>.xls
```

**详细说明**: 见 `scripts/README.md`

### 方式二：使用现有 XLS 文件

1. 运行`./stnet_syllabus -init` 初始化配置目录 `config/`
2. 将 API 密钥写入 `config/api.key` 文件
3. 根据需要修改 `config/config.yaml` 中的配置
4. 将课表文件放入 `input/` 目录（格式要求见下方「输入文件格式」）

## 输入文件格式

所有输入的课表必须是**教务系统导出的网页格式 `.xls`**（实质为 HTML）。真 `.xlsx`、Excel 97-2003 二进制格式或截图粘贴的文件无法解析，会被自动跳过并列出需重新收集的名单。

### 批量模式一：收集表导出（多人收集，推荐）

适合腾讯文档/收集表导出的场景，`input/` 中放入：

1. **映射表 xlsx**（收集结果导出）
   - 列按表头文字自动识别，含「姓名」「学号」「教学安排表」（附件文件名）列即可
   - 「教学安排表」列的内容就是每个人上传的课表附件的原始文件名，程序按这个名字匹配附件，无需任何改名处理
   - 兼容 5 列与 6 列布局（如自动填写的「姓名」列、提交者/提交时间列均不影响）
2. **课表附件**，两种形式任选或同时存在：
   - 收集导出的 `zip` 压缩包（放在 `input/` 根目录或任意子文件夹）
   - 已解压的附件文件夹（`input/` 下的任意子文件夹，名字不限，里面有 `.xls` 即可）

附件**无需重命名**，程序按映射表自动匹配姓名和学号；压缩包与已解压文件夹同时存在时自动去重；重跑时有人重新提交了修正版课表会自动覆盖旧结果，已撤回的自动清理。

### 批量模式二：散装 xls（无映射表）

`input/` 中只有 `.xls` 文件（没有映射表 xlsx 和 zip）时进入直接处理模式：

- 文件内容需包含姓名、学号和学期信息（青果教务系统导出的课表自带，程序从内容中提取并自动重命名）
- 文件名建议按 `<姓名>_<学号>_<学期码>.xls` 命名，例如 `张三_202401010101_20260.xls`
  - 学期码为 4 位：年份后两位 + 学期（`0`=第一学期，`1`=第二学期）
- 文件中学期与配置的 `semester_code` 不一致时会在验证阶段警告

### 单个文件（个人模式）

仅用于给个人生成 ICS 日历，不进入排班流程：

```bash
./stnet_syllabus -ics-input 张三_202401010101_20260.xls -ics-output 张三.ics
```

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
# input放入"所有收集到的xls"，或"收集的附件zip/已解压的附件文件夹（子文件夹名任意，有xls即可）+ 包含姓名、学号和上传文件名映射的xlsx"
./stnet_syllabus

# 执行完整流程并生成 ICS 日历
./stnet_syllabus -ics

# 查看版本号
./stnet_syllabus -version

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
globals:
  organization: "学生网管"          # 组织名称（用于生成 Excel 文件名）
  campus: "科学"                    # 校区名称
  semester_code: "20260"            # 学期代码（年份后两位+学期，0=第一学期）
  semester_start: "2026-09-07"      # 学期第一周周一的日期
  total_weeks: 21                   # 学期总周数
  exam_review_weeks: [20,21]        # 复习周（不排班）

ai:
  api_mode: "openai"                # openai（默认）或 claude
  base_url: "https://api.deepseek.com"
  model: "deepseek-chat"
  concurrency: 5                    # 并发数
  max_retries: 3                    # 重试次数
  request_interval: 500             # 请求间隔（毫秒）
  thinking:
    type: disabled                  # 思考模式开关 enabled | disabled（GLM-4.5+ / DeepSeek）
    clear_thinking: false
  request_body:
    enable_thinking: false          # Qwen 3.6+ 等模型的思考开关

excel:
  freeze: false                     # 是否冻结首行首列（false 去掉预览中的灰色分隔线）

paths:
  input: "./input"
  output: "./output"
  ics: "./output/ics"               # ICS 输出目录
  # ... 其他路径配置
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
| `-version` / `-v` | 显示版本号 |

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

- [x] CLI 参数支持
- [x] ICS 日历导出
- [x] 直接 xls 处理模式
- [x] 版本号显示
- [ ] 更多的可调节配置
- [ ] 微服务模式
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能优化

## 作者

绫袅

## 协议

MIT License
