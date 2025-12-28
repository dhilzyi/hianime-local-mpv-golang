package player

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"hianime-mpv-go/config"
	"hianime-mpv-go/hianime"
	"hianime-mpv-go/jimaku"
	"hianime-mpv-go/state"
	"hianime-mpv-go/ui"
)

//go:embed track.lua
var TrackScript string

func BuildDesktopCommands(metaData hianime.SeriesData, episodeData hianime.Episodes, serverData hianime.ServerList, streamingData hianime.StreamData, historyData state.History, configData config.Settings) []string {
	// Building title display for mpv
	displayTitle := fmt.Sprintf("%s [Ep. %d] %s (%s)", metaData.JapaneseName, episodeData.Number, episodeData.JapaneseTitle, serverData.Name)

	// Making headers
	headerFields := []string{
		fmt.Sprintf("Referer: %s", streamingData.Referer),
		fmt.Sprintf("User-Agent: %s", streamingData.UserAgent),
		fmt.Sprintf("Origin: %s", "https://megacloud.blog"),
	}
	fullHeaders := strings.Join(headerFields, ",")

	// Main commands
	args := []string{
		streamingData.Url,
		"--ytdl-format=bestvideo+bestaudio/best",
		fmt.Sprintf("--http-header-fields=%s", fullHeaders),
		fmt.Sprintf("--title=%s", displayTitle),
		"--script-opts=osc-title=${title}",
	}

	// last position if exist in history
	episodeProgress, exist := historyData.Episode[episodeData.Number]
	if exist {
		args = append(args, fmt.Sprintf("--start=%f", episodeProgress.Position))
	}

	// Chapter command
	if streamingData.Intro.End > 0 || streamingData.Outro.Start > 0 {
		chapter_pathfile := CreateChapters(streamingData, historyData, episodeData)
		if chapter_pathfile != "" {
			fmt.Println("--> Adding chapters to mpv.")
			args = append(args, fmt.Sprintf("--chapters-file=%s", chapter_pathfile))
		}
	} else {
		fmt.Println("--> Intro & Outro doesn't found. Skip creating chapters.")
	}

	// Jimaku subtitle command
	if configData.JimakuEnable {
		jimakuList, err := jimaku.GetSubsJimaku(metaData, episodeData.Number)
		if err != nil {
			fmt.Printf("Failed to get subs from jimaku: '%s'\n", err)
			fmt.Printf("Skipping Jimaku\n")
		} else {
			if len(jimakuList) > 0 {
				for i := range jimakuList {
					args = append(args, fmt.Sprintf("--sub-file=%s", jimakuList[i]))
				}
			}
		}
	} else {
		fmt.Printf("--> Skipping Jimaku\n")
	}

	// Subs from hianime
	for _, track := range streamingData.Tracks {
		if track.Kind == "thumbnails" {
			continue
		}
		if configData.EnglishOnly && track.Label != "English" {
			continue
		}

		args = append(args, fmt.Sprintf("--sub-file=%s", track.File))
	}

	// Sub delay history command
	if historyData.SubDelay != 0 {
		fmt.Println("--> Adding sub-delay from history...")
		args = append(args, fmt.Sprintf("--sub-delay=%.1f", historyData.SubDelay))
	}

	// track script & debug command
	scriptLua, err := EnsureTrackScript("track.lua")
	if err == nil {
		args = append(args, "--script="+scriptLua)
	}

	if config.DebugMode {
		args = append(args, "--v")
	}

	return args
}

// NOTE: For intro and outro in mpv so user can know the timestamps and skip easily.
func CreateChapters(data hianime.StreamData, historyData state.History, episodeData hianime.Episodes) string {

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

	if data.Intro.Start > 0 || data.Intro.End > 0 {
		if data.Intro.Start == 0 {
			writePart(data.Intro.Start, data.Intro.End, "Intro")
		} else {
			writePart(0, data.Intro.Start, "Part A")
			writePart(data.Intro.Start, data.Intro.End, "Intro")
		}
	}

	if data.Outro.Start > 0 && data.Outro.End > 0 {
		writePart(data.Intro.End, data.Outro.Start, "Part B")
		writePart(data.Outro.Start, data.Outro.End, "Outro")
	}

	// Using exact duration from history if exist
	episodeProgress, exist := historyData.Episode[episodeData.Number]
	if exist {
		ui.DebugPrint("[CHAPTER]", "History duration exist")
		writePart(data.Outro.End, int(episodeProgress.Duration), "Part C")
	} else {
		ui.DebugPrint("[CHAPTER]", "History duration not exist")
		writePart(data.Outro.End, 9999999, "Part C")
	}

	f.WriteString(contents)
	return f.Name()
}

// Now it supports windows and linux automatically, without hardcoding the mpv path. I hope
func PlayMpv(cmdMain string, args []string) (bool, float64, float64, float64) {
	cmdName := cmdMain

	var stream_started bool
	var subDelay float64
	var lastPos float64
	var totalDuration float64

	cmd := exec.Command(cmdName, args...)

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
		stream_started = false
	})

	var flag bool
	for scanner.Scan() {
		line := scanner.Text()

		ui.DebugPrint("[MPV]", line)

		if strings.Contains(line, "(+) Video --vid= ") || strings.Contains(line, "h264") {
			timer.Stop()
			if !flag {
				fmt.Println("\nStream is valid. Opening mpv")
				flag = true
			}

			stream_started = true
		} else if strings.Contains(line, "Opening failed") || strings.Contains(line, "HTTP error") {
			fmt.Println("Failed to stream. Potentially dead link...")

			stream_started = false
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
					total, err := strconv.ParseFloat(strings.TrimSpace(totalStr), 64)
					if err != nil {
						fmt.Println("Error while converting to float: " + err.Error())
					}

					lastPos = current
					totalDuration = total
				}
			}

			continue

		} else if strings.Contains(line, "::SUB_DELAY::") {
			parts := strings.Split(line, "::SUB_DELAY::")

			floatDelay, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil {
				fmt.Printf("Error while converting to float: %v\n", err)
			}

			subDelay = floatDelay

			continue
		}
	}

	if err := cmd.Wait(); err != nil {
	}

	return stream_started, subDelay, lastPos, totalDuration
}

func GetMpvBinary(configPath string) string {
	if configPath != "" {
		return configPath
	}
	if runtime.GOOS == "windows" {
		return "mpv.exe"
	}

	if runtime.GOOS == "linux" {
		if isWSL() {
			return "mpv.exe"
		}
		return "mpv"
	}

	return "mpv"
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "microsoft") || strings.Contains(content, "wsl")
}

func EnsureTrackScript(pathFile string) (string, error) {
	dir := "scripts"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("Failed to create series directory: %w", err)
	}

	scriptPath := filepath.Join(dir, pathFile)
	if _, err := os.Stat(scriptPath); err == nil {
		fmt.Println("--> Lua script exist")

	} else if os.IsNotExist(err) {

		errA := os.WriteFile(scriptPath, []byte(TrackScript), 0644)
		if errA != nil {
			return "", fmt.Errorf("Failed to write script :%w", errA)
		}

	} else {
		return "", fmt.Errorf("Error accessing path %s: %w\n", pathFile, err)
	}

	return scriptPath, nil
}
