package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/jacobdanielrose/bytetalk/internal/protocol"
	"github.com/jacobdanielrose/bytetalk/internal/ui"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.dump != nil {
		spew.Fdump(m.dump, msg)
	}
	switch v := msg.(type) {
	case connectedMsg:
		m.client = v.client
		return m, readOneMessageCmd(m.client)
	case receivedMsg:
		var cm ui.ChatMessage
		if mt, err := protocol.Unwrap(v.payload, &cm); err == nil {
			switch mt {
			case protocol.TypeChatMessage:
				m.messages = append(m.messages, cm)
				cm.User.Color = lipgloss.Color(fmt.Sprintf("#%06x", rand.Intn(0xFFFFFF)))
				m.viewport.SetContent(renderMessages(m.messages, m.styles, m.viewport.Width))
				m.viewport.GotoBottom()
			default:
				log.Warn("Unhandled message type", "type", mt)
			}
		}
		if m.client != nil {
			return m, readOneMessageCmd(m.client)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.ResizeModel(v.Width, v.Height)
	case errMsg:
		m.err = v
		return m, nil
	}

	switch m.state {
	case viewLogin:
		return m.updateLogin(msg)
	case viewChat:
		return m.updateChat(msg)
	default:
		return m, nil
	}
}

func (m model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			user := m.signup(m.input.Value())
			m.input.SetValue("")
			return m, tea.Batch(cmd, connectCmd(), signInUsercmd(user))
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	case errMsg:
		m.err = msg
		log.Error("Login loop error", "err", m.err)
	}
	return m, cmd
}

func (m model) updateChat(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		innerW := msg.Width - (m.styles.margin.Left + m.styles.margin.Right) - (m.styles.padding.Left + m.styles.padding.Right)
		innerW = max(innerW, 1)

		innerH := msg.Height - (m.styles.margin.Top + m.styles.margin.Bottom) - (m.styles.padding.Top + m.styles.padding.Bottom)
		innerH = max(innerH, 1)

		// Account for viewport border thickness (NormalBorder = 1 per side)

		// Set widths (textarea has no border)
		m.viewport.Width = max(innerW-m.styles.vpBorderX, 1)
		m.textarea.SetWidth(max(innerW, 1))

		// Set heights (subtract gap and viewport borders)
		m.viewport.Height = max(innerH-m.textarea.Height()-lipgloss.Height(gap)-m.styles.vpBorderY, 1)

		if len(m.messages) > 0 {
			m.viewport.SetContent(renderMessages(m.messages, m.styles, m.viewport.Width))
		}
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.textarea.KeyMap.InputEnd):
			if text := m.textarea.Value(); text != "" && m.client != nil {
				pUser := protocol.User{
					ID:       m.user.UUID,
					Username: m.user.Username,
				}
				chatMsg := protocol.ChatMessage{
					ID:        uuid.New(),
					User:      pUser,
					Text:      text,
					Timestamp: time.Now(),
				}
				if dat, err := protocol.Wrap(protocol.TypeChatMessage, chatMsg); err == nil {
					// Optimistically render existing messages (server echo will append the new one)
					m.viewport.SetContent(renderMessages(m.messages, m.styles, m.viewport.Width))
					m.textarea.Reset()
					m.viewport.GotoBottom()
					return m, tea.Batch(tiCmd, vpCmd, sendCmd(m.client, dat))
				}
			}
			return m, tea.Batch(tiCmd, vpCmd)
		case key.Matches(msg, m.textarea.KeyMap.InsertNewline):
			m.textarea.InsertString("\n")
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println(m.textarea.Value())
			return m, tea.Quit
		}
	case errMsg:
		m.err = msg
	}
	return m, tea.Batch(vpCmd, tiCmd)
}
