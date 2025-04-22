import socket
import json
import threading
import time
import random
import argparse
import sys


class WerewolfClient:
    def __init__(self, host='localhost', port=5001, player_name=None):
        """初始化狼人杀客户端"""
        self.host = host
        self.port = port
        self.player_name = player_name or f"Player_{random.randint(1000, 9999)}"
        self.socket = None
        self.running = False
        self.game_state = {
            "role": None,
            "players": [],
            "day_count": 0,
            "alive": True,
            "is_sheriff": False
        }
        self.action_handlers = {
            "wait_confirm": self.handle_wait_confirm,
            "game_status": self.handle_game_status,
            "sheriff_election": self.handle_sheriff_election,
            "night_action": self.handle_night_action,
            "seer_result": self.handle_seer_result,
            "day_vote": self.handle_day_vote,
            "game_end": self.handle_game_end
        }
        self.action_callback = None
        self.messages = []
        self.connection_status = "未连接"

    def connect(self):
        """连接到游戏服务器"""
        try:
            self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            self.socket.connect((self.host, self.port))
            self.connection_status = "已连接"

            # 发送玩家名称
            self.send_message({"name": self.player_name})

            # 启动消息接收线程
            self.running = True
            threading.Thread(target=self.receive_messages, daemon=True).start()

            return True
        except Exception as e:
            self.log(f"连接失败: {e}")
            self.connection_status = f"连接失败: {e}"
            return False

    def disconnect(self):
        """断开与服务器的连接"""
        self.running = False
        if self.socket:
            try:
                self.socket.close()
            except:
                pass
            self.socket = None
        self.connection_status = "已断开"

    def send_message(self, message):
        """向服务器发送消息"""
        if not self.socket:
            self.log("未连接到服务器")
            return False

        try:
            data = json.dumps(message).encode('utf-8')
            self.socket.sendall(data)
            return True
        except Exception as e:
            self.log(f"发送消息失败: {e}")
            return False

    def receive_messages(self):
        """接收并处理来自服务器的消息"""
        buffer = ""

        while self.running and self.socket:
            try:
                # 接收数据
                data = self.socket.recv(4096)
                if not data:
                    self.log("服务器断开连接")
                    break

                # 解析JSON消息
                buffer += data.decode('utf-8')

                # 处理可能的多条JSON消息
                while True:
                    try:
                        message, buffer = self.parse_json_message(buffer)
                        if not message:
                            break

                        # 处理消息
                        self.handle_message(message)
                    except json.JSONDecodeError:
                        # 不完整的JSON，等待更多数据
                        break

            except Exception as e:
                self.log(f"接收消息错误: {e}")
                break

        self.disconnect()

    def parse_json_message(self, buffer):
        """从缓冲区解析单个JSON消息"""
        # 找到第一个完整的JSON对象
        depth = 0
        inString = False
        escape = False
        start = 0

        for i, char in enumerate(buffer):
            if char == '"' and not escape:
                inString = not inString
            elif char == '\\' and inString:
                escape = not escape
            elif not inString:
                if char == '{':
                    if depth == 0:
                        start = i
                    depth += 1
                elif char == '}':
                    depth -= 1
                    if depth == 0:
                        # 找到完整的JSON
                        try:
                            message = json.loads(buffer[start:i+1])
                            return message, buffer[i+1:]
                        except:
                            pass

            if char != '\\':
                escape = False

        return None, buffer

    def handle_message(self, message):
        """处理接收到的消息"""
        message_type = message.get("type")
        self.log(f"收到消息: {message_type}")

        if message_type in self.action_handlers:
            self.action_handlers[message_type](message)
        else:
            self.log(f"未知消息类型: {message_type}")

    # 以下是各种消息处理函数
    def handle_wait_confirm(self, message):
        """处理等待确认消息"""
        players = message.get("players", [])
        self.log(f"游戏玩家: {', '.join(players)}")

        # 自动确认
        self.send_message({"confirm": True})

    def handle_game_status(self, message):
        """处理游戏状态更新"""
        self.game_state["role"] = message.get("role")
        self.game_state["players"] = message.get("players", [])
        self.game_state["day_count"] = message.get("day_count", 0)

        # 更新自己的状态
        for player in self.game_state["players"]:
            if player[0] == self.player_name:
                self.game_state["alive"] = player[2]
                self.game_state["is_sheriff"] = player[3]
                break

        self.log(f"游戏状态更新: 角色={self.game_state['role']}, 天数={self.game_state['day_count']}")
        self.log_player_status()

    def handle_sheriff_election(self, message):
        """处理警长选举"""
        candidates = message.get("candidates", [])
        self.log(f"警长选举，候选人: {', '.join(candidates)}")

        # 如果设置了回调，使用回调处理选举
        if self.action_callback:
            vote = self.action_callback("sheriff_election", candidates)
        else:
            # 默认随机选择
            vote = random.choice(candidates) if candidates else None

        if vote:
            self.log(f"投票给 {vote} 当警长")
            self.send_message({"vote": vote})

    def handle_night_action(self, message):
        """处理夜晚行动"""
        action = message.get("action")

        if action == "werewolf":
            # 狼人行动
            candidates = message.get("candidates", [])
            self.log(f"请选择击杀目标: {', '.join(candidates)}")

            if self.action_callback:
                target = self.action_callback("werewolf", candidates)
            else:
                target = random.choice(candidates) if candidates else None

            if target:
                self.log(f"选择击杀 {target}")
                self.send_message({"target": target})

        elif action == "witch":
            # 女巫行动
            has_poison = message.get("has_poison", False)
            has_antidote = message.get("has_antidote", False)
            dead_players = message.get("dead_players", [])
            alive_players = message.get("alive_players", [])

            self.log(f"女巫行动 - 解药: {has_antidote}, 毒药: {has_poison}")
            if dead_players and dead_players[0]:
                self.log(f"今晚死亡: {dead_players[0]}")

            response = {}

            # 处理解药
            if has_antidote and dead_players and dead_players[0]:
                if self.action_callback:
                    save = self.action_callback("witch_save", dead_players[0])
                else:
                    save = random.choice([True, False])

                if save:
                    self.log(f"使用解药救 {dead_players[0]}")
                    response["save"] = dead_players[0]

            # 处理毒药
            if has_poison:
                if self.action_callback:
                    poison_target = self.action_callback("witch_poison", alive_players)
                else:
                    poison_target = None if random.random() > 0.3 else random.choice(alive_players)

                if poison_target:
                    self.log(f"使用毒药毒 {poison_target}")
                    response["poison"] = poison_target

            self.send_message(response)

        elif action == "seer":
            # 预言家行动
            candidates = message.get("candidates", [])
            self.log(f"预言家请选择查验目标: {', '.join(candidates)}")

            if self.action_callback:
                target = self.action_callback("seer", candidates)
            else:
                target = random.choice(candidates) if candidates else None

            if target:
                self.log(f"选择查验 {target}")
                self.send_message({"target": target})

    def handle_seer_result(self, message):
        """处理预言家查验结果"""
        target = message.get("target")
        result = message.get("result")

        if target and result:
            self.log(f"查验结果: {target} 是 {result}")

    def handle_day_vote(self, message):
        """处理白天投票"""
        candidates = message.get("candidates", [])
        self.log(f"请投票出局: {', '.join(candidates)}")

        if self.action_callback:
            vote = self.action_callback("day_vote", candidates)
        else:
            vote = random.choice(candidates) if candidates else None

        if vote:
            self.log(f"投票给 {vote}")
            self.send_message({"vote": vote})

    def handle_game_end(self, message):
        """处理游戏结束"""
        winner = message.get("winner", "未知")
        self.log(f"游戏结束，{winner}阵营胜利！")
        self.running = False

    def log(self, message):
        """记录消息"""
        timestamp = time.strftime("%H:%M:%S", time.localtime())
        log_message = f"[{timestamp}] {message}"
        print(log_message)
        self.messages.append(log_message)

    def log_player_status(self):
        """记录玩家状态"""
        alive_players = []
        dead_players = []

        for player in self.game_state["players"]:
            name, role, alive, sheriff = player
            status = f"{name}"
            if sheriff:
                status += "(警长)"
            if role != "未知":
                status += f"[{role}]"

            if alive:
                alive_players.append(status)
            else:
                dead_players.append(status)

        self.log(f"存活玩家: {', '.join(alive_players)}")
        if dead_players:
            self.log(f"死亡玩家: {', '.join(dead_players)}")

    def set_action_callback(self, callback):
        """设置行动回调函数"""
        self.action_callback = callback


def interactive_callback(action_type, options):
    """交互式决策回调函数"""
    print(f"\n=== 需要你的决策: {action_type} ===")

    if action_type == "sheriff_election":
        print("请选择警长候选人:")
        return prompt_selection(options)

    elif action_type == "werewolf":
        print("请选择你要击杀的目标:")
        return prompt_selection(options)

    elif action_type == "witch_save":
        print(f"今晚 {options} 死亡，是否使用解药? (y/n)")
        choice = input("> ").strip().lower()
        return choice.startswith('y')

    elif action_type == "witch_poison":
        print("是否使用毒药? (y/n)")
        choice = input("> ").strip().lower()
        if choice.startswith('y'):
            print("请选择你要毒杀的目标:")
            return prompt_selection(options)
        return None

    elif action_type == "seer":
        print("请选择你要查验的目标:")
        return prompt_selection(options)

    elif action_type == "day_vote":
        print("请选择你要投票出局的玩家:")
        return prompt_selection(options)

    # 默认情况随机选择
    return random.choice(options) if options else None


def prompt_selection(options):
    """提示用户从选项中选择"""
    if not options:
        return None

    for i, option in enumerate(options, 1):
        print(f"{i}. {option}")

    while True:
        try:
            choice = input("请输入选项编号> ").strip()
            idx = int(choice) - 1
            if 0 <= idx < len(options):
                return options[idx]
            print(f"无效的选择，请输入1-{len(options)}之间的数字")
        except ValueError:
            print("请输入有效的数字")


def main():
    """主函数，处理命令行参数并启动客户端"""
    parser = argparse.ArgumentParser(description="狼人杀游戏客户端")

    # 服务器设置
    parser.add_argument("--server", default="localhost", help="服务器地址")
    parser.add_argument("--port", type=int, default=5001, help="服务器端口")
    parser.add_argument("--api-port", type=int, default=5000, help="API服务器端口")

    # 玩家设置
    parser.add_argument("--name", default=None, help="玩家名称")

    # 游戏模式
    mode_group = parser.add_mutually_exclusive_group()
    mode_group.add_argument("--auto", action="store_true", help="自动模式，随机决策")
    mode_group.add_argument("--interactive", action="store_true", help="交互式模式，手动决策")

    # 创建游戏选项
    parser.add_argument("--create", action="store_true", help="创建新游戏")
    parser.add_argument("--real-players", type=int, default=1, help="真实玩家数量")
    parser.add_argument("--ai-players", type=int, default=7, help="AI玩家数量")

    args = parser.parse_args()

    # 设置玩家名称
    player_name = args.name
    if not player_name:
        if args.interactive:
            player_name = input("请输入你的名字> ").strip()
            if not player_name:
                player_name = f"Player_{random.randint(1000, 9999)}"
        else:
            player_name = f"Auto_{random.randint(1000, 9999)}"

    # 决定是否创建新游戏
    game_port = args.port
    if args.create:
        try:
            import requests
            api_url = f"http://{args.server}:{args.api_port}/create_game"
            response = requests.post(api_url, json={
                "real_players": args.real_players,
                "ai_players": args.ai_players
            })

            if response.status_code == 200:
                data = response.json()
                game_port = data.get("port")
                print(f"创建了新游戏，端口: {game_port}")
            else:
                print(f"创建游戏失败: {response.text}")
                return
        except Exception as e:
            print(f"无法连接到API服务器: {e}")
            print(f"使用默认端口 {game_port} 连接")

    # 创建客户端
    client = WerewolfClient(host=args.server, port=game_port, player_name=player_name)

    # 设置交互模式
    if args.interactive:
        client.set_action_callback(interactive_callback)
        print("交互模式已启用，你将需要手动做出所有决策")
    else:
        print("自动模式已启用，客户端将随机做出决策")

    # 连接到服务器
    print(f"连接到服务器 {args.server}:{game_port}...")
    if not client.connect():
        print("连接失败，退出")
        return

    print(f"成功连接！玩家名称: {player_name}")
    print("游戏开始，等待服务器消息...")

    # 保持程序运行，直到游戏结束
    try:
        while client.running:
            time.sleep(0.1)
    except KeyboardInterrupt:
        print("\n用户中断，断开连接")
    finally:
        client.disconnect()

    print("\n=== 游戏结束 ===")


if __name__ == "__main__":
    main()