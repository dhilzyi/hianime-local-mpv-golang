package main

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func sendErr(m *model, err error) {
	m.err = &errData{
		errMsg:   err,
		errState: m.state,
	}

	m.state = StateError
	m.cursor = 0
}

func testLog(channels chan string) {
	channels <- "test"
	interaval := 3 * time.Second
	time.Sleep(interaval)
	channels <- "second message test"
	time.Sleep(interaval)
	channels <- "third message"
	channels <- "complete"

	close(channels)
}

func listenMsg(msgCh <-chan string) tea.Cmd {
	return tea.Printf()
}
