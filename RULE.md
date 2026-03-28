# 项目开发规则

## 提交信息规范

### 功能开发提交
- 格式：`feat: 中文描述...本次开发进度`
- 示例：`feat: 添加校区配置支持`

### 修复提交
- 格式：`fix: 中文描述...修复内容`
- 示例：`fix: 修复AI解析空指针异常`

### 文档提交
- 格式：`docs: 中文描述...文档更新内容`
- 示例：`docs: 更新README配置说明`

### 发布版本提交（main分支）
- 格式：`release: vx.x.x 简要描述`
- 示例：`release: v2.1.1 校区配置、组织名称、Claude API支持`

## 版本号规范
- 格式：`v主版本.次版本.修订版本`
- 示例：`v2.1.1`
- 主版本：重大架构变更
- 次版本：新功能添加
- 修订版本：bug修复或小改进

## 分支策略
1. 拥有两个长期分支：
   - main: 稳定分支。
   - develop (开发分支): 最新的开发进度。所有的新功能都先合并到这里。
   - feat/<功能名>： 用来开发新的工具
   - fix/<修复名>：修复某个工具
2. 开发新功能
如果开发一个大型版本应该先更新版本号在develop分支上。
需要时对大型任务进行适当拆分`feat/<工具名>/<功能名>`，依次使用不同的特性分支完成整体的开发。
```bash
# 检查仓库状态
git status -sb
git checkout -b feat/<工具名> develop
# ... 写代码，提交 ...
git commit -m "feat: 中文描述...本次开发进度"
# ...继续写代码，提交...
git commit -m "feat: 中文描述...本次开发进度"
# 开发完成之后进入合成
```
3. 合并功能
```bash
git checkout develop
git merge --no-ff feat/<工具名>
# 删掉临时的特性分支
git branch -d feat/<工具名>
# 继续开发新功能或者结束开发...
```
4. 发布版本：当develop上累积了足够的更新，合并到main（plan文件夹和AGENTS.md），打标签，完成一个工具的开发。
```bash
# 1. 切换到 main 分支
git checkout main

# 2. 合并 develop，但强制生成合并节点 (--no-ff)，且暂停提交 (--no-commit)
# --no-ff: 即使可以快进，也强制生成一个 commit 节点，确保 main 上有一个独立的版本点
# --no-commit: 合并后暂不生成 commit，给你机会去删除不需要的文件
git merge --no-ff --no-commit develop

# 3. 排除不需要发布的文件夹/文件
# 将不需要的文件夹和文件从暂存区和工作区删除（仅在本次 main 分支的提交中删除，不影响 develop）
# 如果 main 上本来就不存在这些文件，而 develop 上有，这一步会阻止它们进入 main
# 注意：如果出现"冲突（修改/删除）"，需要先使用 git reset HEAD 取消暂存，再手动删除
git reset HEAD plan example AGENTS.md agent-refer .agents 2>/dev/null || true
rm -rf plan example AGENTS.md agent-refer .agents 2>/dev/null || echo "文件夹不存在，跳过清理"

# 4. 提交合并，生成版本节点
# 提交信息格式：release: vx.x.x 简要描述
git commit -m "release: vx.x.x 新增功能简要描述"

# 5. 打标签
# 标签信息格式：vx.x.x
git tag -a vx.x.x -m "vx.x.x"

# 6. 推送
git push origin main --tags

# 7. 构建并发布 Release
# 构建项目
./build.sh

# 创建 GitHub Release
# 标题格式：vx.x.x（仅版本号）
# 内容格式：包含更新内容的描述和 Full Changelog 链接
gh release create vx.x.x --title "vx.x.x" --notes "## 更新内容

- 功能1描述
- 功能2描述

**Full Changelog**: https://github.com/LingNc/stnet_syllabus/compare/v上一个版本...vx.x.x" build/stnet_syllabus build/stnet_syllabus.exe
```

## Release 发布规范

### 发布前检查清单
- [ ] 所有功能已在 develop 分支测试通过
- [ ] 版本号已更新（遵循版本号规范）
- [ ] AGENTS.md 和开发进度文档已更新
- [ ] 变更日志已记录本次更新内容

### 发布流程
1. 按照"发布版本"步骤执行合并和标签
2. 运行 `./build.sh` 构建项目
3. 创建 GitHub Release，包含：
   - 标题：仅版本号（如 `v2.1.1`）
   - 内容：更新内容列表和 Full Changelog 链接
   - 附件：`stnet_syllabus`（Linux）和 `stnet_syllabus.exe`（Windows）

### 发布后检查
- [ ] Release 页面可正常访问
- [ ] 构建文件可正常下载
- [ ] 版本标签指向正确的提交

5. 修复：如果发布后发现有严重 bug
从 main 切出 fix/<工具名>/vx.x.x 分支进行修复
测试稳定后合并回develop和main