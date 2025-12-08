package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ====== WS CONFIG ======

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// В проде лучше сделать нормальную проверку Origin
		return true
	},
}

// ====== МОДЕЛЬ СООБЩЕНИЯ ======

type ChatMessage struct {
	Author string `json:"author"`
	Text   string `json:"text"`
	Room   string `json:"room"`
}

// ====== CLIENT / ROOM / HUB ======

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	room   *Room
	author string
}

type Room struct {
	name       string
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func newRoom(name string) *Room {
	r := &Room{
		name:       name,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	go r.run() // 👈 отдельная горутина на комнату
	return r
}

func (r *Room) run() {
	for {
		select {
		case c := <-r.register:
			r.clients[c] = true

		case c := <-r.unregister:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				close(c.send)
			}

		case msg := <-r.broadcast:
			for c := range r.clients {
				select {
				case c.send <- msg:
				default:
					close(c.send)
					delete(r.clients, c)
				}
			}
		}
	}
}

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func newHub() *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
	}
}

func (h *Hub) getRoom(name string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if name == "" {
		name = "esimde-live"
	}
	if room, ok := h.rooms[name]; ok {
		return room
	}
	room := newRoom(name)
	h.rooms[name] = room
	return room
}

// ====== CLIENT READ / WRITE ======

func (c *Client) readPump() {
	defer func() {
		c.room.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws read error: %v", err)
			}
			break
		}

		var msg ChatMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Println("ws json error:", err)
			continue
		}

		if msg.Author == "" {
			msg.Author = c.author
		}
		if msg.Room == "" {
			msg.Room = c.room.name
		}

		out, err := json.Marshal(msg)
		if err != nil {
			log.Println("ws marshal error:", err)
			continue
		}

		// 👇 отправляем во ВСЮ комнату
		c.room.broadcast <- out
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// LiveChatWS — WebSocket-чат для Esimde Live.
// URL: /ws/live-chat?room=esimde-live&author=Ерек
func (h *Handler) LiveChatWS(w http.ResponseWriter, r *http.Request) {
	roomName := r.URL.Query().Get("room")
	if roomName == "" {
		roomName = "esimde-live"
	}

	author := r.URL.Query().Get("author")
	if author == "" {
		author = "Қатысушы"
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade error", zap.Error(err))
		return
	}

	room := h.chatHub.getRoom(roomName)

	client := &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		room:   room,
		author: author,
	}

	// регистрируем клиента
	room.register <- client

	// читаем и пишем в отдельных горутинах
	go client.writePump()
	go client.readPump()
}
