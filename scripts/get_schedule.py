#!/usr/bin/env python3
"""
郑州轻工业大学教务系统课程表获取脚本

功能：
1. 加载已保存的 Cookie
2. 探索教务系统，找到教学安排/课程表入口
3. 获取最新学期的课程表
4. 导出为 XLS 格式

前置条件：
- 必须先运行 login_agent.py 成功登录并保存 cookie
- cookies.json 文件必须存在于同一目录

教务系统常见结构：
- 首页 -> 教学安排/我的课表/学生课表查询
- 课程表页面通常有列表视图和二维表视图两种模式
- 导出功能通常提供 XLS/Excel 下载
"""

import requests
import json
import time
import re
from pathlib import Path
from urllib.parse import urljoin, parse_qs, urlparse
from datetime import datetime


class ScheduleExplorer:
    """
    课程表探索器

    探索流程：
    1. 加载 Cookie
    2. 访问教务系统首页
    3. 寻找"教学安排"、"我的课表"等相关菜单/链接
    4. 进入课程表查询页面
    5. 分析页面参数（学期、格式等）
    6. 获取课程表数据（列表格式）
    7. 导出为 XLS
    """

    def __init__(self):
        # 脚本所在目录
        self.script_dir = Path(__file__).parent.absolute()
        # cookie 目录
        self.cookie_dir = self.script_dir / "cookie"
        # xls 输出目录
        self.xls_dir = self.script_dir / "xls"
        self.xls_dir.mkdir(exist_ok=True)

        self.cookie_file = None  # 将在 load_cookies 中设置
        self.jwgl_base_url = "https://jwgl.zzuli.edu.cn"
        self.session = requests.Session()
        self.session.headers.update({
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
            "Accept-Encoding": "gzip, deflate, br",
            "Cache-Control": "no-cache",
            "Pragma": "no-cache",
            "Sec-Fetch-Dest": "iframe",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "same-origin",
            "Upgrade-Insecure-Requests": "1",
        })

        self.discovered_urls = {}  # 发现的 URL 字典
        self.schedule_data = None
        self.current_xn_xq = None  # 当前学年学期
        self.home_url = None  # 主页URL，用于Referer
        self.student_name = None  # 学生姓名
        self.student_id = None  # 学生学号

    def load_cookies(self):
        """
        步骤 1: 从 cookie/ 目录加载最新的 Cookie 文件

        返回：
            bool: 是否成功加载
        """
        print("[1/7] 正在加载 Cookie...")

        if not self.cookie_dir.exists():
            print(f"[✗] Cookie 目录不存在: {self.cookie_dir}")
            print("[*] 请先运行 login.py 完成登录")
            return False

        # 查找所有 cookie_*.json 文件
        cookie_files = sorted(self.cookie_dir.glob("cookie_*.json"))
        if not cookie_files:
            print(f"[✗] 未找到 Cookie 文件")
            print(f"[*] 请在 {self.cookie_dir} 目录中放置 cookie 文件")
            print("[*] 或先运行 login.py 完成登录")
            return False

        # 使用最新的 cookie 文件
        self.cookie_file = cookie_files[-1]

        try:
            with open(self.cookie_file, "r", encoding="utf-8") as f:
                data = json.load(f)

            cookies = data.get("cookies", {})
            if not cookies:
                print("[✗] Cookie 文件中没有 cookie 数据")
                return False

            # 设置 cookie 到 session
            for name, value in cookies.items():
                self.session.cookies.set(name, value)

            # 获取学号
            self.student_id = data.get("student_id")

            print(f"[✓] 成功加载 Cookie: {self.cookie_file.name}")
            print(f"[i] Cookie 时间: {data.get('datetime', 'unknown')}")
            if self.student_id:
                print(f"[i] 学号: {self.student_id}")
            return True

        except Exception as e:
            print(f"[✗] 加载 Cookie 失败: {e}")
            return False

    def verify_session(self, max_retries=1, retry_delay=5):
        """
        步骤 2: 验证 Session 是否有效

        青果系统流程：需要先访问 /caslogin 来激活 CASTGC ticket，
        然后才能访问其他功能页面。

        参数：
            max_retries: 网络错误时最大重试次数（默认1次）
            retry_delay: 重试间隔秒数（默认5秒）

        返回：
            bool: Session 是否有效
        """
        print("[2/7] 正在验证登录状态...")

        attempt = 0
        while attempt <= max_retries:
            try:
                # 步骤 2.1: 访问 caslogin 激活 session
                print("[*] 正在激活 CAS session...")
                cas_resp = self.session.get(
                    f"{self.jwgl_base_url}/caslogin",
                    timeout=15,
                    allow_redirects=True
                )

                # 检查是否成功跳转到了主页
                if "frame/homes" in cas_resp.url or "frame/home" in cas_resp.url:
                    print(f"[✓] CAS session 激活成功")
                    print(f"[i] 当前页面: {cas_resp.url}")
                    self.home_page_content = cas_resp.text
                    self.home_page_url = cas_resp.url
                    self.session_active = True
                    return True

                # 如果被重定向到登录页，说明 CASTGC 已过期
                if "caslogin" in cas_resp.url and cas_resp.url.endswith("caslogin"):
                    print("[✗] CASTGC 已过期，需要重新登录")
                    return False

                # 尝试直接访问主页
                resp = self.session.get(self.jwgl_base_url, timeout=15, allow_redirects=True)
                final_url = resp.url

                # 检查是否被重定向到登录页
                if "caslogin" in final_url or "kys.zzuli.edu.cn/cas/login" in final_url:
                    print("[✗] Session 已过期，需要重新登录")
                    return False

                # 保存首页内容供后续分析
                self.home_page_content = resp.text
                self.home_page_url = final_url
                self.session_active = True

                # 从主页面提取当前学年学期
                self._extract_year_term_from_homepage(resp.text)

                print(f"[✓] 登录状态有效")
                print(f"[i] 当前页面: {final_url}")
                return True

            except (requests.exceptions.ConnectionError, requests.exceptions.Timeout) as e:
                attempt += 1
                if attempt <= max_retries:
                    print(f"[!] 网络连接失败: {e}")
                    print(f"[*] {retry_delay}秒后重试 ({attempt}/{max_retries})...")
                    time.sleep(retry_delay)
                else:
                    print(f"[✗] 验证 Session 失败: {e}")
                    print(f"[*] 已达到最大重试次数 ({max_retries})")
                    return False
            except Exception as e:
                print(f"[✗] 验证 Session 失败: {e}")
                return False

    def discover_menu_links(self):
        """
        步骤 3: 探索菜单链接，找到课程表相关入口

        教务系统常见的课程表入口：
        - /jsxsd/xskb/xskb_list.do (经典正方教务)
        - /kbcx/xskbcx (课程查询)
        - /xsgrkbcx (学生个人课程查询)
        - /xskb (课表)

        返回：
            dict: 发现的 URL 字典
        """
        print("[3/7] 正在探索课程表入口...")

        # 常见课程表 URL 模式
        possible_paths = [
            "/jsxsd/xskb/xskb_list.do",      # 正方教务经典路径
            "/jsxsd/xskb/xskb_query.do",
            "/kbcx/xskbcx",
            "/kbcx/xs_kb",
            "/xsgrkbcx",                      # 学生个人课表查询
            "/xskb",
            "/kbxx/xskb",
            "/student/xskb",
            "/course/schedule",
            "/jsxsd/framework/xskbcx_list",
        ]

        discovered = {}

        for path in possible_paths:
            url = urljoin(self.jwgl_base_url, path)
            try:
                resp = self.session.head(url, timeout=10, allow_redirects=True)
                if resp.status_code == 200:
                    discovered[path] = url
                    print(f"[✓] 发现可能入口: {path}")
            except:
                pass

        # 从首页内容中解析链接
        if hasattr(self, 'home_page_content'):
            # 查找包含"课表"、"课程"、"教学"等关键词的链接
            patterns = [
                r'href=["\']([^"\']*xskb[^"\']*)["\']',
                r'href=["\']([^"\']*kbcx[^"\']*)["\']',
                r'href=["\']([^"\']*kebiao[^"\']*)["\']',
                r'href=["\']([^"\']*course[^"\']*schedule[^"\']*)["\']',
                r'href=["\']([^"\']*jsxsd[^"\']*)["\']',
            ]

            for pattern in patterns:
                matches = re.findall(pattern, self.home_page_content, re.IGNORECASE)
                for match in matches:
                    if match.startswith("http"):
                        discovered[match] = match
                    else:
                        full_url = urljoin(self.jwgl_base_url, match)
                        discovered[match] = full_url
                    print(f"[✓] 从首页发现: {match}")

        self.discovered_urls = discovered

        if not discovered:
            print("[!] 未自动发现课程表入口，将使用默认路径尝试")
            self.discovered_urls = {
                "default": f"{self.jwgl_base_url}/jsxsd/xskb/xskb_list.do"
            }

        return discovered

    def _fetch_year_term_from_api(self):
        """
        通过API获取当前学年学期信息

        API: POST /frame/desk/showYearTerm4User.action
        返回: {"xn":"2025","xq_m":"1","xnxqDesc":"2025-2026学年第二学期"}
        学期编码：xq_m=0 表示第一学期, xq_m=1 表示第二学期
        """
        try:
            api_url = f"{self.jwgl_base_url}/frame/desk/showYearTerm4User.action"
            resp = self.session.post(api_url, timeout=15)
            resp.raise_for_status()

            data = resp.json()
            self.current_xn = data.get("xn")
            self.current_xq = data.get("xq_m")

            if self.current_xn and self.current_xq:
                print(f"[✓] 从API获取学年学期: xn={self.current_xn}, xq={self.current_xq} ({data.get('xnxqDesc', '')})")
                return True
            else:
                print("[✗] API返回数据不完整")
                return False

        except Exception as e:
            print(f"[✗] 获取学年学期API失败: {e}")
            return False

    def get_current_semester(self):
        """
        获取当前学年学期

        中国高校通常格式：
        - 2024-2025-1 (第一学期/秋季)
        - 2024-2025-2 (第二学期/春季)

        返回：
            tuple: (学年, 学期, 教学周) 如 ("2024-2025", "1", "8")
        """
        now = datetime.now()
        year = now.year
        month = now.month

        # 学年计算：9月-次年8月为一年
        if month >= 9:
            xn = f"{year}-{year+1}"
            xq = "1"  # 第一学期
        elif month <= 2:
            xn = f"{year-1}-{year}"
            xq = "1"
        else:
            xn = f"{year-1}-{year}"
            xq = "2"  # 第二学期

        # 默认教学周
        jxz = "8"

        self.current_xn_xq = (xn, xq, jxz)
        print(f"[i] 当前学年学期: {xn} 第{xq}学期，教学周: {jxz}")
        return xn, xq, jxz

    def fetch_schedule_page(self):
        """
        步骤 4: 获取课程表页面

        使用官方导出API直接下载XLS文件：
        /wsxk/xkjg.ckdgxsxdkchj_data_exp10319.jsp?params=base64(xn=2025&xq=1&xh=学号)

        返回：
            response: 页面响应
        """
        print("[4/7] 正在获取课程表...")

        # 步骤1: 访问课程表主页面获取学号
        kb_main_url = f"{self.jwgl_base_url}/student/xkjg.wdkb.jsp?menucode=S20301"
        try:
            print(f"[*] 访问课程表页面: {kb_main_url}")
            main_resp = self.session.get(kb_main_url, timeout=15)
            main_resp.raise_for_status()

            # 使用 cookie 中的学号（login.py 已正确获取）
            # 如果 cookie 中没有学号，尝试从 SetMainInfo.jsp 获取
            if not self.student_id:
                print("[!] Cookie 中没有学号，尝试从 SetMainInfo.jsp 获取...")
                try:
                    info_resp = self.session.get(
                        f"{self.jwgl_base_url}/frame/home/js/SetMainInfo.jsp",
                        timeout=15
                    )
                    match = re.search(r'var\s+_loginid\s*=\s*["\'](\d+)["\']', info_resp.text)
                    if match:
                        self.student_id = match.group(1)
                        print(f"[✓] 从 SetMainInfo.jsp 获取到学号: {self.student_id}")
                    else:
                        print("[✗] 无法从 SetMainInfo.jsp 获取学号")
                        return None
                except Exception as e:
                    print(f"[✗] 获取学号失败: {e}")
                    return None
            else:
                print(f"[✓] 使用 Cookie 中的学号: {self.student_id}")

            xh = self.student_id

            # 从页面提取学生姓名
            # 查找包含姓名的元素，如 <span class="user-name">张三</span>
            name_match = re.search(r'<span[^>]*user[^>]*>([^<]+)</span>', main_resp.text, re.IGNORECASE)
            if not name_match:
                name_match = re.search(r'欢迎您[^>]*>([^<]+)</', main_resp.text)
            if name_match:
                self.student_name = name_match.group(1).strip()
                print(f"[✓] 获取到姓名: {self.student_name}")
            else:
                self.student_name = "未知"
                print("[!] 未能获取姓名，使用默认值")

            # 提取学年学期（调用API获取）
            print("[*] 获取当前学年学期...")
            if not self._fetch_year_term_from_api():
                print("[✗] 错误：无法获取学年学期")
                return None

            xn = self.current_xn
            xq = self.current_xq

        except Exception as e:
            print(f"[✗] 获取课程表页面失败: {e}")
            return None

        # 步骤2: 调用官方导出API获取XLS
        import base64

        params_str = f"xn={xn}&xq={xq}&xh={xh}"
        params_b64 = base64.b64encode(params_str.encode()).decode()

        export_url = f"{self.jwgl_base_url}/wsxk/xkjg.ckdgxsxdkchj_data_exp10319.jsp?params={params_b64}"

        headers = {
            "Referer": kb_main_url,
            "Sec-Fetch-Dest": "iframe",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "same-origin",
        }

        try:
            print(f"[*] 请求官方导出: {export_url}")
            resp = self.session.get(export_url, headers=headers, timeout=15)
            resp.raise_for_status()

            # 检查返回的是否是XLS文件
            content_type = resp.headers.get('Content-Type', '')
            print(f"[*] Content-Type: {content_type}")

            if 'excel' in content_type.lower() or 'ms-excel' in content_type.lower():
                print(f"[✓] 成功获取XLS文件")
                print(f"[i] 文件大小: {len(resp.content)} 字节")
                self.current_xn_xq = (xn, xq, "8")
                self.xh = xh
                return resp
            elif "错误" in resp.text or "凭证" in resp.text:
                print("[✗] Session已过期或权限不足")
                return None
            else:
                print(f"[!] 返回内容可能不是XLS")
                print(f"[i] 内容预览: {resp.text[:200]}")
                return None

        except Exception as e:
            print(f"[✗] 官方导出失败: {e}")
            return None

    def run(self):
        """
        执行完整的课程表获取流程
        """
        print("=" * 60)
        print("郑州轻工业大学课程表获取工具")
        print("=" * 60)
        print()

        # 步骤 1: 加载 Cookie
        if not self.load_cookies():
            return False

        # 步骤 2: 验证 Session
        if not self.verify_session():
            print("\n[*] 提示: 请先运行 login.py 重新登录")
            return False

        # 步骤 3: 探索菜单
        self.discover_menu_links()

        # 步骤 4: 获取课程表页面
        resp = self.fetch_schedule_page()
        if not resp:
            print("\n[!] 无法获取课程表页面")
            return False

        # 直接保存XLS文件
        print("\n[✓] 成功获取XLS文件")
        xn, xq, jxz = self.current_xn_xq if self.current_xn_xq else self.get_current_semester()

        # 构建文件名: <姓名>_<学号>_<学年学期>.xls
        # 学年学期格式: 20250 (2025学年第一学期) 或 20251 (2025学年第二学期)
        semester_code = f"{xn}{xq}"
        name = self.student_name or "未知"
        student_id = self.student_id or "unknown"
        filename = f"{name}_{student_id}_{semester_code}.xls"
        filepath = self.xls_dir / filename

        with open(filepath, "wb") as f:
            f.write(resp.content)
        print(f"[✓] 课程表已导出: {filepath}")
        print(f"[i] 文件大小: {len(resp.content)} 字节")
        return True


def main():
    """主函数"""
    explorer = ScheduleExplorer()
    success = explorer.run()

    print("\n" + "=" * 60)
    if success:
        print("课程表获取完成！")
    else:
        print("课程表获取遇到问题，请查看日志")
    print("=" * 60)


if __name__ == "__main__":
    main()
