package ui

import (
	"fmt"
	"os"
	"slices"

	"github.com/dhilzyi/hianime-cli/internal/common"
	"github.com/dhilzyi/hianime-cli/internal/core"
	"github.com/dhilzyi/hianime-cli/internal/state"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

func PrintEpisodes(episodes []core.Episode, history state.History) {
	symbols := tw.NewSymbolCustom("MyGrid").
		WithRow("─").
		WithColumn("│").
		WithCenter("┼")

	table := tablewriter.NewTable(os.Stdout,

		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
			Symbols: symbols,
			Settings: tw.Settings{
				Separators: tw.Separators{
					BetweenColumns: tw.On,
				},
				Lines: tw.Lines{
					ShowTop: tw.On,
				},
			},
		})),

		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
	)
	table.Header([]string{"LAST", "NO", "EPS NAME", "DURATION"})

	for _, eps := range episodes {
		prefix := "  "
		if eps.Number == history.LastEpisode {
			prefix = "-->"
		}

		timeInfo := ""
		if prog, ok := history.Episodes[eps.Number]; ok {
			curr := prettyDuration(prog.Position)
			total := prettyDuration(prog.Duration)
			timeInfo = fmt.Sprintf("%s/%s", curr, total)
		}
		table.Append([]string{
			prefix,
			fmt.Sprintf("%d", eps.Number),
			eps.Titles.GetPreferredTitle(),
			timeInfo,
		})
	}
	table.Render()
}

func DebugPrint(format string, contents ...any) {
	prefix := "[ DEBUG ] "
	fmt.Println(prefix, contents)
}

func PrintSearchResults(searchData []core.SearchResult, order []string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"NO", "NAME", "TYPE", "DURATION", "NUMBER EPS", "YEAR"})

	typeOrders := order

	slices.SortFunc(searchData, func(a, b core.SearchResult) int {
		orderA := typeOrder(a.Type, typeOrders)
		orderB := typeOrder(b.Type, typeOrders)

		if orderA != orderB {
			if orderA < orderB {
				return -1
			}
		}
		return 1
	})

	for i := range searchData {
		ins := searchData[i]

		durationStr := "N/A"
		if ins.Duration > 0 {
			durationStr = fmt.Sprintf("%dm", ins.Duration)
		}

		table.Append([]string{
			fmt.Sprintf("[%d]", i+1),
			common.TruncatedString(ins.Titles.GetPreferredTitle(), 63),
			formatString(ins.Type),
			durationStr,
			formatInt(ins.NumberEpisodes),
			formatInt(ins.Year),
		})
	}

	table.Render()
}

func PrintServers(servers []core.Server) {
	fmt.Printf("\n--- Available Servers ---\n\n")

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"NO", "NAME", "TYPE"})

	for i := range servers {
		inst := servers[i]

		table.Append([]string{
			fmt.Sprintf("[%d]", i+1),
			inst.Name,
			inst.Type,
		})
	}

	table.Render()
}

func PrintRecentHistory(history []state.History) {
	fmt.Printf("\n--- Recent History ---\n\n")
	symbols := tw.NewSymbolCustom("MyGrid").
		WithRow("─").
		WithColumn("│").
		WithCenter("┼")

	table := tablewriter.NewTable(os.Stdout,

		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
			Symbols: symbols,
			Settings: tw.Settings{
				Separators: tw.Separators{
					BetweenColumns: tw.On,
				},
				Lines: tw.Lines{
					ShowTop: tw.On,
				},
			},
		})),

		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
	)
	table.Header([]string{"NO", "NAME", "TYPE", "PROVIDER"})

	var provider string
	for i := range history {
		inst := history[i]
		title := inst.Metadata.Titles.GetPreferredTitle()
		if inst.Provider != "" {
			provider = inst.Provider
		} else {
			provider = "N/A"
		}

		table.Append([]string{
			fmt.Sprintf("[%d]", i+1),
			title,
			"N/A",
			provider,
		})
	}

	table.Render()
}
