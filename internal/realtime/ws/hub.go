package ws

import (
	"context"
	"database/sql"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/jacobdanielrose/bytetalk/internal/database"
	"github.com/jacobdanielrose/bytetalk/internal/protocol"
	"github.com/jacobdanielrose/bytetalk/internal/realtime"
)

type Hub struct {
	clients    map[*realtime.Client]bool
	broadcast  chan []byte
	register   chan *realtime.Client
	unregister chan *realtime.Client
	db         *database.Queries
}

func NewHub(db *database.Queries) *Hub {
	return &Hub{
		clients:    make(map[*realtime.Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *realtime.Client),
		unregister: make(chan *realtime.Client),
		db:         db,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Info("Client registered", "addr", client.Conn.RemoteAddr(), "total", len(h.clients))
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
				log.Info("Client unregistered", "addr", client.Conn.RemoteAddr(), "total", len(h.clients))
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
					log.Info("Client send channel full, unregistering.", "addr", client.Conn.RemoteAddr())
				}
			}
		}
	}
}

func (h *Hub) Broadcast(data []byte) error {
	h.broadcast <- data
	return nil
}

func (h *Hub) writeEnvelope(b []byte) {
	var payload any
	envType, err := protocol.Unwrap(b, &payload)
	if err != nil || envType == "" {
		log.Warn("Dropping non-enveloped or invalid WS frame", "error", err)
		return
	}

	switch envType {
	case protocol.TypeChatMessage:
		msg, ok := payload.(protocol.ChatMessage)
		if !ok {
			log.Error("failed to cast payload to ChatMessage", "payload", payload)
			return
		}
		_, err := h.db.CreateMessage(context.Background(), database.CreateMessageParams{
			ID:        uuid.New(),
			UserID:    msg.User.ID,
			Content:   msg.Text,
			CreatedAt: sql.NullTime{Time: time.Now(), Valid: true},
		})
		if err != nil {
			log.Error("failed to create message", "err", err)
			return
		}
		log.Info("Received and saved message", "user", msg.User, "msgId", msg.ID, "createdAt", time.Now())
		h.broadcast <- b
	default:
		log.Warn("Unknown WS envelope type", "type", envType)
	}

}
