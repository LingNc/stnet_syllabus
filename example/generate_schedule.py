#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
校验 assert_csv 中的课表CSV（节次/周次），仅做解析与冲突提示，不再自动重识别；
最终按周一~周五、第一大节~第五大节汇总所有人的空闲周次，输出 free_time_summary.csv。
"""

import os
import re
import glob
import math
import time
import logging
from typing import Dict, List, Optional, Set, Tuple

import pandas as pd

# 统一使用 schedule_ocr 的日志
import schedule_ocr
from schedule_ocr import logger

# 取消自动重识别机制：如需人工修复请编辑原CSV后重新运行脚本

# 配置日志输出到文件（沿用库设置）
schedule_ocr.set_logger_config(level=logging.DEBUG, log_file='export.log')  # INFO

# ================== 可配置参数（直接在脚本内修改） ==================
# 一学期总周数
TOTAL_WEEKS = 21
# 期末不排班的复习周（列表）
EXAM_REVIEW_WEEKS = [20, 21]
# 如果不使用 EXAM_REVIEW_WEEKS 机制，也可用该参数排除最后 N 周（为0则忽略）
EXCLUDE_LAST_N_WEEKS = 0
# 种类1（环节）CSV 是否表示占用所有工作日所有大节
TYPE1_FULL_OCCUPY = True
# 输出文件名
OUT_CSV_PATH = os.path.join(os.path.dirname(__file__), 'free_time_summary.csv')
# 输入目录
JPG_DIR = os.path.join(os.path.dirname(__file__), 'assert_jpg')
CSV_DIR = os.path.join(os.path.dirname(__file__), 'assert_csv')

# ================== 固定常量 ==================
WEEKDAY_MAP = {'一': 0, '二': 1, '三': 2, '四': 3, '五': 4}
BIG_SLOTS = [(1,2), (3,4), (5,6), (7,8), (9,10)]  # 第一~第五大节


def parse_weeks(weeks_str: str, max_week_hint: int = 18) -> Tuple[Set[int], int, List[str]]:
    """解析周次字符串，返回 (周集合, 可能的最大周数提示, 无法解析的片段列表)。
    不在此函数内记录日志，统一由调用方带上下文记录。
    """
    if not isinstance(weeks_str, str) or not weeks_str.strip():
        return set(), max_week_hint, []

    s = weeks_str.strip().replace(' ', '')
    total: Set[int] = set()
    max_week = max_week_hint
    bad_parts: List[str] = []

    # 将中文顿号、分号等替换成逗号
    s = s.replace('，', ',').replace('；', ',').replace(';', ',')

    # 支持 '单' 或 '双' 修饰
    # 形如: 1-18单, 2-16双, 或者 3-5
    parts = [p for p in s.split(',') if p]
    for p in parts:
        m = re.match(r'^(\d+)(?:-(\d+))?(单|双)?$', p)
        if not m:
            # 尝试移除句点等杂质
            q = re.sub(r'[^0-9\-单双]', '', p)
            m = re.match(r'^(\d+)(?:-(\d+))?(单|双)?$', q)
        if not m:
            bad_parts.append(p)
            continue
        a = int(m.group(1))
        b = int(m.group(2)) if m.group(2) else a
        flag = m.group(3)  # 单/双/None
        if a > b:
            a, b = b, a
        max_week = max(max_week, b)
        rng = list(range(a, b+1))
        if flag == '单':
            rng = [x for x in rng if x % 2 == 1]
        elif flag == '双':
            rng = [x for x in rng if x % 2 == 0]
        total.update(rng)

    return total, max_week, bad_parts


def parse_jieci(jieci: str) -> Optional[Tuple[int, Set[int], Optional[str]]]:
    """解析节次，返回 (weekday_index, 涉及小节集合, 单双周标记)。仅处理周一~周五。
    单双周标记取值：'单' | '双' | None
    支持形如：一[3-4节]双 / 三[7-8节] / 一[3节]
    """
    if not isinstance(jieci, str) or not jieci.strip():
        return None
    s = jieci.strip()
    # 先尝试范围小节，并可带“单/双”
    m = re.search(r'([一二三四五六日天])\s*\[(\d+)-(\d+)节\]\s*(单|双)?', s)
    if not m:
        # 尝试匹配单一小节如 一[3节]
        m2 = re.search(r'([一二三四五六日天])\s*\[(\d+)节\]\s*(单|双)?', s)
        if not m2:
            return None
        day = m2.group(1)
        a = int(m2.group(2))
        b = a
        parity = m2.group(3)
    else:
        day = m.group(1)
        a = int(m.group(2))
        b = int(m.group(3))
        parity = m.group(4)

    if day not in WEEKDAY_MAP:
        return None

    sections = set(range(min(a,b), max(a,b)+1))
    return WEEKDAY_MAP[day], sections, parity


def sections_to_big_slots(sections: Set[int]) -> Set[int]:
    """将小节集合映射到大节索引集合(0..4)。"""
    slots = set()
    for sec in sections:
        idx = (sec - 1) // 2
        if 0 <= idx < len(BIG_SLOTS):
            slots.add(idx)
    return slots


def format_weeks(weeks: Set[int], schedule_range: Set[int]) -> str:
    """根据调度范围(schedule_range)格式化空闲周集合。
    规则：
      - 如果 weeks 覆盖了全部 schedule_range，返回空字符串（后续不加括号）
      - 单连续段触顶/触底使用 “X周及之后/之前”
      - 单独一周："N周"；正常范围："A-B周"；多段离散：逗号拼接后加“周”
    """
    if not schedule_range:
        return ""
    if not weeks:
        return "(无空闲)"
    free = sorted(weeks & schedule_range)
    if not free:
        return "(无空闲)"
    if set(free) == schedule_range:
        return ""  # 全部空闲
    ws = free

    segments: List[Tuple[str, int, int, Optional[str]]] = []  # (type, start, end, label)
    i = 0
    n = len(ws)
    PARITY_MIN_LEN = 3

    while i < n:
        parity_end = i
        while parity_end + 1 < n and ws[parity_end + 1] - ws[parity_end] == 2:
            parity_end += 1
        parity_len = parity_end - i + 1
        if parity_len >= PARITY_MIN_LEN:
            shrink_end = parity_end
            next_idx = parity_end + 1
            if next_idx < n:
                while shrink_end >= i and ws[shrink_end] + 1 == ws[next_idx]:
                    shrink_end -= 1
            if shrink_end >= i:
                parity_len = shrink_end - i + 1
            if parity_len >= PARITY_MIN_LEN:
                label = "单" if ws[i] % 2 == 1 else "双"
                segments.append(("parity", ws[i], ws[shrink_end], label))
                i = shrink_end + 1
                continue

        range_end = i
        while range_end + 1 < n and ws[range_end + 1] - ws[range_end] == 1:
            range_end += 1
        segments.append(("range", ws[i], ws[range_end], None))
        i = range_end + 1

    if not segments:
        return "(无空闲)"

    min_r = min(schedule_range)
    max_r = max(schedule_range)

    if len(segments) == 1 and segments[0][0] == 'range':
        _, a, b, _ = segments[0]
        if a == min_r and b < max_r:
            return f"{b+1}周及之后"
        if a > min_r and b == max_r:
            return f"{a-1}周及之前"
        if a == b:
            return f"{a}周"
        return f"{a}-{b}周"

    parts: List[str] = []
    has_range_segment = any(seg[0] == 'range' for seg in segments)
    for seg_type, start, end, label in segments:
        if seg_type == 'parity' and label:
            if start == end:
                parts.append(f"{start}{label}")
            else:
                parts.append(f"{start}-{end}{label}")
        else:
            if start == end:
                parts.append(f"{start}")
            else:
                parts.append(f"{start}-{end}")

    joined = ",".join(parts)
    if has_range_segment:
        return joined + "周"
    return joined


def load_person_csv_pair(csv_dir: str, name: str, sid: str, semester: str) -> pd.DataFrame:
    """合并一个人的两份CSV(种类0和1)为一个DataFrame。不存在的忽略。"""
    frames = []
    for t in (0, 1):
        path = os.path.join(csv_dir, f"课表_{name}_{sid}_{semester}_{t}.csv")
        if os.path.exists(path):
            try:
                df = pd.read_csv(path)
                frames.append(df)
            except Exception as e:
                logger.warning(f"读取失败: {path}: {e}")
    if frames:
        return pd.concat(frames, ignore_index=True)
    return pd.DataFrame()


def detect_conflicts(df: pd.DataFrame, name: str, sid: str, semester: str, image_type: Optional[int], issues: List[Dict], source_csv: Optional[str] = None) -> None:
    """仅检测并记录冲突，不自动重识别。
    image_type 用于标注是种类0还是1；source_csv 标注来源CSV路径。
    """
    needed_cols = {'课程', '节次', '周次'}
    if not needed_cols.issubset(df.columns):
        return
    df['课程'] = df['课程'].ffill()
    for (course, jieci), g in df.groupby(['课程', '节次'], dropna=False):
        weeks_values = set([str(x).strip() for x in g['周次'].dropna().astype(str) if str(x).strip()])

        # 如果包含 [error]，视为 OCR/解析失败，不计为“周次冲突”，单独记录
        if any('[error]' in w for w in weeks_values):
            issues.append({
                'issue_type': 'ocr_error',
                'name': name,
                'student_id': sid,
                'semester': semester,
                'image_type': image_type,
                'source_csv': source_csv or '',
                'course': course,
                'jieci': jieci,
                'weeks_str': '|'.join(sorted(weeks_values)),
                'details': '该课程/节次存在 [error]，请人工检查原 CSV 或原课表'
            })
            logger.warning(f"发现 OCR/解析失败记录(需人工核对): {name}-{sid}-{semester} 种类{image_type} 课程={course} 节次={jieci} 周次集合={weeks_values}")
            continue

        if len(weeks_values) > 1:
            logger.warning(f"发现周次冲突(需人工核对): {name}-{sid}-{semester} 种类{image_type} 课程={course} 节次={jieci} 周次集合={weeks_values}")
            issues.append({
                'issue_type': 'conflict_weeks',
                'name': name,
                'student_id': sid,
                'semester': semester,
                'image_type': image_type,
                'source_csv': source_csv or '',
                'course': course,
                'jieci': jieci,
                'weeks_str': '|'.join(sorted(weeks_values)),
                'details': '同一课程+节次下周次不一致'
            })


def aggregate_free_time(csv_dir: str, jpg_dir: str, out_csv: str) -> None:
    # 枚举所有人(从文件名提取)
    paths = glob.glob(os.path.join(csv_dir, '课表_*_*.csv'))
    # 更精确：课表_姓名_学号_学期_种类.csv
    pat = re.compile(r'^课表_(.+?)_(\d+)_(\d+)_([01])\.csv$')

    # 收集每个人的基本信息
    person_keys: Set[Tuple[str,str,str]] = set()
    for p in os.listdir(csv_dir):
        m = pat.match(p)
        if m:
            person_keys.add((m.group(1), m.group(2), m.group(3)))

    if not person_keys:
        logger.error(f"未在 {csv_dir} 找到课表CSV")
        return

    # 总最大周次
    global_max_week = TOTAL_WEEKS

    # 每个单元格收集 [ (name, free_weeks_set) ]
    grid: Dict[Tuple[int,int], List[Tuple[str, Set[int]]]] = {}
    for d in range(5):
        for s in range(5):
            grid[(d,s)] = []

    # 问题清单
    issues: List[Dict] = []

    for name, sid, semester in sorted(person_keys):
        # 分别加载并检查种类0和1，然后合并用于统计忙碌周
        dfs: List[Tuple[int, pd.DataFrame, str]] = []
        for t in (0, 1):
            path = os.path.join(csv_dir, f"课表_{name}_{sid}_{semester}_{t}.csv")
            if os.path.exists(path):
                try:
                    df_t = pd.read_csv(path)
                    detect_conflicts(df_t, name, sid, semester, t, issues, source_csv=path)
                    dfs.append((t, df_t, path))
                except Exception as e:
                    logger.warning(f"读取或检查失败: {path}: {e}")

        if not dfs:
            logger.warning(f"无CSV: {name}-{sid}-{semester}")
            continue

        # 合并时标记来源类型
        df_with_type: List[pd.DataFrame] = []
        for t, df_t, path in dfs:
            df_tmp = df_t.copy()
            df_tmp['_source_type'] = t
            df_tmp['_source_csv'] = path
            df_with_type.append(df_tmp)
        df = pd.concat(df_with_type, ignore_index=True)

        # 遍历记录，统计忙碌周
        busy: Dict[Tuple[int,int], Set[int]] = {}
        for d in range(5):
            for s in range(5):
                busy[(d,s)] = set()

        for _, row in df.iterrows():
            jieci = str(row.get('节次', '')).strip()
            weeks_str = str(row.get('周次', '')).strip()
            source_type = int(row.get('_source_type', 0))
            source_csv = str(row.get('_source_csv', ''))

            # 若种类1且启用全占用策略，跳过逐行节次解析与错误记录（后续统一处理）
            if TYPE1_FULL_OCCUPY and source_type == 1:
                continue

            pj = parse_jieci(jieci)
            if not pj:
                # 忽略周六/周日，不作为错误
                if any(ch in jieci for ch in ['六', '日', '天']):
                    continue
                # 节次无法解析
                issues.append({
                    'issue_type': 'unparseable_jieci',
                    'name': name,
                    'student_id': sid,
                    'semester': semester,
                    'image_type': source_type,
                    'source_csv': source_csv,
                    'course': str(row.get('课程', '')),
                    'jieci': jieci,
                    'weeks_str': weeks_str,
                    'details': '节次无法解析'
                })
                continue
            day_idx, sections, parity = pj
            slots = sections_to_big_slots(sections)
            weeks, maxw, bad_parts = parse_weeks(weeks_str, global_max_week)
            if bad_parts:
                issues.append({
                    'issue_type': 'unparseable_weeks',
                    'name': name,
                    'student_id': sid,
                    'semester': semester,
                    'image_type': source_type,
                    'source_csv': source_csv,
                    'course': str(row.get('课程', '')),
                    'jieci': jieci,
                    'weeks_str': weeks_str,
                    'details': f"无法解析片段: {','.join(bad_parts)}"
                })
            # 如果节次里有单/双周标记，则据此进一步过滤周次
            if parity == '单':
                weeks = {w for w in weeks if w % 2 == 1}
            elif parity == '双':
                weeks = {w for w in weeks if w % 2 == 0}
            global_max_week = max(global_max_week, maxw)
            for sidx in slots:
                busy[(day_idx, sidx)].update(weeks)

        # 如果开启 TYPE1_FULL_OCCUPY 并存在种类1文件，则把其所有周次作为占用
        if TYPE1_FULL_OCCUPY:
            t1_path = os.path.join(csv_dir, f"课表_{name}_{sid}_{semester}_1.csv")
            if os.path.exists(t1_path):
                try:
                    df_type1 = pd.read_csv(t1_path)
                    # 提取所有周次并视为全部工作日占用
                    all_weeks_type1: Set[int] = set()
                    for wraw in df_type1.get('周次', []):
                        ws, _, bad_parts = parse_weeks(str(wraw), global_max_week)
                        if bad_parts:
                            issues.append({
                                'issue_type': 'unparseable_weeks_type1',
                                'name': name,
                                'student_id': sid,
                                'semester': semester,
                                'image_type': 1,
                                'source_csv': t1_path,
                                'course': '[环节]',
                                'jieci': '',
                                'weeks_str': str(wraw),
                                'details': f"无法解析片段: {','.join(bad_parts)}"
                            })
                        all_weeks_type1.update(ws)
                    if all_weeks_type1:
                        for d in range(5):
                            for sidx in range(5):
                                busy[(d,sidx)].update(all_weeks_type1)
                except Exception as e:
                    logger.warning(f"读取种类1环节失败 {t1_path}: {e}")

        # 计算空闲周（全周集合减去忙碌周）
        all_weeks = set(range(1, global_max_week+1))
        for d in range(5):
            for s in range(5):
                free = all_weeks - busy[(d,s)]
                grid[(d,s)].append((name, free))

    # 计算调度范围：去掉复习周；若设置 EXCLUDE_LAST_N_WEEKS 也排除
    schedule_range = set(range(1, TOTAL_WEEKS+1))
    schedule_range -= set(EXAM_REVIEW_WEEKS)
    if EXCLUDE_LAST_N_WEEKS > 0:
        for w in range(TOTAL_WEEKS-EXCLUDE_LAST_N_WEEKS+1, TOTAL_WEEKS+1):
            schedule_range.discard(w)

    # 生成汇总表（人类可读）
    columns = ['周一','周二','周三','周四','周五']
    index = ['1-2','3-4','5-6','7-8','9-10']
    table = [["" for _ in range(5)] for _ in range(5)]

    for d in range(5):
        for s in range(5):
            # 先按空闲周集合分组，相同集合的人名用顿号连接
            weeks_to_names: Dict[Tuple[int, ...], List[str]] = {}
            for name, free in grid[(d,s)]:
                weeks_in_range = sorted(free & schedule_range)
                if not weeks_in_range:
                    continue
                key = tuple(weeks_in_range)
                weeks_to_names.setdefault(key, []).append(name)

            entries: List[str] = []
            for weeks_tuple, names in sorted(weeks_to_names.items(), key=lambda item: sorted(item[1])):
                weeks_set = set(weeks_tuple)
                desc = format_weeks(weeks_set, schedule_range)
                if desc == "(无空闲)":
                    continue
                name_str = "、".join(sorted(names))
                if desc == "":
                    entries.append(name_str)
                else:
                    entries.append(f"{name_str}({desc})")
            table[s][d] = " ".join(entries)

    df_out = pd.DataFrame(table, index=index, columns=columns)
    df_out.to_csv(out_csv, index=True, encoding='utf-8-sig')
    logger.info(f"汇总已生成(人类可读): {out_csv}")

    # 生成机器可读表格：结构与 summary 一致，每个单元格存放 "姓名[1,2,3] 姓名2[2,4]" 形式
    machine_table = [["" for _ in range(5)] for _ in range(5)]

    for d in range(5):
        for s in range(5):
            entries: List[str] = []
            for name, free in grid[(d, s)]:
                weeks_in_range = sorted(free & schedule_range)
                if not weeks_in_range:
                    continue
                weeks_str = ",".join(str(w) for w in weeks_in_range)
                entries.append(f"{name}[{weeks_str}]")
            machine_table[s][d] = " ".join(entries)

    df_machine = pd.DataFrame(machine_table, index=index, columns=columns)
    machine_path = os.path.join(os.path.dirname(out_csv), 'free_time_machine.csv')
    df_machine.to_csv(machine_path, index=True, encoding='utf-8-sig')
    logger.info(f"机器可读空闲周已生成(按节次/工作日表格): {machine_path}")

    # 输出问题清单
    issues_path = os.path.join(os.path.dirname(out_csv), 'schedule_issues.csv')
    if issues:
        df_issues = pd.DataFrame(issues)
        # 确保列顺序正确，包含 source_csv 以便回溯
        cols = ['issue_type', 'name', 'student_id', 'semester', 'image_type', 'source_csv', 'course', 'jieci', 'weeks_str', 'details']
        available_cols = [c for c in cols if c in df_issues.columns]
        df_issues = df_issues[available_cols]
        df_issues.to_csv(issues_path, index=False, encoding='utf-8-sig')

        # 额外按人汇总有问题的记录，方便人工核对
        problematic: Dict[Tuple[str, str], List[str]] = {}
        for it in issues:
            key = (it.get('name', ''), it.get('student_id', ''))
            desc = f"{it.get('issue_type', '')} - 课程:{it.get('course', '')} 节次:{it.get('jieci', '')} 周次:{it.get('weeks_str', '')}"
            problematic.setdefault(key, []).append(desc)

        if problematic:
            logger.warning("以下同学的课表CSV存在无法自动解析/存在冲突的条目，请人工核对矫正后再重新聚合：")
            for (p_name, p_sid), items in sorted(problematic.items()):
                logger.warning(f"  {p_name}({p_sid}) 共 {len(items)} 条可疑记录，例如: {items[0]}")

        logger.info(f"问题清单已生成: {issues_path} (共{len(issues)}条)")
    else:
        logger.info("未发现解析/冲突问题")


def main():
    import argparse
    parser = argparse.ArgumentParser(description='校验/修复课表CSV并汇总空闲时间')
    parser.add_argument('--csv-dir', default=CSV_DIR, help='CSV目录（脚本内已配置）')
    parser.add_argument('--jpg-dir', default=JPG_DIR, help='图片目录（脚本内已配置）')
    parser.add_argument('--out', default=OUT_CSV_PATH, help='输出CSV路径（脚本内已配置）')
    args = parser.parse_args()

    if not os.path.exists(args.csv_dir):
        logger.error(f"CSV 目录不存在: {args.csv_dir}")
        return
    if not os.path.exists(args.jpg_dir):
        logger.warning(f"图片目录不存在: {args.jpg_dir}")

    aggregate_free_time(args.csv_dir, args.jpg_dir, args.out)


if __name__ == '__main__':
    main()
