package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http/httputil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"log"
	"net/http"
	"net/url"
)

var BaseUrl string = "https://hianime.to"

type SeriesData struct {
	AnimeID string `json:"anime_id"`
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

func GetEpisodes(series_url string) []Episodes {
	resp, err := http.Get(series_url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	syncData := doc.Find("#syncData")

	rawJson := syncData.Text()
	var data SeriesData
	json.Unmarshal([]byte(rawJson), &data)

	api_url := fmt.Sprintf("%s/ajax/v2/episode/list/%s", BaseUrl, data.AnimeID)

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

	api_doc.Find("a").Each(func(i int, s *goquery.Selection) {
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

	return episodes

}

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
			fmt.Println("Couldn't found data-type: " + err.Error())
		}
		data_id, exists := s.Attr("data-id")
		if !exists {
			fmt.Println("Couldn't found data-id: " + err.Error())
		}
		int_data_id, err := strconv.Atoi(data_id)
		if err != nil {
			fmt.Println("Failed to convert data_id to int: " + err.Error())
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

func ExtractMegacloud(iframe_url string) {
	parsed_url, err := url.Parse(iframe_url)
	if err != nil {
		fmt.Println("Failed to parse url: " + err.Error())
	}
	default_domain := fmt.Sprintf("%s://%s/", parsed_url.Scheme, parsed_url.Host)
	user_agent := "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36"

	req, err := http.NewRequest("GET", iframe_url, nil)
	if err != nil {
		fmt.Println("Failed to fecth iframe link: " + err.Error())
		return
	}

	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Referer", default_domain)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Failed to request with custom headers: " + err.Error())
	}

	defer resp.Body.Close()

	doc_megacloud, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Println("Failed to create new document: " + err.Error())
	}
	megacloud_player := doc_megacloud.Find("#megacloud-player")
	file_id, exists := megacloud_player.Attr("data-id")
	if !exists {
		fmt.Println("Couldn't found 'file_id': " + err.Error())
	}
	re := regexp.MustCompile(`\b[a-zA-Z0-9]{48}\b`)

	singleSelect := doc_megacloud.Selection
	outerHtml, _ := goquery.OuterHtml(singleSelect)
	nonce := re.FindString(outerHtml)
	if nonce == "" {
		fmt.Println("Could not find nonce.")
	} else {
		fmt.Println(nonce)
	}

	sources_url := fmt.Sprintf("%sembed-2/v3/e-1/getSources?id=%s&_k=%s", default_domain, file_id, nonce)
	source_req, err := http.NewRequest("GET", sources_url, nil)
	fmt.Println(sources_url)

	extractor_headers := map[string]string{
		"Accept":           "*/*",
		"X-Requested-With": "application/json",
		"Referer":          iframe_url,
		"User-Agent":       user_agent,
	}
	for key, value := range extractor_headers {
		source_req.Header.Set(key, value)
	}

	reqDump, err := httputil.DumpRequestOut(source_req, true)
	if err != nil {
		fmt.Println("Error dumping request:", err)
	} else {
		fmt.Printf("REQUEST:\n%s\n", string(reqDump))
	}
	source_resp, err := client.Do(source_req)
	if err != nil {
		fmt.Println("Failed to fetch source url: " + err.Error())
	}
	defer source_resp.Body.Close()

	body, err := io.ReadAll(source_resp.Body)
	if err != nil {
		fmt.Println("Failed to read the body: " + err.Error())
	}
	//
	fmt.Println(string(body))
}
func main() {
	url := "https://hianime.to/planetes-210"
	scanner := bufio.NewScanner(os.Stdin)

	var cache_episodes []Episodes
episode_loop:
	for {
		if len(cache_episodes) > 0 {
			for i := range len(cache_episodes) {
				eps := cache_episodes[i]
				fmt.Printf(" [%d] %s ID: %d\n", eps.Number, eps.JapaneseTitle, eps.Id)
			}
		} else {
			cache_episodes = GetEpisodes(url)
			for i := range len(cache_episodes) {
				eps := cache_episodes[i]
				fmt.Printf(" [%d] %s ID: %d\n", eps.Number, eps.JapaneseTitle, eps.Id)
			}
		}

		fmt.Print("\nEnter number episode to watch (or q to go back): ")
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
		if int_input > 0 && int_input <= len(cache_episodes) {
			selected := cache_episodes[int_input-1]
			fmt.Printf("Episode : %d \nTitle: %s \nUrl: %s\n\n", selected.Number, selected.JapaneseTitle, selected.Url)
			servers = GetEpisodeServerId(selected.Id)
		} else {
			fmt.Println("Number is invalid.")
		}
	server_loop:
		for {
			fmt.Print("\n--- Available Servers ---\n")
			for i := range len(servers) {
				ins := servers[i]

				if ins.Type == "dub" {
					fmt.Printf(" [%d] %s (Dub)\n", i+1, ins.Name)
				} else {
					fmt.Printf(" [%d] %s\n", i+1, ins.Name)
				}
			}
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
			if int_server_input > 0 && int_server_input <= len(cache_episodes) {
				selected := servers[int_server_input-1]
				iframe_link := GetIframe(selected.DataId)
				ExtractMegacloud(iframe_link)
			} else {
				fmt.Println("Number is invalid.")
			}
		}
	}

}
