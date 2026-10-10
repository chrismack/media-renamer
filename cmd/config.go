package cmd

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type config struct {
	Format             string   `yaml:"format"`
	TVFormat           string   `yaml:"tv_format"`
	FilmFormat         string   `yaml:"film_format"`
	Directory          string   `yaml:"directory"`
	Patterns           []string `yaml:"patterns"`
	TMDbAPIKey         string   `yaml:"tmdb_api_key"`
	TMDbLanguage       string   `yaml:"tmdb_language"`
	OrganiseShows      bool     `yaml:"organise_shows"`
	ShowFolderFormat   string   `yaml:"show_folder_format"`
	OrganiseSeasons    bool     `yaml:"organise_seasons"`
	SeasonFolderFormat string   `yaml:"season_folder_format"`
}

func loadConfig(path string, explicit bool) (config, error) {
	if path == "" {
		path = "media-renamer.yaml"
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return config{}, nil
		}
		return config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var result config
	if err := yaml.Unmarshal(content, &result); err != nil {
		return config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return result, nil
}
