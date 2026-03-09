# 学生网管课程表解析系统 - Agent 状态记录

## 项目概述

本项目旨在适配教务系统导出的课表文件（.xls 实质为 HTML 格式），将其转化为标准化的 CSV 和 Excel 文件，用于学生网管的排班系统。

**作者**: 绫袅
**协议**: MIT
**技术栈**: Go 语言
**AI 接口**: DeepSeek API (deepseek-chat)

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

## 关键设计决策

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

### 4. 学期代码规则
- 格式: YYYYS
- YYYY: 学年开始年份
- S: 学期（0=第一学期，1=第二学期）
- 例: "2025-2026第二学期" → "20251"

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

## 待解决问题

1. 需要测试 AI 解析的准确性和稳定性
2. 需要优化 Excel 输出格式的美观性
3. 考虑添加进度显示功能

## 参考资源

- 示例代码位于 `example/` 目录
- Python 原型代码供逻辑参考
- 目标输出格式参考 `example/` 中的 CSV 文件
