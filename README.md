# XLink - SSH Client

<div align="center">

[![Python](https://img.shields.io/badge/Python-3.10+-blue.svg)](https://www.python.org/)
[![PySide6](https://img.shields.io/badge/PySide6-6.x-green.svg)](https://www.qt.io/)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](#)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

A lightweight, stable, and beautiful SSH remote connection tool, inspired by Xshell.

轻量、稳定、高颜值的SSH远程连接工具，灵感来源于Xshell。

[English](#english) | [中文](#中文说明)

</div>

---

## English

### Features

#### Session Management
- ✅ Create/Edit/Delete SSH configurations
- ✅ Group-based session organization
- ✅ Password & RSA private key authentication
- ✅ Auto-save with JSON persistence

#### Terminal Interaction
- ✅ ANSI color display
- ✅ UTF-8 encoding support
- ✅ Text selection & copy (drag to select, double-click for word)
- ✅ Copy & paste (Ctrl+C/V)
- ✅ Clear screen (Ctrl+L)
- ✅ Command history (Up/Down arrows)
- ✅ SFTP file manager with directory tree
- ✅ Breadcrumb navigation with path shortcuts

#### Connection Capabilities
- ✅ Password authentication
- ✅ RSA private key authentication
- ✅ Timeout handling
- ✅ Disconnection detection
- ✅ Real-time status indicators

#### UI/UX
- ✅ Left panel: Session list (tree structure)
- ✅ Right panel: Multi-tab terminals
- ✅ Bottom: Status bar
- ✅ Light & Dark theme switching
- ✅ Font size adjustment (Ctrl+±)
- ✅ Window size persistence
- ✅ Unicode icons

#### Error Handling
- ✅ User-friendly connection failure messages
- ✅ Authentication error prompts
- ✅ Network interruption alerts
- ✅ Global exception capture, crash-free

### Quick Start

#### 1. Install Dependencies

```bash
pip install -r requirements.txt
```

#### 2. Run the Application

```bash
python main.py
```

#### 3. Usage Flow

1. **New Session**: Right-click on the left panel → New Session
2. **Fill Configuration**: Enter host address, port, username, password/key
3. **Connect**: Double-click session node or right-click → Connect
4. **Multi-tab**: Open multiple sessions, each in a separate tab
5. **Switch Theme**: View → Theme → Light/Dark

### Keyboard Shortcuts

| Shortcut | Description |
|----------|-------------|
| `Ctrl+N` | New session |
| `Ctrl+Q` | Quit application |
| `Ctrl++` | Increase font size |
| `Ctrl+-` | Decrease font size |
| `Ctrl+C` | Copy selected text / Send interrupt signal |
| `Ctrl+V` | Paste |
| `Ctrl+L` | Clear screen / Focus path input (SFTP) |
| `↑/↓` | Command history |
| `F5` | Refresh session list |

### Build Executable

Package into a standalone executable using PyInstaller:

```bash
# Install PyInstaller
pip install pyinstaller

# Build
pyinstaller --name XLink --windowed --onefile main.py

# Executable will be in dist/XLink.exe
```

### Configuration

Session configurations are stored in `config/sessions.json`:

```json
{
  "sessions": [
    {
      "id": "unique-uuid",
      "name": "Session Name",
      "group": "Group Name",
      "host": "hostname or IP",
      "port": 22,
      "username": "username",
      "auth_type": "password|key",
      "password": "encrypted_password",
      "key_file": "path/to/private/key",
      "timeout": 30
    }
  ],
  "settings": {
    "window_size": [1200, 800],
    "font_size": 12,
    "theme": "light"
  }
}
```

### Project Structure

```
XLink/
├── main.py                     # Application entry point
├── requirements.txt            # Python dependencies
├── README.md                   # Project documentation
├── LICENSE                     # MIT License
├── config/
│   └── sessions.json           # Session configurations (auto-generated)
├── core/
│   ├── __init__.py
│   ├── logger.py               # Logging system
│   ├── password_encryption.py  # Password encryption utilities
│   ├── session_manager.py      # Session configuration management
│   ├── sftp_manager.py         # SFTP file transfer operations
│   ├── ssh_manager.py          # SSH connection management
│   ├── terminal_buffer.py      # ANSI escape sequence parser
│   └── virtual_screen.py       # Virtual terminal screen buffer
├── ui/
│   ├── __init__.py
│   ├── cursor_renderer.py      # Terminal cursor rendering
│   ├── dialogs.py              # Dialog windows
│   ├── main_window.py          # Main application window
│   ├── native_terminal_widget.py # Native terminal emulator
│   ├── session_panel.py        # Session list panel
│   ├── sftp_window.py          # SFTP file manager window
│   ├── tab_manager.py          # Tab management
│   ├── terminal_widget.py      # Terminal widget base
│   └── theme.py                # Theme management
└── resources/
    ├── generate_icon.py        # Icon generation script
    └── xlink.ico               # Application icon
```

### Troubleshooting

#### Connection Failed?
Check the following:
- Host address and port are correct
- Network reachability (ping test)
- Firewall allows port 22
- Username and password/key are correct

#### Chinese Characters Display as Garbled?
Ensure server locale is set to UTF-8:
```bash
# Check current locale
locale

# If UTF-8 is not set, run:
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
```

#### Backup Session Config?
Simply copy the `config/sessions.json` file.

### Roadmap

- [ ] Port forwarding
- [ ] Session import/export
- [ ] Batch operations
- [ ] Enhanced logging
- [ ] Command auto-completion
- [ ] Serial port support
- [ ] SSH tunnel management

---

## 中文说明

### 功能特性

#### 会话管理
- ✅ 新增/编辑/删除SSH配置
- ✅ 分组管理会话
- ✅ 支持密码认证和RSA私钥认证
- ✅ JSON配置自动保存

#### 终端交互
- ✅ ANSI颜色显示
- ✅ UTF-8编码支持
- ✅ 文本选择与复制（鼠标拖拽选择，双击选择单词）
- ✅ 复制粘贴 (Ctrl+C/V)
- ✅ 清屏功能 (Ctrl+L)
- ✅ 命令历史记录 (上下箭头)
- ✅ SFTP文件管理器，支持目录树导航
- ✅ 面包屑导航，支持路径快捷跳转

#### 连接能力
- ✅ 密码认证
- ✅ RSA私钥认证
- ✅ 超时处理
- ✅ 断线检测
- ✅ 实时状态提示

#### 界面布局
- ✅ 左侧会话列表(树形结构)
- ✅ 右侧多标签终端
- ✅ 底部状态栏
- ✅ 浅色/深色主题切换
- ✅ 字体大小调节 (Ctrl+±)
- ✅ 窗口大小记忆
- ✅ Unicode图标

#### 异常处理
- ✅ 连接失败友好提示
- ✅ 认证错误提示
- ✅ 网络中断提示
- ✅ 全局异常捕获，不崩溃

### 快速开始

#### 1. 安装依赖

```bash
pip install -r requirements.txt
```

#### 2. 运行程序

```bash
python main.py
```

#### 3. 使用流程

1. **新建会话**: 点击左侧空白处右键 → 新建会话
2. **填写配置**: 输入主机地址、端口、用户名、密码/私钥
3. **连接服务器**: 双击会话节点或右键 → 连接
4. **多标签管理**: 可以打开多个会话，每个会话独立标签页
5. **切换主题**: 视图 → 主题 → 浅色/深色

### 快捷键

| 快捷键 | 说明 |
|--------|------|
| `Ctrl+N` | 新建会话 |
| `Ctrl+Q` | 退出程序 |
| `Ctrl++` | 放大字体 |
| `Ctrl+-` | 缩小字体 |
| `Ctrl+C` | 复制选中文本 / 发送中断信号 |
| `Ctrl+V` | 粘贴 |
| `Ctrl+L` | 清屏 / 聚焦路径输入框 (SFTP) |
| `↑/↓` | 命令历史 |
| `F5` | 刷新会话列表 |

### 打包发布

使用PyInstaller打包为独立的exe文件:

```bash
# 安装PyInstaller
pip install pyinstaller

# 打包程序
pyinstaller --name XLink --windowed --onefile main.py

# 生成的exe在 dist/XLink.exe
```

### 配置说明

会话配置保存在 `config/sessions.json`，格式如下:

```json
{
  "sessions": [
    {
      "id": "unique-uuid",
      "name": "会话名称",
      "group": "分组名称",
      "host": "主机地址",
      "port": 22,
      "username": "用户名",
      "auth_type": "password|key",
      "password": "加密后的密码",
      "key_file": "私钥路径",
      "timeout": 30
    }
  ],
  "settings": {
    "window_size": [1200, 800],
    "font_size": 12,
    "theme": "light"
  }
}
```

### 项目结构

```
XLink/
├── main.py                     # 主入口程序
├── requirements.txt            # Python依赖清单
├── README.md                   # 项目说明文档
├── LICENSE                     # MIT许可证
├── config/
│   └── sessions.json           # 会话配置存储(自动生成)
├── core/
│   ├── __init__.py
│   ├── logger.py               # 日志系统
│   ├── password_encryption.py  # 密码加密工具
│   ├── session_manager.py      # 会话配置管理
│   ├── sftp_manager.py         # SFTP文件传输操作
│   ├── ssh_manager.py          # SSH连接管理
│   ├── terminal_buffer.py      # ANSI转义序列解析器
│   └── virtual_screen.py       # 虚拟终端屏幕缓冲
├── ui/
│   ├── __init__.py
│   ├── cursor_renderer.py      # 终端光标渲染
│   ├── dialogs.py              # 对话框窗口
│   ├── main_window.py          # 主应用程序窗口
│   ├── native_terminal_widget.py # 原生终端模拟器
│   ├── session_panel.py        # 会话列表面板
│   ├── sftp_window.py          # SFTP文件管理器窗口
│   ├── tab_manager.py          # 标签页管理
│   ├── terminal_widget.py      # 终端控件基础
│   └── theme.py                # 主题管理
└── resources/
    ├── generate_icon.py        # 图标生成脚本
    └── xlink.ico               # 应用程序图标
```

### 常见问题

#### Q: 连接失败怎么办?
A: 检查以下几点:
- 主机地址和端口是否正确
- 网络是否可达(ping测试)
- 防火墙是否开放22端口
- 用户名和密码/私钥是否正确

#### Q: 中文显示乱码?
A: 确保服务器locale设置为UTF-8:
```bash
# 查看当前locale
locale

# 如果没有设置UTF-8，执行:
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
```

#### Q: 如何备份会话配置?
A: 复制 `config/sessions.json` 文件即可。

### 开发计划

- [ ] 端口转发
- [ ] 会话导入/导出
- [ ] 批量操作
- [ ] 增强日志记录
- [ ] 命令自动补全
- [ ] 串口支持
- [ ] SSH隧道管理

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests.

欢迎贡献！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解我们的行为准则和提交Pull Request的流程。

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

本项目采用MIT许可证 - 详情请查看 [LICENSE](LICENSE) 文件。

## Security

If you discover a security vulnerability, please refer to our [Security Policy](SECURITY.md).

如果您发现安全漏洞，请参阅我们的 [安全策略](SECURITY.md)。

---

<div align="center">

**Made with ❤️ by XLink Team**

⭐ Star us on GitHub — it motivates us a lot!

</div>
