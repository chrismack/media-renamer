package media

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Kind string

const (
	TV   Kind = "tv"
	Film Kind = "film"
)

const (
	DefaultTVFormat           = "<Show Name> - S<Season Number>E<Episode Number> - <Episode Title>"
	DefaultFilmFormat         = "<Film Name>"
	DefaultSeasonFolderFormat = "Season <season-number>"
)

type Info struct {
	Kind         Kind
	ShowName     string
	Season       int
	Episode      int
	EpisodeTitle string
	FilmName     string
	ReleaseYear  string
	Resolution   string
	Extension    string
}

var (
	tvPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:^|[\s._-])s(\d{1,2})[\s._-]*e(\d{1,3})(?:\b|[\s._-])`),
		regexp.MustCompile(`(?i)(?:^|[\s._-])(\d{1,2})x(\d{1,3})(?:\b|[\s._-])`),
		regexp.MustCompile(`(?i)(?:^|[\s._-])season[\s._-]*(\d{1,2})[\s._-]*(?:episode|ep)[\s._-]*(\d{1,3})(?:\b|[\s._-])`),
	}
	episodePattern    = regexp.MustCompile(`(?i)(?:^|[\s._-])(?:episode|ep|e)[\s._-]*(\d{1,3})(?:\b|[\s._-])`)
	seasonPathPattern = regexp.MustCompile(`(?i)^(?:(?:season|s)[\s._-]*)?0*(\d{1,2})$`)
	yearPattern       = regexp.MustCompile(`(?:^|[\s._([])((?:19|20)\d{2})(?:$|[\s._)\]])`)
	yearValuePattern  = regexp.MustCompile(`(?:19|20)\d{2}`)
	resolutionPattern = regexp.MustCompile(`(?i)(?:^|[\s._([])(\d{3,4}p|[48]k)(?:$|[\s._)\]])`)
	spacePattern      = regexp.MustCompile(`\s+`)
	releaseTokens     = regexp.MustCompile(`(?i)(?:^|[\s._([])(?:bluray|blu-ray|brrip|webrip|web-dl|web|hdtv|dvdrip|remux|xvid|x26[45]|h26[45]|hevc|av1|aac\d*|dts(?:-hd)?|proper|repack|extended|unrated|hdr10?|dv)(?:$|[\s._)\]-]).*$`)
)

var mediaExtensions = map[string]struct{}{
	".avi": {}, ".m2ts": {}, ".m4v": {}, ".mkv": {}, ".mov": {},
	".mp4": {}, ".mpeg": {}, ".mpg": {}, ".ts": {}, ".webm": {}, ".wmv": {},
}

func IsMediaFile(path string) bool {
	_, ok := mediaExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func Parse(path string) (Info, error) {
	ext := filepath.Ext(path)
	if !IsMediaFile(path) {
		return Info{}, fmt.Errorf("%q is not a supported media file", path)
	}

	base := strings.TrimSuffix(filepath.Base(path), ext)
	info := Info{Extension: strings.TrimPrefix(ext, ".")}
	if match := resolutionPattern.FindStringSubmatch(base); len(match) > 1 {
		info.Resolution = normaliseResolution(match[1])
	}
	year, yearStart := lastYear(base)
	if year != "" {
		info.ReleaseYear = year
	}

	for _, pattern := range tvPatterns {
		loc := pattern.FindStringSubmatchIndex(base)
		if loc == nil {
			continue
		}
		season, _ := strconv.Atoi(base[loc[2]:loc[3]])
		episode, _ := strconv.Atoi(base[loc[4]:loc[5]])
		info.Kind = TV
		info.Season = season
		info.Episode = episode
		info.ShowName = cleanName(base[:loc[0]])
		info.EpisodeTitle = cleanEpisodeTitle(base[loc[1]:])
		if info.ShowName == "" {
			info.ShowName = showNameFromPath(path)
		}
		if info.ReleaseYear == "" {
			info.ReleaseYear, _ = lastYear(info.ShowName)
		}
		if info.ShowName == "" {
			return Info{}, fmt.Errorf("could not determine show name from %q", filepath.Base(path))
		}
		return info, nil
	}

	if season, showName, found := seasonAndShowFromPath(path); found {
		if loc := episodePattern.FindStringSubmatchIndex(base); loc != nil {
			episode, _ := strconv.Atoi(base[loc[2]:loc[3]])
			info.Kind = TV
			info.Season = season
			info.Episode = episode
			info.ShowName = cleanName(base[:loc[0]])
			if info.ShowName == "" {
				info.ShowName = showName
			}
			if info.ReleaseYear == "" {
				info.ReleaseYear, _ = lastYear(info.ShowName)
			}
			info.EpisodeTitle = cleanEpisodeTitle(base[loc[1]:])
			if info.ShowName == "" {
				return Info{}, fmt.Errorf("could not determine show name from path %q", path)
			}
			return info, nil
		}
	}

	info.Kind = Film
	cut := len(base)
	if yearStart >= 0 {
		cut = yearStart
	}
	for _, pattern := range []*regexp.Regexp{resolutionPattern, releaseTokens} {
		if loc := pattern.FindStringIndex(base); loc != nil && loc[0] < cut {
			cut = loc[0]
		}
	}
	info.FilmName = cleanName(base[:cut])
	if info.FilmName == "" {
		return Info{}, fmt.Errorf("could not determine film name from %q", filepath.Base(path))
	}
	return info, nil
}

func Format(info Info, format string) (string, error) {
	if strings.TrimSpace(format) == "" {
		if info.Kind == TV {
			format = DefaultTVFormat
		} else {
			format = DefaultFilmFormat
		}
	}

	values := map[string]string{
		"<Show Name>":      info.ShowName,
		"<Season Number>":  fmt.Sprintf("%02d", info.Season),
		"<Episode Number>": fmt.Sprintf("%02d", info.Episode),
		"<Episode Title>":  info.EpisodeTitle,
		"<Film Name>":      info.FilmName,
		"<Release Year>":   info.ReleaseYear,
		"<Resolution>":     info.Resolution,
		"<File Extension>": info.Extension,
	}
	name := format
	for placeholder, value := range values {
		name = strings.ReplaceAll(name, placeholder, value)
	}
	if strings.Contains(name, "<") || strings.Contains(name, ">") {
		return "", fmt.Errorf("format contains an unknown placeholder: %q", name)
	}

	name = cleanFormattedName(name)
	if name == "" {
		return "", fmt.Errorf("format produced an empty file name")
	}
	if !strings.Contains(format, "<File Extension>") {
		name += "." + info.Extension
	}
	return name, nil
}

func FormatSeasonFolder(season int, format string) (string, error) {
	if strings.TrimSpace(format) == "" {
		format = DefaultSeasonFolderFormat
	}
	number := strconv.Itoa(season)
	folder := strings.NewReplacer(
		"<season-number>", number,
		"<Season Number>", number,
	).Replace(format)
	if strings.Contains(folder, "<") || strings.Contains(folder, ">") {
		return "", fmt.Errorf("season folder format contains an unknown placeholder: %q", folder)
	}
	if strings.ContainsAny(folder, `/\`) || folder == "." || folder == ".." {
		return "", fmt.Errorf("season folder format must produce a single directory name")
	}
	folder = cleanFormattedName(folder)
	if folder == "" {
		return "", fmt.Errorf("season folder format produced an empty directory name")
	}
	return folder, nil
}

func cleanName(value string) string {
	value = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(value)
	value = strings.Trim(value, " \t-_.")
	return titleCase(spacePattern.ReplaceAllString(value, " "))
}

func cleanEpisodeTitle(value string) string {
	if loc := yearPattern.FindStringIndex(value); loc != nil {
		value = value[:loc[0]]
	}
	if loc := resolutionPattern.FindStringIndex(value); loc != nil {
		value = value[:loc[0]]
	}
	value = releaseTokens.ReplaceAllString(value, "")
	return cleanName(value)
}

func cleanFormattedName(value string) string {
	value = strings.NewReplacer("/", "-", `\`, "-", ":", " - ").Replace(value)
	value = spacePattern.ReplaceAllString(strings.TrimSpace(value), " ")
	value = strings.Trim(value, " .-")
	for strings.Contains(value, " -  - ") {
		value = strings.ReplaceAll(value, " -  - ", " - ")
	}
	return strings.Trim(value, " .-")
}

func titleCase(value string) string {
	words := strings.Fields(value)
	minorWords := map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "as": {}, "at": {}, "but": {}, "by": {},
		"for": {}, "in": {}, "nor": {}, "of": {}, "on": {}, "or": {}, "per": {},
		"the": {}, "to": {}, "via": {}, "vs": {},
	}
	for i, word := range words {
		if word == strings.ToUpper(word) && len(word) > 1 {
			continue
		}
		lower := strings.ToLower(word)
		if i > 0 {
			if _, minor := minorWords[lower]; minor {
				words[i] = lower
				continue
			}
		}
		runes := []rune(lower)
		if len(runes) > 0 {
			runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func lastYear(value string) (string, int) {
	matches := yearValuePattern.FindAllStringIndex(value, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		start, end := matches[i][0], matches[i][1]
		if tokenBoundary(value, start-1) && tokenBoundary(value, end) {
			return value[start:end], start
		}
	}
	return "", -1
}

func tokenBoundary(value string, index int) bool {
	if index < 0 || index >= len(value) {
		return true
	}
	return strings.ContainsRune(" ._()[]", rune(value[index]))
}

func seasonAndShowFromPath(path string) (int, string, bool) {
	directory := filepath.Dir(filepath.Clean(path))
	for directory != "." {
		base := filepath.Base(directory)
		match := seasonPathPattern.FindStringSubmatch(base)
		if len(match) > 1 {
			season, _ := strconv.Atoi(match[1])
			showDirectory := filepath.Dir(directory)
			showName := ""
			if showDirectory != directory && filepath.Base(showDirectory) != string(filepath.Separator) {
				showName = cleanName(filepath.Base(showDirectory))
			}
			return season, showName, true
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return 0, "", false
}

func showNameFromPath(path string) string {
	directory := filepath.Dir(filepath.Clean(path))
	if _, showName, found := seasonAndShowFromPath(path); found {
		return showName
	}
	if directory == "." || directory == string(filepath.Separator) {
		return ""
	}
	return cleanName(filepath.Base(directory))
}

func normaliseResolution(value string) string {
	switch strings.ToLower(value) {
	case "4k":
		return "2160p"
	case "8k":
		return "4320p"
	default:
		return strings.ToLower(value)
	}
}
