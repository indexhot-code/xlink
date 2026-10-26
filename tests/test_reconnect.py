# -*- coding: utf-8 -*-
r"""
自动重连端到端测试

验证 SSHConnection 在服务端主动断开后能按退避策略自动重连,
并确认手动 disconnect() 不会触发重连。

运行: E:\miniconda3\python.exe tests/test_reconnect.py
"""

import asyncio
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import asyncssh
from core.ssh_manager import SSHConnection
import core.ssh_manager as sm


class EchoSession(asyncssh.SSHServerSession):
    def __init__(self):
        self._chan = None

    def connection_made(self, chan):
        self._chan = chan

    def shell_requested(self):
        return True

    def pty_requested(self, term_type, term_size, term_modes):
        return True

    def data_received(self, data, datatype):
        if self._chan:
            self._chan.write(data)


class DropOnceServer(asyncssh.SSHServer):
    """接受认证, 并在首个连接建立1秒后主动踢掉, 用于触发客户端重连"""

    dropped = False

    def begin_auth(self, username):
        return True

    def password_auth_supported(self):
        return True

    def validate_password(self, username, password):
        return True

    def session_requested(self):
        return EchoSession()

    def connection_made(self, conn):
        if not DropOnceServer.dropped:
            DropOnceServer.dropped = True
            asyncio.get_running_loop().call_later(1.0, conn.close)


def cleanup_localhost_records():
    """清理测试写入 config/known_hosts.json 的 127.0.0.1 记录"""
    path = os.path.abspath(
        os.path.join(os.path.dirname(sm.__file__), '..', 'config', 'known_hosts.json')
    )
    if not os.path.exists(path):
        return []
    with open(path, encoding='utf-8') as f:
        data = json.load(f)
    removed = [k for k in list(data) if k.startswith('127.0.0.1:')]
    for key in removed:
        data.pop(key)
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(data, f, indent=2, ensure_ascii=False)
    return removed


async def main():
    key = asyncssh.generate_private_key('ssh-ed25519')
    listener = await asyncssh.listen(
        '127.0.0.1', 0,
        server_factory=DropOnceServer,
        server_host_keys=[key]
    )
    port = listener.get_port()

    conn = SSHConnection()
    conn.reconnect_delay = 0.5
    conn.max_reconnect_delay = 2
    conn.max_reconnect_attempts = 3

    statuses = []
    conn.connection_status.connect(lambda s: statuses.append(s))
    conn.reconnecting.connect(lambda n: statuses.append(f'retry{n}'))

    conn.connect('127.0.0.1', port, 'tester', password='x', timeout=10)

    for _ in range(100):
        await asyncio.sleep(0.1)
        if statuses.count('connected') >= 2:
            break

    reconnect_ok = (statuses.count('connected') >= 2
                    and any(s.startswith('retry') for s in statuses))
    counter_reset = conn._reconnect_attempts == 0

    await conn.disconnect()
    disconnect_ok = conn.state == 'disconnected' and statuses[-1] == 'disconnected'

    listener.close()
    await listener.wait_closed()
    removed = cleanup_localhost_records()

    print('=' * 60)
    print('状态序列 =', statuses)
    print(f"{'PASS' if reconnect_ok else 'FAIL'}  服务端断开后自动重连")
    print(f"{'PASS' if counter_reset else 'FAIL'}  重连成功后计数归零")
    print(f"{'PASS' if disconnect_ok else 'FAIL'}  手动断开保持断开(不重连)")
    print('清理测试记录:', removed)
    print('=' * 60)

    return 0 if (reconnect_ok and counter_reset and disconnect_ok) else 1


if __name__ == '__main__':
    sys.exit(asyncio.run(main()))
