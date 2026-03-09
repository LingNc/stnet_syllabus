#!/usr/bin/env python3
"""根据指定周次，从机器可读节次表格生成该周的节次空闲表。

输出结构:
    行: 大节(与 free_time_machine.csv 相同, 如 1-2,3-4,...)
    列: 周一..周五
    单元格: 该周此节次无课的所有人名, 用顿号连接。
"""

import os
import re
import sys
import pandas as pd

BASE_DIR = '/home/lingnc/Ideas/stnet'
MACHINE_FREE_CSV = os.path.join(BASE_DIR, 'free_time_machine.csv')


def parse_cell(cell: str) -> dict:
    """解析单元格，返回 {name: [week,...]}。"""
    result: dict = {}
    if not cell or not str(cell).strip():
        return result
    text = str(cell)
    pattern = r'(\S+?)\[([^\]]*)\]'
    for name, weeks_str in re.findall(pattern, text):
        name = name.strip()
        if not name:
            continue
        weeks = []
        for w in weeks_str.split(',') if weeks_str.strip() else []:
            w = w.strip()
            if not w:
                continue
            try:
                weeks.append(int(w))
            except ValueError:
                continue
        result[name] = sorted(set(weeks))
    return result


def main():
    if len(sys.argv) < 2:
        print("用法: python3 filter_free_week.py <周次> [输出文件]")
        print("例如: python3 filter_free_week.py 10")
        print("      python3 filter_free_week.py 10 output.csv")
        sys.exit(1)

    try:
        target_week = int(sys.argv[1])
    except ValueError:
        print(f"错误: 周次 '{sys.argv[1]}' 不是有效的整数")
        sys.exit(1)

    if target_week < 1 or target_week > 21:
        print(f"错误: 周次 {target_week} 超出范围 (1-21)")
        sys.exit(1)

    output_file = sys.argv[2] if len(sys.argv) > 2 else os.path.join(BASE_DIR, f'free_week_{target_week}.csv')

    if not os.path.exists(MACHINE_FREE_CSV):
        raise FileNotFoundError(f'机器可读节次表不存在: {MACHINE_FREE_CSV}')

    df_free_full = pd.read_csv(MACHINE_FREE_CSV, index_col=0)
    print(f"读取机器可读节次表: {MACHINE_FREE_CSV}")
    print(f"查询周次: 第{target_week}周")

    # 统计全体成员以及本周是否至少有一次空闲
    all_names: set = set()
    names_with_free: set = set()
    for cell in df_free_full.to_numpy().flatten():
        info = parse_cell(str(cell) if not pd.isna(cell) else "")
        for name, weeks in info.items():
            all_names.add(name)
            if target_week in weeks:
                names_with_free.add(name)

    # 只保留前四个大节，并把行标签改为“第一大节”..“第四大节”
    df_free = df_free_full.iloc[:4, :]
    new_index = ["第一大节", "第二大节", "第三大节", "第四大节"][: len(df_free.index)]

    # 构造结果表
    result = pd.DataFrame(index=new_index, columns=df_free.columns)

    # 统计这一周至少有一次空闲的所有人
    any_free_names: set = set(names_with_free)

    for i, slot_idx in enumerate(df_free.index):
        row_label = new_index[i]
        for day in df_free.columns:
            cell = df_free.loc[slot_idx, day]
            info = parse_cell(str(cell) if not pd.isna(cell) else "")
            names = [name for name, weeks in info.items() if target_week in weeks]
            names_sorted = sorted(set(names))
            if names_sorted:
                any_free_names.update(names_sorted)
            result.loc[row_label, day] = "、".join(names_sorted)

    result.to_csv(output_file, encoding='utf-8-sig')
    print(f"\n已保存第{target_week}周按节次无课人员表到: {output_file}")

    # 控制台输出这一周至少有一次空闲的统计
    summary_names = sorted(any_free_names)
    print(f"第{target_week}周共有 {len(summary_names)} 人至少有一次无课时间")
    if summary_names:
        print(",".join(summary_names))

    no_free_names = sorted(all_names - names_with_free)
    print(f"第{target_week}周共有 {len(no_free_names)} 人没有无课时间")
    if no_free_names:
        print(",".join(no_free_names))

if __name__ == '__main__':
    main()
