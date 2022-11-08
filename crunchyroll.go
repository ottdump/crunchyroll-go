package crunchyroll

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// LOCALE represents a locale / language.
type LOCALE string

const (
	JP  LOCALE = "ja-JP"
	US         = "en-US"
	LA         = "es-419"
	LA2        = "es-LA"
	ES         = "es-ES"
	FR         = "fr-FR"
	PT         = "pt-PT"
	BR         = "pt-BR"
	IT         = "it-IT"
	DE         = "de-DE"
	RU         = "ru-RU"
	AR         = "ar-SA"
	ME         = "ar-ME"
	CN         = "zh-CN"
	IN         = "hi-IN"
)

// MediaType represents a media type.
type MediaType string

const (
	MediaTypeSeries MediaType = "series"
	MediaTypeMovie            = "movie_listing"
)

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Country      string `json:"country"`
	AccountID    string `json:"account_id"`
}

// LoginWithCredentials logs in via crunchyroll username or email and password.
func LoginWithCredentials(user string, password string, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	endpoint := "https://beta-api.crunchyroll.com/auth/v1/token"
	values := url.Values{}
	values.Set("username", user)
	values.Set("password", password)
	values.Set("grant_type", "password")
	values.Set("scope", "offline_access")

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic aHJobzlxM2F3dnNrMjJ1LXRzNWE6cHROOURteXRBU2Z6QjZvbXVsSzh6cUxzYTczVE1TY1k=")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := request(req, client)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var loginResp loginResponse
	json.NewDecoder(resp.Body).Decode(&loginResp)

	return postLogin(loginResp, locale, client)
}

// LoginWithSessionID logs in via a crunchyroll session id.
// Session ids are automatically generated as a cookie when visiting https://beta-api.crunchyroll.com.
//
// Deprecated: Login via session id caused some trouble in the past (e.g. #15 or #30) which resulted in
// login not working. Use LoginWithRefreshToken instead.
// The method will stay in the library until session id login is removed completely or login with it
// does not work for a longer period of time.
func LoginWithSessionID(sessionID string, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	endpoint := fmt.Sprintf("https://api.crunchyroll.com/start_session.0.json?session_id=%s",
		sessionID)
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to start session: %s", resp.Status)
	}

	var jsonBody map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse start session with session id response: %w", err)
	}
	if isError, ok := jsonBody["error"]; ok && isError.(bool) {
		return nil, fmt.Errorf("invalid session id (%s): %s", jsonBody["message"].(string), jsonBody["code"])
	}

	var etpRt string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "etp_rt" {
			etpRt = cookie.Value
			break
		}
	}

	return LoginWithRefreshToken(etpRt, locale, client)
}

// LoginWithRefreshToken logs in via the crunchyroll refresh token.
// It can be obtained by copying the etp_rt cookie from beta.crunchyroll.com.
// The etp_rt cookie is automatically set when visiting https://beta-api.crunchyroll.com. Note that you
// need a crunchyroll account to access it.
func LoginWithRefreshToken(refreshToken string, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	endpoint := "https://beta-api.crunchyroll.com/auth/v1/token"
	grantType := url.Values{}
	grantType.Set("refresh_token", refreshToken)
	grantType.Set("grant_type", "refresh_token")
	grantType.Set("scope", "offline_access")

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(grantType.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic aHJobzlxM2F3dnNrMjJ1LXRzNWE6cHROOURteXRBU2Z6QjZvbXVsSzh6cUxzYTczVE1TY1k=")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := request(req, client)
	if err != nil {
		if reqErr := err.(*RequestError); reqErr != nil && reqErr.Response.StatusCode == http.StatusBadRequest {
			endpoint = "https://beta-api.crunchyroll.com/auth/v1/token"
			grantType = url.Values{}
			grantType.Set("grant_type", "etp_rt_cookie")
			grantType.Set("scope", "offline_access")

			req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(grantType.Encode()))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6")
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{
				Name:  "etp_rt",
				Value: refreshToken,
			})
			if resp, err = request(req, client); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	var loginResp loginResponse
	json.NewDecoder(resp.Body).Decode(&loginResp)

	return postLogin(loginResp, locale, client)
}

func postLogin(loginResp loginResponse, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	crunchy := &Crunchyroll{
		Client:       client,
		Context:      context.Background(),
		Locale:       locale,
		RefreshToken: loginResp.RefreshToken,
		cache:        true,
	}

	crunchy.Config.TokenType = loginResp.TokenType
	crunchy.Config.AccessToken = loginResp.AccessToken
	crunchy.Config.AccountID = loginResp.AccountID
	crunchy.Config.CountryCode = loginResp.Country

	var jsonBody map[string]any

	endpoint := "https://beta-api.crunchyroll.com/index/v2"
	resp, err := crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	cms := jsonBody["cms"].(map[string]any)
	// / is trimmed so that urls which require it must be in .../{bucket}/... like format.
	// this just looks cleaner
	crunchy.Config.Bucket = strings.TrimPrefix(cms["bucket"].(string), "/")
	crunchy.Config.Premium = strings.HasSuffix(crunchy.Config.Bucket, "crunchyroll")
	crunchy.Config.Policy = cms["policy"].(string)
	crunchy.Config.Signature = cms["signature"].(string)
	crunchy.Config.KeyPairID = cms["key_pair_id"].(string)

	endpoint = "https://beta-api.crunchyroll.com/accounts/v1/me"
	resp, err = crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&jsonBody)
	crunchy.Config.ExternalID = jsonBody["external_id"].(string)

	endpoint = "https://beta-api.crunchyroll.com/accounts/v1/me/profile"
	resp, err = crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&jsonBody)
	crunchy.Config.MaturityRating = jsonBody["maturity_rating"].(string)

	return crunchy, nil
}

// Crunchyroll is the base struct which is needed for every request and contains the most important information.
// Use LoginWithCredentials, LoginWithRefreshToken or LoginWithSessionID to create a new instance.
type Crunchyroll struct {
	// Client is the http.Client to perform all requests over.
	Client *http.Client
	// Context can be used to stop requests with Client and is context.Background by default.
	Context context.Context
	// Locale specifies in which language all results should be returned / requested.
	Locale LOCALE
	// RefreshToken is the crunchyroll beta equivalent to a session id (prior SessionID field in
	// this struct in v2 and below).
	RefreshToken string

	// Config stores parameters which are needed by some api calls.
	Config struct {
		TokenType   string
		AccessToken string

		Bucket string

		CountryCode    string
		Premium        bool
		Channel        string
		Policy         string
		Signature      string
		KeyPairID      string
		AccountID      string
		ExternalID     string
		MaturityRating string
	}

	// If cache is true, internal caching is enabled.
	cache bool
}

// InvalidateSession logs the user out which invalidates the current session.
// You have to call a login method again and create a new Crunchyroll instance
// if you want to perform any further actions since this instance is not usable
// anymore after calling this.
func (c *Crunchyroll) InvalidateSession() error {
	endpoint := "https://crunchyroll.com/logout"
	_, err := c.request(endpoint, http.MethodGet)
	return err
}

// IsCaching returns if data gets cached or not.
// See SetCaching for more information.
func (c *Crunchyroll) IsCaching() bool {
	return c.cache
}

// SetCaching enables or disables internal caching of requests made.
// Caching is enabled by default.
// If it is disabled the already cached data still gets called.
// The best way to prevent this is to create a complete new Crunchyroll struct.
func (c *Crunchyroll) SetCaching(caching bool) {
	c.cache = caching
}

// request is a base function which handles simple api requests.
func (c *Crunchyroll) request(endpoint string, method string) (*http.Response, error) {
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return c.requestFull(req)
}

// requestFull is a base function which handles full user controlled api requests.
func (c *Crunchyroll) requestFull(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("%s %s", c.Config.TokenType, c.Config.AccessToken))

	return request(req, c.Client)
}

func request(req *http.Request, client *http.Client) (*http.Response, error) {
	resp, err := client.Do(req)
	if err == nil {
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		defer resp.Body.Close()
		defer func() {
			resp.Body = io.NopCloser(&buf)
		}()

		if buf.Len() != 0 {
			var errMap map[string]any

			if err = json.Unmarshal(buf.Bytes(), &errMap); err != nil {
				return nil, &RequestError{Response: resp, Message: fmt.Sprintf("invalid json response: %v", err)}
			}

			if val, ok := errMap["error"]; ok {
				if errorAsString, ok := val.(string); ok {
					if code, ok := errMap["code"].(string); ok {
						return nil, &RequestError{Response: resp, Message: fmt.Sprintf("%s - %s", errorAsString, code)}
					}
					return nil, &RequestError{Response: resp, Message: errorAsString}
				} else if errorAsBool, ok := val.(bool); ok && errorAsBool {
					if msg, ok := errMap["message"].(string); ok {
						return nil, &RequestError{Response: resp, Message: msg}
					}
				}
			} else if _, ok := errMap["code"]; ok {
				if errContext, ok := errMap["context"].([]any); ok && len(errContext) > 0 {
					errField := errContext[0].(map[string]any)
					var code string
					if code, ok = errField["message"].(string); !ok {
						code = errField["code"].(string)
					}
					return nil, &RequestError{Response: resp, Message: fmt.Sprintf("%s - %s", code, errField["field"].(string))}
				} else if errMessage, ok := errMap["message"].(string); ok {
					return nil, &RequestError{Response: resp, Message: errMessage}
				}
			}
		}

		if resp.StatusCode >= 400 {
			return nil, &RequestError{Response: resp, Message: resp.Status}
		}
	}
	return resp, err
}
