package main

import (
	"fmt"
	"strings"

	"github.com/dhilzyi/hianime-cli/internal/app"
	"github.com/dhilzyi/hianime-cli/internal/config"
	"github.com/dhilzyi/hianime-cli/internal/player"

	tea "charm.land/bubbletea/v2"
)

type screenState int

const (
	// Main state
	StateHistory screenState = iota
	StateEpisodes
	StateServer

	StateProcessStream
	StatePlaying

	// Message state
	StateError
	StateLog

	// Miscellaneous
	StateConfigSettings
)

type model struct {
	state  screenState
	cursor int
	subs   bool

	err *errData

	session  *session
	cfg      *config.Config
	appCache *app.Cache
	appPath  config.AppPaths
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Global control
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Per state control
		switch m.state {
		case StateHistory:
			switch msg.String() {
			case "q", "esc":
				return m, tea.Quit
			case "up", "k":
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.session.historyList) - 1
				}
			case "down", "j":
				m.cursor++
				if m.cursor >= len(m.session.historyList) {
					m.cursor = 0
				}
			case "s":
				m.subs = !m.subs
			case "c":
				m.state = StateConfigSettings
			case "d":
				msgCh := make(chan string)
			case "enter":
				m.session.selected.history = m.session.historyList[m.cursor]
				m.session.urlSeries = m.session.selected.history.Metadata.SeriesUrl
				fetchResult, err := app.ResolveInput(app.ResolveParams{URL: m.session.urlSeries}, m.appCache)
				if err != nil {
					sendErr(&m, err)
					return m, nil
				}
				m.session.provider = fetchResult.Provider
				m.session.episodeList = fetchResult.Episodes
				m.session.seriesData = fetchResult.SeriesData

				m.state = StateEpisodes
				m.cursor = 0
			}
		case StateEpisodes:
			switch msg.String() {
			case "q", "esc":
				m.state = StateHistory
				m.cursor = 0
			case "up", "k":
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.session.episodeList) - 1
				}
			case "down", "j":
				m.cursor++
				if m.cursor >= len(m.session.episodeList) {
					m.cursor = 0
				}
			case "enter":
				m.session.selected.episode = m.session.episodeList[m.cursor]
				servers, err := m.session.provider.GetServers(m.session.selected.episode)
				if err != nil {
					sendErr(&m, err)
					return m, nil
				}

				m.session.serverList = servers
				m.cursor = 0
				m.state = StateServer
			}
		case StateServer:
			switch msg.String() {
			case "q", "esc":
				m.cursor = 0
				m.state = StateEpisodes
			case "up", "k":
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.session.serverList) - 1
				}
			case "down", "j":
				m.cursor++
				if m.cursor >= len(m.session.serverList) {
					m.cursor = 0
				}
			case "enter", "space":
				m.session.selected.server = m.session.serverList[m.cursor]
				stream, err := m.session.provider.GetStreamData(m.session.selected.server.Key)
				if err != nil {
					sendErr(&m, err)
					return m, nil
				}
				m.session.selected.stream = stream

				m.cursor = 0
				m.state = StatePlaying
			}
		case StatePlaying:
			switch msg.String() {
			case "q", "esc":
				m.cursor = 0
				m.state = StateServer
			}
		case StateError:
			switch msg.String() {
			case "enter", "q", "esc":
				m.cursor = 0
				m.state = m.err.errState

				m.err = nil
			}
		}

	}

	return m, nil
}

func (m model) View() tea.View {
	var buf strings.Builder

	switch m.state {
	case StateHistory:
		buf.WriteString("--- RECENT HISTORY ---\n\n")
		for i, item := range m.session.historyList {
			if m.cursor == i {
				buf.WriteString("-> ")
			} else {
				buf.WriteString("   ")
			}
			buf.WriteString(item.Metadata.Titles.GetPreferredTitle() + "\n")
		}
		fmt.Fprintf(&buf, "\n\n Subs: %t\n", m.subs)
	case StateEpisodes:
		buf.WriteString("--- EPISODES ---\n\n")
		for i, ep := range m.session.episodeList {
			if m.cursor == i {
				buf.WriteString("-> ")
			} else {
				buf.WriteString("   ")
			}
			fmt.Fprintf(&buf, "[%d] %s\n", ep.Number, ep.Titles.GetPreferredTitle())
		}
		fmt.Fprintf(&buf, "\nSelected: %s", m.session.episodeList[m.cursor].Titles.GetPreferredTitle())
	case StateServer:
		buf.WriteString("--- AVAILABLE SERVERS ---\n\n")
		for i, srv := range m.session.serverList {
			if m.cursor == i {
				buf.WriteString("-> ")
			} else {
				buf.WriteString("   ")
			}
			fmt.Fprintf(&buf, "[%d] %s\n", i+1, srv.Name)
		}
	case StateProcessStream:
		msgChan := make(chan string)
		cmds := player.BuildMpvCommands(
			*m.cfg,
			m.session.selected.series,
			m.session.selected.episode,
			m.session.selected.server,
			m.session.selected.stream,
			m.session.selected.history,
			m.appPath.DataDir,
			false,
			msgChan,
		)
	case StatePlaying:
		buf.WriteString("--- PLAYING ---\n\n")
		buf.WriteString("Info:\n")
		fmt.Fprintf(&buf, "	No.Episode: %d\n", m.session.selected.episode.Number)
		fmt.Fprintf(&buf, "	Episode Title: %s\n", m.session.selected.episode.Titles.GetPreferredTitle())
		buf.WriteString("\nPress 'q' to end session...")
	case StateError:
		buf.WriteString("--- ERROR ---\n\n")
		buf.WriteString("Something unexpected just occured\n")
		fmt.Fprintf(&buf, "Error: %s\n", m.err.errMsg.Error())
		fmt.Fprintf(&buf, "\n\nPress 'enter' to continue...\n")
	}

	return tea.NewView(buf.String())
}
