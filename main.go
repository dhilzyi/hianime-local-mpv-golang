package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"

	"hianime-mpv-go/config"
	"hianime-mpv-go/hianime"
	"hianime-mpv-go/player"
	"hianime-mpv-go/state"
)

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
	fmt.Printf("User run in platform: %s\n", runtime.GOOS)

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
				JapaneseName: seriesMetadata.JapaneseName,
				EnglishName:  seriesMetadata.EnglishName,
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

		var cache_episodes []hianime.Episodes

	episode_loop:
		for {
			fmt.Printf("\n--- Series: %s ---\n\n", seriesMetadata.JapaneseName)

			if len(cache_episodes) > 0 {
				for i := range len(cache_episodes) {
					eps := cache_episodes[i]
					if eps.Number == historySelect.LastEpisode {
						fmt.Printf(" [%d] %s <---\n", eps.Number, eps.JapaneseTitle)
					} else {
						fmt.Printf(" [%d] %s\n", eps.Number, eps.JapaneseTitle)
					}
				}
			} else {
				cache_episodes = hianime.GetEpisodes(seriesMetadata.AnimeID)
				for i := range len(cache_episodes) {
					eps := cache_episodes[i]
					if eps.Number == historySelect.LastEpisode {
						fmt.Printf(" [%d] %s <---\n", eps.Number, eps.JapaneseTitle)
					} else {
						fmt.Printf(" [%d] %s\n", eps.Number, eps.JapaneseTitle)
					}
				}
			}

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
			if int_input > 0 && int_input <= len(cache_episodes) {
				selectedEpisode = cache_episodes[int_input-1]
				servers = hianime.GetEpisodeServerId(selectedEpisode.Id)

				historySelect.LastEpisode = int_input

				history = state.UpdateHistory(history, historySelect)
				state.SaveHistory(history)
			} else {
				fmt.Println("Number is invalid.")
				continue
			}
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
					for i := range servers {
						selectedServer = servers[i]
						fmt.Printf("--> Selecting '%s'....\n", selectedServer.Name)

						attempt, err := hianime.GetStreamData(selectedServer.DataId)
						if err == nil {
							streamData = attempt
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
							break
						}
					} else {
						fmt.Println("Number is invalid.")
						continue
					}
				}

				if streamData.Url == "" {
					continue
				}

				var success bool
				var subDelay, lastPos, totalDur float64
				if runtime.GOOS == "android" {
					androidCommands := player.BuildAndroidCommands(seriesMetadata, selectedEpisode, selectedServer, streamData)
					player.PlayAndroidMpv(androidCommands)

				} else {
					desktopCommands := player.BuildDesktopCommands(seriesMetadata, selectedEpisode, selectedServer, streamData, historySelect)
					success, subDelay, lastPos, totalDur = player.PlayMpv(desktopCommands)
				}
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

					fmt.Println(historySelect)

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
