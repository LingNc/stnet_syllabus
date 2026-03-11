---
name: find-conda
description: 正确定位 Conda 安装路径，激活指定的 Conda 虚拟环境，或使用环境绝对路径执行 Python 程序，避免出现“找不到 conda 命令”或“ModuleNotFoundError”的错误。
license: MIT
metadata:
  author: lingnc
  version: "1.0"
  tags: [conda, python, environment, terminal, execution]
---

# Find Conda

## 技能目标
你的核心任务是接管所有涉及到 Python 程序的执行流程。在运行任何 Python 脚本或安装依赖之前，你必须确保使用的是正确的 Conda 虚拟环境，而不是系统的全局 Python，并且妥善处理 Conda 环境变量未加载的问题。

##  严格限制
1. **禁止直接使用全局 `python`**：在未确认 Conda 环境前，绝对不要直接运行 `python script.py`，这会导致依赖找不到。
2. **禁止盲目使用 `conda activate`**：由于你运行在非交互式 Shell 中，直接执行 `conda activate` 极大概率会报错。必须采用本技能 Step 3 中规定的安全调用方式。

## 执行步骤

当需要执行 Python 代码、安装包或用户明确要求使用 Conda 时，请严格按照以下步骤操作：

### Step 1: 探寻 Conda 安装路径
由于环境变量 PATH 可能未包含 Conda，你需要先找到它。依次尝试执行以下命令查找 Conda 根目录（记为 `<CONDA_BASE>`）：
- `find ~ -maxdepth 2 -type d -name "miniconda3" -o -name "anaconda3" -o -name "miniforge3"`
- 常见的 `<CONDA_BASE>` 路径通常为：`~/miniconda3`, `~/anaconda3`, `/opt/conda` 等。

### Step 2: 确认目标虚拟环境
- 询问用户或根据上下文确定需要使用的环境名称（记为 `<ENV_NAME>`）。
- 列出当前可用的环境以确认其存在：`<CONDA_BASE>/bin/conda env list`

### Step 3: 安全执行 Python 命令 (核心)
为了避开 `conda init` 的报错，你有两种被允许的执行方式。**强烈推荐使用方式 A**。

**方式 A：直接使用环境的解释器绝对路径 (最稳定)**
无需激活环境，直接定位到该环境的 python 执行文件：
```bash
# 执行脚本
<CONDA_BASE>/envs/<ENV_NAME>/bin/python your_script.py

# 安装依赖
<CONDA_BASE>/envs/<ENV_NAME>/bin/pip install package_name

```

**方式 B：通过 Source 脚本激活 (当必须使用纯 conda 命令时)**
如果必须激活环境（例如需要运行依赖于环境变量的特定框架），使用以下格式拼接命令：

```bash
source <CONDA_BASE>/etc/profile.d/conda.sh && conda activate <ENV_NAME> && python your_script.py

```

### Step 4: 验证执行环境

在正式运行复杂的任务前，养成好习惯，先运行以下命令向用户证明你找对了环境：

* `<CONDA_BASE>/envs/<ENV_NAME>/bin/python -c "import sys; print(sys.executable)"`
确保输出的路径是指向 `<ENV_NAME>` 的。