package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

/*
====================================================
CONFIG (tune for high concurrency)
====================================================
*/

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	// hard protection (client can bypass фронт)
	maxMsgSize       = 512
	maxTextRunes     = 50
	maxMsgsPerSecond = 3

	// room tuning
	roomBroadcastBuf   = 8192
	roomRegisterBuf    = 1024
	roomUnregisterBuf  = 1024
	clientSendBuf      = 64
	presenceDebounce   = 500 * time.Millisecond
	registerTimeout    = 2 * time.Second
	unregisterTimeout  = 2 * time.Second
	broadcastDropClose = false // if true -> close slow/noisy client on overload
)

/*
====================================================
WEBSOCKET UPGRADER
====================================================
*/

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: add real origin check in prod
		return true
	},
}

/*
====================================================
MESSAGES
====================================================
*/

type ChatMessage struct {
	Type   string `json:"type,omitempty"` // "chat"
	Author string `json:"author"`
	Text   string `json:"text"`
	Room   string `json:"room"`
}

type PresenceMessage struct {
	Type    string `json:"type"` // "presence"
	Room    string `json:"room"`
	Viewers int    `json:"viewers"`
}

/*
====================================================
CLIENT
====================================================
*/

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	room   *Room
	author string

	// simple rate-limit (token bucket)
	tokens     float64
	lastRefill time.Time
}

/*
====================================================
ROOM
====================================================
*/

type Room struct {
	name       string
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
}

func newRoom(name string) *Room {
	r := &Room{
		name:       name,
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client, roomRegisterBuf),
		unregister: make(chan *Client, roomUnregisterBuf),
		broadcast:  make(chan []byte, roomBroadcastBuf),
	}
	go r.run()
	return r
}

func (r *Room) run() {
	// debounce presence: mark dirty and flush not more often than presenceDebounce
	var presenceDirty bool
	presenceTicker := time.NewTicker(presenceDebounce)
	defer presenceTicker.Stop()

	for {
		select {

		case c := <-r.register:
			r.clients[c] = struct{}{}
			presenceDirty = true

		case c := <-r.unregister:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				close(c.send)
				presenceDirty = true
			}

		case msg := <-r.broadcast:
			// fanout (do not block room)
			for c := range r.clients {
				select {
				case c.send <- msg:
				default:
					// slow/dead client -> drop it (prevents global slowdown)
					close(c.send)
					delete(r.clients, c)
					presenceDirty = true
				}
			}

		case <-presenceTicker.C:
			if presenceDirty {
				r.broadcastPresence()
				presenceDirty = false
			}
		}
	}
}

func (r *Room) broadcastPresence() {
	p := PresenceMessage{
		Type:    "presence",
		Room:    r.name,
		Viewers: len(r.clients),
	}
	data, _ := json.Marshal(p)

	for c := range r.clients {
		select {
		case c.send <- data:
		default:
			close(c.send)
			delete(r.clients, c)
		}
	}
}

/*
====================================================
HUB (ROOM REGISTRY)
====================================================
*/

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

func (h *Hub) GetRoom(name string) *Room {
	if name == "" {
		name = "esimde-live"
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if r, ok := h.rooms[name]; ok {
		return r
	}
	r := newRoom(name)
	h.rooms[name] = r
	return r
}

/*
====================================================
CLIENT: validation + rate limit
====================================================
*/

func (c *Client) allowMessage(now time.Time) bool {
	// refill tokens
	if c.lastRefill.IsZero() {
		c.lastRefill = now
		c.tokens = float64(maxMsgsPerSecond)
	}
	elapsed := now.Sub(c.lastRefill).Seconds()
	if elapsed > 0 {
		c.tokens += elapsed * float64(maxMsgsPerSecond)
		if c.tokens > float64(maxMsgsPerSecond) {
			c.tokens = float64(maxMsgsPerSecond)
		}
		c.lastRefill = now
	}

	if c.tokens >= 1 {
		c.tokens -= 1
		return true
	}
	return false
}

func sanitizeText(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if len([]rune(s)) > maxTextRunes {
		return "", false
	}
	return s, true
}

/*
====================================================
CLIENT READ / WRITE
====================================================
*/

func (c *Client) readPump() {
	defer func() {
		// avoid blocking on unregister if room is busy
		select {
		case c.room.unregister <- c:
		case <-time.After(unregisterTimeout):
		}
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		now := time.Now()
		if !c.allowMessage(now) {
			continue // rate-limit
		}

		var msg ChatMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		txt, ok := sanitizeText(msg.Text)
		if !ok {
			continue
		}

		msg.Type = "chat"
		msg.Text = txt

		if msg.Author == "" {
			msg.Author = c.author
		}
		if msg.Room == "" {
			msg.Room = c.room.name
		}

		out, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		// IMPORTANT: never block readPump on overloaded room
		select {
		case c.room.broadcast <- out:
		default:
			// room overloaded -> drop message (or close client if you want)
			if broadcastDropClose {
				return
			}
		}
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
			if !ok {
				_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}

			// drain queue quickly (reduces wakeups under load)
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

			// write remaining queued messages immediately
			for i := 0; i < 32; i++ { // cap per loop (prevents starvation)
				select {
				case m2, ok2 := <-c.send:
					if !ok2 {
						_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
						return
					}
					if err := c.conn.WriteMessage(websocket.TextMessage, m2); err != nil {
						return
					}
				default:
					i = 999999 // break
				}
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

/*
====================================================
HTTP HANDLER
====================================================
*/

// URL:
// /ws/live-chat?room=esimde-live&author=Erek
func (h *Handler) LiveChatWS(w http.ResponseWriter, r *http.Request) {
	roomName := r.URL.Query().Get("room")
	author := r.URL.Query().Get("author")
	if author == "" {
		author = "Guest"
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", zap.Error(err))
		return
	}

	room := h.chatHub.GetRoom(roomName)

	client := &Client{
		conn:       conn,
		send:       make(chan []byte, clientSendBuf),
		room:       room,
		author:     author,
		tokens:     float64(maxMsgsPerSecond),
		lastRefill: time.Now(),
	}

	// avoid blocking handler if room is temporarily busy
	select {
	case room.register <- client:
	case <-time.After(registerTimeout):
		_ = conn.Close()
		return
	}

	go client.writePump()
	go client.readPump()
}
