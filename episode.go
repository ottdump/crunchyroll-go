package crunchyroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Episode contains all information about an episode.
type Episode struct {
	crunchy *Crunchyroll

	children []*Stream

	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`

	SeriesID        string `json:"series_id"`
	SeriesTitle     string `json:"series_title"`
	SeriesSlugTitle string `json:"series_slug_title"`

	SeasonID        string `json:"season_id"`
	SeasonTitle     string `json:"season_title"`
	SeasonSlugTitle string `json:"season_slug_title"`
	SeasonNumber    int    `json:"season_number"`

	Episode             string  `json:"episode"`
	EpisodeNumber       int     `json:"episode_number"`
	SequenceNumber      float64 `json:"sequence_number"`
	ProductionEpisodeID string  `json:"production_episode_id"`

	Title            string `json:"title"`
	SlugTitle        string `json:"slug_title"`
	Description      string `json:"description"`
	NextEpisodeID    string `json:"next_episode_id"`
	NextEpisodeTitle string `json:"next_episode_title"`

	HDFlag          bool     `json:"hd_flag"`
	MaturityRatings []string `json:"maturity_ratings"`
	IsMature        bool     `json:"is_mature"`
	MatureBlocked   bool     `json:"mature_blocked"`

	EpisodeAirDate       time.Time `json:"episode_air_date"`
	FreeAvailableDate    time.Time `json:"free_available_date"`
	PremiumAvailableDate time.Time `json:"premium_available_date"`

	IsSubbed       bool     `json:"is_subbed"`
	IsDubbed       bool     `json:"is_dubbed"`
	IsClip         bool     `json:"is_clip"`
	SeoTitle       string   `json:"seo_title"`
	SeoDescription string   `json:"seo_description"`
	SeasonTags     []string `json:"season_tags"`

	AvailableOffline bool      `json:"available_offline"`
	MediaType        MediaType `json:"media_type"`
	Slug             string    `json:"slug"`

	Images struct {
		Thumbnail [][]Image `json:"thumbnail"`
	} `json:"images"`

	DurationMS    int    `json:"duration_ms"`
	IsPremiumOnly bool   `json:"is_premium_only"`
	ListingID     string `json:"listing_id"`

	SubtitleLocales []LOCALE `json:"subtitle_locales"`
	Playback        string   `json:"playback"`

	AvailabilityNotes string `json:"availability_notes"`

	StreamID string
}

// HistoryEpisode contains additional information about an episode if the account has watched or started to watch the episode.
type HistoryEpisode struct {
	*Episode

	DatePlayed   time.Time `json:"date_played"`
	ParentID     string    `json:"parent_id"`
	ParentType   MediaType `json:"parent_type"`
	Playhead     uint      `json:"playhead"`
	FullyWatched bool      `json:"fully_watched"`
}

// WatchlistEntryType specifies which type a watchlist entry has.
type WatchlistEntryType string

const (
	WatchlistEntryEpisode = "episode"
	WatchlistEntrySeries  = "series"
)

// EpisodeFromID returns an episode by its api id.
func EpisodeFromID(crunchy *Crunchyroll, id string) (*Episode, error) {
	resp, err := crunchy.request(fmt.Sprintf("https://www.crunchyroll.com/cms/v2/%s/episodes/%s?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		crunchy.Config.Bucket,
		id,
		crunchy.Locale,
		crunchy.Config.Signature,
		crunchy.Config.Policy,
		crunchy.Config.KeyPairID), http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	episode := &Episode{
		crunchy: crunchy,
		ID:      id,
	}
	if err := decodeMapToStruct(jsonBody, episode); err != nil {
		return nil, err
	}
	if episode.Playback != "" {
		streamHref := jsonBody["__links__"].(map[string]interface{})["streams"].(map[string]interface{})["href"].(string)
		if match := regexp.MustCompile(`(?m)^/cms/v2/\S+videos/(\w+)/streams$`).FindAllStringSubmatch(streamHref, -1); len(match) > 0 {
			episode.StreamID = match[0][1]
		}
	}

	return episode, nil
}

// AddToWatchlist adds the current episode to the watchlist.
// Will return an RequestError with the response status code of 409 if the series was already on the watchlist before.
// There is currently a bug, or as I like to say in context of the crunchyroll api, feature, that only series and not
// individual episode can be added to the watchlist. Even though I somehow got an episode to my watchlist on the
// crunchyroll website, it never worked with the api here. So this function actually adds the whole series to the watchlist.
func (e *Episode) AddToWatchlist() error {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content/v1/watchlist/%s?locale=%s", e.crunchy.Config.AccountID, e.crunchy.Locale)
	body, _ := json.Marshal(map[string]string{"content_id": e.SeriesID})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = e.crunchy.requestFull(req)
	return err
}

// RemoveFromWatchlist removes the current episode from the watchlist.
// Will return an RequestError with the response status code of 404 if the series was not on the watchlist before.
func (e *Episode) RemoveFromWatchlist() error {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content/v1/watchlist/%s/%s?locale=%s", e.crunchy.Config.AccountID, e.SeriesID, e.crunchy.Locale)
	_, err := e.crunchy.request(endpoint, http.MethodDelete)
	return err
}

// AudioLocale returns the audio locale of the episode.
// Every episode in a season (should) have the same audio locale,
// so if you want to get the audio locale of a season, just call
// this method on the first episode of the season.
// Will fail if no streams are available, thus use Episode.Available
// to prevent any misleading errors.
func (e *Episode) AudioLocale() (LOCALE, error) {
	streams, err := e.Streams()
	if err != nil {
		return "", err
	}
	return streams[0].AudioLocale, nil
}

// Comment creates a new comment under the episode.
func (e *Episode) Comment(message string, spoiler bool) (*Comment, error) {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/talkbox/guestbooks/%s/comments?locale=%s", e.ID, e.crunchy.Locale)
	var flags []string
	if spoiler {
		flags = append(flags, "spoiler")
	}
	body, _ := json.Marshal(map[string]any{"locale": string(e.crunchy.Locale), "flags": flags, "message": message})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := e.crunchy.requestFull(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	c := &Comment{
		crunchy:   e.crunchy,
		EpisodeID: e.ID,
	}
	if err = json.NewDecoder(resp.Body).Decode(c); err != nil {
		return nil, err
	}

	return c, nil
}

// CommentsOrderType represents a sort type to sort Episode.Comments after.
type CommentsOrderType string

const (
	CommentsOrderAsc  CommentsOrderType = "asc"
	CommentsOrderDesc                   = "desc"
)

// CommentsSortType specified after which factor Episode.Comments should be sorted.
type CommentsSortType string

const (
	CommentsSortPopular CommentsSortType = "popular"
	CommentsSortDate                     = "date"
)

type CommentsOptions struct {
	// Order specified the order how the comments should be returned.
	Order CommentsOrderType `json:"order"`

	// Sort specified after which key the comments should be sorted.
	Sort CommentsSortType `json:"sort"`
}

// Comments returns comments under the given episode.
func (e *Episode) Comments(options CommentsOptions, page uint, size uint) (c []*Comment, err error) {
	options, err = structDefaults(CommentsOptions{Order: CommentsOrderDesc, Sort: CommentsSortPopular}, options)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/talkbox/guestbooks/%s/comments?page=%d&page_size=%d&order=%s&sort=%s&locale=%s", e.ID, page, size, options.Order, options.Sort, e.crunchy.Locale)
	resp, err := e.crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]any
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	if err = decodeMapToStruct(jsonBody["items"].([]any), &c); err != nil {
		return nil, err
	}
	for _, comment := range c {
		comment.crunchy = e.crunchy
		comment.EpisodeID = e.ID
	}

	return
}

// Available returns if downloadable streams for this episodes are available.
func (e *Episode) Available() bool {
	return e.crunchy.Config.Premium || !e.IsPremiumOnly
}

// GetFormat returns the format which matches the given resolution and subtitle locale.
func (e *Episode) GetFormat(resolution string, subtitle LOCALE, hardsub bool) (*Format, error) {
	streams, err := e.Streams()
	if err != nil {
		return nil, err
	}
	var foundStream *Stream
	for _, stream := range streams {
		if hardsub && stream.HardsubLocale == subtitle || stream.HardsubLocale == "" && subtitle == "" {
			foundStream = stream
			break
		} else if !hardsub {
			for _, streamSubtitle := range stream.Subtitles {
				if streamSubtitle.Locale == subtitle {
					foundStream = stream
					break
				}
			}
			if foundStream != nil {
				break
			}
		}
	}

	if foundStream == nil {
		return nil, fmt.Errorf("no matching stream found")
	}
	formats, err := foundStream.Formats()
	if err != nil {
		return nil, err
	}
	var res *Format
	for _, format := range formats {
		if resolution == "worst" || resolution == "best" {
			if res == nil {
				res = format
				continue
			}

			curSplitRes := strings.SplitN(format.Video.Resolution, "x", 2)
			curResX, _ := strconv.Atoi(curSplitRes[0])
			curResY, _ := strconv.Atoi(curSplitRes[1])

			resSplitRes := strings.SplitN(res.Video.Resolution, "x", 2)
			resResX, _ := strconv.Atoi(resSplitRes[0])
			resResY, _ := strconv.Atoi(resSplitRes[1])

			if resolution == "worst" && curResX+curResY < resResX+resResY {
				res = format
			} else if resolution == "best" && curResX+curResY > resResX+resResY {
				res = format
			}
		}

		if format.Video.Resolution == resolution {
			return format, nil
		}
	}

	if res != nil {
		return res, nil
	}

	return nil, fmt.Errorf("no matching resolution found")
}

// Streams returns all streams which are available for the episode.
func (e *Episode) Streams() ([]*Stream, error) {
	if e.children != nil {
		return e.children, nil
	}

	streams, err := fromVideoStreams(e.crunchy, fmt.Sprintf("https://www.crunchyroll.com/cms/v2/%s/videos/%s/streams?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		e.crunchy.Config.Bucket,
		e.StreamID,
		e.crunchy.Locale,
		e.crunchy.Config.Signature,
		e.crunchy.Config.Policy,
		e.crunchy.Config.KeyPairID))
	if err != nil {
		return nil, err
	}

	if e.crunchy.cache {
		e.children = streams
	}
	return streams, nil
}
