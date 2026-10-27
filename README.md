# XLink

轻量级跨平台 SSH 客户端：终端 + 文件管理（SFTP），基于 Wails v3（Go + React + TypeScript）构建，产物为单一可执行文件。

## 功能特性

- **SSH 终端**：基于 xterm.js，WebGL 渲染，支持自定义字号、回滚行数、光标闪烁
- **多标签与克隆**：一主机多会话，右键标签即可克隆连接；重启后按快照恢复标签
- **SFTP 文件管理**：双栏视图（本地树 + 远端树），目录树懒展开、面包屑导航、盘符切换、快速访问（桌面/下载/文档）
- **批量文件操作**：单击选中、Ctrl/Shift 多选，右键菜单支持上传、下载、重命名、删除、新建目录、复制路径
- **原生对话框**：上传文件（多选）、上传文件夹、下载目标目录，均调用 Windows 原生选择窗口
- **可靠传输**：分块传输、断点续传（偏移持久化）、源一致性校验、sha256 前缀校验模式、暂停/继续/取消/重传
- **传输队列**：实时进度、宽度可拖拽调整、成功/失败状态一目了然
- **保险库**：主口令 scrypt 派生 + AES-256-GCM 加密存储凭据；锁定/解锁自动断开与恢复会话
- **系统托盘**：关闭最小化到托盘（可设置），托盘菜单显示/退出
- **界面**：无边框深色窗口、自定义标题栏、Mica/Acrylic 材质、可拖拽调整的目录树与传输队列

## 环境要求

- Windows 10/11（x64）
- Go 1.24 及以上
- Node.js 18 及以上（仅开发构建需要）
- Wails v3 CLI：`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

## 开发

```powershell
# 开发模式（前后端热重载）
wails3 dev

# 运行全部后端测试
go test ./backend/...

# 重新生成前端绑定（服务接口变更后）
wails3 task common:generate:bindings
```

## 构建

```powershell
# 构建桌面版（产物：bin/xlink.exe，单文件，无外部依赖）
wails3 build
```

发布时只需要 `bin/xlink.exe` 一个文件，拷贝到目标机器即可运行。

## 目录结构

```
├── main.go                应用入口：窗口、托盘、服务装配
├── backend/
│   ├── appsettings/       应用设置（终端行为、关闭到托盘、版本）
│   ├── connection/        主机管理与 SSH 连接
│   ├── fatal/             致命错误弹窗（GUI 下可见）
│   ├── layout/            标签布局快照（重启恢复）
│   ├── sftp/              SFTP：传输引擎、文件操作、对话框绑定
│   ├── store/             SQLite 存储与版本化迁移
│   ├── terminal/          终端会话（PTY + 回环 WebSocket 字节流）
│   └── vault/             保险库（主口令加解密凭据）
├── frontend/
│   ├── src/components/    界面组件（双栏、树、队列、设置等）
│   ├── src/stores/        状态管理（useSyncExternalStore）
│   ├── src/lib/           xterm 封装、WebSocket 客户端
│   └── bindings/          由 wails3 自动生成的绑定（勿手改）
└── build/                 构建资源（图标、清单）
```

## 架构说明

- 终端字节流走 127.0.0.1 回环 WebSocket，不经过 Wails 绑定/事件通道
- Wails 绑定只承担控制面（会话管理、配置、文件列表、传输进度查询、托盘）
- SFTP 分块进度以内存原子计数维护，前端定时轮询，不走 JSON 事件
- 前端资源经 `go:embed` 打进二进制，发布产物为单一 exe

## 数据存储

应用数据保存在 `%APPDATA%\XLink`：

- `xlink.db`：主机、凭据（密文）、标签快照、传输队列、应用设置
- 主口令丢失无法找回（无后门），删除 `xlink.db` 即可完全重置
