package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Server struct {
	httpServer *http.Server
	wsServer   *http.Server
	agentLoop  *agent.AgentLoop
	channelMgr *channels.Manager
	cfg        *config.Config

	clients   map[string]*Client
	clientsMu sync.RWMutex

	token string
}

type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	role     string
	clientID string
	server   *Server
}

type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Event   string          `json:"event,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Ok      bool            `json:"ok,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type ConnectParams struct {
	Auth struct {
		Token string `json:"token,omitempty"`
	} `json:"auth,omitempty"`
	Role     string `json:"role"`
	ClientID string `json:"clientId"`
}

type ConnectResponse struct {
	HelloOk struct {
		Version   string `json:"version"`
		AgentID   string `json:"agentId"`
		ClientID  string `json:"clientId"`
		Connected bool   `json:"connected"`
	} `json:"helloOk"`
}

func New(cfg *config.Config, agentLoop *agent.AgentLoop, channelMgr *channels.Manager) *Server {
	return &Server{
		cfg:        cfg,
		agentLoop:  agentLoop,
		channelMgr: channelMgr,
		clients:    make(map[string]*Client),
		token:      cfg.Gateway.Token,
	}
}

func (s *Server) Start(addr string) error {
	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:         addr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.InfoCF("gateway", "Starting WebSocket server", map[string]any{
		"addr": addr,
	})

	return s.httpServer.ListenAndServe()
}

func (s *Server) StartWithMux(addr string, mux *http.ServeMux) error {
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:         addr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		Handler:      mux,
	}

	logger.InfoCF("gateway", "Starting WebSocket server", map[string]any{
		"addr": addr,
	})

	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	s.clientsMu.RLock()
	for _, c := range s.clients {
		c.conn.Close()
	}
	s.clientsMu.RUnlock()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"version":  "0.1.0",
		"clients":  len(s.clients),
		"channels": s.channelMgr.GetEnabledChannels(),
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.ErrorCF("gateway", "WebSocket upgrade failed", map[string]any{
			"error": err.Error(),
		})
		return
	}

	client := &Client{
		conn:     conn,
		send:     make(chan []byte, 256),
		server:   s,
		role:     "client",
		clientID: "",
	}

	s.registerClient(client)

	go s.writePump(client)
	go s.readPump(client)
}

func (s *Server) registerClient(c *Client) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	s.clients[c.clientID] = c
}

func (s *Server) unregisterClient(clientID string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	delete(s.clients, clientID)
}

func (s *Server) readPump(c *Client) {
	defer func() {
		s.unregisterClient(c.clientID)
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.ErrorCF("gateway", "WebSocket read error", map[string]any{
					"error":    err.Error(),
					"clientId": c.clientID,
				})
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			s.sendError(c, "", err.Error())
			continue
		}

		s.handleMessage(c, &msg)
	}
}

func (s *Server) writePump(c *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleMessage(c *Client, msg *Message) {
	switch msg.Type {
	case "req":
		s.handleRequest(c, msg)
	case "event":
		s.handleEvent(c, msg)
	default:
		if msg.Type == "connect" || (msg.Method == "" && msg.Event == "") {
			s.handleConnect(c, msg)
		} else {
			s.sendError(c, msg.ID, "unknown message type")
		}
	}
}

func (s *Server) handleConnect(c *Client, msg *Message) {
	var params ConnectParams
	if err := json.Unmarshal(msg.Payload, &params); err != nil {
		s.sendError(c, msg.ID, "invalid connect params")
		return
	}

	if s.token != "" && params.Auth.Token != s.token {
		s.sendError(c, msg.ID, "unauthorized")
		return
	}

	c.clientID = params.ClientID
	if c.clientID == "" {
		c.clientID = generateClientID()
	}
	c.role = params.Role
	if c.role == "" {
		c.role = "client"
	}

	s.registerClient(c)

	resp := Message{
		Type: "res",
		ID:   msg.ID,
		Ok:   true,
	}

	helloOk := ConnectResponse{}
	helloOk.HelloOk.Version = "0.1.0"
	helloOk.HelloOk.ClientID = c.clientID
	helloOk.HelloOk.Connected = true

	if s.agentLoop != nil {
		defaultAgent := s.agentLoop.GetDefaultAgent()
		if defaultAgent != nil {
			helloOk.HelloOk.AgentID = defaultAgent.ID
		}
	}

	resp.Payload, _ = json.Marshal(helloOk)

	c.send <- mustMarshal(resp)

	logger.InfoCF("gateway", "Client connected", map[string]any{
		"clientId": c.clientID,
		"role":     c.role,
	})
}

func (s *Server) handleRequest(c *Client, msg *Message) {
	switch msg.Method {
	case "status":
		s.handleStatus(c, msg)
	case "send":
		s.handleSend(c, msg)
	case "agent":
		s.handleAgent(c, msg)
	case "channels":
		s.handleChannels(c, msg)
	case "health":
		s.handleHealthReq(c, msg)
	default:
		s.sendError(c, msg.ID, fmt.Sprintf("unknown method: %s", msg.Method))
	}
}

func (s *Server) handleStatus(c *Client, msg *Message) {
	resp := Message{
		Type: "res",
		ID:   msg.ID,
		Ok:   true,
	}

	status := map[string]any{
		"version":  "0.1.0",
		"clients":  len(s.clients),
		"channels": s.channelMgr.GetEnabledChannels(),
		"uptime":   "N/A",
	}

	resp.Payload, _ = json.Marshal(status)
	c.send <- mustMarshal(resp)
}

func (s *Server) handleSend(c *Client, msg *Message) {
	var params struct {
		Message string `json:"message"`
		Channel string `json:"channel"`
		ChatID  string `json:"chatId"`
	}

	if err := json.Unmarshal(msg.Payload, &params); err != nil {
		s.sendError(c, msg.ID, "invalid params")
		return
	}

	if s.agentLoop == nil {
		s.sendError(c, msg.ID, "agent not initialized")
		return
	}

	go func() {
		result, err := s.agentLoop.ProcessDirect(context.Background(), params.Message, "gateway")
		if err != nil {
			c.send <- mustMarshal(Message{
				Type:  "event",
				Event: "agent",
				Payload: mustMarshal(map[string]any{
					"error": err.Error(),
				}),
			})
			return
		}

		c.send <- mustMarshal(Message{
			Type:  "event",
			Event: "agent",
			Payload: mustMarshal(map[string]any{
				"content": result,
			}),
		})
	}()

	c.send <- mustMarshal(Message{
		Type: "res",
		ID:   msg.ID,
		Ok:   true,
		Payload: mustMarshal(map[string]any{
			"accepted": true,
		}),
	})
}

func (s *Server) handleAgent(c *Client, msg *Message) {
	var params struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal(msg.Payload, &params); err != nil {
		s.sendError(c, msg.ID, "invalid params")
		return
	}

	if s.agentLoop == nil {
		s.sendError(c, msg.ID, "agent not initialized")
		return
	}

	go func() {
		result, err := s.agentLoop.ProcessDirect(context.Background(), params.Message, "gateway")
		if err != nil {
			c.send <- mustMarshal(Message{
				Type:  "event",
				Event: "agent",
				Payload: mustMarshal(map[string]any{
					"error": err.Error(),
				}),
			})
			return
		}

		c.send <- mustMarshal(Message{
			Type:  "event",
			Event: "agent",
			Payload: mustMarshal(map[string]any{
				"content": result,
			}),
		})
	}()

	c.send <- mustMarshal(Message{
		Type: "res",
		ID:   msg.ID,
		Ok:   true,
		Payload: mustMarshal(map[string]any{
			"accepted": true,
		}),
	})
}

func (s *Server) handleChannels(c *Client, msg *Message) {
	resp := Message{
		Type: "res",
		ID:   msg.ID,
		Ok:   true,
	}

	channels := s.channelMgr.GetEnabledChannels()
	resp.Payload, _ = json.Marshal(map[string]any{
		"channels": channels,
	})
	c.send <- mustMarshal(resp)
}

func (s *Server) handleHealthReq(c *Client, msg *Message) {
	c.send <- mustMarshal(Message{
		Type: "res",
		ID:   msg.ID,
		Ok:   true,
		Payload: mustMarshal(map[string]any{
			"status": "ok",
		}),
	})
}

func (s *Server) handleEvent(c *Client, msg *Message) {
}

func (s *Server) sendError(c *Client, id, errMsg string) {
	c.send <- mustMarshal(Message{
		Type:  "res",
		ID:    id,
		Ok:    false,
		Error: errMsg,
	})
}

func (s *Server) Broadcast(event string, payload any) {
	msg := Message{
		Type:    "event",
		Event:   event,
		Payload: mustMarshal(payload),
	}

	data := mustMarshal(msg)

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for _, c := range s.clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

func (s *Server) SendTo(clientID string, event string, payload any) error {
	msg := Message{
		Type:    "event",
		Event:   event,
		Payload: mustMarshal(payload),
	}

	data := mustMarshal(msg)

	s.clientsMu.RLock()
	c, ok := s.clients[clientID]
	s.clientsMu.RUnlock()

	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}

	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("client buffer full")
	}
}

func generateClientID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}

func mustMarshal(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
