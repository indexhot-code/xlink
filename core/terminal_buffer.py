# -*- coding: utf-8 -*-
"""
终端缓冲区模块
ANSI转义序列解析器,将ANSI颜色代码转换为Qt格式
"""

import re
from dataclasses import dataclass
from typing import List, Tuple, Optional


@dataclass
class TextSegment:
    """文本段（带样式信息）"""
    text: str
    fg_color: Optional[str] = None
    bg_color: Optional[str] = None
    bold: bool = False
    underline: bool = False
    reverse: bool = False


@dataclass
class TerminalCommand:
    """终端控制命令"""
    command_type: str  # 'clear_screen', 'move_cursor', 'clear_line', etc.
    params: dict = None  # {'row': 0, 'col': 0} for move_cursor
    
    def __post_init__(self):
        if self.params is None:
            self.params = {}


class ANSIParser:
    """ANSI转义序列解析器"""
    
    # DEC特殊图形字符映射 (用于 top/vim 等 TUI 程序的边框绘制)
    DEC_GRAPHICS_MAP = {
        'j': '┘', 'k': '┐', 'l': '┌', 'm': '└',
        'n': '┼', 'q': '─', 't': '├', 'u': '┤',
        'v': '┴', 'w': '┬', 'x': '│',
        'a': '▒', 'b': '␉', 'c': '␌', 'd': '␍',
        'e': '␊', 'f': '°', 'g': '±', 'h': '␤',
        'i': '␋', 'o': '┐', 'p': '┘', 'r': '└',
        's': '┌', 'y': '≤', 'z': '≥',
        '0': '█', '1': '▀', '2': '▄', '3': '▌',
        '4': '▐', '5': '░', '6': '▒', '7': '▓',
        '8': '▔', '9': '▕',
        '`': '◆', '~': '·',
    }
    
    # ANSI颜色映射表
    ANSI_COLORS = {
        30: "#000000",  # 黑色
        31: "#FF0000",  # 红色
        32: "#00FF00",  # 绿色
        33: "#FFFF00",  # 黄色
        34: "#0000FF",  # 蓝色
        35: "#FF00FF",  # 洋红
        36: "#00FFFF",  # 青色
        37: "#FFFFFF",  # 白色
        90: "#808080",  # 亮黑色(灰色)
        91: "#FF5555",  # 亮红色
        92: "#55FF55",  # 亮绿色
        93: "#FFFF55",  # 亮黄色
        94: "#5555FF",  # 亮蓝色
        95: "#FF55FF",  # 亮洋红
        96: "#55FFFF",  # 亮青色
        97: "#FFFFFF",  # 亮白色
    }
    
    # 背景色映射
    BG_COLORS = {
        40: "#000000",
        41: "#FF0000",
        42: "#00FF00",
        43: "#FFFF00",
        44: "#0000FF",
        45: "#FF00FF",
        46: "#00FFFF",
        47: "#FFFFFF",
        100: "#808080",
        101: "#FF5555",
        102: "#55FF55",
        103: "#FFFF55",
        104: "#5555FF",
        105: "#FF55FF",
        106: "#55FFFF",
        107: "#FFFFFF",
    }
    
    # 256色映射表
    _256_COLOR_MAP = {}
    
    def __init__(self):
        # 初始化256色映射表（仅在第一次实例化时）
        if not self.__class__._256_COLOR_MAP:
            self._init_256_colors()
        
        # ANSI转义序列正则表达式
        # 匹配 \033[<params>m 格式(SGR - Select Graphic Rendition)
        self.ansi_pattern = re.compile(r'\033\[([0-9;]*)m')
        
        # 匹配光标移动等其他CSI序列(CSI - Control Sequence Introducer)
        # \033[<params><final-byte> 其中final-byte是 A-Z 或 a-z
        # 注意: 排除 'm' (SGR颜色序列),因为它已经被 ansi_pattern 处理
        self.cursor_pattern = re.compile(r'\033\[[0-9;]*[A-Ha-ln-z]')  # 修复: 包含G (\033[G 光标到行首), 排除m
        
        # 新增: 光标定位序列
        self.cursor_position_pattern = re.compile(r'\033\[(\d+);(\d+)H')  # \033[row;colH
        self.cursor_position_default_pattern = re.compile(r'\033\[H')  # \033[H (默认1;1)
        
        # 新增: 光标移动序列（参数可选）
        self.cursor_up_pattern = re.compile(r'\033\[(\d*)A')  # \033[nA 或 \033[A - 上移
        self.cursor_down_pattern = re.compile(r'\033\[(\d*)B')  # \033[nB 或 \033[B - 下移
        self.cursor_forward_pattern = re.compile(r'\033\[(\d*)C')  # \033[nC 或 \033[C - 右移
        self.cursor_back_pattern = re.compile(r'\033\[(\d*)D')  # \033[nD 或 \033[D - 左移
        
        # 新增: 清行序列
        self.erase_line_pattern = re.compile(r'\033\[([0-2])?K')
        
        # 匹配删除序列 \033[K (删除到行尾) \033[1K (删除到行首) \033[2K (删除整行)
        self.erase_pattern = re.compile(r'\033\[([0-2])?K')
        
        # 匹配所有清屏序列 \033[<n>J
        self.clear_pattern = re.compile(r'\033\[([0-2])?J')
        
        # 匹配光标显示/隐藏 \033[?25h / \033[?25l
        self.hide_cursor = re.compile(r'\033\[\?25[hl]')
        
        # 匹配DEC私有序列 \033[?<params><letter>
        # 例如: \033[?1034h (启用8位输入), \033[?1049h (备用屏幕)
        self.dec_private_pattern = re.compile(r'\033\[\?[0-9;]*[hl]')
        
        # 备用屏幕切换序列
        self.alt_screen_enable_pattern = re.compile(r'\033\[\?1049h')
        self.alt_screen_disable_pattern = re.compile(r'\033\[\?1049l')
        self.alt_screen_47_enable_pattern = re.compile(r'\033\[\?47h')
        self.alt_screen_47_disable_pattern = re.compile(r'\033\[\?47l')
        
        # 光标键模式切换
        self.cursor_key_app_pattern = re.compile(r'\033\[\?1h')
        self.cursor_key_normal_pattern = re.compile(r'\033\[\?1l')
        
        # 滚动区域设置 \033[top;bottomr
        self.scroll_region_pattern = re.compile(r'\033\[(\d*);(\d*)r')
        
        # 匹配字符集选择序列 \033(<charset> 或 \033)<charset>
        # 例如: \033(B (ASCII字符集), \033(0 (图形字符集)
        # 这些序列在 top、vim 等命令中常见
        self.charset_pattern = re.compile(r'\033[()][A-Z0-9]')
        
        # 匹配OSC序列 \033]<num>;<text>\033\ 或 \033]<num>;<text>\x07
        # 例如: \033]0;标题\033\ (设置窗口标题), \033]10;前景色\x07
        # 注意: 使用非贪婪匹配,确保匹配到第一个结束符
        self.osc_pattern = re.compile(r'\033\].*?(?:\033\\|\x07)')
        
        # 匹配所有其他未处理的CSI序列
        # \033[<params><final-byte>
        # 注意: 排除 'm' (SGR颜色序列),因为它已经被 ansi_pattern 处理
        self.csi_pattern = re.compile(r'\033\[[0-9;]*[A-Za-su-z]')  # 排除m
        
        # 当前样式状态
        self.reset_state()
        
        # 字符集状态 (G0/G1)
        self._g0_charset = 'ascii'  # 'ascii' | 'graphics'
        self._g1_charset = 'ascii'
        
        # 控制命令列表
        self.commands = []
    
    def _init_256_colors(self):
        """初始化256色映射表"""
        # 0-15: 标准颜色 (已在ANSI_COLORS和BG_COLORS中定义)
        # 16-231: 6x6x6 色立方体
        for r in range(6):
            for g in range(6):
                for b in range(6):
                    idx = 16 + r * 36 + g * 6 + b
                    # 将0-5映射到0-255
                    red = 0 if r == 0 else 55 + r * 40
                    green = 0 if g == 0 else 55 + g * 40
                    blue = 0 if b == 0 else 55 + b * 40
                    self.__class__._256_COLOR_MAP[idx] = f"#{red:02x}{green:02x}{blue:02x}"
        
        # 232-255: 灰度色阶 (从黑到白的24个灰度)
        for i in range(24):
            gray = 8 + i * 10
            self.__class__._256_COLOR_MAP[232 + i] = f"#{gray:02x}{gray:02x}{gray:02x}"
    
    def reset_state(self):
        """重置样式状态"""
        self.current_fg = None
        self.current_bg = None
        self.current_bold = False
        self.current_underline = False
        self.current_reverse = False
    
    def _apply_charset_mapping(self, text: str) -> str:
        """应用字符集映射（将 DEC 图形字符转换为 Unicode）"""
        if self._g0_charset != 'graphics':
            return text
        result = []
        for ch in text:
            result.append(self.DEC_GRAPHICS_MAP.get(ch, ch))
        return ''.join(result)
    
    def parse(self, text: str) -> List[TextSegment]:
        """
        解析包含ANSI转义序列的文本
        
        Args:
            text: 包含ANSI转义序列的原始文本
            
        Returns:
            文本片段列表
        """
        segments = []
        
        # 重置命令列表
        self.commands = []
        
        # 初始化清屏标志
        clear_found = False
        
        # 调试: 记录解析入口
        # print(f"[ANSI] 开始解析, 文本长度: {len(text)}, 包含ESC: {'\\033' in text}")
        
        # 调试: 检查是否包含ANSI颜色序列
        # if '\033[' in text and 'm' in text:
        #     print(f"[ANSI COLOR] ✅ 检测到ANSI颜色序列!")
        #     color_matches = re.findall(r'\033\[[0-9;]*m', text)
        #     if color_matches:
        #         print(f"[ANSI COLOR] 颜色序列: {color_matches[:5]}...")  # 只显示前5个
        
        dec_matches = self.dec_private_pattern.findall(text)
        
        # 检测清屏命令 - 支持所有格式
        # \033[J, \033[0J, \033[1J, \033[2J, \033[3J
        # 注意: 必须先检测组合序列(如 \033[H\033[2J),再检测单个序列
        # top 命令常用的序列:
        # - \033[H\033[2J: 光标归位+清屏（最常见）
        # - \033[2J\033[H: 清屏+光标归位
        # - \033[H: 仅光标归位（top 刷新时使用）
        
        # 使用正则表达式提取清屏模式
        clear_screen_pattern = re.compile(r'\033\[([0-3])?J')
        clear_matches = list(clear_screen_pattern.finditer(text))
        
        if clear_matches:
            # 获取最后一个清屏命令的模式（通常是最重要的）
            last_match = clear_matches[-1]
            mode_str = last_match.group(1)
            mode = int(mode_str) if mode_str else 0  # 默认为0（从光标到屏幕底部）
            
            self.commands.append(TerminalCommand('clear_screen', {'mode': mode}))
            
            # 移除所有清屏序列
            text = clear_screen_pattern.sub('', text)
            clear_found = True
        
        # 检测组合序列：光标归位+清屏
        cursor_home_clear_pattern = re.compile(r'\033\[H\033\[([0-3])?J')
        combo_matches = list(cursor_home_clear_pattern.finditer(text))
        
        if combo_matches:
            last_match = combo_matches[-1]
            mode_str = last_match.group(1)
            mode = int(mode_str) if mode_str else 2  # 组合序列默认为2（整个屏幕）
            
            # 先清屏，后光标归位
            self.commands.append(TerminalCommand('clear_screen', {'mode': mode}))
            self.commands.append(TerminalCommand('refresh_screen'))
            
            text = cursor_home_clear_pattern.sub('', text)
            clear_found = True
        
        # 单独处理 \033[H (光标归位) - 视为屏幕刷新
        if '\033[H' in text:
            self.commands.append(TerminalCommand('refresh_screen'))
            text = re.sub(r'\033\[H', '', text)
        
        # 新增: 检测光标定位序列 \033[row;colH
        cursor_pos_matches = list(self.cursor_position_pattern.finditer(text))
        for match in cursor_pos_matches:
            row = int(match.group(1)) - 1  # ANSI是1-based,转换为0-based
            col = int(match.group(2)) - 1
            self.commands.append(TerminalCommand('move_cursor', {'row': row, 'col': col}))
        text = self.cursor_position_pattern.sub('', text)
        
        # 新增: 检测光标移动序列
        for match in self.cursor_up_pattern.finditer(text):
            n = int(match.group(1)) if match.group(1) else 1
            self.commands.append(TerminalCommand('cursor_up', {'n': n}))
        text = self.cursor_up_pattern.sub('', text)
        
        for match in self.cursor_down_pattern.finditer(text):
            n = int(match.group(1)) if match.group(1) else 1
            self.commands.append(TerminalCommand('cursor_down', {'n': n}))
        text = self.cursor_down_pattern.sub('', text)
        
        for match in self.cursor_forward_pattern.finditer(text):
            n = int(match.group(1)) if match.group(1) else 1
            self.commands.append(TerminalCommand('cursor_forward', {'n': n}))
        text = self.cursor_forward_pattern.sub('', text)
        
        for match in self.cursor_back_pattern.finditer(text):
            n = int(match.group(1)) if match.group(1) else 1
            self.commands.append(TerminalCommand('cursor_back', {'n': n}))
        text = self.cursor_back_pattern.sub('', text)
        
        # 新增: 检测清行命令 \033[K
        for match in self.erase_line_pattern.finditer(text):
            mode = int(match.group(1)) if match.group(1) else 0
            self.commands.append(TerminalCommand('clear_line', {'mode': mode}))
        text = self.erase_line_pattern.sub('', text)
        
        if not clear_found:
            pass
        
        # 检测备用屏幕切换序列
        if self.alt_screen_enable_pattern.search(text):
            self.commands.append(TerminalCommand('alt_screen_enable'))
            text = self.alt_screen_enable_pattern.sub('', text)
        if self.alt_screen_disable_pattern.search(text):
            self.commands.append(TerminalCommand('alt_screen_disable'))
            text = self.alt_screen_disable_pattern.sub('', text)
        if self.alt_screen_47_enable_pattern.search(text):
            self.commands.append(TerminalCommand('alt_screen_enable'))
            text = self.alt_screen_47_enable_pattern.sub('', text)
        if self.alt_screen_47_disable_pattern.search(text):
            self.commands.append(TerminalCommand('alt_screen_disable'))
            text = self.alt_screen_47_disable_pattern.sub('', text)
        
        # 检测光标键模式切换
        if self.cursor_key_app_pattern.search(text):
            self.commands.append(TerminalCommand('cursor_key_app'))
            text = self.cursor_key_app_pattern.sub('', text)
        if self.cursor_key_normal_pattern.search(text):
            self.commands.append(TerminalCommand('cursor_key_normal'))
            text = self.cursor_key_normal_pattern.sub('', text)
        
        # 检测滚动区域设置
        for match in self.scroll_region_pattern.finditer(text):
            top = int(match.group(1)) - 1 if match.group(1) else 0
            bottom = int(match.group(2)) - 1 if match.group(2) else 999
            self.commands.append(TerminalCommand('set_scroll_region', {'top': top, 'bottom': bottom}))
        text = self.scroll_region_pattern.sub('', text)
        
        # 移除所有控制序列(按优先级顺序)
        
        # 1. 移陰OSC序列(窗口标题等)
        text = self.osc_pattern.sub('', text)
        
        # 2. 移陰DEC私有序列
        text = self.dec_private_pattern.sub('', text)
        
        # 2.5. 检测并处理字符集选择序列 (\033(B \033(0 等)
        charset_matches = list(self.charset_pattern.finditer(text))
        for match in charset_matches:
            seq = match.group(0)
            if seq == '\x1b(0':
                self._g0_charset = 'graphics'
            elif seq == '\x1b(B':
                self._g0_charset = 'ascii'
            elif seq == '\x1b)0':
                self._g1_charset = 'graphics'
            elif seq == '\x1b)B':
                self._g1_charset = 'ascii'
        text = self.charset_pattern.sub('', text)
        
        # 3. 移除光标显示/隐藏
        text = self.hide_cursor.sub('', text)
        
        # 4. 移除清屏序列(已在上面处理,但再次确保)
        text = self.clear_pattern.sub('', text)
        
        # 5. 移除删除序列
        text = self.erase_pattern.sub('', text)
        
        # 6. 先分割ANSI颜色序列 (必须在删除CSI之前!)
        parts = self.ansi_pattern.split(text)
        
        # 7. 对每个部分删除其他CSI序列(包括光标移动序列)
        processed_parts = []
        for i, part in enumerate(parts):
            if i % 2 == 1:  # 奇数索引是颜色参数，保留
                processed_parts.append(part)
            else:  # 偶数索引是文本，删除其中的CSI序列
                # 先删除光标移动序列
                part = self.cursor_pattern.sub('', part)
                # 再删除其他CSI序列
                processed_parts.append(self.csi_pattern.sub('', part))
        
        parts = processed_parts
        
        # parts交替包含: 文本, 参数, 文本, 参数...
        current_text = ""
        
        for i, part in enumerate(parts):
            if i % 2 == 0:
                # 这是普通文本
                if part:
                    current_text += part
            else:
                # 这是ANSI参数
                # 先保存之前的文本
                if current_text:
                    segments.append(TextSegment(
                        text=self._apply_charset_mapping(current_text),
                        fg_color=self.current_fg,
                        bg_color=self.current_bg,
                        bold=self.current_bold,
                        underline=self.current_underline,
                        reverse=self.current_reverse
                    ))
                    current_text = ""
                
                # 更新样式状态
                self._update_style(part)
        
        # 添加最后的文本
        if current_text:
            segments.append(TextSegment(
                text=self._apply_charset_mapping(current_text),
                fg_color=self.current_fg,
                bg_color=self.current_bg,
                bold=self.current_bold,
                underline=self.current_underline,
                reverse=self.current_reverse
            ))
        
        return segments
    
    def _update_style(self, params_str: str):
        """
        根据ANSI参数更新样式状态
        
        Args:
            params_str: 分号分隔的参数列表
        """
        if not params_str:
            self.reset_state()
            return
        
        params = [int(p) for p in params_str.split(';') if p.isdigit()]
        
        # 处理真彩色和256色需要连续读取多个参数
        i = 0
        while i < len(params):
            param = params[i]
            
            if param == 0:
                # 重置所有样式
                self.reset_state()
            elif param == 1:
                # 加粗
                self.current_bold = True
            elif param == 4:
                # 下划线
                self.current_underline = True
            elif param == 7:
                # 反向视频
                self.current_reverse = True
            elif param == 27:
                # 取消反向视频
                self.current_reverse = False
            elif param == 38 and i + 1 < len(params):
                # 前景色扩展
                if params[i + 1] == 5 and i + 2 < len(params):
                    # 256色: 38;5;N
                    color_idx = params[i + 2]
                    self.current_fg = self._256_COLOR_MAP.get(color_idx, "#FFFFFF")
                    i += 2
                elif params[i + 1] == 2 and i + 4 < len(params):
                    # 真彩色: 38;2;R;G;B
                    r, g, b = params[i + 2], params[i + 3], params[i + 4]
                    self.current_fg = f"#{r:02x}{g:02x}{b:02x}"
                    i += 4
            elif param == 48 and i + 1 < len(params):
                # 背景色扩展
                if params[i + 1] == 5 and i + 2 < len(params):
                    # 256色: 48;5;N
                    color_idx = params[i + 2]
                    self.current_bg = self._256_COLOR_MAP.get(color_idx, "#000000")
                    i += 2
                elif params[i + 1] == 2 and i + 4 < len(params):
                    # 真彩色: 48;2;R;G;B
                    r, g, b = params[i + 2], params[i + 3], params[i + 4]
                    self.current_bg = f"#{r:02x}{g:02x}{b:02x}"
                    i += 4
            elif param in self.ANSI_COLORS:
                # 标准前景色
                self.current_fg = self.ANSI_COLORS[param]
            elif param in self.BG_COLORS:
                # 标准背景色
                self.current_bg = self.BG_COLORS[param]
            
            i += 1
    
    @staticmethod
    def strip_ansi(text: str) -> str:
        """
        移除所有ANSI转义序列,返回纯文本
        
        Args:
            text: 包含ANSI序列的文本
            
        Returns:
            纯文本
        """
        ansi_escape = re.compile(r'\033\[[0-9;?]*[A-Za-z]')
        return ansi_escape.sub('', text)
