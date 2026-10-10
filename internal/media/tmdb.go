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

type TMDbClient struct {
	APIKey     string
	Language   string
	BaseURL    string
	HTTPClient *http.Client
}

func (c *TMDbClient) EpisodeMetadata(showName string, season, episode int) (EpisodeMetadata, error) {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.themoviedb.org/3"
	}
	language := c.Language
	if language == "" {
		language = "en-US"
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	var search struct {
		Results []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			FirstAirDate string `json:"first_air_date"`
		} `json:"results"`
	}
	query := url.Values{"query": {showName}, "language": {language}}
	if err := c.get(client, baseURL+"/search/tv", query, &search); err != nil {
		return EpisodeMetadata{}, err
	}
	if len(search.Results) == 0 {
		return EpisodeMetadata{}, fmt.Errorf("show %q was not found on TMDb", showName)
	}

	var result struct {
		Name string `json:"name"`
	}
	path := fmt.Sprintf("%s/tv/%d/season/%d/episode/%d", baseURL, search.Results[0].ID, season, episode)
	if err := c.get(client, path, url.Values{"language": {language}}, &result); err != nil {
		return EpisodeMetadata{}, err
	}
	if strings.TrimSpace(result.Name) == "" {
		return EpisodeMetadata{}, fmt.Errorf("TMDb returned no title for %s S%02dE%02d", showName, season, episode)
	}
	return EpisodeMetadata{
		ShowName:     search.Results[0].Name,
		EpisodeTitle: result.Name,
		ReleaseYear:  yearFromDate(search.Results[0].FirstAirDate),
	}, nil
}

func (c *TMDbClient) get(client *http.Client, endpoint string, query url.Values, destination any) error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("TMDb API key is required")
	}
	query.Set("api_key", c.APIKey)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("create TMDb request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request TMDb: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("TMDb returned HTTP %s", strconv.Itoa(response.StatusCode))
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode TMDb response: %w", err)
	}
	return nil
}
