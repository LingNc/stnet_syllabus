# 学生网管课程表解析和预排班系统

本项目旨在适配教务系统（如郑州轻工业大学教务系统）导出的课表文件（扩展名为 `.xls`，实质为 HTML 格式），并将其转化为标准化、可用于排班系统的 CSV 和 Excel 文件。

## 功能特性

- **数据预处理**: 解压压缩包，根据映射表重命名文件
- **HTML 精简**: 提取核心表格数据，去除冗余样式
- **数据验证**: 校验姓名学号一致性，提取学期信息
- **格式检测**: 自动识别二维表和列表格式
- **智能解析**: 列表格式直接解析，二维表使用 DeepSeek AI 解析
- **空闲时间统计**: 生成机器可读和人类可读的无课表
- **周次切片**: 生成每周独立的无课表
- **Excel 报表**: 美观的 Excel 工作簿输出

## 快速开始

### 安装依赖

```bash
# 克隆仓库
git clone <repository-url>
cd stnet_syllabus

# 安装 Go 依赖
go mod tidy
```

### 配置

1. 将 API 密钥写入 `config/api.key` 文件
2. 根据需要修改 `config/config.yaml` 中的配置
3. 将课表压缩包和映射表放入 `input/` 目录

### 运行

```bash
# 编译程序
go build -o stnet_syllabus ./cmd

# 执行完整流程
./stnet_syllabus

# 执行特定步骤
./stnet_syllabus -step preprocess    # 数据预处理
./stnet_syllabus -step simplify      # HTML 精简
./stnet_syllabus -step validate      # 数据验证
./stnet_syllabus -step split         # 数据拆分
./stnet_syllabus -step parse         # 课表解析
./stnet_syllabus -step aggregate     # 空闲时间聚合
./stnet_syllabus -step weekly        # 周次切片
./stnet_syllabus -step excel         # Excel 生成

# 跳过 AI 解析（仅处理列表格式）
./stnet_syllabus -skip-ai
```

## 项目结构

```
stnet_syllabus/
├── config/              # 配置文件
│   ├── config.yaml      # 全局配置
│   ├── api.key          # API 密钥（gitignore）
│   └── 二维表.prompt     # AI 提示词
├── input/               # 输入数据
├── output/              # 输出数据
│   ├── temp/            # 临时文件
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
└── cmd/
    └── main.go          # 主程序入口
```

## 配置说明

### config.yaml

```yaml
semester:
  code: "20251"                    # 学期代码
  start_date: "2026-03-02"         # 学期开始日期
  total_weeks: 20                  # 学期总周数
  exam_review_weeks: [20]          # 复习周（不排班）

ai:
  base_url: "https://api.deepseek.com/chat/completions"
  model: "deepseek-chat"
  concurrency: 5                   # 并发数
  max_retries: 3                   # 重试次数
  request_interval: 500            # 请求间隔（毫秒）

paths:
  input: "./input"
  output: "./output"
  # ... 其他路径配置
```

## CSV 格式规范

### 课程表
```csv
课程,教师,周次,节次,地点
项目管理A,王曼曼,1-11单,五[3-4]单,三教楼106
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

- [x] 项目初始化
- [x] 数据预处理
- [x] HTML 精简
- [x] 数据验证
- [x] 格式检测与拆分
- [x] 列表格式解析
- [x] 二维表 AI 解析
- [x] 空闲时间聚合
- [x] 周次切片
- [x] Excel 生成
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能优化

## 作者

绫袅

## 协议

MIT License
