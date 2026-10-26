# -*- coding: utf-8 -*-
"""
SSH连接管理模块 - AsyncSSH版本
基于asyncssh实现高性能、异步的SSH连接管理

核心特性:
- 异步非阻塞连接
- 智能心跳保活机制
- 自动重连与异常恢复
- 低延迟数据传输
- 完善的编码处理(UTF-8/Latin-1 fallback)
- 终端模式优化(vt100/xterm)
"""

import asyncio
import time
import json
import os
import hashlib
from typing import Optional, Dict, Any
import asyncssh
from PySide6.QtCore import QObject, Signal as pyqtSignal
from core.logger import get_logger


logger = get_logger("SSHConnection")


KNOWN_HOSTS_FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), 'config', 'known_hosts.json')


class KnownHostsManager:
    """TOFU (Trust On First Use) 主机密钥管理器"""
    
    def __init__(self, hosts_file: str = KNOWN_HOSTS_FILE):
        self._hosts_file = hosts_file
        self._hosts: Dict[str, Dict[str, str]] = {}
        self._load()
    
    def _load(self):
        """加载已知主机列表"""
        try:
            if os.path.exists(self._hosts_file):
                with open(self._hosts_file, 'r', encoding='utf-8') as f:
                    self._hosts = json.load(f)
                logger.debug(f"已加载 {len(self._hosts)} 个已知主机")
        except Exception as e:
            logger.warning(f"加载已知主机文件失败: {e}")
            self._hosts = {}
    
    def _save(self):
        """保存已知主机列表"""
        try:
            os.makedirs(os.path.dirname(self._hosts_file), exist_ok=True)
            with open(self._hosts_file, 'w', encoding='utf-8') as f:
                json.dump(self._hosts, f, indent=2, ensure_ascii=False)
        except Exception as e:
            logger.error(f"保存已知主机文件失败: {e}")
    
    def _make_key(self, host: str, port: int) -> str:
        """生成主机标识键"""
        return f"{host}:{port}"
    
    def check_host_key(self, host: str, port: int, host_key: str) -> str:
        """
        检查主机密钥
        
        Returns:
            'new': 新主机，首次连接
            'match': 密钥匹配
            'mismatch': 密钥不匹配（可能中间人攻击）
        """
        key = self._make_key(host, port)
        host_key_hash = hashlib.sha256(host_key.encode()).hexdigest()
        
        if key not in self._hosts:
            return 'new'
        
        stored_hash = self._hosts[key].get('host_key_hash', '')
        if stored_hash == host_key_hash:
            return 'match'
        
        return 'mismatch'
    
    def add_host(self, host: str, port: int, host_key: str):
        """添加或更新主机密钥"""
        key = self._make_key(host, port)
        host_key_hash = hashlib.sha256(host_key.encode()).hexdigest()
        self._hosts[key] = {
            'host': host,
            'port': port,
            'host_key_hash': host_key_hash,
            'added_at': time.strftime('%Y-%m-%d %H:%M:%S')
        }
        self._save()
        logger.info(f"已保存主机密钥: {host}:{port}")
    
    def remove_host(self, host: str, port: int):
        """移除已知主机"""
        key = self._make_key(host, port)
        if key in self._hosts:
            del self._hosts[key]
            self._save()


class MySSHSession(asyncssh.SSHClientSession):
    """
    自定义SSH会话类，用于处理数据接收和连接状态
    """
    def __init__(self, data_received_callback=None, connection_lost_callback=None):
        self._data_received_callback = data_received_callback
        self._connection_lost_callback = connection_lost_callback
        self._chan = None
    
    def connection_made(self, chan):
        """连接建立时调用"""
        self._chan = chan
        logger.debug("SSH会话连接已建立")
    
    def data_received(self, data: str, datatype: asyncssh.DataType = None):
        """接收到数据时调用 - AsyncSSH已经解码为字符串"""
        try:
            logger.debug(f"[SSH Session] 接收到数据: {len(data)} 字符, 前100字符: {repr(data[:100])}")
            if self._data_received_callback:
                self._data_received_callback(data)
        except Exception as e:
            logger.error(f"数据处理失败: {e}")
    
    def connection_lost(self, exc):
        """连接丢失时调用"""
        logger.debug(f"SSH会话连接丢失: {exc}")
        if self._connection_lost_callback:
            self._connection_lost_callback(exc)
    
    def get_channel(self):
        """获取channel对象"""
        return self._chan


class TOFUHostKeyClient(asyncssh.SSHClient):
    """
    TOFU主机密钥校验客户端

    AsyncSSH在密钥交换阶段(认证凭据发送之前)回调 validate_host_public_key,
    在此完成首次信任(TOFU)与密钥变更检测。

    注意: asyncssh的known_hosts参数不支持协程回调(其返回值会被当作序列切片),
    因此必须通过client_factory注入本类来实现自定义校验。
    """
    
    def __init__(self, known_hosts_mgr: KnownHostsManager, host: str, port: int):
        self._mgr = known_hosts_mgr
        self._fallback_host = host
        self._fallback_port = port
        self.key_status = None  # 'new' / 'match' / 'mismatch'
    
    def validate_host_public_key(self, host, addr, port, key) -> bool:
        """
        返回True表示信任该主机密钥, False则AsyncSSH抛出HostKeyNotVerifiable
        """
        try:
            host_key_str = key.export_public_key()
        except Exception:
            host_key_str = str(key)
        if isinstance(host_key_str, bytes):
            host_key_str = host_key_str.decode('ascii', 'replace')
        
        host = host or self._fallback_host
        port = port or self._fallback_port
        
        status = self._mgr.check_host_key(host, port, host_key_str)
        self.key_status = status
        
        if status == 'match':
            logger.debug(f"主机密钥验证通过: {host}:{port}")
            return True
        
        if status == 'new':
            # TOFU: 首次连接自动信任并记录
            self._mgr.add_host(host, port, host_key_str)
            logger.info(f"TOFU: 首次连接 {host}:{port}, 已在握手阶段保存主机密钥")
            return True
        
        # mismatch
        logger.error(f"主机密钥验证失败: {host}:{port}, 可能存在中间人攻击!")
        return False


class SSHConnection(QObject):
    """
    基于AsyncSSH的SSH连接管理类
    
    特性:
    - 异步非阻塞连接
    - 智能心跳检测(可配置间隔)
    - 自动重连机制(指数退避)
    - 完善的异常处理
    """
    
    # 信号定义
    data_received = pyqtSignal(str)  # 接收到数据
    connection_status = pyqtSignal(str)  # 连接状态: connected/disconnecting/disconnected/error/reconnecting
    error_occurred = pyqtSignal(str)  # 错误信息
    reconnecting = pyqtSignal(int)  # 正在重连,参数: 尝试次数
    
    # 连接状态常量
    STATE_DISCONNECTED = "disconnected"
    STATE_CONNECTING = "connecting"
    STATE_CONNECTED = "connected"
    STATE_DISCONNECTING = "disconnecting"
    STATE_RECONNECTING = "reconnecting"
    
    def __init__(self):
        super().__init__()
        
        # 主机密钥管理器 (TOFU)
        self._known_hosts_mgr = KnownHostsManager()
        
        # AsyncSSH核心对象
        self.conn: Optional[asyncssh.SSHClientConnection] = None
        self.process: Optional[asyncssh.SSHClientProcess] = None
        self.chan = None  # shell通道对象
        
        # 连接状态
        self._state = self.STATE_DISCONNECTED
        self._running = False
        
        # 会话信息
        self.session_info: Dict[str, Any] = {}
        self._connect_params: Dict[str, Any] = {}
        
        # 心跳配置(asyncssh原生keepalive, 连接时下发)
        self.heartbeat_interval = 30  # 秒
        
        # 重连配置
        self.auto_reconnect = True
        self.max_reconnect_attempts = 5
        self.reconnect_delay = 5  # 初始重连延迟(秒)
        self.max_reconnect_delay = 60  # 最大重连延迟(秒)
        self._reconnect_attempts = 0
        
        # 终端配置
        self.term_width = 120
        self.term_height = 40
        self.term_type = 'xterm-256color'
        
        # 数据读取缓冲
        self._read_task: Optional[asyncio.Task] = None
        
        logger.debug("SSHConnection实例已创建")
    
    def _get_event_loop(self) -> asyncio.AbstractEventLoop:
        """获取当前事件循环"""
        try:
            return asyncio.get_event_loop()
        except RuntimeError:
            # 如果没有运行中的事件循环，创建一个新的
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
            return loop
    
    def _run_coroutine(self, coro):
        """在事件循环中运行协程"""
        loop = self._get_event_loop()
        if loop.is_running():
            # 事件循环正在运行，使用create_task
            return asyncio.ensure_future(coro)
        else:
            # 事件循环未运行，直接运行
            return loop.run_until_complete(coro)
    
    @property
    def is_connected(self) -> bool:
        """检查连接状态"""
        return self._state == self.STATE_CONNECTED
    
    @property
    def state(self) -> str:
        """获取当前连接状态"""
        return self._state
    
    def _set_state(self, new_state: str):
        """设置连接状态"""
        old_state = self._state
        self._state = new_state
        logger.debug(f"连接状态变更: {old_state} -> {new_state}")
    
    def connect(self, host: str, port: int, username: str, 
                password: Optional[str] = None, 
                key_file: Optional[str] = None,
                timeout: int = 30,
                auto_reconnect: bool = True):
        """
        建立SSH连接(异步)
        
        Args:
            host: 主机地址
            port: 端口号
            username: 用户名
            password: 密码
            key_file: 私钥文件路径
            timeout: 连接超时时间(秒)
            auto_reconnect: 是否启用自动重连
        """
        # 保存连接参数(用于重连)
        self._connect_params = {
            'host': host,
            'port': port,
            'username': username,
            'password': password,
            'key_file': key_file,
            'timeout': timeout
        }
        
        self.session_info = {
            "host": host,
            "port": port,
            "username": username
        }
        
        self.auto_reconnect = auto_reconnect
        
        # 使用asyncio.create_task异步执行连接
        asyncio.ensure_future(self._do_connect(host, port, username, password, key_file, timeout))
    
    async def _do_connect(self, host: str, port: int, username: str,
                          password: Optional[str], key_file: Optional[str],
                          timeout: int):
        """实际执行连接(异步)"""
        try:
            self._set_state(self.STATE_CONNECTING)
            logger.info(f"正在连接SSH服务器: {host}:{port}")
            
            # 连接参数
            connect_kwargs = {
                'host': host,
                'port': port,
                'username': username,
                'connect_timeout': timeout,
                # 空信任集: 强制所有主机密钥走 validate_host_public_key 自定义校验
                # (不能传None, 传None会完全关闭主机密钥校验)
                'known_hosts': [[], [], []],
                'client_factory': lambda: TOFUHostKeyClient(
                    self._known_hosts_mgr, host, port
                ),
                # 原生keepalive: 每30s发一次, 连续3次无响应则判定断线
                'keepalive_interval': self.heartbeat_interval,
                'keepalive_count_max': 3,
            }
            
            # 认证方式
            if key_file and key_file.strip():
                # 私钥认证
                client_keys = [key_file]
                connect_kwargs['client_keys'] = client_keys
                logger.debug("使用私钥认证")
            elif password:
                # 密码认证
                connect_kwargs['password'] = password
                logger.debug("使用密码认证")
            
            # 建立连接(主机密钥已在密钥交换阶段由TOFUHostKeyClient校验)
            self.conn = await asyncssh.connect(**connect_kwargs)
            
            # 打开交互式shell
            self.chan, self.process = await self.conn.create_session(
                lambda: MySSHSession(
                    data_received_callback=self._on_data_received,
                    connection_lost_callback=self._on_connection_lost
                ),
                command=None,  # None表示打开交互式shell
                term_type=self.term_type,
                term_size=(self.term_width, self.term_height),
                encoding='utf-8'
            )
            
            logger.info(f"Shell会话已创建, chan={self.chan is not None}, process={self.process is not None}")
            
            # 更新状态
            self._set_state(self.STATE_CONNECTED)
            self._reconnect_attempts = 0
            self._running = True

            logger.info(f"SSH连接成功: {host}:{port}")

            # 发送连接成功信号
            self.connection_status.emit(self.STATE_CONNECTED)
            
        except asyncssh.PermissionDenied as e:
            error_msg = "认证失败: 用户名、密码或私钥错误"
            logger.error(error_msg)
            self._set_state(self.STATE_DISCONNECTED)
            self.error_occurred.emit(error_msg)
            # 认证失败重试无意义, 不进入重连流程
            self.connection_status.emit("error")

        except asyncssh.HostKeyNotVerifiable as e:
            error_msg = f"主机密钥验证失败: {host}:{port} 密钥不匹配，可能存在中间人攻击!"
            logger.error(error_msg)
            self._set_state(self.STATE_DISCONNECTED)
            self.error_occurred.emit(error_msg)
            # 疑似中间人攻击, 不自动重连
            self.connection_status.emit("error")

        except (asyncssh.DisconnectError, OSError) as e:
            error_msg = f"连接失败: {str(e)}"
            logger.error(error_msg)
            self._set_state(self.STATE_DISCONNECTED)
            self.error_occurred.emit(error_msg)
            self._maybe_retry_connect()

        except Exception as e:
            error_msg = f"连接失败: {str(e)}"
            logger.error(error_msg, exc_info=True)
            self._set_state(self.STATE_DISCONNECTED)
            self.error_occurred.emit(error_msg)
            self._maybe_retry_connect()

    def _maybe_retry_connect(self):
        """
        连接失败后按需调度下一次重连

        仅在自动重连流程中(已有重连尝试记录)才继续退避重试;
        首次连接失败只报错, 不自动重试。
        """
        if (self._running and self.auto_reconnect
                and 0 < self._reconnect_attempts < self.max_reconnect_attempts):
            asyncio.ensure_future(self._trigger_reconnect())
        else:
            self.connection_status.emit("error")
    
    def _on_connection_lost(self, exc=None):
        """
        连接丢失回调(唯一实现)

        用户主动断开(disconnect已置_running=False)或禁用重连时不重连;
        否则按指数退避调度下一次重连。
        """
        # 用户主动断开(disconnect已置_running=False)或禁用重连时不重连;
        was_disconnecting = (self._state == self.STATE_DISCONNECTING) or not self._running
        logger.warning(f"SSH连接丢失: {exc}")
        self._set_state(self.STATE_DISCONNECTED)

        if was_disconnecting:
            # 主动断开: 状态信号由disconnect()统一发出, 此处不重复
            return

        if not self.auto_reconnect:
            self.connection_status.emit(self.STATE_DISCONNECTED)
            return

        if self._reconnect_attempts >= self.max_reconnect_attempts:
            logger.error(f"重连次数超限({self.max_reconnect_attempts}), 停止重连")
            self._running = False
            self.error_occurred.emit("重连失败: 超过最大重连次数")
            self.connection_status.emit("error")
            return

        self._reconnect_attempts += 1
        asyncio.ensure_future(self._trigger_reconnect())

    def _on_data_received(self, data: str):
        """
        数据接收回调
        
        Args:
            data: 接收到的数据字符串
        """
        logger.info(f"[SSH回调] 接收到 {len(data)} 字节数据, 前100字符: {repr(data[:100])}")
        
        # 发送数据信号
        self.data_received.emit(data)
    
    def send_data(self, data: str):
        """
        发送数据到SSH服务器
        
        Args:
            data: 要发送的字符串
        """
        if not self.is_connected or not self.chan:
            logger.warning("连接未建立,无法发送数据")
            return
        
        try:
            # 直接发送数据
            self.chan.write(data)
        except Exception as e:
            logger.error(f"发送数据失败: {e}")
            self.error_occurred.emit(f"发送数据失败: {str(e)}")
    
    async def disconnect(self):
        """优雅地断开SSH连接"""
        if not self._running:
            return
        
        logger.info("正在断开SSH连接...")
        self._set_state(self.STATE_DISCONNECTING)
        self._running = False
        self.auto_reconnect = False  # 禁用自动重连
        
        # 关闭连接
        await self._close_connection()
        
        # 更新状态
        self._set_state(self.STATE_DISCONNECTED)
        self.connection_status.emit(self.STATE_DISCONNECTED)
        
        logger.info("SSH连接已断开")
    
    async def _close_connection(self):
        """异步关闭连接"""
        if self.process:
            try:
                self.process.close()
                await self.process.wait_closed()
            except Exception as e:
                logger.debug(f"关闭process异常: {e}")
            finally:
                self.process = None
        
        if self.conn:
            try:
                self.conn.close()
                await self.conn.wait_closed()
            except Exception as e:
                logger.debug(f"关闭connection异常: {e}")
            finally:
                self.conn = None
    
    async def _trigger_reconnect(self):
        """
        触发重连(指数退避) - 异步版本

        注意: _reconnect_attempts 已由 _on_connection_lost/_maybe_retry_connect
        递增, 这里只负责延时和重连。
        """
        if not self._running:
            return
        
        if self._reconnect_attempts > self.max_reconnect_attempts:
            logger.error(f"重连次数超限({self.max_reconnect_attempts}),停止重连")
            self._running = False
            self.error_occurred.emit("重连失败: 超过最大重连次数")
            self.connection_status.emit("error")
            return
        
        # 计算退避延迟
        delay = min(
            self.reconnect_delay * (2 ** (self._reconnect_attempts - 1)),
            self.max_reconnect_delay
        )
        
        logger.info(f"将在 {delay:.1f} 秒后尝试第 {self._reconnect_attempts} 次重连...")
        self.reconnecting.emit(self._reconnect_attempts)
        self._set_state(self.STATE_RECONNECTING)
        
        # 在延迟后重连
        await asyncio.sleep(delay)
        
        if not self.auto_reconnect or not self._running:
            return
        
        logger.info("开始重连...")
        
        # 使用保存的参数重连
        params = self._connect_params
        await self._do_connect(
            host=params['host'],
            port=params['port'],
            username=params['username'],
            password=params.get('password'),
            key_file=params.get('key_file'),
            timeout=params['timeout']
        )
    
    def resize_terminal(self, width: int, height: int):
        """
        调整终端大小
        
        Args:
            width: 终端宽度(字符数)
            height: 终端高度(行数)
        """
        if self.is_connected and self.chan:
            try:
                # 直接调用异步方法
                asyncio.ensure_future(self._resize_terminal_async(width, height))
                self.term_width = width
                self.term_height = height
                logger.debug(f"终端大小已调整: {width}x{height}")
            except Exception as e:
                logger.error(f"调整终端大小失败: {e}")
    
    async def _resize_terminal_async(self, width: int, height: int):
        """异步调整终端大小"""
        if self.process:
            self.process.change_terminal_size(width, height)
    
    def get_session_info(self) -> dict:
        """获取会话信息"""
        return self.session_info.copy()
    
    def __del__(self):
        """析构函数: 确保连接被清理"""
        # disconnect现在是异步方法，在析构函数中无法await
        # 直接关闭连接
        if self.conn:
            try:
                self.conn.close()
            except:
                pass
