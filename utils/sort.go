package utils

import (
	"github.com/ottdump/crunchyroll-go/v3"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// SortEpisodesBySeason sorts the given episodes by their seasons.
// Note that the same episodes just with different audio locales will cause problems.
func SortEpisodesBySeason(episodes []*crunchyroll.Episode) [][]*crunchyroll.Episode {
	sortMap := map[string]map[int][]*crunchyroll.Episode{}

	for _, episode := range episodes {
		if _, ok := sortMap[episode.SeriesID]; !ok {
			sortMap[episode.SeriesID] = map[int][]*crunchyroll.Episode{}
		}
		if _, ok := sortMap[episode.SeriesID][episode.SeasonNumber]; !ok {
			sortMap[episode.SeriesID][episode.SeasonNumber] = make([]*crunchyroll.Episode, 0)
		}
		sortMap[episode.SeriesID][episode.SeasonNumber] = append(sortMap[episode.SeriesID][episode.SeasonNumber], episode)
	}

	var eps [][]*crunchyroll.Episode
	for _, series := range sortMap {
		var keys []int
		for seriesNumber := range series {
			keys = append(keys, seriesNumber)
		}
		sort.Ints(keys)

		for _, key := range keys {
			es := series[key]
			if len(es) > 0 {
				sort.Sort(EpisodesByNumber(es))
				eps = append(eps, es)
			}
		}
	}

	return eps
}

// SortEpisodesByAudio sort the given episodes by their audio locale.
func SortEpisodesByAudio(episodes []*crunchyroll.Episode) (map[crunchyroll.LOCALE][]*crunchyroll.Episode, error) {
	eps := map[crunchyroll.LOCALE][]*crunchyroll.Episode{}

	errChan := make(chan error)

	var wg sync.WaitGroup
	var lock sync.Mutex
	for _, episode := range episodes {
		if !episode.Available() {
			continue
		}
		episode := episode
		wg.Add(1)
		go func() {
			defer wg.Done()
			audioLocale, err := episode.AudioLocale()
			if err != nil {
				errChan <- err
				return
			}
			lock.Lock()
			defer lock.Unlock()

			if _, ok := eps[audioLocale]; !ok {
				eps[audioLocale] = make([]*crunchyroll.Episode, 0)
			}
			eps[audioLocale] = append(eps[audioLocale], episode)
		}()
	}
	go func() {
		wg.Wait()
		errChan <- nil
	}()

	if err := <-errChan; err != nil {
		return nil, err
	}
	return eps, nil
}

// MovieListingsByDuration sorts movie listings by their duration.
type MovieListingsByDuration []*crunchyroll.MovieListing

func (mlbd MovieListingsByDuration) Len() int {
	return len(mlbd)
}
func (mlbd MovieListingsByDuration) Swap(i, j int) {
	mlbd[i], mlbd[j] = mlbd[j], mlbd[i]
}
func (mlbd MovieListingsByDuration) Less(i, j int) bool {
	return mlbd[i].DurationMS < mlbd[j].DurationMS
}

// EpisodesByDuration sorts episodes by their duration.
type EpisodesByDuration []*crunchyroll.Episode

func (ebd EpisodesByDuration) Len() int {
	return len(ebd)
}
func (ebd EpisodesByDuration) Swap(i, j int) {
	ebd[i], ebd[j] = ebd[j], ebd[i]
}
func (ebd EpisodesByDuration) Less(i, j int) bool {
	return ebd[i].DurationMS < ebd[j].DurationMS
}

// EpisodesByNumber sorts episodes after their episode number.
type EpisodesByNumber []*crunchyroll.Episode

func (ebn EpisodesByNumber) Len() int {
	return len(ebn)
}
func (ebn EpisodesByNumber) Swap(i, j int) {
	ebn[i], ebn[j] = ebn[j], ebn[i]
}
func (ebn EpisodesByNumber) Less(i, j int) bool {
	return ebn[i].EpisodeNumber < ebn[j].EpisodeNumber
}

// FormatsByResolution sorts formats after their resolution.
type FormatsByResolution []*crunchyroll.Format

func (fbr FormatsByResolution) Len() int {
	return len(fbr)
}
func (fbr FormatsByResolution) Swap(i, j int) {
	fbr[i], fbr[j] = fbr[j], fbr[i]
}
func (fbr FormatsByResolution) Less(i, j int) bool {
	iSplitRes := strings.SplitN(fbr[i].Video.Resolution, "x", 2)
	iResX, _ := strconv.Atoi(iSplitRes[0])
	iResY, _ := strconv.Atoi(iSplitRes[1])

	jSplitRes := strings.SplitN(fbr[j].Video.Resolution, "x", 2)
	jResX, _ := strconv.Atoi(jSplitRes[0])
	jResY, _ := strconv.Atoi(jSplitRes[1])

	return iResX+iResY < jResX+jResY
}

// SubtitlesByLocale sorts subtitles after their locale.
type SubtitlesByLocale []*crunchyroll.Subtitle

func (sbl SubtitlesByLocale) Len() int {
	return len(sbl)
}
func (sbl SubtitlesByLocale) Swap(i, j int) {
	sbl[i], sbl[j] = sbl[j], sbl[i]
}
func (sbl SubtitlesByLocale) Less(i, j int) bool {
	return LocaleLanguage(sbl[i].Locale) < LocaleLanguage(sbl[j].Locale)
}
