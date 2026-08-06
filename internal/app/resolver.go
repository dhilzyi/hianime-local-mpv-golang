package app

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/dhilzyi/hianime-cli/internal/anilist"
	"github.com/dhilzyi/hianime-cli/internal/core"
	"github.com/dhilzyi/hianime-cli/internal/state"
	"github.com/dhilzyi/hianime-cli/sources/anikoto"
	"github.com/dhilzyi/hianime-cli/sources/animenosub"
	"github.com/dhilzyi/hianime-cli/sources/reanime"
)

type FetchResult struct {
	Provider   core.Provider
	SeriesData core.SeriesData
	Episodes   []core.Episode
}

type ResolveParams struct {
	URL       string
	AnilistID int
}

type ProviderType int

const (
	UnknownProvider ProviderType = iota
	AnimeNoSub
	Kuudere
	Reanime
	Anikoto
)

type InputType int

const (
	InputURL InputType = iota
	InputHistoryIndex
	InputCommand
	InputAnilistID
)

func getProvider(p ResolveParams) (core.Provider, ProviderType) {
	if strings.Contains(p.URL, "animenosub") {
		return animenosub.New(p.URL), AnimeNoSub
	} else if strings.Contains(p.URL, "reanime") {
		r, err := reanime.New(p.URL)
		if err != nil {
			return nil, UnknownProvider
		}
		return r, Reanime
	} else if strings.Contains(p.URL, "anikoto") {
		a, err := anikoto.New(p.URL)
		if err != nil {
			return nil, UnknownProvider
		}
		return a, Anikoto
	}
	return nil, UnknownProvider
}

func classifyInput(input string) (InputType, int) {
	if input == "q" {
		return InputCommand, 0
	}

	if i, err := strconv.Atoi(input); err == nil {
		if i >= 21 {
			return InputAnilistID, i
		} else {
			return InputHistoryIndex, i
		}
	}

	return InputURL, 0
}

func handleURLInput(p ResolveParams, provider core.Provider) ([]core.Episode, core.SeriesData, error) {
	episodes, series, err := provider.GetEpisodes()
	if err != nil {
		return nil, core.SeriesData{}, err
	}

	return episodes, *series, nil
}

func fromCache(entry *CacheEntry, p ResolveParams) *FetchResult {
	provider, _ := getProvider(p)
	return &FetchResult{
		Provider:   provider,
		SeriesData: entry.SeriesData,
		Episodes:   entry.Episodes,
	}
}

func ResolveInput(p ResolveParams, cache *Cache) (*FetchResult, error) {
	if p.AnilistID != 0 {
		if entry, ok := cache.byAnilistID[p.AnilistID]; ok {
			fmt.Println("Info: cache hit by anilistId")
			return fromCache(entry, p), nil
		}
	}

	provider, providerType := getProvider(p)
	if provider == nil || providerType == UnknownProvider {
		return nil, fmt.Errorf("provider is not found")
	} else if providerType == Kuudere {
		return nil, fmt.Errorf("kuudere is deprecated")
	}

	providerID, err := provider.ExtractProviderID()
	if err != nil {
		return nil, err
	}
	if entry, ok := cache.byProviderID[providerID]; ok {
		fmt.Println("Info: cache hit by providerID")

		return fromCache(entry, p), nil
	}

	episodes, series, err := handleURLInput(p, provider)
	if err != nil {
		return nil, err
	}
	if len(episodes) < 1 {
		return nil, fmt.Errorf("no episodes data is found")
	}

	entry := &CacheEntry{
		SeriesData: series,
		Episodes:   episodes,
	}

	if providerID != "" {
		cache.byProviderID[providerID] = entry
	} else if series.AnilistID != 0 {
		cache.byAnilistID[series.AnilistID] = entry
	} else if p.AnilistID != 0 {
		cache.byAnilistID[p.AnilistID] = entry
	}

	return &FetchResult{
		Provider:   provider,
		SeriesData: series,
		Episodes:   episodes,
	}, nil
}

func getHistoryByIndex(history []state.History, index int) (*state.History, error) {
	if index <= 0 || index > len(history) {
		return &state.History{}, fmt.Errorf("invalid index")
	}
	return &history[index-1], nil
}

func findOrCreateHistory(histories []state.History, seriesdata core.SeriesData, providerName string) (*state.History, error) {
	for i := range histories {
		hist := &histories[i]

		if hist.Provider != providerName {
			continue
		}

		if seriesdata.AnilistID != 0 && hist.Metadata.AnilistID == seriesdata.AnilistID {
			fmt.Println("Info: history hit by anilistID")
			return hist, nil
		}

		if seriesdata.Titles.EnglishTitle != "" &&
			hist.Metadata.Titles.EnglishTitle == seriesdata.Titles.EnglishTitle {
			fmt.Println("Info: history hit by english title")
			return hist, nil
		}

		if seriesdata.Titles.RomajiTitle != "" &&
			hist.Metadata.Titles.RomajiTitle == seriesdata.Titles.RomajiTitle {
			fmt.Println("Info: history hit by romaji title")
			return hist, nil
		}
	}

	needsMetadata := seriesdata.AnilistID == 0 ||
		(seriesdata.Titles.EnglishTitle == "" && seriesdata.Titles.RomajiTitle == "")

	if needsMetadata {
		if err := anilist.FillSeriesData(&seriesdata); err == nil {
			fmt.Println("Info: successfully filled missing metadata from Anilist")
		} else {
			log.Println("Warning: failed to fill metadata:", err)
		}
	}

	newHistory := &state.History{
		Metadata:    seriesdata,
		LastEpisode: 1,
		Episodes:    make(map[int]state.EpisodeProgress),
		Provider:    providerName,
	}

	fmt.Println("Info: Create new history complete")
	return newHistory, nil
}
