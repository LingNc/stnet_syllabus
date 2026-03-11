import csv
import re
import io
import datetime
from datetime import timedelta
from ics import Calendar, Event
from ics.alarm import DisplayAlarm
import pytz # 依赖 pytz 库

# --- 1. 配置项 ---
# 输入你的CSV文件路径
CSV_PATH = "class_cal/final_schedule.csv" # 这是你设置的路径
# !重要: 请将此日期修改为你学期第一天的周一
SEMESTER_START_DATE = datetime.date(2025, 9, 1) # 示例日期，请修改

# 指定你的时区
TIMEZONE = pytz.timezone("Asia/Shanghai")
# 定义 UTC 时区
UTC_TIMEZONE = pytz.utc

# !重要: 你更新后的作息时间
PERIOD_TIMES = {
    # ... (你的时间表保持不变) ...
    1: {'start': datetime.time(8, 0), 'end': datetime.time(8, 45)},
    2: {'start': datetime.time(8, 55), 'end': datetime.time(9, 40)},
    3: {'start': datetime.time(10, 0), 'end': datetime.time(10, 45)},
    4: {'start': datetime.time(10, 55), 'end': datetime.time(11, 40)},
    5: {'start': datetime.time(14, 30), 'end': datetime.time(15, 15)},
    6: {'start': datetime.time(15, 25), 'end': datetime.time(16, 10)},
    7: {'start': datetime.time(16, 30), 'end': datetime.time(17, 15)},
    8: {'start': datetime.time(17, 25), 'end': datetime.time(18, 10)},
    9: {'start': datetime.time(19, 30), 'end': datetime.time(20, 15)},
    10: {'start': datetime.time(20, 25), 'end': datetime.time(21, 10)},
}

# 提前多少分钟提醒
ALARM_TIME = 15

# ... (DAY_MAP 保持不变) ...
DAY_MAP = {
    '一': 0,
    '二': 1,
    '三': 2,
    '四': 3,
    '五': 4,
    '六': 5,
    '日': 6,
}

# --- 2. 课程表数据 ---
CSV_DATA: str

# 从CSV文件读取数据
try:
    with open(CSV_PATH, 'r', encoding='utf-8') as f:
        CSV_DATA = f.read()
except FileNotFoundError:
    print(f"错误: 找不到 {CSV_PATH} 文件")
    CSV_DATA = ""
except Exception as e:
    print(f"读取文件 {CSV_PATH} 时出错: {e}")
    CSV_DATA = ""


# --- 3. 辅助解析函数 ---
# (parse_course_name, parse_time_slot, get_recurring_week_groups 保持不变)

def parse_course_name(name_str):
    if not name_str:
        return None
    match = re.match(r'\[.*?\](.*)', name_str)
    if match:
        return match.group(1).strip()
    return name_str.strip()

def parse_time_slot(time_str):
    match = re.match(r'(一|二|三|四|五|六|日)\[(\d+)-(\d+)节\](单|双)?', time_str)
    if not match:
        return None
    day_str, start_p, end_p, parity_str = match.groups()
    day_of_week = DAY_MAP.get(day_str)
    start_period = int(start_p)
    end_period = int(end_p)
    if parity_str == '单':
        parity = 'odd'
    elif parity_str == '双':
        parity = 'even'
    else:
        parity = 'all'
    return day_of_week, start_period, end_period, parity

def get_recurring_week_groups(week_str, parity):
    active_weeks = set()
    ranges = re.findall(r'(\d+)-(\d+)', week_str)
    if not ranges and week_str.strip():
        single_weeks = re.findall(r'(\d+)', week_str)
        ranges = [(w, w) for w in single_weeks]

    for start_str, end_str in ranges:
        start_week = int(start_str)
        end_week = int(end_str)
        for week_num in range(start_week, end_week + 1):
            if parity == 'all':
                active_weeks.add(week_num)
            elif parity == 'odd' and week_num % 2 != 0:
                active_weeks.add(week_num)
            elif parity == 'even' and week_num % 2 == 0:
                active_weeks.add(week_num)
    if not active_weeks:
        return []
    sorted_weeks = sorted(list(active_weeks))
    groups = []
    if not sorted_weeks:
        return groups
    interval = 1 if parity == 'all' else 2
    current_group_start = sorted_weeks[0]
    current_group_count = 1
    for i in range(1, len(sorted_weeks)):
        expected_next_week = sorted_weeks[i-1] + interval
        if sorted_weeks[i] == expected_next_week:
            current_group_count += 1
        else:
            groups.append((current_group_start, current_group_count, interval))
            current_group_start = sorted_weeks[i]
            current_group_count = 1
    groups.append((current_group_start, current_group_count, interval))
    return groups

# --- 4. 主函数 ---

# 用于存储每个事件的 RRULE 和 DURATION 信息
event_rrules = {}  # 格式: {event_uid: rrule_str}
event_durations = {}  # 格式: {event_uid: duration_seconds}

def create_schedule_ics(csv_content, semester_start_date):
    if semester_start_date.weekday() != 0:
        raise ValueError(f"学期开始日期 {semester_start_date} 不是周一！请修改 SEMESTER_START_DATE。")

    calendar = Calendar()
    f = io.StringIO(csv_content)
    reader = csv.DictReader(f)
    processed_events = 0
    skipped_rows = 0

    for row in reader:
        try:
            course_name = parse_course_name(row['课程'])
            if not course_name:
                skipped_rows += 1
                continue
            teacher = row['任课老师']
            location = row['地点']
            week_str = row['周次']
            time_str = row['节次']
            time_slot = parse_time_slot(time_str)
            if not time_slot:
                print(f"警告: 无法解析节次 '{time_str}' (课程: {course_name})，已跳过。")
                skipped_rows += 1
                continue
            day_of_week, start_period, end_period, parity = time_slot
            try:
                start_time = PERIOD_TIMES[start_period]['start']
                end_time = PERIOD_TIMES[end_period]['end']
            except KeyError:
                print(f"警告: 找不到节次 {start_period} 或 {end_period} 的时间 (课程: {course_name})，已跳过。请检查 PERIOD_TIMES 设置。")
                skipped_rows += 1
                continue
            recurrence_groups = get_recurring_week_groups(week_str, parity)
            if not recurrence_groups:
                print(f"警告: 无法解析周次 '{week_str}' (课程: {course_name})，已跳过。")
                skipped_rows += 1
                continue

            for start_week, count, interval in recurrence_groups:
                # --- (1) 计算开始时间 (本地时区) ---
                days_to_add = (start_week - 1) * 7 + day_of_week
                event_date = semester_start_date + datetime.timedelta(days=days_to_add)
                naive_start_datetime = datetime.datetime.combine(event_date, start_time)
                naive_end_datetime = datetime.datetime.combine(event_date, end_time)
                start_datetime = TIMEZONE.localize(naive_start_datetime)
                end_datetime = TIMEZONE.localize(naive_end_datetime)

                # --- (2) 计算持续时间 (用于 DURATION 而不是 DTEND) ---
                duration = end_datetime - start_datetime

                # --- (3) 创建事件 ---
                e = Event()
                e.name = course_name
                e.begin = start_datetime
                e.end = end_datetime
                e.location = location
                e.description = teacher

                # --- (4) 构建 RRULE 字符串 (简化格式，仅用 COUNT，不用 UNTIL) ---
                # 根据 day_of_week 转换为 BYDAY 格式
                day_names = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU']
                byday = day_names[day_of_week]

                if interval == 1:
                    # 每周一次
                    rrule_str = f'FREQ=WEEKLY;WKST=MO;COUNT={count};BYDAY={byday}'
                else:
                    # 隔周
                    rrule_str = f'FREQ=WEEKLY;WKST=MO;COUNT={count};INTERVAL={interval};BYDAY={byday}'

                event_rrules[e.uid] = rrule_str
                # 保存 duration（秒数）
                event_durations[e.uid] = int(duration.total_seconds())
                # --- 修改结束 ---

                # --- (5) 添加提醒 (ALARM_TIME 分钟) ---
                alarm = DisplayAlarm(trigger=timedelta(minutes=-ALARM_TIME))
                e.alarms.append(alarm)

                calendar.events.add(e)
                processed_events += 1

        except Exception as e:
            print(f"处理行 {row} 时发生致命错误: {e}")
            skipped_rows += 1

    print(f"处理完成。共创建 {processed_events} 个(含循环)日历事件，跳过 {skipped_rows} 行无效数据。")
    return calendar, event_rrules, event_durations

# --- 辅助函数：将 RRULE 注入到 ICS 内容中，并优化格式 ---
def inject_rrules_to_ics(ics_content: str, rrule_dict: dict, duration_dict: dict) -> str:
    """
    将 event_rrules 字典中的 RRULE 注入到序列化的 ICS 内容中。
    同时：
    1. 删除 DTEND 行
    2. 将 DTSTART 的时间戳改为本地时间格式 (TZID=Asia/Shanghai)
    3. 添加动态计算的 DURATION、TRANSP、STATUS 等字段
    4. 在 SUMMARY 后面插入 RRULE
    """
    lines = ics_content.split('\n')
    result_lines = []

    # 分块处理每个 VEVENT
    i = 0
    while i < len(lines):
        line = lines[i]

        # 识别 VEVENT 块的开始
        if line.startswith('BEGIN:VEVENT'):
            event_lines = [line]
            i += 1

            # 读取整个 VEVENT 块
            while i < len(lines) and not lines[i].startswith('END:VEVENT'):
                event_lines.append(lines[i])
                i += 1

            if i < len(lines):
                event_lines.append(lines[i])  # END:VEVENT

            # 首先找到 UID（用于后续查询 duration）
            event_uid = None
            for eline in event_lines:
                if eline.startswith('UID:'):
                    event_uid = eline.replace('UID:', '').strip()
                    break

            # 处理这个块：找到 SUMMARY 等
            has_summary = False
            processed_event = []

            for j, eline in enumerate(event_lines):
                # 跳过 UID 行，稍后重新添加
                if eline.startswith('UID:'):
                    continue

                # 处理 DTSTART
                if eline.startswith('DTSTART:'):
                    try:
                        utc_str = eline.replace('DTSTART:', '').strip()
                        if utc_str.endswith('Z'):
                            utc_dt = datetime.datetime.strptime(utc_str, '%Y%m%dT%H%M%SZ')
                            utc_dt = UTC_TIMEZONE.localize(utc_dt)
                            local_dt = utc_dt.astimezone(TIMEZONE)
                            local_str = local_dt.strftime('%Y%m%dT%H%M%S')
                            processed_event.append(f'DTSTART;TZID=Asia/Shanghai:{local_str}')
                            continue
                    except Exception:
                        pass

                # 跳过 DTEND
                if eline.startswith('DTEND:'):
                    continue

                # 处理 SUMMARY：在后面添加字段和 RRULE
                if eline.startswith('SUMMARY:'):
                    processed_event.append(eline)
                    processed_event.append('TRANSP:OPAQUE')
                    processed_event.append('STATUS:CONFIRMED')
                    # 使用动态的 DURATION（如果没有，默认 1 小时）
                    duration_seconds = duration_dict.get(event_uid, 3600)
                    # 将秒数转换为 ISO 8601 DURATION 格式 (PThHmMsS)
                    hours = duration_seconds // 3600
                    minutes = (duration_seconds % 3600) // 60
                    seconds = duration_seconds % 60
                    if hours > 0:
                        duration_str = f'PT{hours}H{minutes}M{seconds}S'
                    elif minutes > 0:
                        duration_str = f'PT{minutes}M{seconds}S'
                    else:
                        duration_str = f'PT{seconds}S'
                    processed_event.append(f'DURATION:{duration_str}')
                    has_summary = True
                    continue

                # 其他行保持不变
                processed_event.append(eline)

            # 在 END:VEVENT 前插入 UID 和 RRULE
            if event_uid:
                # 插入 UID
                processed_event.insert(-1, f'UID:{event_uid}')
                # 插入 RRULE
                if event_uid in rrule_dict:
                    processed_event.insert(-1, f'RRULE:{rrule_dict[event_uid]}')

            # 将处理后的块加入结果
            result_lines.extend(processed_event)
            i += 1
        else:
            result_lines.append(line)
            i += 1

    return '\n'.join(result_lines)

# --- 5. 执行脚本 ---

if __name__ == "__main__":
    if not CSV_DATA:
        print("CSV_DATA 为空，程序退出。请检查CSV文件路径或文件内容。")
    else:
        try:
            start_date = SEMESTER_START_DATE

            course_calendar, rrule_dict, duration_dict = create_schedule_ics(CSV_DATA, start_date)

            # 获取序列化的 ICS 内容
            ics_content = course_calendar.serialize()

            # 注入 RRULE 和动态 DURATION
            ics_content_with_rrule = inject_rrules_to_ics(ics_content, rrule_dict, duration_dict)

            # 写入文件
            output_filename = 'ics/course_schedule_final_compatible.ics'
            with open(output_filename, 'w', encoding='utf-8') as f:
                f.write(ics_content_with_rrule)

            print(f"\n成功！课程表已生成: {output_filename}")
            print(f"此文件已使用【RRULE 循环规则】，格式兼容小米日历及其他标准日历应用。")

        except ValueError as e:
            print(f"\n【配置错误】: {e}")
        except Exception as e:
            print(f"\n【意外错误】: {e}")