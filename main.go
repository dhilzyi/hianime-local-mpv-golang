package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"hianime-mpv-go/config"
	"hianime-mpv-go/hianime"
	"hianime-mpv-go/player"
	"hianime-mpv-go/state"
	"hianime-mpv-go/ui"
)

var cacheEpisodes = make(map[string][]hianime.Episodes) // "AnimeID" : {{Eps: 1, ...}, ...}

func main() {
	var url string
	scanner := bufio.NewScanner(os.Stdin)
	history, err := state.LoadHistory()
	if err != nil {
		fmt.Println(err)
	}
	configSession, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Fail to load config file: " + err.Error())
	}

	flag.BoolVar(&config.DebugMode, "debug", false, "Enable verbose debug logging")
	flag.Parse()
series_loop:
	for {
		if len(history) > 0 {
			fmt.Printf("\n--- Recent History ---\n\n")
			for i := range history {
				fmt.Printf(" [%d] %s\n", i+1, history[i].JapaneseName)
			}

		} else {
			fmt.Printf("\n--- No recent history found ---\n\n")
		}
		fmt.Print("\nEnter number or paste hianime url to play: ")
		scanner.Scan()

		seriesInput := scanner.Text()
		if seriesInput == "q" {
			break series_loop
		}

		var historySelect state.History
		var seriesMetadata hianime.SeriesData

		if strings.Contains(seriesInput, "hianime.to") {
			url = seriesInput
			seriesMetadata = hianime.GetSeriesData(url)
			new_history := state.History{
				Url:          seriesMetadata.SeriesUrl,
				JapaneseName: strings.TrimSpace(seriesMetadata.JapaneseName),
				EnglishName:  strings.TrimSpace(seriesMetadata.EnglishName),
				AnilistID:    seriesMetadata.AnilistID,
				LastEpisode:  1,
				Episode:      make(map[int]state.EpisodeProgress),
			}
			historySelect = new_history

			history = state.UpdateHistory(history, new_history)
			state.SaveHistory(history)
		} else {
			int_series, err := strconv.Atoi(seriesInput)
			if err != nil {
				fmt.Println("Failed to convert to integer. Input number or paste url")
				continue
			}

			historySelect = history[int_series-1]
			url = historySelect.Url

			seriesMetadata = hianime.GetSeriesData(url)

			history = state.UpdateHistory(history, historySelect)
			state.SaveHistory(history)
		}

	episode_loop:
		for {
			fmt.Printf("\n--- Series: %s ---\n\n", seriesMetadata.JapaneseName)

			episodeCache, exists := cacheEpisodes[seriesMetadata.AnimeID]
			if !exists {
				episodeCache = hianime.GetEpisodes(seriesMetadata.AnimeID)
				cacheEpisodes[seriesMetadata.AnimeID] = episodeCache
			}

			ui.PrintEpisodes(episodeCache, historySelect)

			fmt.Print("\nEnter number episode to watch (or 'q' to go back): ")
			scanner.Scan()

			episodeInput := scanner.Text()
			episodeInput = strings.TrimSpace(episodeInput)

			if episodeInput == "q" {
				break episode_loop
			}

			int_input, err := strconv.Atoi(episodeInput)
			if err != nil {
				fmt.Printf("Error when converting to int: %s\n", err.Error())
				continue
			}

			var servers []hianime.ServerList
			var selectedEpisode hianime.Episodes
			if int_input > 0 && int_input <= len(episodeCache) {
				selectedEpisode = episodeCache[int_input-1]
				servers = hianime.GetEpisodeServerId(selectedEpisode.Id)

				historySelect.LastEpisode = int_input

				history = state.UpdateHistory(history, historySelect)
				state.SaveHistory(history)
			} else {
				fmt.Println("Number is invalid.")
				continue
			}

			var testedServer int
		server_loop:
			for {
				if len(servers) == 0 {
					fmt.Println("\nNo available servers found.")
					break
				}

				var selectedServer hianime.ServerList
				var streamData hianime.StreamData

				if configSession.AutoSelectServer {
					fmt.Println("\n--> Auto-select server enabled.")
					for i := testedServer; i < len(servers); i++ {
						selectedServer = servers[i]
						if strings.Contains("HD-3", selectedServer.Name) {
							fmt.Printf("-> Skipping 'HD-3' server")
							continue
						}

						fmt.Printf("--> Selecting '%s'....\n", selectedServer.Name)

						attempt, err := hianime.GetStreamData(selectedServer.DataId)
						if err == nil {
							streamData = attempt
							testedServer = i + 1
							break
						}
					}

				} else {
					fmt.Print("\n--- Available Servers ---\n")

					for i := range len(servers) {
						ser_ins := servers[i]

						if ser_ins.Type == "dub" {
							fmt.Printf(" [%d] %s (Dub)\n", i+1, ser_ins.Name)
						} else {
							fmt.Printf(" [%d] %s\n", i+1, ser_ins.Name)
						}
					}
					fmt.Print("\nEnter server number (or 'q' to go back): ")
					scanner.Scan()

					server_input := scanner.Text()
					server_input = strings.TrimSpace(server_input)

					if server_input == "q" {
						break server_loop
					}
					int_server_input, err := strconv.Atoi(server_input)
					if err != nil {
						fmt.Printf("Error when converting to int: %s\n", err.Error())
						continue
					}

					if int_server_input > 0 && int_server_input <= len(servers) {
						selectedServer = servers[int_server_input-1]

						attempt, err := hianime.GetStreamData(selectedServer.DataId)
						if err == nil {
							streamData = attempt
							fmt.Println(streamData)
						}
					} else {
						fmt.Println("Number is invalid.")
						continue
					}
				}

				if streamData.Url == "" {
					fmt.Println("Couldn't find streamdata url!")
					continue
				}

				// get mpv path automatically according user platforms.
				binName := player.GetMpvBinary(configSession.MpvPath)
				desktopCommands := player.BuildDesktopCommands(seriesMetadata, selectedEpisode, selectedServer, streamData, historySelect, configSession)

				success, subDelay, lastPos, totalDur := player.PlayMpv(binName, desktopCommands)

				if success {
					cleanDelay := math.Round(subDelay*10) / 10
					historySelect.SubDelay = cleanDelay

					if historySelect.Episode == nil {
						historySelect.Episode = make(map[int]state.EpisodeProgress)
					}

					historySelect.Episode[selectedEpisode.Number] = state.EpisodeProgress{
						Position: lastPos,
						Duration: totalDur,
					}

					history = state.UpdateHistory(history, historySelect)
					state.SaveHistory(history)

					break server_loop
				} else {
					continue
				}
			}
		}
	}
}
