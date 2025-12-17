package hianime

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var BaseUrl string = "https://hianime.to"

// This is where the hianime scrapper logic lives. Check types.go in this same directory to see all the struct types.

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

	// series_html, err := doc.Html()
	// os.WriteFile("a.html", []byte(series_html), 0644)

	header := doc.Find("h2.film-name")

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

func GetIframe(server_id int) (StreamData, error) {
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

	return ExtractMegacloud(url)
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
