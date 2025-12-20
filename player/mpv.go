package player

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"hianime-mpv-go/hianime"
	"hianime-mpv-go/jimaku"
	"hianime-mpv-go/state"
)

func BuildDesktopCommands(metaData hianime.SeriesData, episodeData hianime.Episodes, serverData hianime.ServerList, streamingData hianime.StreamData, historyData state.History) []string {
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

	lastPosition, exist := historyData.Episode[episodeData.Number]
	if exist {
		args = append(args, fmt.Sprintf("--start=%f", lastPosition.Position-1))
	}

	// Chapter command
	if streamingData.Intro.End > 0 && streamingData.Outro.Start > 0 {
		chapter_filename := CreateChapters(streamingData)
		if chapter_filename != "" {
			fmt.Println("--> Adding chapters to mpv.")
			args = append(args, fmt.Sprintf("--chapters-file=%s", chapter_filename))
		}
	} else {
		fmt.Println("--> Intro & Outro doesn't found. Skip creating chapters.")
	}

	// Jimaku subtitle command
	jimakuList, err := jimaku.GetSubsJimaku(metaData, episodeData.Number)
	if err != nil {
		fmt.Printf("Failed to get subs from jimaku: '%s'\n", err)
		fmt.Printf("Skipping Jimaku\n")
	}
	if len(jimakuList) > 0 {
		for i := range jimakuList {
			args = append(args, fmt.Sprintf("--sub-file=%s", jimakuList[i]))
		}
	}

	// Subs from hianime
	if streamingData.Tracks[0].File != "" {
		for i := range streamingData.Tracks {
			ins := streamingData.Tracks[i]
			if !strings.Contains(ins.File, "thumbnail") {
				args = append(args, fmt.Sprintf("--sub-file=%s", ins.File))
			}
		}
	}

	// Sub delay history command
	if historyData.SubDelay != 0 {
		fmt.Println("--> Adding sub-delay from history...")
		args = append(args, fmt.Sprintf("--sub-delay=%.1f", historyData.SubDelay))
	}

	return args
}

func BuildAndroidCommands(metaData hianime.SeriesData, episodeData hianime.Episodes, serverData hianime.ServerList, streamingData hianime.StreamData) []string {

	headerFields := []string{
		fmt.Sprintf("Referer: %s", streamingData.Referer),
		fmt.Sprintf("User-Agent: %s", streamingData.UserAgent),
		fmt.Sprintf("Origin: %s", "https://megacloud.blog"),
	}

	fullHeaders := strings.Join(headerFields, ",")
	mpvCommands := []string{
		"start",
		"--user",
		"0",
		"-a",
		"android.intent.action.VIEW",
		"-d",
		streamingData.Url,
		"-n",
		"is.xyz.mpv/.MPVActivity",
		"--es",
		fmt.Sprintf("--http-header-fields=%s", fullHeaders),
	}

	jimakuList, err := jimaku.GetSubsJimaku(metaData, episodeData.Number)
	if err != nil {
		fmt.Printf("Failed to get subs from jimaku: '%s'\n", err)
		fmt.Printf("Skipping Jimaku\n")
	}
	if len(jimakuList) > 0 {
		for i := range jimakuList {
			mpvCommands = append(mpvCommands, fmt.Sprintf("--sub-file=%s", jimakuList[i]))
		}
	}

	return mpvCommands
}

// NOTE: For intro and outro in mpv so user can know the timestamps and skip easily.
func CreateChapters(data hianime.StreamData) string {

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

func PlayAndroidMpv(mpvCommands []string) {
	cmdName := "am"

	cmd := exec.Command(cmdName, mpvCommands...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Failed to put stdout: " + err.Error())
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Error while running mpv: " + err.Error())
	}

	if err := cmd.Wait(); err != nil {
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}
}

// TODO: Support other platforms.
func PlayMpv(mpv_commands []string) (bool, float64, float64, float64) {
	cmdName := "mpv.exe"

	track_script := "player/track.lua"
	mpv_commands = append(mpv_commands, "--script="+track_script)

	var stream_started bool
	var subDelay float64
	var lastPos float64
	var totalDuration float64

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
		stream_started = false
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
