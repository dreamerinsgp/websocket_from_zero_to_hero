package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WebSocket升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境应该限制）
	},
}

// 消息类型
type Message struct {
	Action  string      `json:"action"`
	Channel string      `json:"channel"`
	Data    interface{} `json:"data,omitempty"`
}

type Response struct {
	ClientID string      `json:"clientId"`
	Action   string      `json:"action"`
	Channel  string      `json:"channel"`
	Code     int         `json:"code"`
	Msg      string      `json:"msg"`
	Data     interface{} `json:"data,omitempty"`
}

// 客户端连接
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan []byte
	Channels map[string]bool // 订阅的频道
}

// WebSocket服务器
type Server struct {
	clients       map[*Client]bool            // 所有连接的客户端
	subscriptions map[string]map[*Client]bool // 频道 -> 客户端映射
	register      chan *Client                // 注册新客户端
	unregister    chan *Client                // 注销客户端
	broadcast     chan BroadcastMsg           // 广播消息
	mu            sync.RWMutex                // 读写锁
}

type BroadcastMsg struct {
	Channel string
	Data    interface{}
}

// 创建新服务器
func NewServer() *Server {
	return &Server{
		clients:       make(map[*Client]bool),
		subscriptions: make(map[string]map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan BroadcastMsg),
	}
}

// 运行服务器
func (s *Server) Run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			s.mu.Unlock()
			log.Printf("客户端 %s 已连接，当前连接数: %d", client.ID, len(s.clients))

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.Send)
				// 从所有订阅中移除
				for channel := range client.Channels {
					if subs, ok := s.subscriptions[channel]; ok {
						delete(subs, client)
						if len(subs) == 0 {
							delete(s.subscriptions, channel)
						}
					}
				}
			}
			s.mu.Unlock()
			log.Printf("客户端 %s 已断开，当前连接数: %d", client.ID, len(s.clients))

		case msg := <-s.broadcast:
			s.mu.RLock()
			subs, ok := s.subscriptions[msg.Channel]
			if !ok {
				s.mu.RUnlock()
				log.Printf("频道 %s 没有订阅者", msg.Channel)
				continue
			}
			// 复制订阅列表，避免长时间持有锁
			clients := make([]*Client, 0, len(subs))
			for client := range subs {
				clients = append(clients, client)
			}
			s.mu.RUnlock()

			// 发送消息给所有订阅者
			response := Response{
				Action:  "message",
				Channel: msg.Channel,
				Code:    200,
				Msg:     "success",
				Data:    msg.Data,
			}
			data, _ := json.Marshal(response)
			for _, client := range clients {
				select {
				case client.Send <- data:
				default:
					// 发送失败，关闭连接
					close(client.Send)
					s.unregister <- client
				}
			}
			log.Printf("向频道 %s 的 %d 个订阅者广播消息", msg.Channel, len(clients))
		}
	}
}

// 处理WebSocket连接
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级HTTP连接为WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}

	// 创建客户端
	client := &Client{
		ID:       uuid.New().String(),
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Channels: make(map[string]bool),
	}

	// 注册客户端
	s.register <- client

	// 发送连接确认消息
	response := Response{
		ClientID: client.ID,
		Action:   "connect",
		Code:     200,
		Msg:      "success",
	}
	data, _ := json.Marshal(response)
	client.Send <- data

	// 启动goroutine处理读写
	go s.writePump(client)
	go s.readPump(client)
}

// 读取消息
func (s *Server) readPump(client *Client) {
	defer func() {
		s.unregister <- client
		client.Conn.Close()
	}()

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("读取错误: %v", err)
			}
			break
		}

		// 解析消息
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("消息解析失败: %v", err)
			continue
		}

		// 处理消息
		s.handleMessage(client, &msg)
	}
}

// 写入消息
func (s *Server) writePump(client *Client) {
	defer client.Conn.Close()

	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				// 通道已关闭
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("写入错误: %v", err)
				return
			}
		}
	}
}

// 处理消息
func (s *Server) handleMessage(client *Client, msg *Message) {
	switch msg.Action {
	case "subscribe":
		s.handleSubscribe(client, msg.Channel)
	case "unsubscribe":
		s.handleUnsubscribe(client, msg.Channel)
	case "ping":
		s.handlePing(client)
	default:
		log.Printf("未知操作: %s", msg.Action)
	}
}

// 处理订阅
func (s *Server) handleSubscribe(client *Client, channel string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 添加到客户端的订阅列表
	client.Channels[channel] = true

	// 添加到频道的订阅列表
	if s.subscriptions[channel] == nil {
		s.subscriptions[channel] = make(map[*Client]bool)
	}
	s.subscriptions[channel][client] = true

	// 发送订阅确认
	response := Response{
		ClientID: client.ID,
		Action:   "subscribe",
		Channel:  channel,
		Code:     200,
		Msg:      "success",
	}
	data, _ := json.Marshal(response)
	client.Send <- data

	log.Printf("客户端 %s 订阅了频道 %s", client.ID, channel)
}

// 处理取消订阅
func (s *Server) handleUnsubscribe(client *Client, channel string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从客户端订阅列表移除
	delete(client.Channels, channel)

	// 从频道订阅列表移除
	if subs, ok := s.subscriptions[channel]; ok {
		delete(subs, client)
		if len(subs) == 0 {
			delete(s.subscriptions, channel)
		}
	}

	// 发送取消订阅确认
	response := Response{
		ClientID: client.ID,
		Action:   "unsubscribe",
		Channel:  channel,
		Code:     200,
		Msg:      "success",
	}
	data, _ := json.Marshal(response)
	client.Send <- data

	log.Printf("客户端 %s 取消订阅频道 %s", client.ID, channel)
}

// 处理心跳
func (s *Server) handlePing(client *Client) {
	response := Response{
		ClientID: client.ID,
		Action:   "pong",
		Code:     200,
		Msg:      "success",
	}
	data, _ := json.Marshal(response)
	client.Send <- data
}

// 广播消息到频道
func (s *Server) BroadcastToChannel(channel string, data interface{}) {
	s.broadcast <- BroadcastMsg{
		Channel: channel,
		Data:    data,
	}
}

func main() {
	server := NewServer()
	go server.Run()

	// HTTP路由
	http.HandleFunc("/ws", server.HandleWebSocket)

	// 测试用的广播接口（可选）
	http.HandleFunc("/broadcast", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Channel string      `json:"channel"`
			Data    interface{} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		server.BroadcastToChannel(req.Channel, req.Data)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Broadcast sent"))
	})

	port := ":8089"
	log.Printf("WebSocket服务器启动在端口 %s", port)
	log.Printf("WebSocket端点: ws://localhost%s/ws", port)
	log.Printf("广播测试端点: http://localhost%s/broadcast", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}
