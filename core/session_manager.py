# -*- coding: utf-8 -*-
"""
会话配置管理模块
负责SSH会话配置的JSON持久化存储和读写操作
"""

import json
import os
import uuid
from typing import List, Dict, Optional
from pathlib import Path
from core.password_encryption import encrypt_password, decrypt_password, get_password_encryption


class SessionManager:
    """会话配置管理器"""
    
    def __init__(self, config_file: str = None):
        """
        初始化会话管理器
        
        Args:
            config_file: 配置文件路径,默认使用config/sessions.json
        """
        if config_file is None:
            # 使用项目目录下的config/sessions.json
            base_dir = Path(__file__).parent.parent
            self.config_file = base_dir / "config" / "sessions.json"
        else:
            self.config_file = Path(config_file)
        
        # 确保配置目录存在
        self.config_file.parent.mkdir(parents=True, exist_ok=True)
        
        # 加载配置
        self.data = self._load_config()
    
    def _load_config(self) -> Dict:
        """
        从JSON文件加载配置(自动解密密码)
        
        Returns:
            配置字典
        """
        if self.config_file.exists():
            try:
                with open(self.config_file, 'r', encoding='utf-8') as f:
                    data = json.load(f)
                
                # 解密所有会话的密码
                if 'sessions' in data:
                    for session in data['sessions']:
                        if session.get('password'):
                            # 解密密码(如果已是明文会自动返回)
                            session['password'] = decrypt_password(session['password'])
                
                return data
            except (json.JSONDecodeError, IOError) as e:
                print(f"加载配置文件失败: {e}, 使用默认配置")
        
        # 返回默认配置结构
        return {
            "sessions": [],
            "settings": {
                "window_size": [1200, 800],
                "font_size": 12,
                "theme": "light"
            }
        }
    
    def _save_config(self):
        """保存配置到JSON文件(自动加密密码)"""
        try:
            # 加密所有会话的密码
            import copy
            data_to_save = copy.deepcopy(self.data)
            
            if 'sessions' in data_to_save:
                for session in data_to_save['sessions']:
                    if session.get('password'):
                        # 加密密码
                        session['password'] = encrypt_password(session['password'])
            
            with open(self.config_file, 'w', encoding='utf-8') as f:
                json.dump(data_to_save, f, ensure_ascii=False, indent=2)
        except IOError as e:
            print(f"保存配置文件失败: {e}")
    
    def get_sessions(self) -> List[Dict]:
        """
        获取所有会话列表
        
        Returns:
            会话配置列表
        """
        return self.data.get("sessions", [])
    
    def get_session_by_id(self, session_id: str) -> Optional[Dict]:
        """
        根据ID获取会话配置
        
        Args:
            session_id: 会话ID
            
        Returns:
            会话配置字典,未找到返回None
        """
        for session in self.get_sessions():
            if session.get("id") == session_id:
                return session
        return None
    
    def add_session(self, session: Dict) -> str:
        """
        添加新会话
        
        Args:
            session: 会话配置字典(不包含id)
            
        Returns:
            新会话的ID
        """
        session_id = str(uuid.uuid4())
        session["id"] = session_id
        
        self.data["sessions"].append(session)
        self._save_config()
        return session_id
    
    def update_session(self, session_id: str, session: Dict) -> bool:
        """
        更新会话配置
        
        Args:
            session_id: 会话ID
            session: 新的会话配置
            
        Returns:
            是否更新成功
        """
        for i, s in enumerate(self.data["sessions"]):
            if s.get("id") == session_id:
                session["id"] = session_id  # 保持ID不变
                self.data["sessions"][i] = session
                self._save_config()
                return True
        return False
    
    def delete_session(self, session_id: str) -> bool:
        """
        删除会话
        
        Args:
            session_id: 会话ID
            
        Returns:
            是否删除成功
        """
        for i, s in enumerate(self.data["sessions"]):
            if s.get("id") == session_id:
                del self.data["sessions"][i]
                self._save_config()
                return True
        return False
    
    def get_groups(self) -> List[str]:
        """
        获取所有分组名称
        
        Returns:
            分组名称列表
        """
        groups = set()
        for session in self.get_sessions():
            group = session.get("group", "默认分组")
            groups.add(group)
        return sorted(list(groups))
    
    def get_sessions_by_group(self, group: str) -> List[Dict]:
        """
        获取指定分组的会话列表
        
        Args:
            group: 分组名称
            
        Returns:
            该分组下的会话列表
        """
        return [
            s for s in self.get_sessions() 
            if s.get("group", "默认分组") == group
        ]
    
    # 设置相关方法
    
    def get_setting(self, key: str, default=None):
        """
        获取设置值
        
        Args:
            key: 设置键名
            default: 默认值
            
        Returns:
            设置值
        """
        return self.data.get("settings", {}).get(key, default)
    
    def set_setting(self, key: str, value):
        """
        设置值并保存
        
        Args:
            key: 设置键名
            value: 设置值
        """
        if "settings" not in self.data:
            self.data["settings"] = {}
        self.data["settings"][key] = value
        self._save_config()
    
    def get_window_size(self) -> tuple:
        """获取窗口大小"""
        size = self.get_setting("window_size", [1200, 800])
        return tuple(size)
    
    def set_window_size(self, width: int, height: int):
        """设置窗口大小"""
        self.set_setting("window_size", [width, height])
    
    def get_font_size(self) -> int:
        """获取字体大小"""
        return self.get_setting("font_size", 12)
    
    def set_font_size(self, size: int):
        """设置字体大小"""
        self.set_setting("font_size", size)
    
    def get_theme(self) -> str:
        """获取主题"""
        return self.get_setting("theme", "light")
    
    def set_theme(self, theme: str):
        """设置主题"""
        self.set_setting("theme", theme)
    
    # ==================== 批量导入导出功能 ====================
    
    def export_sessions(self, file_path: str, format: str = 'json', 
                        include_password: bool = False) -> bool:
        """
        导出会话配置
        
        Args:
            file_path: 导出文件路径
            format: 导出格式 ('json' 或 'csv')
            include_password: 是否包含密码(默认不包含, 防止明文泄露)
            
        Returns:
            是否成功
        """
        try:
            sessions = self.get_sessions()
            
            if format == 'json':
                # JSON格式: 保留完整信息(密码默认脱敏)
                import copy
                export_data = copy.deepcopy(sessions)
                if not include_password:
                    for session in export_data:
                        if session.get('password'):
                            session['password'] = ''
                            session['password_stripped'] = True  # 标记密码已被移除
                
                with open(file_path, 'w', encoding='utf-8') as f:
                    json.dump(export_data, f, ensure_ascii=False, indent=2)
            elif format == 'csv':
                # CSV格式: 简化信息(不含密码)
                import csv
                with open(file_path, 'w', encoding='utf-8-sig', newline='') as f:
                    writer = csv.writer(f)
                    # 写入表头
                    writer.writerow(['名称', '主机', '端口', '用户名', '认证方式', '分组'])
                    # 写入数据
                    for session in sessions:
                        writer.writerow([
                            session.get('name', ''),
                            session.get('host', ''),
                            session.get('port', 22),
                            session.get('username', ''),
                            session.get('auth_method', 'password'),
                            session.get('group', 'default')
                        ])
            else:
                raise ValueError(f"不支持的导出格式: {format}")
            
            return True
            
        except Exception as e:
            print(f"导出会话失败: {e}")
            return False
    
    def import_sessions(self, file_path: str, mode: str = 'merge') -> tuple:
        """
        导入会话配置
        
        Args:
            file_path: 导入文件路径
            mode: 导入模式 ('merge' 合并, 'replace' 替换)
            
        Returns:
            (成功数, 失败数, 错误信息列表)
        """
        try:
            # 读取文件
            if file_path.endswith('.json'):
                with open(file_path, 'r', encoding='utf-8') as f:
                    imported_sessions = json.load(f)
            elif file_path.endswith('.csv'):
                import csv
                imported_sessions = []
                with open(file_path, 'r', encoding='utf-8-sig') as f:
                    reader = csv.DictReader(f)
                    for row in reader:
                        session = {
                            'id': str(uuid.uuid4()),
                            'name': row.get('名称', ''),
                            'host': row.get('主机', ''),
                            'port': int(row.get('端口', 22)),
                            'username': row.get('用户名', ''),
                            'auth_method': row.get('认证方式', 'password'),
                            'group': row.get('分组', 'default')
                        }
                        imported_sessions.append(session)
            else:
                return (0, 0, ["不支持的文件格式"])
            
            if not isinstance(imported_sessions, list):
                return (0, 0, ["文件格式错误"])
            
            # 统一字段命名(兼容旧版导出: hostname->host, key_file->key_path, auth_type->auth_method)
            for session in imported_sessions:
                if 'hostname' in session and 'host' not in session:
                    session['host'] = session.pop('hostname')
                if 'key_file' in session and 'key_path' not in session:
                    session['key_path'] = session.pop('key_file')
                if 'auth_type' in session and 'auth_method' not in session:
                    session['auth_method'] = session.pop('auth_type')
            
            # 处理导入
            current_sessions = self.get_sessions()
            success_count = 0
            fail_count = 0
            errors = []
            
            if mode == 'replace':
                # 替换模式: 清空现有,全部导入
                self.data['sessions'] = imported_sessions
                success_count = len(imported_sessions)
            elif mode == 'merge':
                # 合并模式: 添加新会话,跳过重复
                existing_names = {s.get('name') for s in current_sessions}
                
                for session in imported_sessions:
                    session_name = session.get('name', '')
                    
                    # 检查是否已存在
                    if session_name in existing_names:
                        # 重命名
                        base_name = session_name
                        counter = 1
                        while session_name in existing_names:
                            session_name = f"{base_name} ({counter})"
                            counter += 1
                        session['name'] = session_name
                        session['id'] = str(uuid.uuid4())
                        existing_names.add(session_name)
                    else:
                        if 'id' not in session:
                            session['id'] = str(uuid.uuid4())
                        existing_names.add(session_name)
                    
                    current_sessions.append(session)
                    success_count += 1
            else:
                return (0, 0, [f"不支持的导入模式: {mode}"])
            
            # 保存
            self._save_config()
            
            return (success_count, fail_count, errors)
            
        except Exception as e:
            return (0, 0, [str(e)])
    
    def get_session_count(self) -> int:
        """获取会话数量"""
        return len(self.get_sessions())
