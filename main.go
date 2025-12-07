package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"html"
	"strings"

	"log"
	"net/http"
)

var BaseUrl string = "https://hianime.to"

type SeriesData struct {
	Page             string `json:"page"`
	Name             string `json:"name"`
	AnimeID          string `json:"anime_id"`
	MalID            string `json:"mal_id"`
	AnilistID        string `json:"anilist_id"`
	SeriesURL        string `json:"series_url"`
	SelectorPosition string `json:"selector_position"`
}

type Episodes struct {
	Number        int
	EnglishTitle  string
	JapaneseTitle string
	Url           string
}

type AjaxResponse struct {
	Status bool   `json:"status"`
	Html   string `json:"html"`
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

		titleDiv := s.Find(".ep-name")
		englishTitle := html.UnescapeString(titleDiv.Text())

		japaneseTitle := ""
		rawJName, exists := titleDiv.Attr("data-jname")
		if exists {
			japaneseTitle = html.UnescapeString(rawJName)
		}

		data_structure := Episodes{
			Number:        i,
			EnglishTitle:  englishTitle,
			JapaneseTitle: japaneseTitle,
			Url:           BaseUrl + html.UnescapeString(href),
		}
		episodes = append(episodes, data_structure)
	})

	for i := 1; i < len(episodes); i++ {
		ep := episodes[i]
		fmt.Printf("Episode %d \nJp Title: %s \nEn Title: %s \n%s\n", ep.Number, ep.JapaneseTitle, ep.EnglishTitle, ep.Url)

	}
	return episodes

}
func GetSeriesMetaData(series_url string) {
	resp, err := http.Get(series_url)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatal(err)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(*doc.Find("h2.film-name a.dynamic-name"))

	// metadata := (string "test")
	// return metadata
}

// func ExtractMegacloud(iframe string) map[string]string {
//
// }
func main() {
	url := "https://hianime.to/watari-kuns-is-about-to-collapse-19769"

	GetEpisodes(url)

}
