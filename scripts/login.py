#!/usr/bin/env python3
"""
郑州轻工业大学教务系统扫码登录脚本

功能：
1. 访问统一身份认证页面
2. 获取二维码图片
3. 轮询登录状态
4. 保存登录后的 Cookie

使用流程：
1. 运行脚本
2. 扫描二维码（使用"i轻工大"APP）
3. 等待登录完成
4. Cookie 将保存在 cookies.json 文件中

技术细节：
- 该网站使用 CAS (Central Authentication Service) 统一认证
- 扫码登录基于二维码 token 轮询机制
- 登录成功后会跳转到教务系统并携带 ticket 参数
- Cookie 中包含 SESSION 等关键会话信息
"""

import requests
import time
import json
import re
import os
import uuid as uuid_module
import io
from urllib.parse import urljoin, urlparse, parse_qs
from pathlib import Path

# 二维码生成库
try:
    import qrcode
    from PIL import Image
    HAS_QR = True
except ImportError:
    HAS_QR = False


class ZZULICASLoginAgent:
    """
    郑州轻工业大学 CAS 登录代理

    登录流程说明：
    1. 访问登录页 -> 获取二维码 token
    2. 获取二维码图片 -> 展示给用户扫码
    3. 轮询登录状态 -> 等待用户确认
    4. 登录成功 -> 获取 Cookie 和 ticket
    5. 验证登录 -> 访问教务系统确认登录有效
    """

    def __init__(self, cookie_file="cookies.json"):
        # 基础 URL
        self.cas_base_url = "https://kys.zzuli.edu.cn/cas"
        self.jwgl_base_url = "https://jwgl.zzuli.edu.cn"

        # 创建 session 自动管理 cookie
        self.session = requests.Session()
        self.session.headers.update({
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
            "Accept-Encoding": "gzip, deflate, br",
            "Connection": "keep-alive",
            "Upgrade-Insecure-Requests": "1",
        })

        self.cookie_file = Path(cookie_file)
        self.qr_token = None
        self.login_ticket = None

    def get_login_page(self):
        """
        步骤 1: 获取登录页面

        返回：
            response: 登录页面响应
        """
        login_url = f"{self.cas_base_url}/login"
        params = {
            "service": f"{self.jwgl_base_url}/caslogin"
        }

        print(f"[1/5] 正在获取登录页面...")
        response = self.session.get(login_url, params=params, timeout=30)
        response.raise_for_status()

        # 检查是否已经是登录状态
        if "caslogin" in response.url and "ticket" in response.url:
            print("[✓] 检测到已登录状态")
            self._extract_ticket_from_url(response.url)
            return response

        print(f"[✓] 获取登录页面成功")
        return response

    def get_qr_token(self):
        """
        步骤 2: 生成二维码 UUID

        根据页面 JavaScript 分析：
        ```javascript
        function getUuid() {
            return 'xxxx4xxxyxxxxxxx'.replace(/[xy]/g, function (c) {
                var r = Math.random() * 16 | 0,
                    v = c == 'x' ? r : (r & 0x3 | 0x8);
                return v.toString(16);
            });
        }
        ```

        格式: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
        - x: 随机十六进制数字
        - 4: 固定版本号
        - y: 8, 9, a, 或 b (variant)

        返回：
            str: 二维码 UUID
        """
        print(f"[2/5] 正在生成二维码 UUID...")

        # 按照前端代码生成 UUID
        import random

        def replace_func(c):
            r = int(random.random() * 16) & 0xF
            if c == 'x':
                v = r
            else:  # c == 'y'
                v = (r & 0x3) | 0x8
            return format(v, 'x')

        uuid_template = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'
        self.qr_token = ''.join(replace_func(c) if c in 'xy' else c for c in uuid_template)
        print(f"[✓] 生成 UUID: {self.qr_token}")
        return self.qr_token

    def get_qr_image(self):
        """
        步骤 3: 生成二维码图片

        根据页面中的 JavaScript 代码：
        text: location.origin + '/cas/openAuth?uuid=' + uuid

        即: https://kys.zzuli.edu.cn/cas/openAuth?uuid=xxxx

        返回：
            bytes: 二维码图片数据 或 None
        """
        print(f"[3/5] 正在生成二维码图片...")

        if not HAS_QR:
            print("[!] 未安装 qrcode 库，尝试安装: pip install qrcode pillow")
            return None

        if not self.qr_token:
            print("[!] 未生成 UUID，请先调用 get_qr_token()")
            return None

        # 构建二维码内容
        qr_text = f"{self.cas_base_url}/openAuth?uuid={self.qr_token}"
        print(f"[i] 二维码内容: {qr_text}")

        try:
            # 生成二维码
            qr = qrcode.QRCode(
                version=1,
                error_correction=qrcode.constants.ERROR_CORRECT_L,
                box_size=10,
                border=4,
            )
            qr.add_data(qr_text)
            qr.make(fit=True)

            # 创建图片
            img = qr.make_image(fill_color="black", back_color="white")

            # 保存到内存
            img_buffer = io.BytesIO()
            img.save(img_buffer, format='PNG')
            img_bytes = img_buffer.getvalue()

            print(f"[✓] 生成二维码图片成功 ({len(img_bytes)} bytes)")
            return img_bytes

        except Exception as e:
            print(f"[✗] 生成二维码失败: {e}")
            return None

    def poll_login_status(self, timeout=300, interval=1.2):
        """
        步骤 4: 轮询登录状态

        根据页面 JavaScript 分析：
        - 接口: casSweepCodeLoginQueryController
        - 方法: POST
        - 参数: uuid, token
        - 状态码: waitSweep, waitAuthorized, alreadyAuthorized

        参数：
            timeout: 最大等待时间（秒）
            interval: 轮询间隔（秒），默认 1.2 秒（与前端一致）

        返回：
            dict: 登录结果，包含 ticket 和 cookies
        """
        print(f"[4/5] 正在等待扫码登录...")
        print(f"[*] 请使用 'i轻工大' APP 扫描二维码")
        print(f"[*] 等待中（最长 {timeout} 秒）...")

        start_time = time.time()
        scanned_notified = False
        authorized_notified = False
        api_token = ""  # 用于接口轮询的token

        # 轮询接口地址
        poll_url = f"{self.cas_base_url}/casSweepCodeLoginQueryController"

        while time.time() - start_time < timeout:
            try:
                # 调用状态查询接口
                resp = self.session.post(
                    poll_url,
                    data={
                        "uuid": self.qr_token,
                        "token": api_token
                    },
                    timeout=10
                )

                if resp.status_code == 200:
                    try:
                        # 解析响应
                        data = resp.json()

                        if data.get("success"):
                            obj = data.get("obj", {})
                            code = obj.get("code", "")
                            msg = obj.get("msg", "")

                            # 更新token用于下一次请求
                            if "token" in obj:
                                api_token = obj["token"]

                            # 状态处理
                            if code == "waitSweep":
                                # 等待扫码状态，正常轮询
                                pass

                            elif code == "waitAuthorized":
                                # 已扫码，等待授权
                                if not scanned_notified:
                                    scanned_notified = True
                                    print("[i] 二维码已扫描，请在手机上确认登录...")

                            elif code == "alreadyAuthorized":
                                # 已授权，登录成功！直接使用当前 session
                                if not authorized_notified:
                                    authorized_notified = True
                                    print("[✓] 登录已确认！获取会话信息...")

                                # 尝试访问教务系统获取 ticket
                                try:
                                    jwgl_resp = self.session.get(
                                        f"{self.jwgl_base_url}/caslogin",
                                        timeout=15,
                                        allow_redirects=True
                                    )
                                    # 如果跳转到了带 ticket 的 URL，提取它
                                    if "ticket=" in jwgl_resp.url:
                                        self._extract_ticket_from_url(jwgl_resp.url)
                                except Exception as e:
                                    pass

                                # 直接返回成功（使用当前 session 的 cookies）
                                print(f"[✓] 登录成功！已获取会话 Cookie")
                                return self._build_success_result()

                            elif code == "expired" or code == "timeout":
                                raise TimeoutError("二维码已过期，请重新运行脚本")

                            else:
                                # 其他状态，显示信息
                                if msg and not scanned_notified:
                                    print(f"[i] 状态: {msg}")

                    except json.JSONDecodeError:
                        # 不是 JSON 响应，可能是页面
                        pass
                    except Exception as e:
                        pass

            except requests.exceptions.Timeout:
                # 请求超时，继续轮询
                pass
            except Exception as e:
                # 其他错误，继续轮询
                pass

            # 显示进度
            elapsed = int(time.time() - start_time)
            if elapsed % 10 == 0:
                print(f"[*] 已等待 {elapsed} 秒... 请确认手机上的登录")

            time.sleep(interval)

        raise TimeoutError("登录超时，请检查是否已扫码并在手机上确认")

    def _extract_ticket_from_url(self, url):
        """从 URL 中提取 ticket 参数"""
        parsed = urlparse(url)
        params = parse_qs(parsed.query)
        self.login_ticket = params.get("ticket", [None])[0]
        if self.login_ticket:
            print(f"[i] 获取到登录 ticket: {self.login_ticket[:20]}...")

    def _build_success_result(self):
        """构建登录成功结果"""
        # 处理可能有重复 name 的 cookie
        cookies = {}
        cookie_items = []
        for cookie in self.session.cookies:
            cookies[cookie.name] = cookie.value
            cookie_items.append(f"{cookie.name}={cookie.value}")

        return {
            "status": "success",
            "ticket": self.login_ticket,
            "cookies": cookies,
            "cookie_string": "; ".join(cookie_items),
        }

    def save_cookies(self, result):
        """
        步骤 5: 保存 Cookie 到文件

        参数：
            result: 登录结果字典
        """
        print(f"[5/5] 正在保存 Cookie...")

        save_data = {
            "timestamp": time.time(),
            "datetime": time.strftime("%Y-%m-%d %H:%M:%S"),
            "ticket": result.get("ticket"),
            "cookies": result.get("cookies"),
            "cookie_string": result.get("cookie_string"),
            "base_urls": {
                "cas": self.cas_base_url,
                "jwgl": self.jwgl_base_url,
            }
        }

        with open(self.cookie_file, "w", encoding="utf-8") as f:
            json.dump(save_data, f, indent=2, ensure_ascii=False)

        print(f"[✓] Cookie 已保存到: {self.cookie_file.absolute()}")
        print(f"[i] 共 {len(result.get('cookies', {}))} 个 cookie")

    def verify_login(self):
        """
        验证登录是否有效

        返回：
            bool: 登录是否有效
        """
        print("[*] 正在验证登录状态...")

        try:
            # 尝试访问教务系统首页
            resp = self.session.get(self.jwgl_base_url, timeout=10, allow_redirects=True)
            final_url = resp.url

            # 检查是否被重定向到 CAS 登录页
            if "caslogin" in final_url or "kys.zzuli.edu.cn/cas/login" in final_url:
                print("[✗] 登录验证失败：被重定向到登录页面")
                return False

            # 检查页面内容 - 青果软件特征
            page_text = resp.text
            kingo_indicators = [
                "KINGOSOFT", "kingo", "青果", "教务", "课程", "学生",
                "frame/homes", "homes.html", "main.jsp", "homepage",
                "username", "欢迎您", "退出登录"
            ]

            found_indicators = [ind for ind in kingo_indicators if ind.lower() in page_text.lower()]

            if len(found_indicators) >= 2:
                print(f"[✓] 登录验证成功！检测到特征: {found_indicators[:3]}")
                return True

            # 如果没有找到特征，但URL不是登录页，也可能成功
            if "login" not in final_url.lower() and "cas" not in final_url.lower():
                print(f"[!] 登录状态可能正常 (URL: {final_url[:60]}...)")
                return True

            print(f"[!] 无法确认登录状态，页面URL: {final_url}")
            return False

        except Exception as e:
            print(f"[✗] 验证登录时出错: {e}")
            return False

    def run(self):
        """
        执行完整的登录流程

        完整流程：
        1. 获取登录页面
        2. 获取二维码 token
        3. 获取二维码图片
        4. 轮询等待登录
        5. 保存 cookie
        6. 验证登录
        """
        print("=" * 60)
        print("郑州轻工业大学教务系统扫码登录")
        print("=" * 60)
        print()

        try:
            # 步骤 1: 获取登录页面
            self.get_login_page()

            # 步骤 2: 获取二维码 token
            self.get_qr_token()

            # 步骤 3: 获取二维码图片
            qr_image = self.get_qr_image()
            if qr_image:
                # 保存二维码图片供用户扫描
                qr_file = Path("qrcode.png")
                with open(qr_file, "wb") as f:
                    f.write(qr_image)
                print(f"[i] 二维码已保存到: {qr_file.absolute()}")
                print(f"[*] 请打开 {qr_file.absolute()} 扫描二维码")
            else:
                print("[!] 未能获取二维码图片，请手动访问登录页面扫码")
                print(f"    地址: {self.cas_base_url}/login?service={self.jwgl_base_url}/caslogin")

            # 步骤 4: 轮询等待登录
            result = self.poll_login_status()

            # 步骤 5: 保存 cookie
            self.save_cookies(result)

            # 步骤 6: 验证登录
            if self.verify_login():
                print("\n[✓] 登录流程完成！")
                print(f"[*] 可以使用这些 cookie 访问教务系统了")
                return True
            else:
                print("\n[!] 登录可能未完成，请检查")
                return False

        except TimeoutError as e:
            print(f"\n[✗] 登录超时: {e}")
            return False
        except Exception as e:
            print(f"\n[✗] 登录过程出错: {e}")
            import traceback
            traceback.print_exc()
            return False


def main():
    """主函数"""
    agent = ZZULICASLoginAgent()
    success = agent.run()

    if success:
        print("\n" + "=" * 60)
        print("登录成功！可以运行 explore_schedule.py 获取课程表")
        print("=" * 60)
    else:
        print("\n" + "=" * 60)
        print("登录失败，请检查网络或重试")
        print("=" * 60)


if __name__ == "__main__":
    main()
