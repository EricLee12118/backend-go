package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsOwner bool   `json:"isOwner"`
}

type Message struct {
	Type    string      `json:"type"`
	Room    string      `json:"room"`
	Content string      `json:"content"`
	Sender  string      `json:"sender"`
	Data    interface{} `json:"data,omitempty"` // 用于携带额外数据
}

type Room struct {
	ID        string                     `json:"id"`
	Name      string                     `json:"name"`
	CreatedAt time.Time                  `json:"createdAt"`
	Owner     string                     `json:"owner"` // 房主ID
	Users     map[string]User            `json:"users"` // 用户列表，key为用户ID
	clients   map[*websocket.Conn]string // 连接到用户ID的映射
	mu        sync.Mutex
}

type RoomInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Owner     string    `json:"owner"`     // 房主名
	UserCount int       `json:"userCount"` // 用户数量
}

var (
	rooms    = make(map[string]*Room)
	roomsMu  sync.Mutex
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func getRoom(roomID string) *Room {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	if room, exists := rooms[roomID]; exists {
		return room
	}

	return nil
}

func createRoom(roomID, roomName, ownerID, ownerName string) *Room {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	// 如果房间已存在，返回现有房间
	if room, exists := rooms[roomID]; exists {
		return room
	}

	// 创建新房间
	owner := User{
		ID:      ownerID,
		Name:    ownerName,
		IsOwner: true,
	}

	room := &Room{
		ID:        roomID,
		Name:      roomName,
		CreatedAt: time.Now(),
		Owner:     ownerID,
		Users:     make(map[string]User),
		clients:   make(map[*websocket.Conn]string),
	}

	room.Users[ownerID] = owner
	rooms[roomID] = room
	log.Printf("创建房间: %s (ID: %s) 房主: %s", roomName, roomID, ownerName)
	return room
}

func removeRoom(roomID string) {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	if room, exists := rooms[roomID]; exists {
		// 关闭所有连接
		room.mu.Lock()
		for conn := range room.clients {
			// 不直接关闭连接，只发送房间关闭消息
			closeMsg := Message{
				Type:    "room_closed",
				Room:    roomID,
				Content: "Room has been closed by the owner",
			}
			closeBytes, _ := json.Marshal(closeMsg)
			conn.WriteMessage(websocket.TextMessage, closeBytes)
		}
		room.mu.Unlock()

		// 删除房间
		delete(rooms, roomID)
		log.Printf("房间 %s 已销毁", roomID)
	}
}

// 新增：退出房间函数

func leaveRoom(room *Room, conn *websocket.Conn, userID string) (bool, string) {
	if room == nil || userID == "" {
		return false, ""
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	userName := ""
	isOwner := false

	// 获取用户信息
	if user, exists := room.Users[userID]; exists {
		userName = user.Name
		isOwner = user.IsOwner
	}

	// 从房间中移除用户
	delete(room.clients, conn)
	delete(room.Users, userID)

	// 返回是否是房主和用户名
	return isOwner, userName
}

func getAllRooms() []RoomInfo {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	var roomInfos []RoomInfo
	for _, room := range rooms {
		room.mu.Lock()
		ownerName := ""
		for _, user := range room.Users {
			if user.ID == room.Owner {
				ownerName = user.Name
				break
			}
		}

		roomInfo := RoomInfo{
			ID:        room.ID,
			Name:      room.Name,
			CreatedAt: room.CreatedAt,
			Owner:     ownerName,
			UserCount: len(room.Users),
		}
		room.mu.Unlock()

		roomInfos = append(roomInfos, roomInfo)
	}

	return roomInfos
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	var userID string
	var userName string
	var room *Room

	// 消息处理循环
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)

			// 如果已经加入房间，则处理离开逻辑
			if room != nil && userID != "" {
				isOwner, userName := leaveRoom(room, conn, userID)

				if isOwner {
					// 如果是房主断开连接，销毁房间
					closeMsg := Message{
						Type:    "system",
						Room:    room.ID,
						Content: "Room is closing because the owner left",
					}
					broadcastToRoom(room, closeMsg)
					removeRoom(room.ID)
				} else if userName != "" {
					// 通知其他人该用户已离开
					userCount := 0
					room.mu.Lock()
					userCount = len(room.Users)
					room.mu.Unlock()

					leaveMsg := Message{
						Type:    "system",
						Room:    room.ID,
						Content: userName + " left the room",
						Data: struct {
							UserID    string `json:"userId"`
							UserCount int    `json:"userCount"`
						}{
							UserID:    userID,
							UserCount: userCount,
						},
					}
					broadcastToRoom(room, leaveMsg)
				}
			}
			return
		}

		var message Message
		if err := json.Unmarshal(msg, &message); err != nil {
			log.Println("JSON decode error:", err)
			continue
		}

		// 处理不同类型的消息
		switch message.Type {
		case "get_rooms":
			// 获取房间列表
			roomInfos := getAllRooms()
			response := Message{
				Type:    "rooms_list",
				Content: "Available rooms",
				Data:    roomInfos,
			}

			responseBytes, _ := json.Marshal(response)
			if err := conn.WriteMessage(websocket.TextMessage, responseBytes); err != nil {
				log.Println("Write error:", err)
				continue
			}

		case "create_room":
			// 创建房间
			userID = message.Sender
			userName = message.Content

			var roomName string
			// 从message.Data中提取roomName
			if message.Data != nil {
				dataMap, ok := message.Data.(map[string]interface{})
				if ok {
					if nameVal, exists := dataMap["roomName"]; exists {
						roomName, _ = nameVal.(string)
					}
				}
			}

			if roomName == "" {
				roomName = "Room " + message.Room // 使用ID作为默认名称
			}

			room = createRoom(message.Room, roomName, userID, userName)

			// 将用户添加到房间
			room.mu.Lock()
			room.clients[conn] = userID
			room.mu.Unlock()

			// 发送确认消息
			confirmMsg := Message{
				Type:    "system",
				Room:    message.Room,
				Content: userName + " created and joined the room",
			}
			broadcastToRoom(room, confirmMsg)

			// 发送房间信息
			sendRoomInfo(conn, room)

		case "join":
			// 加入现有房间
			userID = message.Sender
			userName = message.Content

			room = getRoom(message.Room)
			if room == nil {
				// 房间不存在
				errorMsg := Message{
					Type:    "error",
					Content: "Room does not exist",
				}
				errorBytes, _ := json.Marshal(errorMsg)
				conn.WriteMessage(websocket.TextMessage, errorBytes)
				continue
			}

			// 将用户添加到房间
			room.mu.Lock()
			room.clients[conn] = userID
			room.Users[userID] = User{
				ID:      userID,
				Name:    userName,
				IsOwner: false, // 加入的用户不是房主
			}
			room.mu.Unlock()

			// 通知房间有新用户加入
			joinMsg := Message{
				Type:    "system",
				Room:    message.Room,
				Content: userName + " joined the room",
			}
			broadcastToRoom(room, joinMsg)

			// 发送房间信息
			sendRoomInfo(conn, room)

		case "leave_room":
			// 用户主动离开房间
			if room == nil || userID == "" {
				continue
			}

			isOwner, leavingUserName := leaveRoom(room, conn, userID)

			// 发送确认离开消息给请求离开的用户
			leaveConfirmMsg := Message{
				Type:    "leave_confirmed",
				Room:    room.ID,
				Content: "You have left the room",
			}
			leaveConfirmBytes, _ := json.Marshal(leaveConfirmMsg)
			conn.WriteMessage(websocket.TextMessage, leaveConfirmBytes)

			if isOwner {
				// 如果是房主离开，销毁房间
				closeMsg := Message{
					Type:    "system",
					Room:    room.ID,
					Content: "Room is closing because the owner left",
				}
				broadcastToRoom(room, closeMsg)
				removeRoom(room.ID)
			} else {
				// 通知其他人该用户已离开
				userCount := 0
				room.mu.Lock()
				userCount = len(room.Users)
				room.mu.Unlock()

				leaveMsg := Message{
					Type:    "system",
					Room:    room.ID,
					Content: leavingUserName + " left the room",
					Data: struct {
						UserID    string `json:"userId"`
						UserCount int    `json:"userCount"`
					}{
						UserID:    userID,
						UserCount: userCount,
					},
				}
				broadcastToRoom(room, leaveMsg)
			}

			// 重置用户状态
			room = nil

		case "chat":
			// 聊天消息
			if room == nil {
				log.Println("Chat message received but user not in a room")
				continue
			}

			// 替换发送者名称为实际存储的用户名
			if userName != "" {
				message.Sender = userName
			}

			// 广播消息
			broadcastToRoom(room, message)
		}
	}
}

func sendRoomInfo(conn *websocket.Conn, room *Room) {
	room.mu.Lock()
	defer room.mu.Unlock()

	// 找到房主名称
	ownerName := ""
	for _, user := range room.Users {
		if user.ID == room.Owner {
			ownerName = user.Name
			break
		}
	}

	roomInfo := struct {
		Room  RoomInfo        `json:"room"`
		Users map[string]User `json:"users"`
	}{
		Room: RoomInfo{
			ID:        room.ID,
			Name:      room.Name,
			CreatedAt: room.CreatedAt,
			Owner:     ownerName,
			UserCount: len(room.Users),
		},
		Users: room.Users,
	}

	// 发送房间信息
	roomInfoMsg := Message{
		Type:    "room_info",
		Room:    room.ID,
		Content: "Room information",
		Data:    roomInfo,
	}

	roomInfoBytes, _ := json.Marshal(roomInfoMsg)
	conn.WriteMessage(websocket.TextMessage, roomInfoBytes)
}

func broadcastToRoom(room *Room, message Message) {
	msgBytes, _ := json.Marshal(message)

	room.mu.Lock()
	defer room.mu.Unlock()

	for client := range room.clients {
		err := client.WriteMessage(websocket.TextMessage, msgBytes)
		if err != nil {
			log.Println("Write error:", err)
			client.Close()
			delete(room.clients, client)
		}
	}
}

func main() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/ws", handleWebSocket)

	// 添加一个API端点，用于获取房间列表
	http.HandleFunc("/api/rooms", func(w http.ResponseWriter, r *http.Request) {
		roomInfos := getAllRooms()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(roomInfos)
	})

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
