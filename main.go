package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"hianime-mpv-go/config"
	"hianime-mpv-go/jimaku"
	// "hianime-mpv-go/player"
	"hianime-mpv-go/state"

	"log"
	"net/http"
	"net/url"
)

var BaseUrl string = "https://hianime.to"

type SeriesData struct {
	AnimeID      string `json:"anime_id"`
	EnglishName  string `json:"name"`
	AnilistID    string `json:"anilist_id"`
	SeriesUrl    string `json:"series_url"`
	JapaneseName string
}

type Episodes struct {
	Number        int
	EnglishTitle  string
	JapaneseTitle string
	Url           string
	Id            int
}

type ServerList struct {
	Type   string
	Name   string
	DataId int
	Id     int
}
type AjaxResponse struct {
	Status bool   `json:"status"`
	Html   string `json:"html"`
}

type MegacloudUrl struct {
	Type string `json:"type"`
	Url  string `json:"link"`
}

type Sources struct {
	Sources   []Source  `json:"sources"`
	Tracks    []Track   `json:"tracks"`
	Encrypted bool      `json:"encrypted"`
	Intro     Timestamp `json:"intro"`
	Outro     Timestamp `json:"outro"`
	Server    int       `json:"server"`
}
type Source struct {
	File string `json:"file"`
	Type string `json:"type"`
}

type Track struct {
	File    string `json:"file"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Default bool   `json:"default"`
}

type Timestamp struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type StreamData struct {
	Url       string
	UserAgent string
	Referer   string
	Origin    string
	Tracks    []Track
	Intro     Timestamp
	Outro     Timestamp
}

func GetSeriesData(series_url string) SeriesData {
	resp, err := http.Get(series_url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	series_html, err := doc.Html()
	os.WriteFile("a.html", []byte(series_html), 0644)

	header := doc.Find("h2.film-name")

	// Check parent first, IF NOT found, check child ("a")
	jname, exists := header.Attr("data-jname")
	if !exists {
		jname, exists = header.Find("a").Attr("data-jname")
	}

	syncData := doc.Find("#syncData")

	rawJson := syncData.Text()
	var data SeriesData
	json.Unmarshal([]byte(rawJson), &data)

	data.EnglishName = header.Text()
	data.JapaneseName = jname

	return data
}

func GetEpisodes(animeId string) []Episodes {
	api_url := fmt.Sprintf("%s/ajax/v2/episode/list/%s", BaseUrl, animeId)

	api_resp, err := http.Get(api_url)
	if err != nil {
		log.Fatal(err)
	}

	defer api_resp.Body.Close()
	var json_resp AjaxResponse
	if err := json.NewDecoder(api_resp.Body).Decode(&json_resp); err != nil {
		panic("Failed to decode JSON: " + err.Error())
	}

	api_doc, err := goquery.NewDocumentFromReader(strings.NewReader(json_resp.Html))
	if err != nil {
		log.Fatal(err)
	}

	var episodes []Episodes

	api_doc.Find("a.ep-item").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			fmt.Println("Couldn't found href.")
		}

		data_id, exists := s.Attr("data-id")
		if !exists {
			fmt.Println("Couldn't found data-id.")
		}

		id_int, err := strconv.Atoi(data_id)
		if err != nil {
			fmt.Print("Failed to convert to integer: " + err.Error())
		}

		titleDiv := s.Find(".ep-name")
		englishTitle := html.UnescapeString(titleDiv.Text())

		japaneseTitle := ""
		rawJName, exists := titleDiv.Attr("data-jname")
		if exists {
			japaneseTitle = html.UnescapeString(rawJName)
		}

		data_structure := Episodes{
			Number:        i + 1,
			EnglishTitle:  englishTitle,
			JapaneseTitle: japaneseTitle,
			Url:           BaseUrl + html.UnescapeString(href),
			Id:            id_int,
		}
		episodes = append(episodes, data_structure)
	})

	// api_html, err := api_doc.Html()
	//
	// os.WriteFile("onepiece.html", []byte(api_html), 0644)

	return episodes
}

//TODO: Refactor all function into modular.

func GetEpisodeServerId(episode_id int) []ServerList {
	servers_url := fmt.Sprintf("%s/ajax/v2/episode/servers?episodeId=%d", BaseUrl, episode_id)

	server_resp, err := http.Get(servers_url)
	if err != nil {
		fmt.Println("Error while requesting server urls: " + err.Error())
	}
	defer server_resp.Body.Close()

	var server_json AjaxResponse
	if err := json.NewDecoder(server_resp.Body).Decode(&server_json); err != nil {
		fmt.Println("Error while converting to json: " + err.Error())
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(server_json.Html))
	if err != nil {
		fmt.Println("Failed fecthing json to doc: ", err.Error())
	}

	var servers_list []ServerList

	doc.Find(".server-item").Each(func(i int, s *goquery.Selection) {
		data_type, exists := s.Attr("data-type")
		if !exists {
			fmt.Println("Couldn't found 'data-type': " + err.Error())
		}
		data_id, exists := s.Attr("data-id")
		if !exists {
			fmt.Println("Couldn't found data-id: " + err.Error())
		}
		int_data_id, err := strconv.Atoi(data_id)
		if err != nil {
			fmt.Println("Failed to convert 'data_id' to int: " + err.Error())
		}

		name := s.Find("a").Text()
		instance := ServerList{
			Type:   data_type,
			Name:   name,
			DataId: int_data_id,
		}

		servers_list = append(servers_list, instance)
	})

	return servers_list
}

func GetIframe(server_id int) string {
	server_url := fmt.Sprintf("%s/ajax/v2/episode/sources?id=%d", BaseUrl, server_id)

	resp, err := http.Get(server_url)
	if err != nil {
		fmt.Println("Failed to connect with server url: " + err.Error())
	}
	defer resp.Body.Close()

	var resp_json MegacloudUrl
	if err := json.NewDecoder(resp.Body).Decode(&resp_json); err != nil {
		fmt.Println("Failed to decode JSON: " + err.Error())
	}

	var url string
	if resp_json.Type == "iframe" {
		url = resp_json.Url
	}

	return url
}

func GetNonce(html string) string {
	reStandard := regexp.MustCompile(`\b[a-zA-Z0-9]{48}\b`)
	nonce := reStandard.FindString(html)
	if nonce != "" {
		return nonce
	}

	reSplit := regexp.MustCompile(`x:\s*"(\w+)",\s*y:\s*"(\w+)",\s*z:\s*"(\w+)"`)
	matches := reSplit.FindStringSubmatch(html)

	if len(matches) == 4 {
		return matches[1] + matches[2] + matches[3]
	}

	return ""
}

func ExtractMegacloud(iframe_url string) (StreamData, error) {
	parsed_url, err := url.Parse(iframe_url)
	if err != nil {
		return StreamData{}, fmt.Errorf("Failed to parse url: %w", err)
	}
	default_domain := fmt.Sprintf("%s://%s/", parsed_url.Scheme, parsed_url.Host)
	user_agent := "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36"

	req, err := http.NewRequest("GET", iframe_url, nil)
	if err != nil {
		return StreamData{}, fmt.Errorf("Failed to fecth iframe link: %w", err)
	}

	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Referer", default_domain)

	client := &http.Client{}

	max_attempt := 3
	var file_id string
	var nonce string

	for i := range max_attempt {
		fmt.Printf("--> Attempt %d/%d to extract...\n", i+1, max_attempt)

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Failed to request with custom headers: " + err.Error())
			continue
		}

		defer resp.Body.Close()

		doc_megacloud, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			fmt.Println("Failed to create new document: " + err.Error())
			continue
		}
		megacloud_player := doc_megacloud.Find("#megacloud-player")
		id, exists := megacloud_player.Attr("data-id")
		if !exists {
			fmt.Println("Couldn't found 'file_id'.")
			continue
		} else {
			file_id = id
		}

		singleSelect := doc_megacloud.Selection
		outerHtml, _ := goquery.OuterHtml(singleSelect)

		nonce = GetNonce(outerHtml)
		if nonce == "" {
			fmt.Println("Could not find nonce.")
			time.Sleep(1 * time.Second)
			continue
		} else {
			fmt.Println("\n--> Extract success.")
			break
		}
	}

	sources_url := fmt.Sprintf("%sembed-2/v3/e-1/getSources?id=%s&_k=%s", default_domain, file_id, nonce)
	source_req, err := http.NewRequest("GET", sources_url, nil)
	if err != nil {
		return StreamData{}, fmt.Errorf("Failed when requesting source url: %w", err)
	}

	extractor_headers := map[string]string{
		"Accept":           "*/*",
		"X-Requested-With": "application/json",
		"Referer":          iframe_url,
		"User-Agent":       user_agent,
	}
	for key, value := range extractor_headers {
		source_req.Header.Set(key, value)
	}

	source_resp, err := client.Do(source_req)
	if err != nil {
		return StreamData{}, fmt.Errorf("Failed to fetch source url: %w", err)
	}
	defer source_resp.Body.Close()

	var source_json Sources

	// doc, err := goquery.NewDocumentFromReader(source_resp.Body)
	// fmt.Println(doc.Text())

	if err := json.NewDecoder(source_resp.Body).Decode(&source_json); err != nil {
		return StreamData{}, fmt.Errorf("Failed to convert to JSON: %w", err)
	}

	map_struct := StreamData{}

	//  NOTE: Still can't play server 'HD-3' (url=douvid.xyz), because it was returning EXT encrypted, and impossible for mpv to play.
	if !source_json.Encrypted || strings.Contains(source_json.Sources[0].File, ".m3u8") {
		map_struct = StreamData{
			Url:       source_json.Sources[0].File,
			UserAgent: user_agent,
			Referer:   default_domain,
			Origin:    default_domain,
			Tracks:    source_json.Tracks,
			Intro:     source_json.Intro,
			Outro:     source_json.Outro,
		}
	} else {
		return StreamData{}, fmt.Errorf("Files are encrypted. Try other servers.")
	}

	return map_struct, nil
}

// NOTE: For intro and outro in mpv so user can know the timestamps.
func CreateChapters(data StreamData) string {
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
		var series_metadata SeriesData

		if strings.Contains(series_input, "hianime.to") {
			url = series_input
			series_metadata = GetSeriesData(url)
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

			series_metadata = GetSeriesData(url)

			history = state.UpdateHistory(history, history_select)
			state.SaveHistory(history)
		}

		var cache_episodes []Episodes

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
				cache_episodes = GetEpisodes(series_metadata.AnimeID)
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

			var servers []ServerList
			var selected_ep Episodes
			if int_input > 0 && int_input <= len(cache_episodes) {
				selected_ep = cache_episodes[int_input-1]
				servers = GetEpisodeServerId(selected_ep.Id)

				history_select.LastEpisode = int_input

				history = state.UpdateHistory(history, history_select)
				state.SaveHistory(history)
			} else {
				fmt.Println("Number is invalid.")
			}
		server_loop:
			for {
				if !(len(servers) > 0) {
					fmt.Println("\nNo available servers found.")
					break
				}

				var selected_server ServerList
				var stream_data StreamData

				if config_session.AutoSelectServer {
					fmt.Println("\n--> Auto-select server enabled.")
					for i := range servers {
						selected_server = servers[i]
						fmt.Printf("--> Selecting '%s'....\n", selected_server.Name)

						iframe_link := GetIframe(selected_server.DataId)
						attempt, err := ExtractMegacloud(iframe_link)
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
						iframe_link := GetIframe(selected_server.DataId)
						attempt, err := ExtractMegacloud(iframe_link)
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

					jimaku_list, err := jimaku.GetSubsJimaku(series_metadata.AnilistID, selected_ep.Number)
					if err != nil {
						fmt.Printf("Failed to get subs from jimaku: %s", err)
					}

					if len(jimaku_list) > 0 {
						for i := range jimaku_list {
							mpv_commands = append(mpv_commands, fmt.Sprintf("--sub-file=%s", jimaku_list[i]))
						}
					}

					if stream_data.Tracks[0].File != "" {
						for i := range stream_data.Tracks {
							ins := stream_data.Tracks[i]
							mpv_commands = append(mpv_commands, fmt.Sprintf("--sub-file=%s", ins.File))
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
