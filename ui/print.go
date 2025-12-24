package ui

import (
	"fmt"

	"hianime-mpv-go/config"
	"hianime-mpv-go/hianime"
	"hianime-mpv-go/state"
)

func prettyDuration(seconds float64) string {
	m := int(seconds) / 60
	s := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func PrintEpisodes(episodes []hianime.Episodes, history state.History) {
	for _, eps := range episodes {
		prefix := "  "
		if eps.Number == history.LastEpisode {
			prefix = "-->"
		}

		timeInfo := ""
		if prog, ok := history.Episode[eps.Number]; ok {
			curr := prettyDuration(prog.Position)
			total := prettyDuration(prog.Duration)
			timeInfo = fmt.Sprintf("\t%s/%s", curr, total)
		}

		var title string
		if eps.JapaneseTitle == "" {
			title = eps.EnglishTitle
		} else {
			title = eps.JapaneseTitle
		}
		fmt.Printf(" %s [%d] %s		%s\n", prefix, eps.Number, title, timeInfo)
	}

}

func DebugPrint(format string, contents ...any) {
	if config.DebugMode {
		prefix := "[ DEBUG ] "
		fmt.Println(prefix, contents)
	}
}
