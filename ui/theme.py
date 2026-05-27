# -*- coding: utf-8 -*-
"""
主题管理模块
提供浅色和深色主题的QSS样式表
"""


# 浅色主题样式表
LIGHT_THEME = """
/* 主窗口 */
QMainWindow {
    background-color: #f5f5f5;
}

/* 分割器 */
QSplitter::handle {
    background-color: #d0d0d0;
    width: 2px;
}

QSplitter::handle:hover {
    background-color: #2196F3;
}

/* 会话列表树 */
QTreeWidget {
    background-color: #ffffff;
    border: 1px solid #e0e0e0;
    border-radius: 4px;
    padding: 4px;
    font-size: 13px;
}

QTreeWidget::item {
    padding: 6px 8px;
    border-radius: 3px;
}

QTreeWidget::item:hover {
    background-color: #e3f2fd;
}

QTreeWidget::item:selected {
    background-color: #bbdefb;
    color: #1976D2;
}

/* 标签页 */
QTabWidget::pane {
    border: 1px solid #e0e0e0;
    border-radius: 4px;
    background-color: #ffffff;
}

QTabBar::tab {
    background-color: #e8e8e8;
    color: #666666;
    padding: 10px 20px;
    margin-right: 2px;
    border-top-left-radius: 4px;
    border-top-right-radius: 4px;
    font-size: 13px;
    min-width: 120px;
}

QTabBar::tab:hover {
    background-color: #f0f0f0;
    color: #333333;
}

QTabBar::tab:selected {
    background-color: #ffffff;
    color: #2196F3;
    font-weight: bold;
    border-bottom: 2px solid #2196F3;
}

QTabBar::tab:first:selected {
    margin-left: 0;
}

/* 状态栏 */
QStatusBar {
    background-color: #2196F3;
    color: #ffffff;
    font-size: 12px;
    padding: 4px;
}

QStatusBar QLabel {
    color: #ffffff;
    padding: 0 8px;
}

/* 按钮 */
QPushButton {
    background-color: #2196F3;
    color: white;
    border: none;
    border-radius: 4px;
    padding: 8px 16px;
    font-size: 13px;
    font-weight: bold;
}

QPushButton:hover {
    background-color: #1976D2;
}

QPushButton:pressed {
    background-color: #0d47a1;
}

QPushButton:disabled {
    background-color: #b0b0b0;
    color: #e0e0e0;
}

/* 菜单栏和菜单 */
QMenuBar {
    background-color: #ffffff;
    border-bottom: 1px solid #e0e0e0;
    padding: 4px;
    font-size: 13px;
}

QMenuBar::item {
    padding: 6px 12px;
    border-radius: 4px;
}

QMenuBar::item:selected {
    background-color: #e3f2fd;
    color: #1976D2;
}

QMenu {
    background-color: #ffffff;
    border: 1px solid #e0e0e0;
    border-radius: 4px;
    padding: 4px;
}

QMenu::item {
    padding: 8px 24px 8px 12px;
    border-radius: 3px;
}

QMenu::item:selected {
    background-color: #e3f2fd;
    color: #1976D2;
}

QMenu::separator {
    height: 1px;
    background-color: #e0e0e0;
    margin: 4px 0;
}

/* 工具栏 */
QToolBar {
    background-color: #ffffff;
    border-bottom: 1px solid #e0e0e0;
    padding: 4px;
    spacing: 4px;
}

QToolButton {
    background-color: transparent;
    border: none;
    border-radius: 4px;
    padding: 6px;
}

QToolButton:hover {
    background-color: #e3f2fd;
}

/* 对话框 */
QDialog {
    background-color: #f5f5f5;
}

QGroupBox {
    font-size: 13px;
    font-weight: bold;
    border: 1px solid #e0e0e0;
    border-radius: 4px;
    margin-top: 8px;
    padding-top: 16px;
}

QGroupBox::title {
    subcontrol-origin: margin;
    left: 12px;
    padding: 0 6px;
    color: #2196F3;
}

/* 输入框 */
QLineEdit, QSpinBox {
    background-color: #ffffff;
    border: 1px solid #d0d0d0;
    border-radius: 4px;
    padding: 6px 10px;
    font-size: 13px;
}

QLineEdit:focus, QSpinBox:focus {
    border: 2px solid #2196F3;
    padding: 5px 9px;
}

QLabel {
    font-size: 13px;
    color: #333333;
}

/* 滚动条 */
QScrollBar:vertical {
    background-color: #f0f0f0;
    width: 12px;
    border-radius: 6px;
}

QScrollBar::handle:vertical {
    background-color: #c0c0c0;
    border-radius: 6px;
    min-height: 30px;
}

QScrollBar::handle:vertical:hover {
    background-color: #a0a0a0;
}

QScrollBar::add-line, QScrollBar::sub-line {
    height: 0;
}

QScrollBar:horizontal {
    background-color: #f0f0f0;
    height: 12px;
    border-radius: 6px;
}

QScrollBar::handle:horizontal {
    background-color: #c0c0c0;
    border-radius: 6px;
    min-width: 30px;
}

QScrollBar::handle:horizontal:hover {
    background-color: #a0a0a0;
}

/* 复选框和单选框 */
QCheckBox, QRadioButton {
    font-size: 13px;
    spacing: 8px;
}

/* 下拉框 */
QComboBox {
    background-color: #ffffff;
    border: 1px solid #d0d0d0;
    border-radius: 4px;
    padding: 6px 10px;
    font-size: 13px;
}

QComboBox:hover {
    border: 1px solid #2196F3;
}

QComboBox::drop-down {
    border: none;
    width: 24px;
}

QComboBox QAbstractItemView {
    background-color: #ffffff;
    border: 1px solid #e0e0e0;
    selection-background-color: #e3f2fd;
    selection-color: #1976D2;
}
"""

# 深色主题样式表
DARK_THEME = """
/* 主窗口 */
QMainWindow {
    background-color: #1e1e1e;
}

/* 分割器 */
QSplitter::handle {
    background-color: #3c3c3c;
    width: 2px;
}

QSplitter::handle:hover {
    background-color: #007acc;
}

/* 会话列表树 */
QTreeWidget {
    background-color: #252526;
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    padding: 4px;
    font-size: 13px;
    color: #cccccc;
}

QTreeWidget::item {
    padding: 6px 8px;
    border-radius: 3px;
}

QTreeWidget::item:hover {
    background-color: #2a2d2e;
}

QTreeWidget::item:selected {
    background-color: #094771;
    color: #ffffff;
}

/* 标签页 */
QTabWidget::pane {
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    background-color: #1e1e1e;
}

QTabBar::tab {
    background-color: #2d2d2d;
    color: #969696;
    padding: 10px 20px;
    margin-right: 2px;
    border-top-left-radius: 4px;
    border-top-right-radius: 4px;
    font-size: 13px;
    min-width: 120px;
}

QTabBar::tab:hover {
    background-color: #383838;
    color: #cccccc;
}

QTabBar::tab:selected {
    background-color: #1e1e1e;
    color: #007acc;
    font-weight: bold;
    border-bottom: 2px solid #007acc;
}

QTabBar::tab:first:selected {
    margin-left: 0;
}

/* 状态栏 */
QStatusBar {
    background-color: #007acc;
    color: #ffffff;
    font-size: 12px;
    padding: 4px;
}

QStatusBar QLabel {
    color: #ffffff;
    padding: 0 8px;
}

/* 按钮 */
QPushButton {
    background-color: #007acc;
    color: white;
    border: none;
    border-radius: 4px;
    padding: 8px 16px;
    font-size: 13px;
    font-weight: bold;
}

QPushButton:hover {
    background-color: #1c97ea;
}

QPushButton:pressed {
    background-color: #005a9e;
}

QPushButton:disabled {
    background-color: #3c3c3c;
    color: #808080;
}

/* 菜单栏和菜单 */
QMenuBar {
    background-color: #2d2d2d;
    border-bottom: 1px solid #3c3c3c;
    padding: 4px;
    font-size: 13px;
    color: #cccccc;
}

QMenuBar::item {
    padding: 6px 12px;
    border-radius: 4px;
}

QMenuBar::item:selected {
    background-color: #3e3e42;
    color: #ffffff;
}

QMenu {
    background-color: #2d2d2d;
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    padding: 4px;
    color: #cccccc;
}

QMenu::item {
    padding: 8px 24px 8px 12px;
    border-radius: 3px;
}

QMenu::item:selected {
    background-color: #3e3e42;
    color: #ffffff;
}

QMenu::separator {
    height: 1px;
    background-color: #3c3c3c;
    margin: 4px 0;
}

/* 工具栏 */
QToolBar {
    background-color: #2d2d2d;
    border-bottom: 1px solid #3c3c3c;
    padding: 4px;
    spacing: 4px;
}

QToolButton {
    background-color: transparent;
    border: none;
    border-radius: 4px;
    padding: 6px;
    color: #cccccc;
}

QToolButton:hover {
    background-color: #3e3e42;
}

/* 对话框 */
QDialog {
    background-color: #252526;
    color: #cccccc;
}

QGroupBox {
    font-size: 13px;
    font-weight: bold;
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    margin-top: 8px;
    padding-top: 16px;
    color: #cccccc;
}

QGroupBox::title {
    subcontrol-origin: margin;
    left: 12px;
    padding: 0 6px;
    color: #007acc;
}

/* 输入框 */
QLineEdit, QSpinBox {
    background-color: #3c3c3c;
    border: 1px solid #555555;
    border-radius: 4px;
    padding: 6px 10px;
    font-size: 13px;
    color: #cccccc;
}

QLineEdit:focus, QSpinBox:focus {
    border: 2px solid #007acc;
    padding: 5px 9px;
}

QLabel {
    font-size: 13px;
    color: #cccccc;
}

/* 滚动条 */
QScrollBar:vertical {
    background-color: #1e1e1e;
    width: 12px;
    border-radius: 6px;
}

QScrollBar::handle:vertical {
    background-color: #424242;
    border-radius: 6px;
    min-height: 30px;
}

QScrollBar::handle:vertical:hover {
    background-color: #4e4e4e;
}

QScrollBar::add-line, QScrollBar::sub-line {
    height: 0;
}

QScrollBar:horizontal {
    background-color: #1e1e1e;
    height: 12px;
    border-radius: 6px;
}

QScrollBar::handle:horizontal {
    background-color: #424242;
    border-radius: 6px;
    min-width: 30px;
}

QScrollBar::handle:horizontal:hover {
    background-color: #4e4e4e;
}

/* 复选框和单选框 */
QCheckBox, QRadioButton {
    font-size: 13px;
    spacing: 8px;
    color: #cccccc;
}

/* 下拉框 */
QComboBox {
    background-color: #3c3c3c;
    border: 1px solid #555555;
    border-radius: 4px;
    padding: 6px 10px;
    font-size: 13px;
    color: #cccccc;
}

QComboBox:hover {
    border: 1px solid #007acc;
}

QComboBox::drop-down {
    border: none;
    width: 24px;
}

QComboBox QAbstractItemView {
    background-color: #2d2d2d;
    border: 1px solid #3c3c3c;
    selection-background-color: #094771;
    selection-color: #ffffff;
}
"""


class ThemeManager:
    """主题管理器"""
    
    THEMES = {
        "light": LIGHT_THEME,
        "dark": DARK_THEME
    }
    
    # 终端配色方案
    TERMINAL_COLORS = {
        "light": {
            "background": "#ffffff",
            "foreground": "#333333",
            "cursor": "#2196F3",
            "selection": "#bbdefb",
            "black": "#212121",
            "red": "#c62828",
            "green": "#2e7d32",
            "yellow": "#f9a825",
            "blue": "#1565c0",
            "magenta": "#6a1b9a",
            "cyan": "#00838f",
            "white": "#bdbdbd",
            "bright_black": "#616161",
            "bright_red": "#ef5350",
            "bright_green": "#66bb6a",
            "bright_yellow": "#ffee58",
            "bright_blue": "#42a5f5",
            "bright_magenta": "#ab47bc",
            "bright_cyan": "#26c6da",
            "bright_white": "#f5f5f5",
        },
        "dark": {
            "background": "#1e1e1e",
            "foreground": "#d4d4d4",
            "cursor": "#ffffff",
            "selection": "#264f78",
            "black": "#000000",
            "red": "#cd3131",
            "green": "#0dbc79",
            "yellow": "#e5e510",
            "blue": "#2472c8",
            "magenta": "#bc3fbc",
            "cyan": "#11a8cd",
            "white": "#e5e5e5",
            "bright_black": "#666666",
            "bright_red": "#f14c4c",
            "bright_green": "#23d18b",
            "bright_yellow": "#f5f543",
            "bright_blue": "#3b8eea",
            "bright_magenta": "#d670d6",
            "bright_cyan": "#29b8db",
            "bright_white": "#ffffff",
        }
    }
    
    @staticmethod
    def get_theme(theme_name: str) -> str:
        """
        获取主题样式表
        
        Args:
            theme_name: 主题名称 (light/dark)
            
        Returns:
            QSS样式表字符串
        """
        return ThemeManager.THEMES.get(theme_name, LIGHT_THEME)
    
    @staticmethod
    def get_terminal_colors(theme_name: str = "dark") -> dict:
        """
        获取终端配色方案
        
        Args:
            theme_name: 主题名称 (light/dark)
            
        Returns:
            终端配色字典
        """
        return ThemeManager.TERMINAL_COLORS.get(theme_name, ThemeManager.TERMINAL_COLORS["dark"])
    
    @staticmethod
    def get_available_themes() -> list:
        """获取可用主题列表"""
        return list(ThemeManager.THEMES.keys())
