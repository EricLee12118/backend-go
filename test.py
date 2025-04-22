import asyncio
import json
import websockets
import argparse
import sys
import uuid
import datetime

class ChatClient:
    def __init__(self, server_url, username):
        self.server_url = server_url
        self.username = username
        self.user_id = str(uuid.uuid4())[:8]
        self.websocket = None
        self.running = True
        self.room_id = None
        self.room_name = None
        self.is_owner = False
        self.available_rooms = []
        self.in_room = False

    async def connect(self):
        """连接到WebSocket服务器"""
        try:
            self.websocket = await websockets.connect(self.server_url)
            print("已连接到服务器")
            
            await self.room_selection_flow()
            
        except Exception as e:
            print(f"连接错误: {e}")
            self.running = False

    async def room_selection_flow(self):
        """房间选择流程"""
        # 获取房间列表
        await self.get_rooms()
        
        # 选择创建或加入房间
        room_choice = await self.prompt_room_choice()
        if room_choice == "create":
            await self.create_room()
        elif room_choice == "join":
            await self.join_existing_room()
        else:
            print("无效选择，退出程序")
            self.running = False
            return
        
        self.in_room = True
        receive_task = asyncio.create_task(self.receive_messages())
        await self.handle_user_input()
        await receive_task
        if self.running:
            await self.room_selection_flow()

    async def get_rooms(self):
        """获取可用房间列表"""
        try:
            get_rooms_msg = {
                "type": "get_rooms",
                "sender": self.user_id,
                "content": self.username,
                "room": ""
            }
            await self.websocket.send(json.dumps(get_rooms_msg))
            
            response = await self.websocket.recv()
            data = json.loads(response)
            
            if data.get("type") == "rooms_list":
                self.available_rooms = data.get("data", [])
                return self.available_rooms
            return []
        except Exception as e:
            print(f"获取房间列表失败: {e}")
            return []

    async def prompt_room_choice(self):
        """提示用户选择创建新房间或加入现有房间"""
        print("\n--- 聊天室选项 ---")
        print("1. 创建新房间")
        
        if self.available_rooms:
            print("2. 加入现有房间")
            valid_choices = ["1", "2"]
        else:
            print("(没有可用的房间)")
            valid_choices = ["1"]
        
        while True:
            choice = input("请选择 (输入数字): ").strip()
            if choice in valid_choices:
                if choice == "1":
                    return "create"
                else:
                    return "join"
            print("无效选择，请重试")

    async def create_room(self):
        """创建新房间"""
        room_name = input("请输入房间名称: ").strip()
        if not room_name:
            room_name = f"{self.username}的房间"
            
        self.room_id = str(uuid.uuid4())[:8]  # 生成房间ID
        self.room_name = room_name
        self.is_owner = True
        
        create_msg = {
            "type": "create_room",
            "room": self.room_id,
            "sender": self.user_id,
            "content": self.username,
            "data": {"roomName": room_name}
        }
        
        await self.websocket.send(json.dumps(create_msg))
        print(f"已创建并加入房间: {room_name} (ID: {self.room_id})")

    async def join_existing_room(self):
        """加入现有房间"""
        if not self.available_rooms:
            print("没有可用的房间")
            return False
            
        print("\n--- 可用房间 ---")
        for i, room in enumerate(self.available_rooms, 1):
            created_time = None
            if isinstance(room["createdAt"], str):
                try:
                    created_time = datetime.datetime.fromisoformat(room["createdAt"].replace("Z", "+00:00"))
                except ValueError:
                    created_time = datetime.datetime.now()
            else:
                created_time = datetime.datetime.now()
                
            created_str = created_time.strftime("%Y-%m-%d %H:%M:%S")
            print(f"{i}. {room['name']} (人数: {room['userCount']}, 房主: {room['owner']}, 创建于: {created_str})")
        
        while True:
            try:
                choice = int(input("\n请选择房间编号: "))
                if 1 <= choice <= len(self.available_rooms):
                    selected_room = self.available_rooms[choice-1]
                    self.room_id = selected_room["id"]
                    self.room_name = selected_room["name"]
                    break
                else:
                    print("无效选择，请重试")
            except ValueError:
                print("请输入数字")
        
        join_msg = {
            "type": "join",
            "room": self.room_id,
            "sender": self.user_id,
            "content": self.username
        }
        
        await self.websocket.send(json.dumps(join_msg))
        print(f"正在加入房间: {self.room_name}")
        return True

    async def leave_room(self):
        """主动离开当前房间"""
        if not self.in_room or not self.room_id:
            return
            
        leave_msg = {
            "type": "leave_room",
            "room": self.room_id,
            "sender": self.user_id,
            "content": self.username
        }
        
        try:
            await self.websocket.send(json.dumps(leave_msg))
            print("正在离开房间，等待确认...")
            
            # 等待服务器确认
            for _ in range(5):  # 最多等待5个消息
                message = await self.websocket.recv()
                msg_data = json.loads(message)
                
                if msg_data.get("type") == "leave_confirmed":
                    self.in_room = False
                    self.room_id = None
                    self.room_name = None
                    self.is_owner = False
                    print("已成功离开房间")
                    return True
                    
                # 如果收到的是其他消息，还需继续等待
                if msg_data.get("type") in ["system", "room_closed"]:
                    if "Room is closing" in msg_data.get("content", ""):
                        self.in_room = False
                        self.room_id = None
                        self.room_name = None
                        self.is_owner = False
                        print("房间已关闭")
                        return True
            
            print("未收到离开确认，请重试")
            return False
                
        except Exception as e:
            print(f"离开房间时出错: {e}")
            return False

    async def receive_messages(self):
        """接收并显示来自服务器的消息"""
        try:
            while self.running and self.in_room:
                message = await self.websocket.recv()
                msg_data = json.loads(message)
                
                msg_type = msg_data.get("type", "")
                
                if msg_type == "system":
                    print(f"\n[系统消息] {msg_data['content']}")
                    
                    # 检查是否是房间关闭消息
                    if "Room is closing" in msg_data['content']:
                        print("\n房间已关闭，因为房主离开了")
                        self.in_room = False
                        self.room_id = None
                        self.room_name = None
                        self.is_owner = False
                        break
                        
                elif msg_type == "chat":
                    print(f"\n[{msg_data['sender']}] {msg_data['content']}")
                    
                elif msg_type == "room_info":
                    data = msg_data.get("data", {})
                    room_info = data.get("room", {})
                    users = data.get("users", {})
                    
                    print("\n--- 房间信息 ---")
                    print(f"房间名称: {room_info.get('name')}")
                    print(f"房主: {room_info.get('owner')}")
                    print(f"当前人数: {room_info.get('userCount')}")
                    
                    print("\n--- 当前用户 ---")
                    for user_id, user in users.items():
                        owner_mark = " (房主)" if user.get("isOwner") else ""
                        print(f"- {user.get('name')}{owner_mark}")
                    print("-------------------")
                    
                elif msg_type == "room_closed" or msg_type == "leave_confirmed":
                    print(f"\n[系统消息] {msg_data['content']}")
                    self.in_room = False
                    self.room_id = None
                    self.room_name = None
                    self.is_owner = False
                    break
                    
                elif msg_type == "error":
                    print(f"\n[错误] {msg_data['content']}")
                    if "Room does not exist" in msg_data['content']:
                        self.in_room = False
                        break
                
                # 重新显示输入提示
                if self.running and self.in_room:
                    print("> ", end="", flush=True)
                
        except websockets.exceptions.ConnectionClosed:
            if self.running:
                print("\n与服务器的连接已关闭")
                self.running = False
                self.in_room = False
        except Exception as e:
            if self.running:
                print(f"\n接收消息时出错: {e}")
                self.in_room = False

    async def handle_user_input(self):
        """处理用户输入并发送消息"""
        print("\n开始聊天，输入消息并按回车发送")
        print("命令：")
        print("  /exit - 退出程序")
        print("  /leave - 离开当前房间")
        print("> ", end="", flush=True)
        
        loop = asyncio.get_event_loop()
        
        while self.running and self.in_room:
            message = await loop.run_in_executor(None, sys.stdin.readline)
            message = message.strip()
            
            if not self.running or not self.in_room:
                break
                
            if message.lower() == "/exit":
                print("正在退出程序...")
                self.running = False
                if self.in_room:
                    await self.leave_room()
                break
                
            if message.lower() == "/leave":
                await self.leave_room()
                break
                
            if message and self.websocket and self.in_room:
                chat_message = {
                    "type": "chat",
                    "room": self.room_id,
                    "sender": self.username,
                    "content": message
                }
                try:
                    await self.websocket.send(json.dumps(chat_message))
                except Exception as e:
                    print(f"发送消息失败: {e}")
                    self.in_room = False
                    break
                    
            print("> ", end="", flush=True)
    
    async def disconnect(self):
        """关闭WebSocket连接"""
        if self.websocket:
            # 如果还在房间里，先离开房间
            if self.in_room:
                await self.leave_room()
                
            await self.websocket.close()
            print("已断开与服务器的连接")

def main():
    parser = argparse.ArgumentParser(description="WebSocket聊天室客户端")
    parser.add_argument("--server", default="ws://localhost:8080/ws", 
                      help="WebSocket服务器地址 (默认: ws://localhost:8080/ws)")
    parser.add_argument("--username", required=True, help="用户名")
    
    args = parser.parse_args()
    
    print(f"正在连接到服务器 {args.server}...")
    
    client = ChatClient(args.server, args.username)
    
    try:
        asyncio.run(client.connect())
    except KeyboardInterrupt:
        print("\n程序被中断")
    finally:
        try:
            asyncio.run(client.disconnect())
        except:
            pass

if __name__ == "__main__":
    main()