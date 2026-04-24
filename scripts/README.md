# 教务系统数据获取脚本

本目录包含用于从郑州轻工业大学教务系统自动获取课表数据的 Python 脚本。

## 脚本说明

### login.py
**功能**: 扫码登录教务系统，获取并保存 cookies

**使用**:
```bash
python3 login.py
```

**流程**:
1. 生成二维码并显示
2. 用户使用"i轻工大"APP扫码
3. 等待登录完成
4. 保存 cookies 到 `cookies.json`

**输出**: `cookies.json` - 包含登录会话信息

---

### get_schedule.py
**功能**: 使用官方API导出课表XLS文件

**前置条件**: 已运行 `login.py` 获取 cookies

**使用**:
```bash
python3 get_schedule.py
```

**流程**:
1. 加载 cookies.json
2. 验证登录状态
3. 调用官方导出API获取课表
4. 保存为 `课程表_年份_学期.xls`

**输出**: `课程表_2025_1.xls` (或类似文件名)

**API端点**: `/wsxk/xkjg.ckdgxsxdkchj_data_exp10319.jsp`

---

## 完整工作流程

```bash
# 1. 登录获取 cookies
cd scripts/
python3 login.py
# 扫码登录...

# 2. 导出课表
python3 get_schedule.py
# 课表已保存为: 课程表_2025_1.xls

# 3. 使用 stnet_syllabus 处理
cd ..
./stnet_syllabus -ics-input scripts/课程表_2025_1.xls -ics-output output/calendar.ics
```

## 依赖

```bash
pip3 install requests qrcode pillow
```

## 配置说明

- **学年学期编码**: 系统使用 `0=第一学期, 1=第二学期`
- **学号**: 自动从页面获取，无需手动配置
- **API**: 使用官方导出接口，参数通过 Base64 编码

## 注意事项

1. cookies.json 包含敏感信息，已添加到 .gitignore
2. 如登录过期，重新运行 `login.py`
3. 脚本仅在郑州轻工业大学教务系统测试通过
