package service

import (
	"backend/dao"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
)

type RoomService interface {
	RoomListen() (err error)
}

// Player 结构体表示游戏中的玩家
type Player struct {
	Name    string
	Role    Role
	Alive   bool
	Sheriff bool
	IsAI    bool
	Conn    net.Conn
}

// Role 表示玩家角色
type Role struct {
	Name        string
	Description string
	IsWolf      bool
}

// Game 表示游戏状态
type Game struct {
	Players   []*Player
	DayCount  int
	Events    []Event
	VoteMutex sync.Mutex
}

// Event 表示游戏事件
type Event interface {
	Execute(*Game)
}

// NightEvent 夜晚事件
type NightEvent struct {
	Name        string
	Description string
}

func (e NightEvent) Execute(g *Game) {
	// 夜晚逻辑
}

// DayEvent 白天事件
type DayEvent struct {
	Name        string
	Description string
}

func (e DayEvent) Execute(g *Game) {
	// 白天逻辑
}

// JoinRoomSever 游戏服务器
type JoinRoomSever struct {
	Listener       net.Listener
	Game           *Game
	Players        []*Player
	ClientConns    []net.Conn
	NumRealPlayers int
	NumAIPlayers   int
}

// NewJoinRoomServer 创建新的游戏服务器
func NewJoinRoomServer(host string, port int) (*JoinRoomSever, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	return &JoinRoomSever{
		Listener:       listener,
		Game:           &Game{},
		NumRealPlayers: 8,
		NumAIPlayers:   0,
	}, nil
}

type JoinRoomServiceImpl struct {
	joinRoomDAO dao.JoinRoomDAO
}

func NewJoinRoomService(joinRoomDAO dao.JoinRoomDAO) RoomService {
	return &JoinRoomServiceImpl{joinRoomDAO: joinRoomDAO}
}

// Start 启动游戏服务器
func (gs *JoinRoomSever) Start() error {
	fmt.Println("等待玩家连接...")

	// 接受玩家连接
	for i := 0; i < gs.NumRealPlayers; i++ {
		conn, err := gs.Listener.Accept()
		if err != nil {
			return err
		}
		fmt.Printf("玩家%d已连接: %s\n", i+1, conn.RemoteAddr())
		gs.ClientConns = append(gs.ClientConns, conn)
	}

	// 接收玩家名称
	for _, conn := range gs.ClientConns {
		var msg map[string]interface{}
		if err := gs.receiveMessage(conn, &msg); err != nil {
			return err
		}
		name := msg["name"].(string)
		gs.Players = append(gs.Players, &Player{
			Name:  name,
			Alive: true,
			Conn:  conn,
		})
	}

	// 发送等待确认消息
	players := make([]string, len(gs.Players))
	for i, p := range gs.Players {
		players[i] = p.Name
	}
	gs.broadcastMessage(map[string]interface{}{
		"type":    "wait_confirm",
		"players": players,
	})

	// 接收确认
	allConfirmed := true
	for _, conn := range gs.ClientConns {
		var msg map[string]interface{}
		if err := gs.receiveMessage(conn, &msg); err != nil {
			return err
		}
		if !msg["confirm"].(bool) {
			allConfirmed = false
			break
		}
	}

	if allConfirmed {
		// 添加AI玩家
		names := []string{"Stephanie", "Wendy", "Elmy", "Sham", "Jeffry", "Kelly"}
		for i := 0; i < gs.NumAIPlayers; i++ {
			if i < len(names) {
				gs.Players = append(gs.Players, &Player{
					Name:  names[i],
					IsAI:  true,
					Alive: true,
				})
			}
		}

		// 随机分配角色
		gs.randomAllocate()

		// 发送游戏状态
		gs.sendGameStatus()

		// 设置游戏事件
		gs.Game.Events = []Event{
			NightEvent{"黑夜", "狼人行动"},
			DayEvent{"白天", "讨论和投票"},
		}

		// 运行游戏
		gs.runGame()
	} else {
		gs.broadcastMessage(map[string]interface{}{
			"type": "game_cancelled",
		})
	}

	return nil
}

// broadcastMessage 广播消息给所有客户端
func (gs *JoinRoomSever) broadcastMessage(msg map[string]interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("JSON编码错误:", err)
		return
	}

	for _, conn := range gs.ClientConns {
		if _, err := conn.Write(data); err != nil {
			log.Println("发送消息失败:", err)
		}
	}
}

// sendMessage 发送消息给特定玩家
func (gs *JoinRoomSever) sendMessage(msg map[string]interface{}, playerIndex int) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if playerIndex < len(gs.ClientConns) {
		_, err = gs.ClientConns[playerIndex].Write(data)
		return err
	}
	return nil
}

// receiveMessage 从连接接收消息
func (gs *JoinRoomSever) receiveMessage(conn net.Conn, msg *map[string]interface{}) error {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf[:n], msg)
}

// randomAllocate 随机分配角色
func (gs *JoinRoomSever) randomAllocate() {
	// 这里实现角色分配逻辑
	// 简化示例：给第一个玩家分配狼人角色
	if len(gs.Players) > 0 {
		gs.Players[0].Role = Role{Name: "狼人", IsWolf: true}
	}
}

// sendGameStatus 发送游戏状态给所有玩家
func (gs *JoinRoomSever) sendGameStatus() {
	for i, player := range gs.Players {
		playersInfo := make([]map[string]interface{}, len(gs.Players))
		for j, p := range gs.Players {
			role := "未知"
			if p == player || (p.Role.IsWolf && player.Role.IsWolf) {
				role = p.Role.Name
			}
			playersInfo[j] = map[string]interface{}{
				"name":    p.Name,
				"role":    role,
				"alive":   p.Alive,
				"sheriff": p.Sheriff,
			}
		}

		status := map[string]interface{}{
			"type":      "game_status",
			"role":      player.Role.Name,
			"players":   playersInfo,
			"day_count": gs.Game.DayCount,
		}

		if player.Conn != nil {
			if err := gs.sendMessage(status, i); err != nil {
				log.Println("发送游戏状态失败:", err)
			}
		}
	}
}

// runGame 运行游戏主循环
func (gs *JoinRoomSever) runGame() {
	// 实现游戏主循环
	for _, event := range gs.Game.Events {
		event.Execute(gs.Game)
	}
}

func (s *JoinRoomServiceImpl) RoomListen() (err error) {
	server, err := NewJoinRoomServer("localhost", 5000)
	if err != nil {
		log.Fatal("无法启动服务器:", err)
		return err
	}
	defer server.Listener.Close()

	if err := server.Start(); err != nil {
		log.Fatal("服务器错误:", err)
		return err
	}
	return nil
}
