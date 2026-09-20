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
1. 生成二维码并保存到当前目录 (`qrcode.png`)
2. 显示二维码绝对路径，用户使用"i轻工大"APP扫码
3. 等待登录完成
4. 登录成功后自动删除二维码图片
5. 保存 cookies 到 `cookie/cookie_<学号>.json`

**输出**:
- **二维码**: `qrcode.png` (当前目录，登录后自动删除)
- **Cookie**: `cookie/cookie_<学号>.json` (脚本目录下的 cookie/ 文件夹)

---

### get_schedule.py
**功能**: 使用官方API导出课表XLS文件

**前置条件**: 已运行 `login.py` 获取 cookies

**使用**:
```bash
python3 get_schedule.py
```

**流程**:
1. 从 `cookie/` 目录加载最新的 cookie 文件
2. 验证登录状态
3. 自动获取学生姓名和学号
4. 调用官方导出API获取课表
5. 保存为 `xls/<姓名>_<学号>_<学年学期>.xls`

**输出**: `xls/<姓名>_<学号>_<学年学期>.xls`

**文件名示例**:
- `张三_202300001234_20250.xls` (2025学年第一学期)
- `李四_202300001235_20251.xls` (2025学年第二学期)

**API端点**: `/wsxk/xkjg.ckdgxsxdkchj_data_exp10319.jsp`

---

## 完整工作流程

```bash
# 1. 登录获取 cookies
cd scripts/
python3 login.py
# 二维码已保存到: /path/to/current/dir/qrcode.png
# 请使用"i轻工大"APP扫描二维码
# 登录成功后会自动删除二维码
# Cookie 已保存到: cookie/cookie_<学号>.json

# 2. 导出课表
python3 get_schedule.py
# 课表已保存到: xls/<姓名>_<学号>_<学年学期>.xls
# 例如: xls/张三_202300001234_20250.xls

# 3. 使用 stnet_syllabus 处理（可选）
cd ..
./stnet_syllabus -ics-input scripts/xls/<姓名>_<学号>_<学年学期>.xls
```

## 依赖

```bash
pip3 install requests qrcode pillow
```

## 配置说明

- **学年学期编码**: 系统使用 `0=第一学期, 1=第二学期`
- **学号**: 自动从页面获取，无需手动配置
- **API**: 使用官方导出接口，参数通过 Base64 编码

## 目录结构

```
scripts/
├── login.py                 # 登录脚本
├── get_schedule.py          # 课表导出脚本
├── cookie/                  # Cookie 存储目录 (gitignored)
│   └── cookie_<学号>.json   # 登录会话信息
└── xls/                     # 课表输出目录 (gitignored)
    └── <姓名>_<学号>_<学年学期>.xls
```

**执行时当前目录**:
```
qrcode.png                 # 二维码图片 (临时，登录后自动删除)
```

## 注意事项

1. `cookie/` 和 `xls/` 目录包含敏感信息，已添加到 `.gitignore`
2. 二维码保存在执行命令的当前目录 (`pwd`)，登录成功后自动删除
3. 如登录过期，重新运行 `login.py`
4. 脚本仅在郑州轻工业大学教务系统测试通过
