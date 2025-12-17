package jimaku

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"hianime-mpv-go/hianime"
)

type Search []SearchElement
type SearchElement struct {
	ID         int64  `json:"id"`
	AnilistID  int64  `json:"anilist_id"`
	RomajiName string `json:"name"`
}

type Files []FileElement
type FileElement struct {
	Name string `json:"name"`
	Url  string `json:"url"`
	Size int64  `json:"size"`
}

var UserAgent = ""
var JimakuBaseUrl string = "https://jimaku.cc"

// Set your JimakuAPI to environment table or just put it directly in this variable as a string.
var JimakuApi string = os.Getenv("JIMAKU_API_KEY") // or "xxxxxxxxx"

func downloadFile(url string, file_path string) (string, error) {
	cleanPath := strings.TrimRight(file_path, ".")
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create dir: %w", err)
	}

	out, err := os.Create(cleanPath)
	if err != nil {
		return "", fmt.Errorf("Failed to create file %s: %w", cleanPath, err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("Couldn't fetch the following url: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error while copying the data to the file: %w", err)
	}

	return cleanPath, nil
}

func getFiles(entry_id, episodeNum int) (Files, error) {
	url_files := fmt.Sprintf("%s/api/entries/%d/files", JimakuBaseUrl, entry_id)

	req, err := http.NewRequest("GET", url_files, nil)
	if err != nil {
		return Files{}, fmt.Errorf("Failed fetching entry id: %w", err)
	}

	query := req.URL.Query()
	query.Add("episode", fmt.Sprintf("%d", episodeNum))

	req.URL.RawQuery = query.Encode()

	fmt.Println(req.URL.RawPath)

	req.Header.Add("Authorization", JimakuApi)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Files{}, fmt.Errorf("Failed to request entry id: %w", err)
	}

	defer res.Body.Close()

	var subs_files Files

	if err = json.NewDecoder(res.Body).Decode(&subs_files); err != nil {
		return Files{}, fmt.Errorf("Failed convert subs files to JSON: %w", err)
	}

	// for i := range subs_files {
	// 	ins := subs_files[i]
	// 	sizeMB := float64(ins.Size) / (1024 * 1024)
	// 	fmt.Printf("Name: %s\nUrl: %s\nSize: %.2f Mb\n\n", ins.Name, ins.Url, sizeMB)
	// }

	return subs_files, nil

}

func GetSubsJimaku(series_data hianime.SeriesData, episodeNum int) ([]string, error) {
	if JimakuApi == "" {
		return []string{}, fmt.Errorf("No Jimaku API found in the enviroment variable.")
	}
	fmt.Println("\n--> JimakuApiKey found. Querying into the Jimaku api....")

	url_search := fmt.Sprintf("%s/api/entries/search", JimakuBaseUrl)

	req, err := http.NewRequest("GET", url_search, nil)
	if err != nil {
		return []string{}, fmt.Errorf("Failed when parsing url: %w", err)
	}
	req.Header.Add("Authorization", JimakuApi)

	query := req.URL.Query()
	query.Add("anime", "true")

	if series_data.AnilistID != "" {
		query.Add("anilist_id", series_data.AnilistID)
	} else {
		fmt.Println("--> AnilistID not found. Processing with query method.")
		query.Add("query", series_data.JapaneseName)
	}

	req.URL.RawQuery = query.Encode()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return []string{}, fmt.Errorf("Failed to request query: %w", err)
	}

	defer res.Body.Close()

	var data Search
	if res.StatusCode != http.StatusOK {
		return []string{}, fmt.Errorf("Bad status when querying: %s", res.Status)
	}

	if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
		return []string{}, fmt.Errorf("Failed to decode to JSON: %w", err)
	}

	if data[0].ID < 0 {
		return []string{}, fmt.Errorf("Nothing found.")
	}

	fmt.Println("--> Jimaku id for series found. Requesting files list...")
	files_list, err := getFiles(int(data[0].ID), episodeNum)
	if err != nil {
		return []string{}, fmt.Errorf("Failed when getting files: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}

	default_path := filepath.Join(homeDir, "subtitle")
	re := regexp.MustCompile(`[<>:"/\\|?*\.]`)
	cleanName := re.ReplaceAllString(data[0].RomajiName, "")

	series_dir := filepath.Join(default_path, cleanName)

	if err := os.MkdirAll(series_dir, 0755); err != nil {
		return []string{}, fmt.Errorf("Failed to create series directory: %w", err)
	}

	var name_list []string
	fmt.Println("--> Files list received. Downloading subtitle....")

	for i := range files_list {
		ins := files_list[i]

		// TODO : Handle zip, 7z, rar formats

		ext := strings.ToLower(path.Ext(ins.Url))
		if ext != ".srt" && ext != ".ass" {
			fmt.Printf("Skipping unsupported format: %s (extension %s)\n", ins.Url, ext)
			continue
		}

		rawFilename := path.Base(ins.Url)

		filename, err := url.QueryUnescape(rawFilename)
		if err != nil {
			filename = rawFilename
		}

		fullPath := filepath.Join(series_dir, filename)

		if _, err := os.Stat(fullPath); err == nil {
			fmt.Printf("	File already exists, skipping: %s\n", fullPath)
			name_list = append(name_list, fullPath)
			continue
		} else if os.IsNotExist(err) {
			fmt.Printf("	Downloading: %s\n", filename)
		} else {
			fmt.Printf("Error accessing path %s: %v\n", fullPath, err)
		}

		downloadedPath, err := downloadFile(ins.Url, fullPath)
		if err != nil {
			fmt.Printf("Failed to download %s (index %d) file", ins.Url, i)
			continue
		}

		name_list = append(name_list, downloadedPath)
	}

	if len(name_list) == 0 {
		return name_list, fmt.Errorf("Failed to retrieve.")
	}

	return name_list, nil
}
