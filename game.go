package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// 用于创建游戏的请求结构
type CreateGameRequest struct {
	RealPlayers int `json:"real_players"`
	AIPlayers   int `json:"ai_players"`
}

// 创建游戏的响应结构
type CreateGameResponse struct {
	GameID int `json:"game_id"`
	Port   int `json:"port"`
}

// 游戏状态响应结构
type GameStatusResponse struct {
	GameID  int         `json:"game_id"`
	Running bool        `json:"running"`
	Results *GameResult `json:"results,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ============ 角色定义 ============

// 角色接口
type Role interface {
	GetName() string
	NightAction(player *Player, allPlayers []*Player) map[string]interface{}
	DayAction(player *Player, allPlayers []*Player) map[string]interface{}
}

// 狼人角色
type Wolf struct {
	Name string
}

func NewWolf() *Wolf {
	return &Wolf{Name: "狼人"}
}

func (w *Wolf) GetName() string {
	return w.Name
}

func (w *Wolf) NightAction(player *Player, allPlayers []*Player) map[string]interface{} {
	if player.IsAI {
		validTargets := []*Player{}
		for _, p := range allPlayers {
			if p.Alive && !p.IsWolf() {
				validTargets = append(validTargets, p)
			}
		}
		if len(validTargets) > 0 {
			target := validTargets[rand.Intn(len(validTargets))]
			return map[string]interface{}{"vote": target.Name}
		}
	}
	return nil
}

func (w *Wolf) DayAction(player *Player, allPlayers []*Player) map[string]interface{} {
	return nil
}

// 平民角色
type Villager struct {
	Name string
}

func NewVillager() *Villager {
	return &Villager{Name: "平民"}
}

func (v *Villager) GetName() string {
	return v.Name
}

func (v *Villager) NightAction(player *Player, allPlayers []*Player) map[string]interface{} {
	return nil
}

func (v *Villager) DayAction(player *Player, allPlayers []*Player) map[string]interface{} {
	return nil
}

// 女巫角色
type Witch struct {
	Name           string
	HasPoison      bool
	HasAntidote    bool
	Game           *WerewolfGame
	PoisonedTarget string
}

func NewWitch() *Witch {
	return &Witch{
		Name:           "女巫",
		HasPoison:      true,
		HasAntidote:    true,
		PoisonedTarget: "",
	}
}

func (w *Witch) GetName() string {
	return w.Name
}

func (w *Witch) NightAction(player *Player, allPlayers []*Player) map[string]interface{} {
	if !player.IsAI {
		return nil
	}

	// AI女巫使用解药
	if player.IsAI && w.HasAntidote && w.Game != nil && w.Game.WolfKillTarget != "" {
		// 只对当晚狼人杀死的人使用解药
		if rand.Float64() < 0.7 {
			w.HasAntidote = false
			w.Game.Antidote = true
			return map[string]interface{}{
				"action": "save",
				"target": w.Game.WolfKillTarget,
			}
		}
	}

	// AI女巫使用毒药
	if player.IsAI && w.HasPoison && w.PoisonedTarget == "" {
		validTargets := []*Player{}
		for _, p := range allPlayers {
			if p.Alive && p.IsWolf() {
				validTargets = append(validTargets, p)
			}
		}
		if len(validTargets) > 0 && rand.Float64() < 0.3 {
			target := validTargets[rand.Intn(len(validTargets))]
			w.HasPoison = false
			w.PoisonedTarget = target.Name
			return map[string]interface{}{
				"action": "poison",
				"target": target.Name,
			}
		}
	}

	return nil
}

func (w *Witch) DayAction(player *Player, allPlayers []*Player) map[string]interface{} {
	return nil
}

// 预言家角色
type Seer struct {
	Name string
}

func NewSeer() *Seer {
	return &Seer{Name: "预言家"}
}

func (s *Seer) GetName() string {
	return s.Name
}

func (s *Seer) NightAction(player *Player, allPlayers []*Player) map[string]interface{} {
	if !player.IsAI {
		return nil
	}

	validTargets := []*Player{}
	for _, p := range allPlayers {
		if p.Alive && p != player {
			validTargets = append(validTargets, p)
		}
	}

	if len(validTargets) > 0 {
		target := validTargets[rand.Intn(len(validTargets))]
		result := "狼人"
		if !target.IsWolf() {
			result = "好人"
		}
		return map[string]interface{}{
			"action": "check",
			"target": target.Name,
			"result": result,
		}
	}
	return nil
}

func (s *Seer) DayAction(player *Player, allPlayers []*Player) map[string]interface{} {
	return nil
}

// 猎人角色
type Hunter struct {
	Name string
}

func NewHunter() *Hunter {
	return &Hunter{Name: "猎人"}
}

func (h *Hunter) GetName() string {
	return h.Name
}

func (h *Hunter) NightAction(player *Player, allPlayers []*Player) map[string]interface{} {
	return nil
}

func (h *Hunter) DayAction(player *Player, allPlayers []*Player) map[string]interface{} {
	if !player.Alive {
		if !player.IsAI {
			return nil
		}

		validTargets := []*Player{}
		for _, p := range allPlayers {
			if p.Alive && p != player {
				validTargets = append(validTargets, p)
			}
		}
		if len(validTargets) > 0 {
			target := validTargets[rand.Intn(len(validTargets))]
			target.Alive = false
			return map[string]interface{}{"target": target.Name}
		}
	}
	return nil
}

// 创建角色函数
func CreateRole(roleName string) Role {
	switch roleName {
	case "狼人":
		return NewWolf()
	case "平民":
		return NewVillager()
	case "女巫":
		return NewWitch()
	case "预言家":
		return NewSeer()
	case "猎人":
		return NewHunter()
	default:
		return NewVillager()
	}
}

// ============ 玩家定义 ============

// 玩家结构体
type Player struct {
	Name     string
	Role     Role
	IsAI     bool
	Alive    bool
	Votes    float64
	Sheriff  bool
	Poisoned bool
}

// 创建新玩家
func NewPlayer(name string, isAI bool) *Player {
	return &Player{
		Name:     name,
		Role:     nil,
		IsAI:     isAI,
		Alive:    true,
		Votes:    0,
		Sheriff:  false,
		Poisoned: false,
	}
}

// 判断是否是狼人
func (p *Player) IsWolf() bool {
	_, ok := p.Role.(*Wolf)
	return ok
}

// 判断是否是预言家
func (p *Player) IsSeer() bool {
	_, ok := p.Role.(*Seer)
	return ok
}

// 判断是否是女巫
func (p *Player) IsWitch() bool {
	witch, ok := p.Role.(*Witch)
	return ok && witch != nil
}

// 判断是否是猎人
func (p *Player) IsHunter() bool {
	_, ok := p.Role.(*Hunter)
	return ok
}

// 夜间行动
func (p *Player) NightAction(allPlayers []*Player) map[string]interface{} {
	if p.Role != nil {
		return p.Role.NightAction(p, allPlayers)
	}
	return nil
}

// 白天行动
func (p *Player) DayAction(allPlayers []*Player) map[string]interface{} {
	if p.Role != nil {
		return p.Role.DayAction(p, allPlayers)
	}
	return nil
}

// ============ 游戏事件定义 ============

// 游戏事件接口
type GameEvent interface {
	GetName() string
	GetDescription() string
	Execute(game *WerewolfGame)
}

// 白天事件
type DayEvent struct {
	Name        string
	Description string
}

func NewDayEvent(name, description string) *DayEvent {
	return &DayEvent{
		Name:        name,
		Description: description,
	}
}

func (d *DayEvent) GetName() string {
	return d.Name
}

func (d *DayEvent) GetDescription() string {
	return d.Description
}

func (d *DayEvent) Execute(game *WerewolfGame) {
	if game.Sheriff == nil {
		fmt.Println("警长选举", "玩家投票选举警长")
		game.ElectSheriff()
	}
	fmt.Printf("\n=== %s ===\n", d.Name)
	game.DayActions()
	game.Vote()
}

// 夜晚事件
type NightEvent struct {
	Name        string
	Description string
}

func NewNightEvent(name, description string) *NightEvent {
	return &NightEvent{
		Name:        name,
		Description: description,
	}
}

func (n *NightEvent) GetName() string {
	return n.Name
}

func (n *NightEvent) GetDescription() string {
	return n.Description
}

func (n *NightEvent) Execute(game *WerewolfGame) {
	fmt.Printf("\n=== %s ===\n", n.Name)
	game.NightActions()
}

// ============ 游戏核心定义 ============

// 狼人杀游戏
type WerewolfGame struct {
	Players          []*Player
	Events           []GameEvent
	DayCount         int
	Sheriff          *Player
	SheriffElect     bool
	WolfKillTarget   string
	Antidote         bool
	HumanWolfVotes   map[string]int
	PoisonedPlayers  []string
	WinnerIsWerewolf bool
	Logs             []string
	mu               sync.Mutex
}

// 创建新游戏
func NewWerewolfGame() *WerewolfGame {
	return &WerewolfGame{
		Players:         []*Player{},
		Events:          []GameEvent{},
		DayCount:        1,
		Sheriff:         nil,
		SheriffElect:    false,
		WolfKillTarget:  "",
		Antidote:        false,
		HumanWolfVotes:  make(map[string]int),
		PoisonedPlayers: []string{},
		Logs:            []string{},
	}
}

// 添加日志
func (g *WerewolfGame) Log(message string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	fmt.Println(message)
	g.Logs = append(g.Logs, message)
}

// 随机分配角色
func (g *WerewolfGame) RandomAllocate() {
	numPlayers := len(g.Players)
	roles := []Role{}

	// 分配狼人
	werewolfCount := 1
	if numPlayers >= 5 {
		werewolfCount = 2
	}
	for i := 0; i < werewolfCount; i++ {
		roles = append(roles, NewWolf())
	}

	// 分配预言家和女巫
	roles = append(roles, NewSeer())
	witch := NewWitch()
	witch.Game = g
	roles = append(roles, witch)

	// 分配剩余平民
	neededVillagers := numPlayers - len(roles)
	for i := 0; i < neededVillagers; i++ {
		roles = append(roles, NewVillager())
	}

	rand.Shuffle(len(roles), func(i, j int) {
		roles[i], roles[j] = roles[j], roles[i]
	})

	// 分配给玩家
	for i, player := range g.Players {
		if i < len(roles) {
			player.Role = roles[i]
		} else {
			player.Role = NewVillager()
		}
	}

	// 记录分配情况
	g.Log("=== 角色分配 ===")
	for _, p := range g.Players {
		g.Log(fmt.Sprintf("%s 的角色是 %s", p.Name, p.Role.GetName()))
	}
}

// 添加玩家
func (g *WerewolfGame) AddPlayer(player *Player) {
	g.Players = append(g.Players, player)
}

// 选举警长
func (g *WerewolfGame) ElectSheriff() {
	rounds := 3
	for roundNumber := 1; roundNumber <= rounds; roundNumber++ {
		maxVotes := float64(0)
		for _, p := range g.Players {
			if p.Alive && p.Votes > maxVotes {
				maxVotes = p.Votes
			}
		}

		candidates := []*Player{}
		for _, p := range g.Players {
			if p.Alive && p.Votes == maxVotes {
				candidates = append(candidates, p)
			}
		}

		if len(candidates) == 1 {
			g.Sheriff = candidates[0]
			g.Sheriff.Sheriff = true
			g.Log(fmt.Sprintf("\n%s 当选警长！", g.Sheriff.Name))
			g.resetVotes()
			return
		}

		g.Log(fmt.Sprintf("第 %d 轮选举没有选出警长。", roundNumber))
		g.resetVotes()
	}

	g.Log("警长选举失败，本局没有警长")
}

// 重置投票
func (g *WerewolfGame) resetVotes() {
	for _, p := range g.Players {
		p.Votes = 0
	}
}

// 投票出局
func (g *WerewolfGame) Vote() {
	alivePlayers := []*Player{}
	for _, p := range g.Players {
		if p.Alive {
			alivePlayers = append(alivePlayers, p)
		}
	}

	if len(alivePlayers) == 0 {
		return
	}

	maxVotes := float64(0)
	for _, p := range alivePlayers {
		if p.Votes > maxVotes {
			maxVotes = p.Votes
		}
	}

	candidates := []*Player{}
	for _, p := range alivePlayers {
		if p.Votes == maxVotes {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 1 {
		killed := candidates[0]
		killed.Alive = false
		g.Log(fmt.Sprintf("\n%s 被投票出局", killed.Name))
		if killed.Sheriff {
			g.TransferSheriff()
		}
	} else {
		g.Log("平票，无人出局")
	}

	g.resetVotes()
}

// 转移警长
func (g *WerewolfGame) TransferSheriff() {
	candidates := []*Player{}
	for _, p := range g.Players {
		if p.Alive && !p.Sheriff {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) > 0 {
		newSheriff := candidates[rand.Intn(len(candidates))]
		g.Sheriff = newSheriff
		newSheriff.Sheriff = true
		g.Log(fmt.Sprintf("%s 成为新警长！", newSheriff.Name))
	} else {
		g.Log("没有合适玩家继承警徽")
	}
}

// 检查游戏是否结束
func (g *WerewolfGame) CheckGameEnd() bool {
	aliveWerewolves := 0
	aliveVillagers := 0

	for _, p := range g.Players {
		if p.Alive {
			if p.IsWolf() {
				aliveWerewolves++
			} else {
				aliveVillagers++
			}
		}
	}

	g.Log(fmt.Sprintf("当前存活情况: %d 狼人, %d 好人", aliveWerewolves, aliveVillagers))

	if aliveWerewolves == 0 {
		g.Log("\n好人阵营胜利！")
		g.WinnerIsWerewolf = false
		return true
	} else if aliveWerewolves >= aliveVillagers {
		g.Log("\n狼人阵营胜利！")
		g.WinnerIsWerewolf = true
		return true
	}
	return false
}

// 白天行动
func (g *WerewolfGame) DayActions() {
	g.Log(fmt.Sprintf("第 %d 天白天", g.DayCount))

	// 宣布夜晚死亡的玩家
	if g.WolfKillTarget != "" {
		var target *Player
		for _, p := range g.Players {
			if p.Name == g.WolfKillTarget {
				target = p
				break
			}
		}

		if target != nil && !target.Alive {
			g.Log(fmt.Sprintf("\n%s 昨晚被狼人杀死了", target.Name))

			// 如果是警长，需要转移警徽
			if target.Sheriff {
				g.TransferSheriff()
			}

			// 猎人死亡可能触发带走一个人
			if target.IsHunter() {
				g.Log(fmt.Sprintf("%s 是猎人，可以带走一个人", target.Name))
				// 猎人逻辑在PlayerDayAction中处理
			}
		}
	}

	// 处理被毒的玩家
	for _, poisonedName := range g.PoisonedPlayers {
		var poisoned *Player
		for _, p := range g.Players {
			if p.Name == poisonedName && p.Alive {
				poisoned = p
				break
			}
		}

		if poisoned != nil {
			poisoned.Alive = false
			g.Log(fmt.Sprintf("%s 被毒死了", poisoned.Name))

			if poisoned.Sheriff {
				g.TransferSheriff()
			}
		}
	}

	g.PoisonedPlayers = []string{}
	g.WolfKillTarget = "" // 清空击杀目标
	g.DayCount++
}

// 夜晚行动
func (g *WerewolfGame) NightActions() {
	g.Log(fmt.Sprintf("第 %d 天黑夜", g.DayCount))
	g.WolfKillTarget = ""

	votes := make(map[string]int)

	// AI狼人投票
	for _, player := range g.Players {
		if player.IsWolf() && player.Alive && player.IsAI {
			actionResult := player.NightAction(g.Players)
			if actionResult != nil {
				if target, ok := actionResult["vote"].(string); ok {
					votes[target]++
					g.Log(fmt.Sprintf("%s votes for %s", player.Name, target))
				}
			}
		}
	}

	// 加入人类狼人的投票
	for targetName, count := range g.HumanWolfVotes {
		votes[targetName] += count
	}

	g.Log(fmt.Sprintf("狼人投票结果: %v", votes))

	if len(votes) > 0 {
		maxVotes := 0
		for _, count := range votes {
			if count > maxVotes {
				maxVotes = count
			}
		}

		candidates := []string{}
		for name, count := range votes {
			if count == maxVotes {
				candidates = append(candidates, name)
			}
		}

		if len(candidates) > 0 {
			g.WolfKillTarget = candidates[rand.Intn(len(candidates))]
		}
	}

	if g.WolfKillTarget != "" {
		g.Log(fmt.Sprintf("今晚狼人选择了击杀 %s", g.WolfKillTarget))
	}

	g.HumanWolfVotes = make(map[string]int)
}

// 完成夜晚行动，处理最终结果
func (g *WerewolfGame) FinishNight() {
	// 处理狼人杀人
	if g.WolfKillTarget != "" && !g.Antidote {
		var target *Player
		for _, p := range g.Players {
			if p.Name == g.WolfKillTarget {
				target = p
				break
			}
		}

		if target != nil {
			target.Alive = false // 标记为死亡，但不立即宣布
			g.Log(fmt.Sprintf("狼人选择了击杀 %s", target.Name))
		}
	} else if g.WolfKillTarget != "" && g.Antidote {
		g.Log(fmt.Sprintf("%s 被女巫救活了", g.WolfKillTarget))
		g.WolfKillTarget = "" // 清空击杀目标，表示已被救活
	}

	// 收集女巫毒杀的目标
	for _, player := range g.Players {
		if player.IsWitch() {
			witch, ok := player.Role.(*Witch)
			if ok && witch.PoisonedTarget != "" {
				g.PoisonedPlayers = append(g.PoisonedPlayers, witch.PoisonedTarget)
				g.Log(fmt.Sprintf("女巫对 %s 使用了毒药，将在明天白天死亡", witch.PoisonedTarget))
				witch.PoisonedTarget = ""
			}
		}
	}

	g.Antidote = false
}

// ============ 游戏结果定义 ============

// GameResult 存储游戏结果
type GameResult struct {
	GameID         int
	Port           int
	Duration       time.Duration
	WinningFaction string
	Players        []PlayerInfo
	Logs           []string
	Error          error
}

type PlayerInfo struct {
	Name   string
	Role   string
	Alive  bool
	IsWolf bool
}

// ============ 游戏实例定义 ============

// GameInstance 表示一个游戏实例
type GameInstance struct {
	GameID    int
	Port      int
	Server    *GameServer
	IsRunning bool
	StartTime time.Time
	Result    *GameResult
	mu        sync.Mutex
}

// ============ 游戏服务器定义 ============

// 客户端连接
type ClientConnection struct {
	conn   net.Conn
	player *Player
}

// 游戏服务器
type GameServer struct {
	game           *WerewolfGame
	clients        []*ClientConnection
	voteLock       sync.Mutex
	numRealPlayers int
	numAIPlayers   int
	port           int
	host           string
	listener       net.Listener
	running        bool
	result         *GameResult
}

// 创建新服务器
func NewGameServer() *GameServer {
	return &GameServer{
		game:     NewWerewolfGame(),
		clients:  []*ClientConnection{},
		voteLock: sync.Mutex{},
		running:  false,
	}
}

// 启动服务器 - 现在接受参数并返回结果
func (s *GameServer) Start(host string, port int, numRealPlayers, numAIPlayers int) (*GameResult, error) {
	s.host = host
	s.port = port
	s.numRealPlayers = numRealPlayers
	s.numAIPlayers = numAIPlayers

	// 创建结果对象
	result := &GameResult{
		Port:    port,
		Players: []PlayerInfo{},
	}
	s.result = result

	// 设置开始时间
	startTime := time.Now()

	// 创建监听器
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("无法启动服务器: %v", err)
	}
	s.listener = listener
	defer listener.Close()

	s.game.Log(fmt.Sprintf("服务器启动在 %s, 等待玩家连接...", addr))

	// 设置监听超时，避免无限等待
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(30 * time.Second))

	// 接受玩家连接
	for i := 0; i < numRealPlayers; i++ {
		conn, err := listener.Accept()
		if err != nil {
			// 如果是因为超时导致的错误，就继续下一步
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				s.game.Log(fmt.Sprintf("等待玩家连接超时，将自动添加 AI 替代"))
				break
			}
			return nil, fmt.Errorf("接受连接失败: %v", err)
		}
		s.game.Log(fmt.Sprintf("玩家%d已连接: %v", i+1, conn.RemoteAddr()))

		client := &ClientConnection{
			conn:   conn,
			player: nil,
		}
		s.clients = append(s.clients, client)
	}

	// 接收玩家名称
	players := []*Player{}
	for i, client := range s.clients {
		// 设置读取超时
		client.conn.SetReadDeadline(time.Now().Add(10 * time.Second))

		var message map[string]interface{}
		if err := json.NewDecoder(client.conn).Decode(&message); err != nil {
			s.game.Log(fmt.Sprintf("接收玩家名称失败，使用默认名称: %v", err))
			message = map[string]interface{}{
				"name": fmt.Sprintf("Player%d", i+1),
			}
		}

		name, ok := message["name"].(string)
		if !ok {
			name = fmt.Sprintf("Player%d", i+1)
		}

		player := NewPlayer(name, false)
		client.player = player
		players = append(players, player)
		s.game.AddPlayer(player)
	}

	// 发送等待确认消息
	playerNames := []string{}
	for _, p := range players {
		playerNames = append(playerNames, p.Name)
	}

	s.BroadcastMessage(map[string]interface{}{
		"type":    "wait_confirm",
		"players": playerNames,
	})

	// 接收确认
	confirmations := []bool{}
	for _, client := range s.clients {
		client.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		var message map[string]interface{}
		if err := json.NewDecoder(client.conn).Decode(&message); err != nil {
			s.game.Log(fmt.Sprintf("接收确认失败: %v", err))
			confirmations = append(confirmations, false)
			continue
		}
		client.conn.SetReadDeadline(time.Time{}) // 重置超时

		confirm, ok := message["confirm"].(bool)
		if !ok {
			confirmations = append(confirmations, false)
			continue
		}

		confirmations = append(confirmations, confirm)
	}

	// 检查所有玩家是否确认
	allConfirmed := true
	for _, confirm := range confirmations {
		if !confirm {
			allConfirmed = false
			break
		}
	}

	// 添加AI玩家
	aiNames := []string{"Stephanie", "Wendy", "Elmy", "Sham", "Jeffry", "Kelly", "Tony", "Alice", "Bob", "Charlie"}

	// 计算需要添加的AI玩家数量
	aiPlayersToAdd := numAIPlayers
	if !allConfirmed {
		// 如果有玩家未确认，则添加更多AI以满足总人数
		aiPlayersToAdd = numRealPlayers + numAIPlayers
	}

	for i := 0; i < aiPlayersToAdd; i++ {
		name := fmt.Sprintf("AI%d", i+1)
		if i < len(aiNames) {
			name = aiNames[i]
		}
		s.game.AddPlayer(NewPlayer(name, true))
	}

	// 分配角色
	s.game.RandomAllocate()
	s.SendGameStatus()

	// 设置游戏事件
	s.game.Events = []GameEvent{
		NewNightEvent("黑夜", "狼人行动"),
		NewDayEvent("白天", "讨论和投票"),
	}

	// 运行游戏
	s.running = true
	s.RunGameLoop()

	// 准备结果
	result.Duration = time.Since(startTime)
	if s.game.WinnerIsWerewolf {
		result.WinningFaction = "狼人"
	} else {
		result.WinningFaction = "好人"
	}

	for _, p := range s.game.Players {
		result.Players = append(result.Players, PlayerInfo{
			Name:   p.Name,
			Role:   p.Role.GetName(),
			Alive:  p.Alive,
			IsWolf: p.IsWolf(),
		})
	}

	result.Logs = s.game.Logs

	return result, nil
}

// 停止服务器
func (s *GameServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}

	for _, client := range s.clients {
		if client.conn != nil {
			client.conn.Close()
		}
	}

	s.running = false
}

// 广播消息给所有客户端
func (s *GameServer) BroadcastMessage(message map[string]interface{}) {
	for _, client := range s.clients {
		err := json.NewEncoder(client.conn).Encode(message)
		if err != nil {
			s.game.Log(fmt.Sprintf("发送消息失败: %v", err))
		}
	}
}

// 发送消息给指定客户端
func (s *GameServer) SendMessage(message map[string]interface{}, index int) {
	if index >= 0 && index < len(s.clients) {
		err := json.NewEncoder(s.clients[index].conn).Encode(message)
		if err != nil {
			s.game.Log(fmt.Sprintf("发送消息失败: %v", err))
		}
	} else {
		s.game.Log(fmt.Sprintf("客户端索引越界: %d，客户端数量: %d", index, len(s.clients)))
	}
}

// 接收消息
func (s *GameServer) ReceiveMessage(index int) map[string]interface{} {
	if index >= 0 && index < len(s.clients) {
		// 设置读取超时
		s.clients[index].conn.SetReadDeadline(time.Now().Add(10 * time.Second))

		var message map[string]interface{}
		err := json.NewDecoder(s.clients[index].conn).Decode(&message)

		// 重置超时
		s.clients[index].conn.SetReadDeadline(time.Time{})

		if err != nil {
			s.game.Log(fmt.Sprintf("接收消息失败: %v", err))
			return nil
		}
		return message
	}
	return nil
}

// 发送游戏状态
func (s *GameServer) SendGameStatus() {
	for i, client := range s.clients {
		player := client.player
		if player == nil {
			continue
		}

		playersInfo := [][]interface{}{}
		for _, p := range s.game.Players {
			roleName := "未知"
			if p == player || (p.IsWolf() && player.IsWolf()) {
				roleName = p.Role.GetName()
			}
			playersInfo = append(playersInfo, []interface{}{p.Name, roleName, p.Alive, p.Sheriff})
		}

		status := map[string]interface{}{
			"type":      "game_status",
			"role":      player.Role.GetName(),
			"players":   playersInfo,
			"day_count": s.game.DayCount,
		}

		s.SendMessage(status, i)
	}
}

// 运行游戏
func (s *GameServer) RunGameLoop() {
	s.game.Log("=== 狼人杀游戏开始 ===")

	gameOver := false
	for !gameOver {
		// 夜晚阶段
		s.game.Log("\n=== 黑夜 ===")
		s.HandleNightPhase()
		s.game.FinishNight()

		// 发送游戏状态更新
		time.Sleep(1 * time.Second)
		s.SendGameStatus()

		// 警长选举（只在第一天）
		if s.game.Sheriff == nil && !s.game.SheriffElect {
			s.game.Log("警长选举，玩家投票选举警长")
			s.HandleSheriffElection()
			s.game.SheriffElect = true
		}

		if s.game.Sheriff == nil && s.game.SheriffElect {
			s.game.TransferSheriff()
		}

		// 白天阶段
		s.game.Log("\n=== 白天 ===")
		s.HandleDayPhase()

		// 发送游戏状态更新
		time.Sleep(1 * time.Second)
		s.SendGameStatus()

		winner := ""
		if s.game.WinnerIsWerewolf {
			winner = "狼人"
		} else {
			winner = "好人"
		}
		if s.game.CheckGameEnd() {
			s.BroadcastMessage(map[string]interface{}{
				"type":   "game_end",
				"winner": winner,
			})
			gameOver = true
		}
	}
}

// 处理警长选举
func (s *GameServer) HandleSheriffElection() {
	var wg sync.WaitGroup

	// 处理真人玩家投票
	for i, client := range s.clients {
		if client.player != nil && client.player.Alive {
			wg.Add(1)
			go func(idx int, p *Player) {
				defer wg.Done()
				s.PlayerSheriffVote(idx, p)
			}(i, client.player)
		}
	}

	// 处理AI玩家投票
	validCandidates := []*Player{}
	for _, p := range s.game.Players {
		if p.Alive && p.IsAI {
			validCandidates = append(validCandidates, p)
		}
	}

	for _, voter := range validCandidates {
		s.voteLock.Lock()
		if len(validCandidates) > 0 {
			target := validCandidates[rand.Intn(len(validCandidates))]
			target.Votes++
			s.game.Log(fmt.Sprintf("%s (%s) 投票给 %s", voter.Name, voter.Role.GetName(), target.Name))
		}
		s.voteLock.Unlock()
	}

	// 等待所有投票完成
	wg.Wait()

	// 选举警长
	s.game.ElectSheriff()
}

// 处理玩家警长投票
func (s *GameServer) PlayerSheriffVote(playerIndex int, player *Player) {
	candidates := []string{}
	for _, p := range s.game.Players {
		if p.Alive {
			candidates = append(candidates, p.Name)
		}
	}

	s.SendMessage(map[string]interface{}{
		"type":       "sheriff_election",
		"candidates": candidates,
	}, playerIndex)

	response := s.ReceiveMessage(playerIndex)
	if response != nil {
		if targetName, ok := response["vote"].(string); ok {
			s.voteLock.Lock()
			for _, p := range s.game.Players {
				if p.Name == targetName {
					p.Votes++
					s.game.Log(fmt.Sprintf("%s 投票给 %s", player.Name, targetName))
					break
				}
			}
			s.voteLock.Unlock()
		}
	}
}

// 处理玩家白天投票
func (s *GameServer) PlayerDayVote(playerIndex int, player *Player) {
	candidates := []string{}
	for _, p := range s.game.Players {
		if p.Alive && p != player {
			candidates = append(candidates, p.Name)
		}
	}

	s.SendMessage(map[string]interface{}{
		"type":       "day_vote",
		"candidates": candidates,
	}, playerIndex)

	response := s.ReceiveMessage(playerIndex)
	if response != nil {
		if targetName, ok := response["vote"].(string); ok {
			s.voteLock.Lock()
			for _, p := range s.game.Players {
				if p.Name == targetName {
					voteValue := 1.0
					if player.Sheriff {
						voteValue = 1.5
					}
					p.Votes += voteValue
					s.game.Log(fmt.Sprintf("%s (%s) 投票给 %s", player.Name, player.Role.GetName(), p.Name))
					break
				}
			}
			s.voteLock.Unlock()
		}
	}
}

// 处理夜晚阶段
func (s *GameServer) HandleNightPhase() {
	nightLock := sync.Mutex{}
	var wg sync.WaitGroup

	// 狼人阶段
	processWolves := func() {
		// 处理人类狼人
		for i, client := range s.clients {
			if client.player != nil && client.player.Alive && client.player.IsWolf() {
				wg.Add(1)
				go func(idx int, p *Player) {
					defer wg.Done()
					s.PlayerNightAction(idx, p, "werewolf")
				}(i, client.player)
			}
		}
		wg.Wait()
	}

	// 女巫阶段
	processWitches := func() {
		// 处理AI女巫
		for _, p := range s.game.Players {
			if p.Alive && p.IsWitch() && p.IsAI {
				actionResult := p.NightAction(s.game.Players)
				if actionResult != nil {
					nightLock.Lock()
					s.game.Log(fmt.Sprintf("女巫 %s (AI) 执行行动: %v", p.Name, actionResult))
					nightLock.Unlock()
				}
			}
		}

		// 处理人类女巫
		for i, client := range s.clients {
			if client.player != nil && client.player.Alive && client.player.IsWitch() {
				wg.Add(1)
				go func(idx int, p *Player) {
					defer wg.Done()
					s.PlayerNightAction(idx, p, "witch")
				}(i, client.player)
			}
		}
		wg.Wait()
	}

	// 预言家阶段
	processSeers := func() {
		// 处理AI预言家
		for _, p := range s.game.Players {
			if p.Alive && p.IsSeer() && p.IsAI {
				actionResult := p.NightAction(s.game.Players)
				if actionResult != nil {
					nightLock.Lock()
					s.game.Log(fmt.Sprintf("预言家 %s (AI) 执行行动: %v", p.Name, actionResult))
					nightLock.Unlock()
				}
			}
		}

		// 处理人类预言家
		for i, client := range s.clients {
			if client.player != nil && client.player.Alive && client.player.IsSeer() {
				wg.Add(1)
				go func(idx int, p *Player) {
					defer wg.Done()
					s.PlayerNightAction(idx, p, "seer")
				}(i, client.player)
			}
		}
		wg.Wait()
	}

	// 按顺序执行各角色行动
	s.game.NightActions() // 处理狼人投票
	processWolves()
	processWitches()
	processSeers()
}

func (s *GameServer) HandleDayPhase() {
	s.game.DayActions()

	var wg sync.WaitGroup

	// 处理人类玩家投票 - 只有活着的玩家才能投票
	for i, client := range s.clients {
		if client.player != nil && client.player.Alive {
			wg.Add(1)
			go func(idx int, p *Player) {
				defer wg.Done()
				s.PlayerDayVote(idx, p)
			}(i, client.player)
		}
	}

	// 处理AI玩家投票 - 只有活着的玩家才能投票
	validCandidates := []*Player{}
	for _, p := range s.game.Players {
		if p.Alive {
			validCandidates = append(validCandidates, p)
		}
	}

	for _, voter := range s.game.Players {
		if voter.IsAI && voter.Alive {
			s.voteLock.Lock()
			voteCandidates := []*Player{}
			for _, p := range validCandidates {
				if p != voter {
					voteCandidates = append(voteCandidates, p)
				}
			}

			if len(voteCandidates) > 0 {
				target := voteCandidates[rand.Intn(len(voteCandidates))]
				voteValue := 1.0
				if voter.Sheriff {
					voteValue = 1.5
				}
				target.Votes += voteValue
				s.game.Log(fmt.Sprintf("%s (%s) 投票给 %s", voter.Name, voter.Role.GetName(), target.Name))
			} else {
				s.game.Log(fmt.Sprintf("%s 没有可投票的目标", voter.Name))
			}
			s.voteLock.Unlock()
		}
	}

	// 等待所有投票完成
	wg.Wait()

	// 执行投票结果
	s.game.Vote()
}

func (s *GameServer) PlayerNightAction(playerIndex int, player *Player, roleType string) {
	switch roleType {
	case "werewolf":
		candidates := []string{}
		for _, p := range s.game.Players {
			if p.Alive && !p.IsWolf() {
				candidates = append(candidates, p.Name)
			}
		}

		s.SendMessage(map[string]interface{}{
			"type":       "night_action",
			"action":     "werewolf",
			"candidates": candidates,
		}, playerIndex)

		response := s.ReceiveMessage(playerIndex)
		if response != nil {
			if targetName, ok := response["target"].(string); ok {
				s.voteLock.Lock()
				s.game.HumanWolfVotes[targetName]++
				s.game.Log(fmt.Sprintf("狼人 %s (真人) 选择击杀 %s", player.Name, targetName))
				s.voteLock.Unlock()
			}
		}

	case "witch":
		// 女巫行动
		witch, ok := player.Role.(*Witch)
		if !ok {
			return
		}

		// 准备可用操作和目标
		deadTarget := s.game.WolfKillTarget
		alivePlayers := []string{}
		for _, p := range s.game.Players {
			if p.Alive && p != player {
				alivePlayers = append(alivePlayers, p.Name)
			}
		}

		// 发送女巫操作选项
		s.SendMessage(map[string]interface{}{
			"type":          "night_action",
			"action":        "witch",
			"has_poison":    witch.HasPoison,
			"has_antidote":  witch.HasAntidote,
			"dead_players":  []string{deadTarget},
			"alive_players": alivePlayers,
		}, playerIndex)

		response := s.ReceiveMessage(playerIndex)
		if response != nil {
			// 处理解药
			if saveTarget, ok := response["save"].(string); ok && witch.HasAntidote && saveTarget == s.game.WolfKillTarget {
				s.game.Antidote = true
				witch.HasAntidote = false
				s.game.Log(fmt.Sprintf("女巫 %s (真人) 使用解药救活 %s", player.Name, saveTarget))
			}

			// 处理毒药
			if poisonTarget, ok := response["poison"].(string); ok && witch.HasPoison {
				for _, p := range s.game.Players {
					if p.Name == poisonTarget && p.Alive {
						witch.HasPoison = false
						witch.PoisonedTarget = poisonTarget
						s.game.Log(fmt.Sprintf("女巫 %s (真人) 对 %s 使用了毒药", player.Name, poisonTarget))
						break
					}
				}
			}
		}

	case "seer":
		// 预言家行动
		candidates := []string{}
		for _, p := range s.game.Players {
			if p.Alive && p != player {
				candidates = append(candidates, p.Name)
			}
		}

		s.SendMessage(map[string]interface{}{
			"type":       "night_action",
			"action":     "seer",
			"candidates": candidates,
		}, playerIndex)

		response := s.ReceiveMessage(playerIndex)
		if response != nil {
			if targetName, ok := response["target"].(string); ok {
				var target *Player
				for _, p := range s.game.Players {
					if p.Name == targetName {
						target = p
						break
					}
				}

				if target != nil {
					role := "狼人"
					if !target.IsWolf() {
						role = "好人"
					}

					s.SendMessage(map[string]interface{}{
						"type":   "seer_result",
						"action": "seer",
						"target": targetName,
						"result": role,
					}, playerIndex)

					s.game.Log(fmt.Sprintf("预言家 %s 查验 %s 的身份是 %s", player.Name, targetName, role))
				}
			}
		}
	}
}

// ============ 游戏管理器定义 ============

// GameManager 管理多个游戏实例
type GameManager struct {
	instances  map[int]*GameInstance
	nextGameID int
	basePort   int
	mu         sync.Mutex
}

// 创建新的游戏管理器
func NewGameManager(basePort int) *GameManager {
	return &GameManager{
		instances:  make(map[int]*GameInstance),
		nextGameID: 1,
		basePort:   basePort,
	}
}

// 创建新游戏
func (gm *GameManager) StartNewGame(numRealPlayers, numAIPlayers int) (int, error) {
	gm.mu.Lock()
	gameID := gm.nextGameID
	gm.nextGameID++
	port := gm.basePort + gameID
	gm.mu.Unlock()

	instance := &GameInstance{
		GameID:    gameID,
		Port:      port,
		IsRunning: true,
		StartTime: time.Now(),
	}

	// 保存实例
	gm.mu.Lock()
	gm.instances[gameID] = instance
	gm.mu.Unlock()

	// 启动游戏服务器（异步）
	go func() {
		server := NewGameServer()
		instance.Server = server

		result, err := server.Start("localhost", port, numRealPlayers, numAIPlayers)

		// 游戏结束后更新状态
		gm.mu.Lock()
		instance.IsRunning = false
		instance.Result = result
		if err != nil {
			if instance.Result != nil {
				instance.Result.Error = err
			} else {
				instance.Result = &GameResult{Error: err}
			}
		}
		gm.mu.Unlock()
	}()

	return gameID, nil
}

// 获取游戏状态
func (gm *GameManager) GetGameStatus(gameID int) (bool, *GameResult, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	instance, exists := gm.instances[gameID]
	if !exists {
		return false, nil, fmt.Errorf("游戏 ID %d 不存在", gameID)
	}

	return instance.IsRunning, instance.Result, nil
}

// 等待游戏完成
func (gm *GameManager) WaitForGameToComplete(gameID int, timeout time.Duration) (*GameResult, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		isRunning, result, err := gm.GetGameStatus(gameID)
		if err != nil {
			return nil, err
		}

		if !isRunning {
			return result, nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 如果超时，强制停止游戏
	gm.mu.Lock()
	instance, exists := gm.instances[gameID]
	gm.mu.Unlock()

	if exists && instance.IsRunning && instance.Server != nil {
		instance.Server.Stop()
		instance.IsRunning = false
		if instance.Result == nil {
			instance.Result = &GameResult{
				GameID: gameID,
				Port:   instance.Port,
				Error:  fmt.Errorf("游戏超时"),
			}
		} else {
			instance.Result.Error = fmt.Errorf("游戏超时")
		}
	}

	return nil, fmt.Errorf("等待游戏 %d 完成超时", gameID)
}

// 运行多个游戏
func (gm *GameManager) RunMultipleGames(count, numRealPlayers, numAIPlayers int, timeout time.Duration) []*GameResult {
	var wg sync.WaitGroup
	results := make([]*GameResult, count)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			gameID, err := gm.StartNewGame(numRealPlayers, numAIPlayers)
			if err != nil {
				results[index] = &GameResult{Error: err}
				return
			}

			result, err := gm.WaitForGameToComplete(gameID, timeout)
			if err != nil {
				results[index] = &GameResult{Error: err}
				return
			}

			results[index] = result
		}(i)
	}

	wg.Wait()
	return results
}

// ============ 主函数 ============

func main() {
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())

	// 创建游戏管理器（基础端口5100用于TCP游戏服务器）
	manager := NewGameManager(5100)

	// 设置HTTP API端口（5000用于HTTP管理API）
	apiPort := 5000

	// 处理创建游戏请求
	http.HandleFunc("/create_game", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "仅支持POST请求", http.StatusMethodNotAllowed)
			return
		}

		var req CreateGameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "请求格式无效", http.StatusBadRequest)
			return
		}

		// 验证请求参数
		if req.RealPlayers < 0 {
			req.RealPlayers = 0
		}
		if req.AIPlayers <= 0 {
			req.AIPlayers = 6 // 至少需要6个AI玩家以保证游戏角色分配
		}

		// 创建新游戏
		gameID, err := manager.StartNewGame(req.RealPlayers, req.AIPlayers)
		if err != nil {
			http.Error(w, fmt.Sprintf("创建游戏失败: %v", err), http.StatusInternalServerError)
			return
		}

		// 获取游戏实例
		instance, exists := manager.instances[gameID]
		if !exists {
			http.Error(w, "游戏创建异常", http.StatusInternalServerError)
			return
		}

		// 返回游戏信息
		resp := CreateGameResponse{
			GameID: gameID,
			Port:   instance.Port,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "响应编码失败", http.StatusInternalServerError)
			return
		}

		log.Printf("游戏 #%d 已创建: 真实玩家=%d, AI玩家=%d, 端口=%d\n",
			gameID, req.RealPlayers, req.AIPlayers, instance.Port)
	})

	// 处理游戏状态请求
	http.HandleFunc("/game_status/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "仅支持GET请求", http.StatusMethodNotAllowed)
			return
		}

		// 解析游戏ID
		path := r.URL.Path[len("/game_status/"):]
		gameID, err := strconv.Atoi(path)
		if err != nil {
			http.Error(w, "游戏ID格式无效", http.StatusBadRequest)
			return
		}

		// 获取游戏状态
		isRunning, result, err := manager.GetGameStatus(gameID)

		resp := GameStatusResponse{
			GameID:  gameID,
			Running: isRunning,
		}

		if err != nil {
			resp.Error = err.Error()
		} else if !isRunning && result != nil {
			resp.Results = result
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "响应编码失败", http.StatusInternalServerError)
			return
		}
	})

	// 处理游戏列表请求
	http.HandleFunc("/games", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "仅支持GET请求", http.StatusMethodNotAllowed)
			return
		}

		manager.mu.Lock()
		defer manager.mu.Unlock()

		games := make([]map[string]interface{}, 0, len(manager.instances))
		for id, instance := range manager.instances {
			gameInfo := map[string]interface{}{
				"id":        id,
				"port":      instance.Port,
				"running":   instance.IsRunning,
				"startTime": instance.StartTime.Format(time.RFC3339),
			}
			games = append(games, gameInfo)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(games); err != nil {
			http.Error(w, "响应编码失败", http.StatusInternalServerError)
			return
		}
	})

	// 处理停止游戏请求
	http.HandleFunc("/stop_game/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "仅支持POST请求", http.StatusMethodNotAllowed)
			return
		}

		// 解析游戏ID
		path := r.URL.Path[len("/stop_game/"):]
		gameID, err := strconv.Atoi(path)
		if err != nil {
			http.Error(w, "游戏ID格式无效", http.StatusBadRequest)
			return
		}

		// 获取游戏实例
		manager.mu.Lock()
		instance, exists := manager.instances[gameID]
		manager.mu.Unlock()

		if !exists {
			http.Error(w, fmt.Sprintf("游戏 #%d 不存在", gameID), http.StatusNotFound)
			return
		}

		// 停止游戏
		if instance.IsRunning && instance.Server != nil {
			instance.Server.Stop()
			instance.IsRunning = false
			log.Printf("游戏 #%d 已手动停止\n", gameID)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("游戏 #%d 已停止", gameID)))
	})

	// 启动定时任务，清理已完成的游戏
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			manager.mu.Lock()

			// 找出完成超过24小时的游戏
			now := time.Now()
			gameIDsToRemove := []int{}

			for id, instance := range manager.instances {
				if !instance.IsRunning && now.Sub(instance.StartTime) > 24*time.Hour {
					gameIDsToRemove = append(gameIDsToRemove, id)
				}
			}

			// 删除这些游戏
			for _, id := range gameIDsToRemove {
				delete(manager.instances, id)
				log.Printf("已清理游戏 #%d (已完成超过24小时)\n", id)
			}

			manager.mu.Unlock()
		}
	}()

	// 启动HTTP服务
	serverAddr := fmt.Sprintf(":%d", apiPort)
	log.Printf("狼人杀游戏服务器启动在 http://localhost%s\n", serverAddr)
	log.Printf("API端点:\n")
	log.Printf("  - POST /create_game - 创建新游戏\n")
	log.Printf("  - GET /game_status/{id} - 获取游戏状态\n")
	log.Printf("  - GET /games - 获取游戏列表\n")
	log.Printf("  - POST /stop_game/{id} - 停止游戏\n")

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("启动HTTP服务器失败: %v", err)
	}
}
