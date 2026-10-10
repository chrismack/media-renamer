package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Plan struct {
	Source string
	Target string
	Info   Info
}

type stagedRename struct {
	source string
	temp   string
	target string
}

type EpisodeMetadata struct {
	ShowName     string
	EpisodeTitle string
	ReleaseYear  string
}

type MetadataLookup interface {
	EpisodeMetadata(showName string, season, episode int) (EpisodeMetadata, error)
}

type Planner struct {
	TVFormat           string
	FilmFormat         string
	OrganiseShows      bool
	ShowFolderFormat   string
	OrganiseSeasons    bool
	SeasonFolderFormat string
	Lookup             MetadataLookup
}

func (p Planner) Build(paths []string) ([]Plan, error) {
	plans := make([]Plan, 0, len(paths))
	targets := make(map[string]string, len(paths))
	sources := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", path, err)
		}
		sources[canonicalPath(absolute)] = struct{}{}
	}

	for _, path := range paths {
		absolute, _ := filepath.Abs(path)
		info, err := Parse(absolute)
		if err != nil {
			return nil, err
		}
		format := p.FilmFormat
		if info.Kind == TV {
			format = p.TVFormat
		}
		if !formatMatchesKind(info.Kind, format) {
			plans = append(plans, Plan{Source: absolute, Target: absolute, Info: info})
			continue
		}
		effectiveTVFormat := format
		if effectiveTVFormat == "" {
			effectiveTVFormat = DefaultTVFormat
		}
		needsEpisodeTitle := info.EpisodeTitle == "" && containsAny(
			effectiveTVFormat,
			"<Episode Title>",
			"<Episode_Title>",
			"<Episode-Title>",
			"<EpisodeTitle>",
		)
		needsReleaseYear := info.ReleaseYear == "" && strings.Contains(effectiveTVFormat, "<Release Year>")
		if info.Kind == TV && (needsEpisodeTitle || needsReleaseYear) && p.Lookup != nil {
			metadata, lookupErr := p.Lookup.EpisodeMetadata(info.ShowName, info.Season, info.Episode)
			err = lookupErr
			if err != nil {
				return nil, fmt.Errorf("look up metadata for %q: %w", filepath.Base(path), err)
			}
			if info.ShowName == "" && metadata.ShowName != "" {
				info.ShowName = metadata.ShowName
			}
			if info.EpisodeTitle == "" {
				info.EpisodeTitle = metadata.EpisodeTitle
			}
			if info.ReleaseYear == "" {
				info.ReleaseYear = metadata.ReleaseYear
			}
		}
		name, err := Format(info, format)
		if err != nil {
			return nil, fmt.Errorf("format %q: %w", filepath.Base(path), err)
		}
		targetDirectory := filepath.Dir(absolute)
		if info.Kind == TV {
			if p.OrganiseShows {
				sourceInSeasonFolder := directoryIsSeason(filepath.Base(targetDirectory), info.Season)
				showFolder, showFormatErr := FormatShowFolder(info.ShowName, p.ShowFolderFormat)
				if showFormatErr != nil {
					return nil, fmt.Errorf("format show folder for %q: %w", filepath.Base(path), showFormatErr)
				}
				showParent := targetDirectory
				if sourceInSeasonFolder {
					showParent = filepath.Dir(targetDirectory)
				}
				if !strings.EqualFold(filepath.Base(showParent), showFolder) {
					showParent = filepath.Join(showParent, showFolder)
				}
				targetDirectory = showParent
				if sourceInSeasonFolder && !p.OrganiseSeasons {
					targetDirectory = filepath.Join(targetDirectory, filepath.Base(filepath.Dir(absolute)))
				}
			}

			if p.OrganiseSeasons {
				seasonFolder, formatErr := FormatSeasonFolder(info.Season, p.SeasonFolderFormat)
				if formatErr != nil {
					return nil, fmt.Errorf("format season folder for %q: %w", filepath.Base(path), formatErr)
				}
				if !strings.EqualFold(filepath.Base(targetDirectory), seasonFolder) {
					targetDirectory = filepath.Join(targetDirectory, seasonFolder)
				}
			}
		}
		target := filepath.Join(targetDirectory, name)
		key := canonicalPath(target)
		if previous, exists := targets[key]; exists && canonicalPath(previous) != canonicalPath(absolute) {
			return nil, fmt.Errorf("%q and %q would both become %q", previous, absolute, target)
		}
		targets[key] = absolute
		plans = append(plans, Plan{Source: absolute, Target: target, Info: info})
	}

	for _, plan := range plans {
		if samePath(plan.Source, plan.Target) {
			continue
		}
		if _, source := sources[canonicalPath(plan.Target)]; source {
			continue
		}
		if _, err := os.Lstat(plan.Target); err == nil {
			return nil, fmt.Errorf("target already exists: %q", plan.Target)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check target %q: %w", plan.Target, err)
		}
	}
	return plans, nil
}

func Apply(plans []Plan) error {
	staged := make([]stagedRename, 0, len(plans))
	for i, plan := range plans {
		if plan.Source == plan.Target {
			continue
		}
		temp := filepath.Join(filepath.Dir(plan.Source), fmt.Sprintf(".media-renamer-%d-%s", i, filepath.Base(plan.Source)))
		if _, err := os.Lstat(temp); !os.IsNotExist(err) {
			if err == nil {
				return rollbackStaged(staged, fmt.Errorf("temporary path already exists: %q", temp))
			}
			return rollbackStaged(staged, fmt.Errorf("check temporary path %q: %w", temp, err))
		}
		if err := os.Rename(plan.Source, temp); err != nil {
			return rollbackStaged(staged, fmt.Errorf("stage %q: %w", plan.Source, err))
		}
		staged = append(staged, stagedRename{source: plan.Source, temp: temp, target: plan.Target})
	}

	createdDirectories := make([]string, 0)
	seenDirectories := make(map[string]struct{})
	for _, rename := range staged {
		directory := filepath.Dir(rename.target)
		key := canonicalPath(directory)
		if _, seen := seenDirectories[key]; seen {
			continue
		}
		seenDirectories[key] = struct{}{}
		if _, err := os.Stat(directory); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			err = rollbackStaged(staged, err)
			return fmt.Errorf("check target directory %q: %w", directory, err)
		}
		newDirectories, err := createDirectoryTree(directory)
		if err != nil {
			err = rollbackStaged(staged, err)
			removeDirectories(createdDirectories)
			return fmt.Errorf("create target directory %q: %w", directory, err)
		}
		createdDirectories = append(createdDirectories, newDirectories...)
	}

	for i, rename := range staged {
		if err := os.Rename(rename.temp, rename.target); err != nil {
			for j := i - 1; j >= 0; j-- {
				_ = os.Rename(staged[j].target, staged[j].source)
			}
			for j := i; j < len(staged); j++ {
				_ = os.Rename(staged[j].temp, staged[j].source)
			}
			removeDirectories(createdDirectories)
			return fmt.Errorf("rename %q to %q: %w", rename.source, rename.target, err)
		}
	}
	return nil
}

func rollbackStaged(staged []stagedRename, cause error) error {
	for i := len(staged) - 1; i >= 0; i-- {
		_ = os.Rename(staged[i].temp, staged[i].source)
	}
	return cause
}

func canonicalPath(path string) string {
	return filepath.Clean(path)
}

func samePath(first, second string) bool {
	first = canonicalPath(first)
	second = canonicalPath(second)
	if first == second {
		return true
	}

	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	if firstErr != nil || secondErr != nil {
		return false
	}
	return os.SameFile(firstInfo, secondInfo)
}

func removeDirectories(directories []string) {
	for i := len(directories) - 1; i >= 0; i-- {
		_ = os.Remove(directories[i])
	}
}

func createDirectoryTree(directory string) ([]string, error) {
	var missing []string
	for current := directory; ; current = filepath.Dir(current) {
		if _, err := os.Stat(current); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}

	created := make([]string, 0, len(missing))
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o755); err != nil {
			removeDirectories(created)
			return nil, err
		}
		created = append(created, missing[i])
	}
	return created, nil
}

func directoryIsSeason(directory string, season int) bool {
	match := seasonPathPattern.FindStringSubmatch(directory)
	if len(match) < 2 {
		return false
	}
	number, err := strconv.Atoi(match[1])
	return err == nil && number == season
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func formatMatchesKind(kind Kind, format string) bool {
	if format == "" {
		return true
	}
	tvPlaceholders := []string{
		"<Show Name>", "<Show_Name>", "<Show-Name>", "<ShowName>",
		"<Season Number>", "<Episode Number>",
		"<Episode Title>", "<Episode_Title>", "<Episode-Title>", "<EpisodeTitle>",
	}
	filmPlaceholders := []string{"<Film Name>", "<Film_Name>", "<Film-Name>", "<FilmName>"}
	if kind == TV {
		return !containsAny(format, filmPlaceholders...)
	}
	return !containsAny(format, tvPlaceholders...)
}
