package main

import (
	"fmt"
	"os"

	"github.com/dhilzyi/hianime-cli/internal/app"
	"github.com/dhilzyi/hianime-cli/internal/config"
	"github.com/dhilzyi/hianime-cli/internal/state"

	tea "charm.land/bubbletea/v2"
)

func main() {
	appDir, err := config.InitPath()
	if err != nil {
		fmt.Println("Fail to initialize path: " + err.Error())
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(appDir.ConfigDir, "v1.9.0")
	if err != nil {
		fmt.Println("Fail to load config file: " + err.Error())
	}
	history, err := state.LoadHistory(appDir.DataDir)
	if err != nil {
		fmt.Println("Fail to load history file: " + err.Error())
	}

	cache := app.NewCache()
	initialModel := model{
		state:  StateHistory,
		cursor: 0,
		cfg:    cfg,

		session: &session{
			historyList: history,
		},
		appCache: cache,
		appPath:  appDir,
	}

	p := tea.NewProgram(initialModel)
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
