package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/jacobdanielrose/bytetalk/internal/protocol"
	"github.com/jacobdanielrose/bytetalk/internal/realtime"
)

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type connectedMsg struct {
	client *realtime.Client
	status int
}

type receivedMsg struct {
	payload []byte
}

func connectCmd() tea.Cmd {
	return func() tea.Msg {
		c, resp, err := websocket.DefaultDialer.Dial(
			fmt.Sprintf("ws://%s/ws", serv),
			nil,
		)
		if err != nil {
			return errMsg{err}
		}
		return connectedMsg{
			client: realtime.NewClient(c),
			status: resp.StatusCode,
		}
	}
}

func readOneMessageCmd(c *realtime.Client) tea.Cmd {
	return func() tea.Msg {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			return errMsg{err}
		}
		return receivedMsg{payload: data}
	}
}

func sendCmd(c *realtime.Client, data []byte) tea.Cmd {
	return func() tea.Msg {
		if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return errMsg{err}
		}
		return nil
	}
}

func signInUsercmd(user protocol.User) tea.Cmd {
	return func() tea.Msg {
		userBytes, err := json.Marshal(user)
		if err != nil {
			return errMsg{err}
		}
		// Check to see if user already exists
		resp, err := http.Get(fmt.Sprintf("http://%s/users/%s", serv, user.Username))
		if err != nil {
			return errMsg{err}
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return errMsg{errors.New("user already exists")}
		}

		resp, err = http.Post(
			fmt.Sprintf("http://%s/users/", serv),
			"application/json",
			bytes.NewReader(userBytes),
		)
		if err != nil {
			return errMsg{err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return errMsg{errors.New(resp.Status)}
		}
		return nil
	}
}
