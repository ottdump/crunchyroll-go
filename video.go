package crunchyroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type video struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`

	Description string `json:"description"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	SlugTitle   string `json:"slug_title"`

	Images struct {
		PosterTall [][]Image `json:"poster_tall"`
		PosterWide [][]Image `json:"poster_wide"`
	} `json:"images"`
}

// Video is the base for Movie and Season.
type Video interface{}

// Movie contains information about a movie.
type Movie struct {
	video
	Video

	crunchy *Crunchyroll

	children []*MovieListing

	// not generated when calling MovieFromID.
	MovieListingMetadata struct {
		AvailabilityNotes   string   `json:"availability_notes"`
		AvailableOffline    bool     `json:"available_offline"`
		DurationMS          int      `json:"duration_ms"`
		ExtendedDescription string   `json:"extended_description"`
		FirstMovieID        string   `json:"first_movie_id"`
		IsDubbed            bool     `json:"is_dubbed"`
		IsMature            bool     `json:"is_mature"`
		IsPremiumOnly       bool     `json:"is_premium_only"`
		IsSubbed            bool     `json:"is_subbed"`
		MatureRatings       []string `json:"mature_ratings"`
		MovieReleaseYear    int      `json:"movie_release_year"`
		SubtitleLocales     []LOCALE `json:"subtitle_locales"`
	} `json:"movie_listing_metadata"`

	Playback string `json:"playback"`

	PromoDescription string `json:"promo_description"`
	PromoTitle       string `json:"promo_title"`
	SearchMetadata   struct {
		Score float64 `json:"score"`
	}
}

// MovieFromID returns a movie by its api id.
func MovieFromID(crunchy *Crunchyroll, id string) (*Movie, error) {
	resp, err := crunchy.request(fmt.Sprintf("https://www.crunchyroll.com/cms/v2/%s/movies/%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
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

	movieListing := &Movie{
		crunchy: crunchy,
	}
	movieListing.ID = id
	if err = decodeMapToStruct(jsonBody, movieListing); err != nil {
		return nil, err
	}

	return movieListing, nil
}

// MovieListing returns all videos corresponding with the movie.
func (m *Movie) MovieListing() (movieListings []*MovieListing, err error) {
	if m.children != nil {
		return m.children, nil
	}

	resp, err := m.crunchy.request(fmt.Sprintf("https://www.crunchyroll.com/cms/v2/%s/movies?movie_listing_id=%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		m.crunchy.Config.Bucket,
		m.ID,
		m.crunchy.Locale,
		m.crunchy.Config.Signature,
		m.crunchy.Config.Policy,
		m.crunchy.Config.KeyPairID), http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	for _, item := range jsonBody["items"].([]interface{}) {
		movieListing := &MovieListing{
			crunchy: m.crunchy,
		}
		if err = decodeMapToStruct(item, movieListing); err != nil {
			return nil, err
		}
		movieListings = append(movieListings, movieListing)
	}

	if m.crunchy.cache {
		m.children = movieListings
	}
	return movieListings, nil
}

// Series contains information about an anime series.
type Series struct {
	video
	Video

	crunchy *Crunchyroll

	children []*Season

	PromoDescription string `json:"promo_description"`
	PromoTitle       string `json:"promo_title"`

	AvailabilityNotes   string   `json:"availability_notes"`
	EpisodeCount        int      `json:"episode_count"`
	ExtendedDescription string   `json:"extended_description"`
	IsDubbed            bool     `json:"is_dubbed"`
	IsMature            bool     `json:"is_mature"`
	IsSimulcast         bool     `json:"is_simulcast"`
	IsSubbed            bool     `json:"is_subbed"`
	MatureBlocked       bool     `json:"mature_blocked"`
	MatureRatings       []string `json:"mature_ratings"`
	SeasonCount         int      `json:"season_count"`

	// not generated when calling SeriesFromID.
	SearchMetadata struct {
		Score float64 `json:"score"`
	}
}

// SeriesFromID returns a series by its api id.
func SeriesFromID(crunchy *Crunchyroll, id string) (*Series, error) {
	resp, err := crunchy.request(fmt.Sprintf("https://www.crunchyroll.com/cms/v2/%s/movies?movie_listing_id=%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
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

	series := &Series{
		crunchy: crunchy,
	}
	series.ID = id
	if err = decodeMapToStruct(jsonBody, series); err != nil {
		return nil, err
	}

	return series, nil
}

// AddToWatchlist adds the current episode to the watchlist.
// Will return an RequestError with the response status code of 409 if the series was already on the watchlist before.
func (s *Series) AddToWatchlist() error {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content/v1/watchlist/%s?locale=%s", s.crunchy.Config.AccountID, s.crunchy.Locale)
	body, _ := json.Marshal(map[string]string{"content_id": s.ID})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = s.crunchy.requestFull(req)
	return err
}

// RemoveFromWatchlist removes the current episode from the watchlist.
// Will return an RequestError with the response status code of 404 if the series was not on the watchlist before.
func (s *Series) RemoveFromWatchlist() error {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content/v1/watchlist/%s/%s?locale=%s", s.crunchy.Config.AccountID, s.ID, s.crunchy.Locale)
	_, err := s.crunchy.request(endpoint, http.MethodDelete)
	return err
}

// Similar returns similar series and movies to the current series within the given limit.
func (s *Series) Similar(limit uint) (ss []*Series, m []*Movie, err error) {
	similarToEndpoint := fmt.Sprintf("https://www.crunchyroll.com/content/v1/%s/similar_to?guid=%s&n=%d&locale=%s",
		s.crunchy.Config.AccountID, s.ID, limit, s.crunchy.Locale)
	resp, err := s.crunchy.request(similarToEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'similar_to' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		switch item.(map[string]interface{})["type"] {
		case MediaTypeSeries:
			series := &Series{
				crunchy: s.crunchy,
			}
			if err := decodeMapToStruct(item, series); err != nil {
				return nil, nil, err
			}
			if err := decodeMapToStruct(item.(map[string]interface{})["series_metadata"].(map[string]interface{}), series); err != nil {
				return nil, nil, err
			}

			ss = append(ss, series)
		case MediaTypeMovie:
			movie := &Movie{
				crunchy: s.crunchy,
			}
			if err := decodeMapToStruct(item, movie); err != nil {
				return nil, nil, err
			}

			m = append(m, movie)
		}
	}
	return
}

// Seasons returns all seasons of a series.
func (s *Series) Seasons() (seasons []*Season, err error) {
	if s.children != nil {
		return s.children, nil
	}

	resp, err := s.crunchy.request(fmt.Sprintf("https://www.crunchyroll.com/cms/v2/%s/seasons?series_id=%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		s.crunchy.Config.Bucket,
		s.ID,
		s.crunchy.Locale,
		s.crunchy.Config.Signature,
		s.crunchy.Config.Policy,
		s.crunchy.Config.KeyPairID), http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	for _, item := range jsonBody["items"].([]interface{}) {
		season := &Season{
			crunchy: s.crunchy,
		}
		if err = decodeMapToStruct(item, season); err != nil {
			return nil, err
		}
		seasons = append(seasons, season)
	}

	if s.crunchy.cache {
		s.children = seasons
	}
	return
}

// Rating returns the series rating.
func (s *Series) Rating() (*Rating, error) {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content-reviews/v2/user/%s/rating/series/%s", s.crunchy.Config.AccountID, s.ID)
	resp, err := s.crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rating := &Rating{}
	json.NewDecoder(resp.Body).Decode(rating)

	return rating, nil
}

// ReviewSortType represents a sort type to sort Series.Reviews items after.
type ReviewSortType string

const (
	ReviewSortNewest  ReviewSortType = "newest"
	ReviewSortOldest                 = "oldest"
	ReviewSortHelpful                = "helpful"
)

// ReviewOptions represents options for fetching series reviews.
type ReviewOptions struct {
	// Sort specifies how the items should be sorted.
	Sort ReviewSortType `json:"sort"`
	// Filter specified after which the returning items should be filtered.
	Filter ReviewRating `json:"filter"`
}

// Reviews returns user reviews for the series.
func (s *Series) Reviews(options ReviewOptions, page uint, size uint) (BulkResult[*UserReview], error) {
	options, err := structDefaults(ReviewOptions{Sort: ReviewSortNewest}, options)
	if err != nil {
		return BulkResult[*UserReview]{}, err
	}
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content-reviews/v2/%s/user/%s/review/series/%s/list?page=%d&page_size=%d&sort=%s&filter=%s", s.crunchy.Locale, s.crunchy.Config.AccountID, s.ID, page, size, options.Sort, options.Filter)
	resp, err := s.crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return BulkResult[*UserReview]{}, err
	}
	defer resp.Body.Close()

	var result BulkResult[*UserReview]
	json.NewDecoder(resp.Body).Decode(&result)

	for _, review := range result.Items {
		review.crunchy = s.crunchy
		review.SeriesID = s.ID
	}

	return result, nil
}

// Rate rates the current series.
func (s *Series) Rate(rating ReviewRating) error {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content-reviews/v2/en-US/user/%s/review/series/%s", s.crunchy.Config.AccountID, s.ID)
	body, _ := json.Marshal(map[string]string{"rating": string(rating)})
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = s.crunchy.requestFull(req)
	return err
}

// CreateReview creates a review for the current series with the logged-in account.
// Will fail if a review is already present. Check Series.HasOwnerReview if the account
// has already written a review. If this is the case, use Series.GetOwnerReview and user
// OwnerReview.Edit to edit the review.
func (s *Series) CreateReview(title, content string, spoiler bool) (*OwnerReview, error) {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content-reviews/v2/en-US/user/%s/review/series/%s", s.crunchy.Config.AccountID, s.ID)
	body, _ := json.Marshal(map[string]any{
		"title":   title,
		"body":    content,
		"spoiler": spoiler,
	})
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := s.crunchy.requestFull(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	review := &OwnerReview{}
	json.NewDecoder(resp.Body).Decode(review)
	review.crunchy = s.crunchy
	review.SeriesID = s.ID

	return review, nil
}

// GetOwnerReview returns the series review, written by the current logged-in account.
// Returns an error if no review was written yet.
func (s *Series) GetOwnerReview() (*OwnerReview, error) {
	endpoint := fmt.Sprintf("https://www.crunchyroll.com/content-reviews/v2/en-US/user/%s/review/series/%s", s.crunchy.Config.AccountID, s.ID)
	resp, err := s.crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	review := &OwnerReview{}
	json.NewDecoder(resp.Body).Decode(review)
	review.crunchy = s.crunchy
	review.SeriesID = s.ID

	return review, nil
}

// HasOwnerReview returns if the logged-in account has written a review for the series.
func (s *Series) HasOwnerReview() bool {
	_, err := s.GetOwnerReview()
	return err == nil
}
