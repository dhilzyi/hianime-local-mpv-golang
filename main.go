package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"hianime-mpv-go/config"
	"hianime-mpv-go/hianime"
	"hianime-mpv-go/jimaku"
	"hianime-mpv-go/state"
)

// NOTE: For intro and outro in mpv so user can know the timestamps.
func CreateChapters(data hianime.StreamData) string {
	if data.Intro.Start < 0 && data.Intro.End < 0 {
		return ""
	} else if data.Outro.Start <= 0 && data.Outro.End <= 0 {
		return ""
	}

	f, err := os.CreateTemp("", "hianime_chapters_*.txt")
	if err != nil {
		fmt.Println("Error while creating temporary file: " + err.Error())
		return ""
	}

	contents := ";FFMETADATA1\n"

	writePart := func(start, end int, title string) {
		contents += "[CHAPTER]\n"
		contents += "TIMEBASE=1/1\n"
		contents += fmt.Sprintf("START=%d\n", start)
		contents += fmt.Sprintf("END=%d\n", end)
		contents += fmt.Sprintf("title=%s\n\n", title)
	}

	if data.Intro.Start == 0 {
		writePart(data.Intro.Start, data.Intro.End, "Intro")
	} else {
		writePart(0, data.Intro.Start, "Part A")
		writePart(data.Intro.Start, data.Intro.End, "Intro")
	}

	writePart(data.Intro.End, data.Outro.Start, "Part B")
	writePart(data.Outro.Start, data.Outro.End, "Outro")
	writePart(data.Outro.End, 9999999, "Part C")

	f.WriteString(contents)
	return f.Name()
}

func PlayMpv(mpv_commands []string) int {
	cmdName := "mpv.exe"

	// spyPath, err := player.GenerateSpyScript()
	// if err != nil {
	// 	fmt.Println("Failed to create status tracker:", err)
	// } else {
	// 	defer os.Remove(spyPath)
	// 	mpv_commands = append(mpv_commands, "--script="+spyPath)
	// }
	var return_value int
	cmd := exec.Command(cmdName, mpv_commands...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Failed to put stdout: " + err.Error())
	}

	fmt.Println("\n--> Executing mpv commands...")

	if err := cmd.Start(); err != nil {
		fmt.Println("Error while running mpv: " + err.Error())
	}

	scanner := bufio.NewScanner(stdout)
	timer := time.AfterFunc(20*time.Second, func() {
		fmt.Println("\n--> MPV is timeout. Killing process...")
		cmd.Process.Kill()
		return_value = 0
	})

	var flag bool
	for scanner.Scan() {
		line := scanner.Text()

		// fmt.Println(line)

		if strings.Contains(line, "(+) Video --vid= ") || strings.Contains(line, "h264") {
			timer.Stop()
			if !flag {
				fmt.Println("\nStream is valid. Opening mpv")
				flag = true
			}
			return_value = 1
		} else if strings.Contains(line, "Opening failed") || strings.Contains(line, "HTTP error") {
			fmt.Println("Failed to stream. Potentially dead link...")
			return_value = 0
			timer.Stop()
			cmd.Process.Kill()
			break
		} else if strings.Contains(line, "::STATUS::") {
			parts := strings.Split(line, "::STATUS::")

			if len(parts) > 0 {
				currentStr, totalStr, found := strings.Cut(parts[1], "/")

				if found {
					current, err := strconv.ParseFloat(currentStr, 64)
					if err != nil {
						fmt.Println("Error while converting to float: " + err.Error())
					}
					total, err := strconv.ParseFloat(totalStr, 64)
					if err != nil {
						fmt.Println("Error while converting to float: " + err.Error())
					}

					fmt.Println(current)
					fmt.Println(total)
				}
			}

			continue
		} else if strings.Contains(line, "::SUB_DELAY::") {
			parts := strings.Split(line, "::SUB_DELAY::")

			for i := range parts {
				fmt.Println(parts[i])
			}

			raw, _, found := strings.Cut(parts[1], "/")
			if found {
				delay, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					fmt.Println("Error while converting to float: " + err.Error())
				}
				fmt.Printf("DELAY %f", delay)
			}
			continue
		}
	}

	if err := cmd.Wait(); err != nil {
	}

	return return_value
}

func main() {
	var url string
	scanner := bufio.NewScanner(os.Stdin)
	history, err := state.LoadHistory()
	if err != nil {
		fmt.Println(err)
	}
	config_session, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Fail to load config file: " + err.Error())
	}

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

		series_input := scanner.Text()
		if series_input == "q" {
			break series_loop
		}

		var history_select state.History
		var series_metadata hianime.SeriesData

		if strings.Contains(series_input, "hianime.to") {
			url = series_input
			series_metadata = hianime.GetSeriesData(url)
			new_data := state.History{
				Url:          series_metadata.SeriesUrl,
				JapaneseName: series_metadata.JapaneseName,
				EnglishName:  series_metadata.EnglishName,
				AnilistID:    series_metadata.AnilistID,
				LastEpisode:  1,
			}
			history_select = new_data

			history = state.UpdateHistory(history, new_data)
			state.SaveHistory(history)
		} else {
			int_series, err := strconv.Atoi(series_input)
			if err != nil {
				fmt.Println("Failed to convert to integer. Input number or paste url")
				continue
			}

			history_select = history[int_series-1]
			url = history_select.Url

			series_metadata = hianime.GetSeriesData(url)

			history = state.UpdateHistory(history, history_select)
			state.SaveHistory(history)
		}

		var cache_episodes []hianime.Episodes

	episode_loop:
		for {
			fmt.Printf("\n--- Series: %s ---\n\n", series_metadata.JapaneseName)

			if len(cache_episodes) > 0 {
				for i := range len(cache_episodes) {
					eps := cache_episodes[i]
					if eps.Number == history_select.LastEpisode {
						fmt.Printf(" [%d] %s <---\n", eps.Number, eps.JapaneseTitle)
					} else {
						fmt.Printf(" [%d] %s\n", eps.Number, eps.JapaneseTitle)
					}
				}
			} else {
				cache_episodes = hianime.GetEpisodes(series_metadata.AnimeID)
				for i := range len(cache_episodes) {
					eps := cache_episodes[i]
					if eps.Number == history_select.LastEpisode {
						fmt.Printf(" [%d] %s <---\n", eps.Number, eps.JapaneseTitle)
					} else {
						fmt.Printf(" [%d] %s\n", eps.Number, eps.JapaneseTitle)
					}
				}
			}

			fmt.Print("\nEnter number episode to watch (or 'q' to go back): ")
			scanner.Scan()

			eps_input := scanner.Text()
			eps_input = strings.TrimSpace(eps_input)

			if eps_input == "q" {
				break episode_loop
			}

			int_input, err := strconv.Atoi(eps_input)
			if err != nil {
				fmt.Printf("Error when converting to int: %s\n", err.Error())
				continue
			}

			var servers []hianime.ServerList
			var selected_ep hianime.Episodes
			if int_input > 0 && int_input <= len(cache_episodes) {
				selected_ep = cache_episodes[int_input-1]
				servers = hianime.GetEpisodeServerId(selected_ep.Id)

				history_select.LastEpisode = int_input

				history = state.UpdateHistory(history, history_select)
				state.SaveHistory(history)
			} else {
				fmt.Println("Number is invalid.")
				continue
			}
		server_loop:
			for {
				if !(len(servers) > 0) {
					fmt.Println("\nNo available servers found.")
					break
				}

				var selected_server hianime.ServerList
				var stream_data hianime.StreamData

				if config_session.AutoSelectServer {
					fmt.Println("\n--> Auto-select server enabled.")
					for i := range servers {
						selected_server = servers[i]
						fmt.Printf("--> Selecting '%s'....\n", selected_server.Name)

						attempt, err := hianime.GetIframe(selected_server.DataId)
						if err == nil {
							stream_data = attempt
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
						selected_server = servers[int_server_input-1]

						attempt, err := hianime.GetIframe(selected_server.DataId)
						if err == nil {
							stream_data = attempt
							break
						}
					} else {
						fmt.Println("Number is invalid.")
						continue
					}
				}

				if stream_data.Url == "" {
					fmt.Println("Couldn't found stream url")
					continue
				} else {
					display_title := fmt.Sprintf("%s [Ep. %d] %s (%s)", series_metadata.JapaneseName, selected_ep.Number, selected_ep.JapaneseTitle, selected_server.Name)
					header_fields := []string{
						fmt.Sprintf("Referer: %s", stream_data.Referer),
						fmt.Sprintf("User-Agent: %s", stream_data.UserAgent),
						fmt.Sprintf("Origin: %s", "https://megacloud.blog"),
					}
					fullHeaders := strings.Join(header_fields, ",")

					mpv_commands := []string{
						stream_data.Url,
						"--ytdl-format=bestvideo+bestaudio/best",
						fmt.Sprintf("--http-header-fields=%s", fullHeaders),
						fmt.Sprintf("--title=%s", display_title),
						"--script-opts=osc-title=${title}",
					}

					chapter_filename := CreateChapters(stream_data)
					if chapter_filename != "" {
						mpv_commands = append(mpv_commands, fmt.Sprintf("--chapters-file=%s", chapter_filename))
					}

					// mpv_commands = append(mpv_commands, "--v")

					jimaku_list, err := jimaku.GetSubsJimaku(series_metadata, selected_ep.Number)
					if err != nil {
						fmt.Printf("Failed to get subs from jimaku: '%s'\n", err)
						fmt.Printf("Skipping Jimaku\n")
					}

					if len(jimaku_list) > 0 {
						for i := range jimaku_list {
							mpv_commands = append(mpv_commands, fmt.Sprintf("--sub-file=%s", jimaku_list[i]))
						}
					}

					if stream_data.Tracks[0].File != "" {
						for i := range stream_data.Tracks {
							ins := stream_data.Tracks[i]
							if !strings.Contains(ins.File, "thumbnail") {
								mpv_commands = append(mpv_commands, fmt.Sprintf("--sub-file=%s", ins.File))
							}
						}
					}
					callback := PlayMpv(mpv_commands)
					if callback == 1 {
						break server_loop
					} else {
						break
					}
				}
			}
		}
	}
}
