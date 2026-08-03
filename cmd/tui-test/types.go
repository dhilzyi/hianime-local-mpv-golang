package main

import (
	"github.com/dhilzyi/hianime-cli/internal/core"
	"github.com/dhilzyi/hianime-cli/internal/state"
)

type session struct {
	urlSeries string

	selected selected

	provider   core.Provider
	seriesData core.SeriesData

	historyList []state.History
	episodeList []core.Episode
	serverList  []core.Server
}

type errData struct {
	errMsg   error
	errState screenState
}

type selected struct {
	history state.History
	series  core.SeriesData
	episode core.Episode
	server  core.Server
	stream  core.StreamData
}
