package ws

import (
	"net/http"

	"github.com/charmbracelet/log"

	"github.com/gorilla/websocket"
	"github.com/jacobdanielrose/bytetalk/internal/realtime"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("upgrade error", "err", err, "remote", r.RemoteAddr)
		return
	}
	client := realtime.NewClient(conn)
	client.OnMessage = hub.writeEnvelope

	hub.register <- client
	defer func() { hub.unregister <- client }()

	go client.WritePump()
	client.ReadPump()

}
