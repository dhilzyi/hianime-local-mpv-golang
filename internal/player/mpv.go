package player

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/dhilzyi/hianime-cli/internal/config"
	"github.com/dhilzyi/hianime-cli/internal/core"
	"github.com/dhilzyi/hianime-cli/internal/jimaku"
	"github.com/dhilzyi/hianime-cli/internal/state"
	"github.com/dhilzyi/hianime-cli/internal/ui"
)

//go:embed track.lua
var trackScript string
var ScriptName string = "track.lua"

func BuildMpvCommands(
	cfg config.Config,
	metaData core.SeriesData,
	episodeData core.Episode,
	serverData core.Server,
	streamingData core.StreamData,
	historyData state.History,
	datadir string,
	verbose bool,
	msgCh chan string,
) []string {
	// Building title display for mpv
	displayTitle := fmt.Sprintf("[Ep. %d] %s (%s)", episodeData.Number, episodeData.Titles.GetPreferredTitle(), serverData.Name)

	// Building headers if provided by providers
	headerFields := []string{}
	for k, v := range streamingData.Headers {
		headerFields = append(headerFields, fmt.Sprintf("%s: %s", k, v))
	}

	// Building basic arguments
	args := []string{
		streamingData.Url,
		"--http-header-fields=" + strings.Join(headerFields, ","),
		"--title=" + displayTitle,
		"--script-opts-append=osc-title=${title}",
		"--ytdl=no",
		"--hwdec=auto",
		"--profile=fast",

		"--demuxer-lavf-o=allowed_extensions=ALL",
		"--demuxer-max-bytes=150MiB",
	}

	// Chapter command
	if len(streamingData.Chapters) > 0 {
		chapterFile := createChapters(streamingData.Chapters, episodeData)
		fmt.Println("--> Info: Adding episode chapters.")
		args = append(args, fmt.Sprintf("--chapters-file=%s", chapterFile))
	}

	// Building jimaku subtitles
	if cfg.JimakuEnable {
		jimakuList, err := jimaku.GetSubsJimaku(&metaData, episodeData.Number)
		if err != nil {
			fmt.Printf("Warning: Failed to get subs from jimaku: '%v'\n", err)
		} else {
			for i := range jimakuList {
				args = append(args, fmt.Sprintf("--sub-file=%s", jimakuList[i]))
			}
		}
	} else {
		fmt.Printf("--> Warning: skipping jimaku\n")
	}

	// Use last position from history if exist
	episodeProgress, exist := historyData.Episodes[episodeData.Number]
	if exist && episodeProgress.Position <= episodeProgress.Duration {
		fmt.Println("--> Info: Load last position where you have left")
		args = append(args, fmt.Sprintf("--start=%f", episodeProgress.Position))
	}

	// Building subs from providers
	subsProvided := buildProvidedSubs(cfg, streamingData.Tracks)
	if len(subsProvided) > 0 {
		fmt.Printf("--> Info: Adding '%d' subtitle from site.\n", len(subsProvided))
		args = append(args, subsProvided...)
	}

	// Sub delay history command
	if historyData.SubDelay != 0 {
		fmt.Println("--> Info: Adding sub-delay from history...")
		args = append(args, fmt.Sprintf("--sub-delay=%.1f", historyData.SubDelay))
	}

	// track script & debug command
	scriptLua, err := ensureTrackScript(datadir)
	if err == nil {
		args = append(args, "--scripts-append="+scriptLua)
	} else {
		fmt.Println("Warning: failed to include lua script")
	}

	if verbose {
		args = append(args, "--v")
	}
	return args
}

// Now it supports windows and linux automatically, without hardcoding the mpv path. I hope
func PlayMpv(cmdMain string, args []string, verbose bool) (bool, float64, float64, float64) {
	cmdName := cmdMain

	var streamStarted bool
	var subDelay float64
	var lastPos float64
	var totalDuration float64

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cmd := exec.CommandContext(ctx, cmdName, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Failed to put stdout: " + err.Error())
	}

	fmt.Println("\n--> Executing mpv commands...")

	if err := cmd.Start(); err != nil {
		fmt.Println("Error while running mpv: " + err.Error())
	}

	scanner := bufio.NewScanner(stdout)
	done := make(chan struct{})
	timer := time.AfterFunc(60*time.Second, func() {
		fmt.Println("\n--> MPV is timeout. Killing process...")
		if cmd.Process != nil {
			streamStarted = false
			cmd.Process.Kill()
		}
	})

	var flag bool
	for scanner.Scan() {
		line := scanner.Text()

		if verbose {
			ui.DebugPrint("[MPV]", line)
		}

		if strings.Contains(line, "Video  --vid=") {
			timer.Stop()
			if !flag {
				fmt.Println("\nStream is valid. Opening mpv")
				flag = true
			}

			streamStarted = true
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
		return false, subDelay, lastPos, totalDuration
	}

	close(done)
	timer.Stop()

	return streamStarted, subDelay, lastPos, totalDuration
}
