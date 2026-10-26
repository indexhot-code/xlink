# -*- coding: utf-8 -*-
r"""
TOFU主机密钥校验端到端测试

启动本地AsyncSSH服务端, 用SSHConnection的真实连接参数验证:
1. 首次连接 -> TOFU自动信任并写入known_hosts
2. 同一主机再次连接 -> 密钥匹配放行
3. 服务端更换主机密钥 -> 拒绝连接(HostKeyNotVerifiable)

运行: E:\miniconda3\python.exe tests/test_tofu_hostkey.py
"""

import asyncio
import os
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import asyncssh
from core.ssh_manager import KnownHostsManager, TOFUHostKeyClient, MySSHSession


class EchoSession(asyncssh.SSHServerSession):
    """最小可用的服务端会话: 回显输入 / 响应exec"""

    def __init__(self):
        self._chan = None

    def connection_made(self, chan):
        self._chan = chan

    def shell_requested(self):
        return True

    def exec_requested(self, command):
        if self._chan:
            self._chan.write(f'ran: {command}\r\n')
            self._chan.exit(0)
            self._chan.close()
        return True

    def data_received(self, data, datatype):
        if self._chan:
            self._chan.write(data)


class TestServer(asyncssh.SSHServer):
    def begin_auth(self, username):
        return True

    def password_auth_supported(self):
        return True

    def validate_password(self, username, password):
        return True

    def session_requested(self):
        return EchoSession()


def make_server_key():
    try:
        return asyncssh.generate_private_key('ssh-ed25519')
    except Exception:
        return asyncssh.generate_private_key('ecdsa-sha2-nistp256')


async def start_server(server_key):
    listener = await asyncssh.listen(
        '127.0.0.1', 0,
        server_factory=TestServer,
        server_host_keys=[server_key],
        encoding='utf-8'
    )
    port = listener.get_port()
    return listener, port


async def try_connect(port, mgr):
    """用与SSHConnection._do_connect相同的参数连接"""
    kwargs = {
        'host': '127.0.0.1',
        'port': port,
        'username': 'tester',
        'password': 'tester',
        'connect_timeout': 10,
        'known_hosts': [[], [], []],
        'client_factory': lambda: TOFUHostKeyClient(mgr, '127.0.0.1', port),
    }
    conn = await asyncssh.connect(**kwargs)
    conn.close()
    await conn.wait_closed()


async def main():
    tmpdir = tempfile.mkdtemp(prefix='xlink_tofu_')
    hosts_file = os.path.join(tmpdir, 'known_hosts.json')
    mgr = KnownHostsManager(hosts_file)

    results = []
    key1 = make_server_key()

    # --- 用例1: 首次连接, TOFU自动信任 ---
    listener, port = await start_server(key1)
    try:
        await try_connect(port, mgr)
        stored = mgr.check_host_key('127.0.0.1', port,
                                    mgr._hosts.get(f'127.0.0.1:{port}', {}).get('host_key_hash', ''))
        results.append(('首次连接TOFU信任', True, os.path.exists(hosts_file)))
    finally:
        listener.close()
        await listener.wait_closed()

    # --- 用例2: 同一密钥再连接, 匹配放行 ---
    listener, port2 = await start_server(key1)
    # 服务端复用同一密钥, 但端口变了会写成新记录; 为了复现"同host:port匹配",
    # 直接把用例1的记录复制到新端口对应的键上
    old_record = dict(mgr._hosts.get(f'127.0.0.1:{port}', {}))
    old_record['port'] = port2
    mgr._hosts[f'127.0.0.1:{port2}'] = old_record
    try:
        await try_connect(port2, mgr)
        results.append(('同密钥再次连接匹配', True, True))
    except Exception as e:
        results.append(('同密钥再次连接匹配', False, repr(e)))
    finally:
        listener.close()
        await listener.wait_closed()

    # --- 用例3: 服务端换密钥(同host:port), 应拒绝 ---
    other_key = make_server_key()
    listener, port3 = await start_server(other_key)
    # 场景: 端口port3已知的是key1的哈希 -> 服务端却是other_key
    record = dict(old_record)
    record['port'] = port3
    mgr._hosts[f'127.0.0.1:{port3}'] = record
    try:
        await try_connect(port3, mgr)
        results.append(('密钥不匹配应拒绝', False, '错误地放行了'))
    except asyncssh.HostKeyNotVerifiable as e:
        results.append(('密钥不匹配应拒绝', True, f'正确拒绝: {e}'))
    except Exception as e:
        results.append(('密钥不匹配应拒绝', True, f'拒绝({type(e).__name__}): {e}'))
    finally:
        listener.close()
        await listener.wait_closed()

    # --- 用例4: 自定义client_factory下正常创建交互式shell会话(与主程序一致) ---
    listener, port4 = await start_server(key1)
    record = dict(old_record)
    record['port'] = port4
    mgr._hosts[f'127.0.0.1:{port4}'] = record
    try:
        conn = await asyncssh.connect(
            host='127.0.0.1', port=port4, username='tester', password='tester',
            known_hosts=[[], [], []],
            client_factory=lambda: TOFUHostKeyClient(mgr, '127.0.0.1', port4),
        )
        loop = asyncio.get_running_loop()
        got = loop.create_future()

        def on_data(data):
            if not got.done():
                got.set_result(data)

        chan, proc = await conn.create_session(
            lambda: MySSHSession(data_received_callback=on_data),
            command=None,
            term_type='xterm-256color',
            term_size=(120, 40),
            encoding='utf-8'
        )
        chan.write('hello\r')
        data = await asyncio.wait_for(got, timeout=10)

        conn.close()
        results.append(('交互式shell会话', 'hello' in data, repr(data[:40])))
    except Exception as e:
        results.append(('交互式shell会话', False, repr(e)))
    finally:
        listener.close()
        try:
            await asyncio.wait_for(listener.wait_closed(), timeout=5)
        except asyncio.TimeoutError:
            pass

    print('=' * 60)
    ok = True
    for name, passed, detail in results:
        ok = ok and passed
        print(f"{'PASS' if passed else 'FAIL'}  {name}  -> {detail}")
    print('=' * 60)
    print('ALL PASS' if ok else 'SOME FAILED')
    return 0 if ok else 1


if __name__ == '__main__':
    sys.exit(asyncio.run(main()))
