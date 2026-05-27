# -*- coding: utf-8 -*-
"""
虚拟屏幕缓冲区模块
实现完整的终端屏幕仿真,支持光标定位、清屏、清行等操作
"""

from dataclasses import dataclass, field
from typing import List, Tuple, Optional, Set


@dataclass
class Cell:
    """屏幕单元格"""
    char: str = ' '
    fg_color: str = '#D4D4D4'  # 默认前景色
    bg_color: str = '#1E1E1E'  # 默认背景色
    bold: bool = False
    underline: bool = False
    reverse: bool = False  # 反向视频
    
    def reset(self, fg: str = '#D4D4D4', bg: str = '#1E1E1E'):
        """重置单元格"""
        self.char = ' '
        self.fg_color = fg
        self.bg_color = bg
        self.bold = False
        self.underline = False
        self.reverse = False


class VirtualScreen:
    """
    虚拟屏幕缓冲区
    维护终端的二维屏幕状态,支持NCURSES全屏刷新
    支持滚动缓冲区(scrollback buffer)保存历史输出
    """
    
    MAX_SCROLLBACK_ROWS = 10000  # 滚动缓冲区最大行数
    
    def __init__(self, rows: int = 24, cols: int = 80, 
                 default_fg: str = '#D4D4D4', default_bg: str = '#1E1E1E'):
        """
        初始化虚拟屏幕
        
        Args:
            rows: 屏幕行数(默认24)
            cols: 屏幕列数(默认80)
            default_fg: 默认前景色(默认'#D4D4D4')
            default_bg: 默认背景色(默认'#1E1E1E')
        """
        self.rows = rows
        self.cols = cols
        self.default_fg = default_fg
        self.default_bg = default_bg
        self.cells = [[Cell(fg_color=default_fg, bg_color=default_bg) for _ in range(cols)] for _ in range(rows)]
        self.cursor_row = 0
        self.cursor_col = 0
        self.modified_rows: Set[int] = set()  # 追踪修改的行,用于增量渲染
        self.modified_cols: dict = {}  # 追踪每行修改的列范围 {row: (min_col, max_col)}
        
        # 滚动区域(默认整个屏幕)
        self.scroll_top = 0
        self.scroll_bottom = rows - 1
        
        # 备用屏幕缓冲区(用于 vim/less/top 等 TUI 程序)
        self._alt_screen_active = False
        self._main_cells = None
        self._main_cursor = (0, 0)
        self._main_modified_rows = set()
        self._main_modified_cols = {}
        self._main_scroll_top = 0
        self._main_scroll_bottom = rows - 1
        
        # 滚动缓冲区(scrollback buffer)
        self.scrollback_buffer: List[List[Cell]] = []  # 保存超出屏幕的历史行
        self.scrollback_position = 0  # 当前滚动位置(0表示在底部)
        
        # 调试计数
        self._write_count = 0
        self._render_count = 0
    
    def move_cursor(self, row: int, col: int):
        """
        移动光标到指定位置
        
        Args:
            row: 目标行号(0-based)
            col: 目标列号(0-based)
        """
        # 边界检查
        self.cursor_row = max(0, min(row, self.rows - 1))
        self.cursor_col = max(0, min(col, self.cols - 1))
    
    def move_cursor_up(self, n: int = 1):
        """光标上移"""
        self.cursor_row = max(0, self.cursor_row - n)
    
    def move_cursor_down(self, n: int = 1):
        """光标下移"""
        self.cursor_row = min(self.rows - 1, self.cursor_row + n)
    
    def move_cursor_forward(self, n: int = 1):
        """光标右移"""
        self.cursor_col = min(self.cols - 1, self.cursor_col + n)
    
    def move_cursor_back(self, n: int = 1):
        """光标左移"""
        self.cursor_col = max(0, self.cursor_col - n)
    
    def _reset_cell(self, row: int, col: int):
        """重置指定单元格为默认状态"""
        self.cells[row][col].reset(self.default_fg, self.default_bg)
    
    def write_char(self, char: str, attrs: Optional[dict] = None):
        """
        在当前光标位置写入字符
        
        Args:
            char: 要写入的字符
            attrs: 字符属性 {'fg': '#FF0000', 'bg': '#000000', 'bold': True, 'underline': False}
        """
        if attrs is None:
            attrs = {}
        
        # 边界检查: 如果光标超出屏幕,先滚动
        if self.cursor_row >= self.rows:
            # 滚动屏幕并保存到缓冲区
            self._scroll_up()
            # 光标回到最后一行
            self.cursor_row = self.rows - 1
        
        if self.cursor_col >= self.cols:
            return
        
        cell = self.cells[self.cursor_row][self.cursor_col]
        cell.char = char
        cell.fg_color = attrs.get('fg', '#D4D4D4')
        cell.bg_color = attrs.get('bg', '#1E1E1E')
        cell.bold = attrs.get('bold', False)
        cell.underline = attrs.get('underline', False)
        cell.reverse = attrs.get('reverse', False)
        
        # 标记此行已修改
        self.modified_rows.add(self.cursor_row)
        
        # 更新修改的列范围
        if self.cursor_row not in self.modified_cols:
            self.modified_cols[self.cursor_row] = [self.cursor_col, self.cursor_col]
        else:
            self.modified_cols[self.cursor_row][0] = min(self.modified_cols[self.cursor_row][0], self.cursor_col)
            self.modified_cols[self.cursor_row][1] = max(self.modified_cols[self.cursor_row][1], self.cursor_col)
        
        # 光标右移
        self.cursor_col += 1
        if self.cursor_col >= self.cols:
            self.cursor_col = 0
            self.cursor_row += 1
            # 如果超出滚动区域底部,滚动屏幕并保存到缓冲区
            if self.cursor_row >= self.rows:
                self._scroll_up()
                self.cursor_row = self.rows - 1
                # 滚动后光标回到最后一行，标记该行已修改
                self.modified_rows.add(self.cursor_row)
        
        self._write_count += 1
    
    def write_text(self, text: str, attrs: Optional[dict] = None):
        """
        写入文本字符串
        
        Args:
            text: 要写入的文本
            attrs: 字符属性
        """
        for char in text:
            if char == '\n':
                # 换行
                self.cursor_col = 0
                self.cursor_row += 1
                # 如果光标超出屏幕底部,触发滚动
                if self.cursor_row >= self.rows:
                    self._scroll_up()
                    self.cursor_row = self.rows - 1
                    # 滚动后光标回到最后一行，标记该行已修改
                    self.modified_rows.add(self.cursor_row)
            elif char == '\r':
                # 回车
                self.cursor_col = 0
            elif char == '\x08':  # Backspace退格
                # 光标左移一格,但不删除字符(删除由服务器发送的空格处理)
                self.move_cursor_back(1)
            elif ord(char) < 32 and char not in ('\t',):  # 过滤控制字符,保留制表符
                # 忽略其他控制字符(如\x07响铃等)
                continue
            else:
                self.write_char(char, attrs)
    
    def clear_screen(self, mode: int = 2):
        """
        清屏
        
        Args:
            mode: 清屏模式
                0: 从光标到屏幕底部
                1: 从屏幕顶部到光标
                2: 整个屏幕(默认)
                3: 整个屏幕+清除滚动缓冲区
        """
        if mode == 3:
            # 清除滚动缓冲区
            self.scrollback_buffer.clear()
            self.scrollback_position = 0
        
        if mode == 2 or mode == 3:
            # 清整个屏幕
            for row in range(self.rows):
                for col in range(self.cols):
                    self._reset_cell(row, col)
            self.modified_rows = set(range(self.rows))
            self.modified_cols = {r: [0, self.cols - 1] for r in range(self.rows)}
            self.cursor_row = 0
            self.cursor_col = 0
        elif mode == 0:
            # 从光标到屏幕底部
            # 先清除光标所在行的右侧
            for col in range(self.cursor_col, self.cols):
                self._reset_cell(self.cursor_row, col)
            self.modified_rows.add(self.cursor_row)
            # 更新修改的列范围
            if self.cursor_row not in self.modified_cols:
                self.modified_cols[self.cursor_row] = [self.cursor_col, self.cols - 1]
            else:
                self.modified_cols[self.cursor_row][0] = min(self.modified_cols[self.cursor_row][0], self.cursor_col)
                self.modified_cols[self.cursor_row][1] = max(self.modified_cols[self.cursor_row][1], self.cols - 1)
            
            # 再清除下面的所有行
            for row in range(self.cursor_row + 1, self.rows):
                for col in range(self.cols):
                    self._reset_cell(row, col)
                self.modified_rows.add(row)
                self.modified_cols[row] = [0, self.cols - 1]
        elif mode == 1:
            # 从屏幕顶部到光标
            # 先清除上面的所有行
            for row in range(0, self.cursor_row):
                for col in range(self.cols):
                    self._reset_cell(row, col)
                self.modified_rows.add(row)
                self.modified_cols[row] = [0, self.cols - 1]
            
            # 再清除光标所在行的左侧
            for col in range(0, self.cursor_col + 1):
                self._reset_cell(self.cursor_row, col)
            self.modified_rows.add(self.cursor_row)
            # 更新修改的列范围
            if self.cursor_row not in self.modified_cols:
                self.modified_cols[self.cursor_row] = [0, self.cursor_col]
            else:
                self.modified_cols[self.cursor_row][0] = min(self.modified_cols[self.cursor_row][0], 0)
                self.modified_cols[self.cursor_row][1] = max(self.modified_cols[self.cursor_row][1], self.cursor_col)
    
    def clear_line(self, mode: int = 0):
        """
        清行
        
        Args:
            mode: 清行模式
                0: 从光标到行尾(默认)
                1: 从行首到光标
                2: 整行
        """
        if mode == 0:
            # 从光标到行尾
            for col in range(self.cursor_col, self.cols):
                self._reset_cell(self.cursor_row, col)
            # 更新修改的列范围
            if self.cursor_row not in self.modified_cols:
                self.modified_cols[self.cursor_row] = [self.cursor_col, self.cols - 1]
            else:
                self.modified_cols[self.cursor_row][0] = min(self.modified_cols[self.cursor_row][0], self.cursor_col)
                self.modified_cols[self.cursor_row][1] = max(self.modified_cols[self.cursor_row][1], self.cols - 1)
        elif mode == 1:
            # 从行首到光标
            for col in range(0, self.cursor_col + 1):
                self._reset_cell(self.cursor_row, col)
            # 更新修改的列范围
            if self.cursor_row not in self.modified_cols:
                self.modified_cols[self.cursor_row] = [0, self.cursor_col]
            else:
                self.modified_cols[self.cursor_row][0] = min(self.modified_cols[self.cursor_row][0], 0)
                self.modified_cols[self.cursor_row][1] = max(self.modified_cols[self.cursor_row][1], self.cursor_col)
        elif mode == 2:
            # 整行
            for col in range(self.cols):
                self._reset_cell(self.cursor_row, col)
            self.modified_cols[self.cursor_row] = [0, self.cols - 1]
        
        self.modified_rows.add(self.cursor_row)
    
    def insert_line(self, n: int = 1):
        """在光标位置插入n行(下面的行下移)"""
        for _ in range(n):
            # 滚动区域底部的行被删除
            # 所有行下移一行
            for row in range(self.scroll_bottom, self.cursor_row, -1):
                self.cells[row] = self.cells[row - 1][:]
                self.modified_rows.add(row)
            
            # 光标所在行清空
            for col in range(self.cols):
                self._reset_cell(self.cursor_row, col)
            self.modified_rows.add(self.cursor_row)
    
    def delete_line(self, n: int = 1):
        """删除光标位置的n行(下面的行上移)"""
        for _ in range(n):
            # 所有行上移一行
            for row in range(self.cursor_row, self.scroll_bottom):
                self.cells[row] = self.cells[row + 1][:]
                self.modified_rows.add(row)
            
            # 滚动区域底部的新行清空
            for col in range(self.cols):
                self._reset_cell(self.scroll_bottom, col)
            self.modified_rows.add(self.scroll_bottom)
    
    def enable_alt_screen(self):
        """切换到备用屏幕缓冲区（保存主屏幕状态）"""
        if self._alt_screen_active:
            return
        
        self._main_cells = self.cells
        self._main_cursor = (self.cursor_row, self.cursor_col)
        self._main_modified_rows = self.modified_rows.copy()
        self._main_modified_cols = {k: v[:] for k, v in self.modified_cols.items()}
        self._main_scroll_top = self.scroll_top
        self._main_scroll_bottom = self.scroll_bottom
        
        self.cells = [[Cell(fg_color=self.default_fg, bg_color=self.default_bg) for _ in range(self.cols)] for _ in range(self.rows)]
        self.cursor_row = 0
        self.cursor_col = 0
        self.modified_rows = set(range(self.rows))
        self.modified_cols = {r: [0, self.cols - 1] for r in range(self.rows)}
        self.scroll_top = 0
        self.scroll_bottom = self.rows - 1
        self._alt_screen_active = True
    
    def disable_alt_screen(self):
        """恢复到主屏幕缓冲区"""
        if not self._alt_screen_active:
            return
        
        self.cells = self._main_cells
        self.cursor_row, self.cursor_col = self._main_cursor
        self.modified_rows = self._main_modified_rows
        self.modified_cols = self._main_modified_cols
        self.scroll_top = self._main_scroll_top
        self.scroll_bottom = self._main_scroll_bottom
        self._main_cells = None
        self._alt_screen_active = False
    
    @property
    def is_alt_screen_active(self) -> bool:
        """是否处于备用屏幕模式"""
        return self._alt_screen_active
    
    def _scroll_up(self):
        """向上滚动一行(在滚动区域内)"""
        if not self._alt_screen_active:
            top_row_cells = self.cells[self.scroll_top][:]
            self._add_to_scrollback(top_row_cells)
        
        for row in range(self.scroll_top, self.scroll_bottom):
            # 深拷贝下一行的Cell对象，避免多行引用相同的Cell
            self.cells[row] = [Cell(cell.char, cell.fg_color, cell.bg_color, cell.bold, cell.underline, cell.reverse) 
                              for cell in self.cells[row + 1]]
            self.modified_rows.add(row)
            # 标记整行都修改了（因为内容完全来自下一行）
            self.modified_cols[row] = [0, self.cols - 1]
        
        # 滚动区域底部的新行清空
        for col in range(self.cols):
            self._reset_cell(self.scroll_bottom, col)
        self.modified_rows.add(self.scroll_bottom)
        self.modified_cols[self.scroll_bottom] = [0, self.cols - 1]
    
    def _add_to_scrollback(self, row_cells: List[Cell]):
        """
        添加一行到滚动缓冲区
        
        Args:
            row_cells: 要保存的行单元格列表
        """
        # 深拷贝行数据
        copied_row = [Cell(cell.char, cell.fg_color, cell.bg_color, cell.bold, cell.underline) 
                     for cell in row_cells]
        
        self.scrollback_buffer.append(copied_row)
        
        # 限制缓冲区大小，避免内存溢出
        if len(self.scrollback_buffer) > self.MAX_SCROLLBACK_ROWS:
            # 移除最旧的行
            self.scrollback_buffer.pop(0)
    
    def scroll_up(self, lines: int = 1):
        """
        向上滚动查看历史输出
        
        Args:
            lines: 滚动行数
        """
        max_scroll = len(self.scrollback_buffer)
        self.scrollback_position = min(self.scrollback_position + lines, max_scroll)
    
    def scroll_down(self, lines: int = 1):
        """
        向下滚动
        
        Args:
            lines: 滚动行数
        """
        self.scrollback_position = max(0, self.scrollback_position - lines)
    
    def is_at_bottom(self) -> bool:
        """检查是否在滚动缓冲区底部"""
        return self.scrollback_position == 0
    
    def get_total_rows(self) -> int:
        """获取总行数（缓冲区 + 屏幕）"""
        return len(self.scrollback_buffer) + self.rows
    
    def get_visible_row(self, screen_row: int) -> Optional[List[Cell]]:
        """
        获取可见行的数据
        
        Args:
            screen_row: 屏幕上的行号(0-based)
            
        Returns:
            该行的单元格列表，如果越界则返回None
        """
        if self._alt_screen_active:
            if 0 <= screen_row < self.rows:
                return self.cells[screen_row]
            return None
        
        if self.scrollback_position == 0:
            if 0 <= screen_row < self.rows:
                return self.cells[screen_row]
            return None
        
        # 当向上滚动时，显示滚动缓冲区的内容
        # scrollback_position 表示从缓冲区底部向上滚动的行数
        buffer_len = len(self.scrollback_buffer)
        
        # 计算屏幕第0行对应的缓冲区索引
        # 如果 scrollback_position <= buffer_len，屏幕顶部显示 buffer[buffer_len - scrollback_position]
        # 屏幕底部显示到 buffer 末尾或继续显示屏幕内容
        
        start_buffer_idx = buffer_len - self.scrollback_position
        
        if start_buffer_idx + screen_row < buffer_len:
            # 在缓冲区中
            actual_idx = start_buffer_idx + screen_row
            if 0 <= actual_idx < buffer_len:
                return self.scrollback_buffer[actual_idx]
            return None
        else:
            # 在屏幕中
            screen_idx = start_buffer_idx + screen_row - buffer_len
            if 0 <= screen_idx < self.rows:
                return self.cells[screen_idx]
            return None
    
    def reset_scrollback_position(self):
        """重置滚动位置到底部"""
        self.scrollback_position = 0
    
    def set_scroll_region(self, top: int, bottom: int):
        """
        设置滚动区域
        
        Args:
            top: 滚动区域顶部行号(0-based)
            bottom: 滚动区域底部行号(0-based)
        """
        self.scroll_top = max(0, min(top, self.rows - 1))
        self.scroll_bottom = max(self.scroll_top, min(bottom, self.rows - 1))
    
    def get_modified_rows_data(self) -> List[Tuple[int, List[Cell], Optional[Tuple[int, int]]]]:
        """
        获取修改过的行数据,用于渲染
        
        Returns:
            [(行号, [Cell, Cell, ...], (min_col, max_col)), ...]
            注意: 返回Cell对象的深拷贝,避免后续修改影响渲染
            (min_col, max_col) 是修改的列范围,如果为None表示整行都修改了
        """
        result = []
        for row_idx in sorted(self.modified_rows):
            # 创建Cell对象的深拷贝
            copied_cells = [Cell(cell.char, cell.fg_color, cell.bg_color, cell.bold, cell.underline) 
                          for cell in self.cells[row_idx]]
            
            # 获取修改的列范围
            col_range = self.modified_cols.get(row_idx)
            result.append((row_idx, copied_cells, col_range))
        
        self.modified_rows.clear()
        self.modified_cols.clear()
        self._render_count += 1
        return result
    
    def get_row_text(self, row: int) -> str:
        """获取指定行的纯文本"""
        if 0 <= row < self.rows:
            return ''.join(cell.char for cell in self.cells[row])
        return ''
    
    def resize(self, rows: int, cols: int):
        """
        调整屏幕大小
        
        Args:
            rows: 新行数
            cols: 新列数
        """
        # 保留现有内容
        old_cells = self.cells
        old_rows = self.rows
        old_cols = self.cols
        
        self.rows = rows
        self.cols = cols
        self.cells = [[Cell(fg_color=self.default_fg, bg_color=self.default_bg) for _ in range(cols)] for _ in range(rows)]
        
        # 复制旧数据
        for r in range(min(rows, old_rows)):
            for c in range(min(cols, old_cols)):
                self.cells[r][c] = old_cells[r][c]
        
        # 标记所有行为已修改
        self.modified_rows = set(range(rows))
        self.modified_cols = {r: [0, cols - 1] for r in range(rows)}
        
        # 调整光标位置
        self.cursor_row = min(self.cursor_row, rows - 1)
        self.cursor_col = min(self.cursor_col, cols - 1)
        
        # 重置滚动区域
        self.scroll_top = 0
        self.scroll_bottom = rows - 1
    
    def get_debug_info(self) -> dict:
        """获取调试信息"""
        return {
            'rows': self.rows,
            'cols': self.cols,
            'cursor': (self.cursor_row, self.cursor_col),
            'modified_rows_count': len(self.modified_rows),
            'write_count': self._write_count,
            'render_count': self._render_count,
            'scrollback_rows': len(self.scrollback_buffer),
            'scrollback_position': self.scrollback_position
        }
