# 学生网管课程表解析系统 - Agent 状态记录

**作者**: 绫袅
**协议**: MIT
**技术栈**: Go 语言
**AI 接口**: DeepSeek API (deepseek-chat)

---

## 项目概述

本项目旨在适配教务系统导出的课表文件（扩展名为 `.xls`，实质为 HTML 格式），并将其转化为标准化、可用于排班系统的 CSV 和 Excel 文件。由于导出的课表包含"二维表"和"列表"两种不同结构，系统将采用程序正则解析（针对列表）与 AI 大模型智能转换（针对二维表）相结合的方式进行处理。

---

## 快速开始

```bash
# 初始化配置
./stnet_syllabus -init

# 编辑配置文件
vim config/config.yaml
vim config/api.key  # 填入你的 DeepSeek API 密钥

# 准备输入数据
mkdir -p input/
# 将压缩包和映射表放入 input/ 目录

# 运行完整流程
./stnet_syllabus

# 或生成 ICS 日历（批量模式）
./stnet_syllabus -ics

# 或个人 ICS 模式
./stnet_syllabus -ics-input me-list.xls -ics-output me.ics
```

---

## 核心命令

| 命令 | 说明 |
|------|------|
| `-init` | 初始化配置目录 |
| `-ics` | 启用 ICS 批量导出 |
| `-ics-input <file>` | 个人模式：输入 xls 文件 |
| `-ics-output <file>` | 个人模式：输出 ics 文件 |
| `-step <name>` | 执行指定步骤 |
| `-skip-ai` | 跳过 AI 解析 |
| `-aikey <key>` | 指定 API 密钥 |
| `-config <file>` | 指定配置文件 |

**完整 CLI 参数说明**: 见 `agent-refer/04-技术规范.md` 第6节

---

## 处理流程

1. **预处理** - 解压、重命名、映射学号
2. **简化** - HTML 精简、编码转换 (GBK→UTF-8)
3. **验证** - 校验姓名学号一致性
4. **拆分** - 格式检测、课程/环节分离
5. **解析** - 列表直接解析 / 2D表 AI 解析 → CSV
6. **聚合** - 生成总无课表
7. **切片** - 按周生成独立视图
8. **导出** - Excel 报表 / ICS 日历

**详细流程说明**: 见 `agent-refer/01-详细流程.md`

---

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

---

## 目录结构

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
├── pkg/                 # 公共包
│   ├── models/          # 数据模型
│   └── utils/           # 工具函数
└── agent-refer/         # 详细参考文档
    ├── 01-详细流程.md
    ├── 02-变更日志.md
    ├── 03-开发进度.md
    └── 04-技术规范.md
```

---

## 状态速览

### 开发进度 (v1.9.0)
- [x] ICS 日历导出（批量模式 + 个人模式）
- [x] `-init` 零配置启动
- [x] CLI 参数覆盖（-ics-input, -ics-output 等）
- [x] 2D 表环节数据提取

**完整进度**: 见 `agent-refer/03-开发进度.md`

### 最新变更
- 支持直接处理 xls 文件模式（无需 zip/映射表）
- 修复 2D 表环节数据提取失败问题
- 环节事件在周一到周五每天都生成
- 修复 ICS 时区定义 (VTIMEZONE)
- 修复 ICS 格式兼容性 (DURATION替代DTEND)
- 修复列表格式环节数据导入
- 添加全天事件格式支持

**完整变更日志**: 见 `agent-refer/02-变更日志.md`

---

## 参考文档

| 文档 | 内容 |
|------|------|
| `agent-refer/01-详细流程.md` | Step 1-8 详细说明 |
| `agent-refer/02-变更日志.md` | 完整变更记录 |
| `agent-refer/03-开发进度.md` | 开发进度清单 |
| `agent-refer/04-技术规范.md` | 技术规范、CLI参数、CSV格式、架构设计 |

---

## Agent 自举说明

**本文档作为项目全局状态机与综述，能够防止代码库膨胀后上下文丢失，确保后续 AI 辅助开发时拥有完整的记忆和设计初衷。**

### 实时同步要求
1. 当 PLAN.md 中的流程描述更新时，必须同步更新 AGENTS.md 中的对应章节
2. **当修复关键 Bug 时，必须在 `agent-refer/02-变更日志.md` 中记录问题和解决方案**
3. **当添加新功能时，必须在 `agent-refer/02-变更日志.md` 中记录功能说明和实现改动**
4. 当完成开发任务时，必须更新 `agent-refer/03-开发进度.md` 中的进度清单
5. 当添加新的技术规范时，必须在 `agent-refer/04-技术规范.md` 中补充
6. 当功能有较大变更时，必须同步更新 README.md
7. **隐私策略**：所有文档中不得出现真实人名、学号等敏感信息

### 文档更新原则
- **必须写**：Bug 修复、功能添加、重大重构必须在对应文档中记录
- **何时写**：与代码提交同步进行，不要拖延到后续会话
- **写多少**：足够让后续开发者理解改动原因和影响范围

### 维护责任
- 每次会话开始前，AI 工具应优先读取 AGENTS.md 了解项目状态
- 每次重要变更后，AI 应更新 `agent-refer/` 下的对应文档
- 保持 AGENTS.md 与 PLAN.md 的描述一致性
- **Git 操作规范**：根据 `RULE.md` 执行 git 操作
  - 完成一次任务后进行 git 提交
  - **不要添加署名**（不要包含 `Co-Authored-By` 或类似标记）
  - 当前分支是特性/修复分支时，直接提交到当前分支

### 必须做的事
- 完成用户的任务后，必须回复：“我已严格遵守AGENTS.md。”。