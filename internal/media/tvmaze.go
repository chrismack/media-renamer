package media

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TVmazeClient struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	shows      map[string]tvmazeShow
}

type tvmazeShow struct {
	ID        int
	Name      string
	Premiered string
}

func (c *TVmazeClient) EpisodeMetadata(showName string, season, episode int) (EpisodeMetadata, error) {
	show, err := c.findShow(showName)
	if err != nil {
		return EpisodeMetadata{}, err
	}

	var result struct {
		Name string `json:"name"`
	}
	path := fmt.Sprintf("%s/shows/%d/episodebynumber", c.baseURL(), show.ID)
	query := url.Values{
		"season": {strconv.Itoa(season)},
		"number": {strconv.Itoa(episode)},
	}
	if err := c.get(path, query, &result); err != nil {
		return EpisodeMetadata{}, err
	}
	if strings.TrimSpace(result.Name) == "" {
		return EpisodeMetadata{}, fmt.Errorf("TVmaze returned no title for %s S%02dE%02d", showName, season, episode)
	}
	return EpisodeMetadata{
		ShowName:     show.Name,
		EpisodeTitle: result.Name,
		ReleaseYear:  yearFromDate(show.Premiered),
	}, nil
}

func (c *TVmazeClient) findShow(showName string) (tvmazeShow, error) {
	key := strings.ToLower(strings.TrimSpace(showName))
	if show, exists := c.shows[key]; exists {
		return show, nil
	}

	var results []struct {
		Show struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Premiered string `json:"premiered"`
		} `json:"show"`
	}
	if err := c.get(c.baseURL()+"/search/shows", url.Values{"q": {showName}}, &results); err != nil {
		return tvmazeShow{}, err
	}
	if len(results) == 0 {
		return tvmazeShow{}, fmt.Errorf("show %q was not found on TVmaze", showName)
	}

	selected := results[0].Show
	for _, result := range results {
		if strings.EqualFold(strings.TrimSpace(result.Show.Name), strings.TrimSpace(showName)) {
			selected = result.Show
			break
		}
	}
	if c.shows == nil {
		c.shows = make(map[string]tvmazeShow)
	}
	show := tvmazeShow{ID: selected.ID, Name: selected.Name, Premiered: selected.Premiered}
	c.shows[key] = show
	return show, nil
}

func (c *TVmazeClient) get(endpoint string, query url.Values, destination any) error {
	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		endpoint+"?"+query.Encode(),
		nil,
	)
	if err != nil {
		return fmt.Errorf("create TVmaze request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent())

	response, err := c.httpClient().Do(request)
	if err != nil {
		return fmt.Errorf("request TVmaze: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("TVmaze returned HTTP %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode TVmaze response: %w", err)
	}
	return nil
}

func (c *TVmazeClient) baseURL() string {
	if baseURL := strings.TrimRight(c.BaseURL, "/"); baseURL != "" {
		return baseURL
	}
	return "https://api.tvmaze.com"
}

func (c *TVmazeClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (c *TVmazeClient) userAgent() string {
	if c.UserAgent != "" {
		return c.UserAgent
	}
	return "media-renamer/1.0"
}
