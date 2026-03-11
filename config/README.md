# 配置文件说明

本目录包含系统运行所需的配置文件。

## 文件清单

### config.yaml
系统主配置文件，包含以下配置项：

- **semester**: 学期配置
  - code: 学期代码（如 20251 表示 2025-2026学年第二学期）
  - start_date: 学期开始日期
  - total_weeks: 学期总周数
  - exam_review_weeks: 复习周列表（这些周不排班）

- **time_slots**: 课程时间映射表
  - 每节课的开始和结束时间
  - 用于后续 iCal/日历功能

- **ai**: AI接口配置
  - base_url: DeepSeek API地址
  - model: 模型名称
  - concurrency: 并发数限制
  - max_retries: 重试次数
  - request_interval: 请求间隔（毫秒）

- **paths**: 输入输出路径配置
  - input: 输入目录（放置压缩包和映射表）
  - output: 输出根目录
  - temp_*: 各阶段临时文件目录
  - final: 最终Excel输出目录

- **parser**: 解析配置
  - type1_full_occupy: 环节是否占用全部工作日
  - csv_encoding: CSV文件编码

- **excel**: Excel样式配置
  - header: 表头样式（字体、加粗、背景色、边框、行高）
  - data: 数据行样式（行高、自动换行）
  - column: 列宽设置（最小/最大宽度、字符系数）
  - table: 表格内容（max_periods: 节次数量，4=1-8节，5=1-10节）

### api.key
DeepSeek API密钥文件。

格式要求：
- 第一行非注释行为有效密钥
- 以 # 开头的行为注释，会被忽略
- 示例：
  ```
  # 这是注释
  sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  ```

### 二维表.prompt
AI提示词文件，用于指导DeepSeek解析二维表格式课表。

包含：
- 角色定义
- 输入格式说明
- 输出格式规范（CSV格式要求）
- 示例说明

## 配置修改建议

1. **学期信息**: 每学期开始前更新 `semester` 部分
2. **API密钥**: 如需更换密钥，直接修改 `api.key` 文件
3. **Excel样式**: 调整 `excel` 部分可改变输出表格外观
4. **节次数量**: 修改 `excel.table.max_periods` 控制显示节次（4节或5节）

## 注意事项

- 请勿将 `api.key` 提交到Git仓库（已加入.gitignore）
- 修改配置后重启程序生效
- YAML格式严格区分缩进，请使用空格而非Tab
