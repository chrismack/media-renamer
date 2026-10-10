package media

import (
	"fmt"
	"os"
	"path/filepath"
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
		effectiveTVFormat := format
		if effectiveTVFormat == "" {
			effectiveTVFormat = DefaultTVFormat
		}
		needsEpisodeTitle := info.EpisodeTitle == "" && strings.Contains(effectiveTVFormat, "<Episode Title>")
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
		if info.Kind == TV && p.OrganiseSeasons {
			folder, formatErr := FormatSeasonFolder(info.Season, p.SeasonFolderFormat)
			if formatErr != nil {
				return nil, fmt.Errorf("format season folder for %q: %w", filepath.Base(path), formatErr)
			}
			if !strings.EqualFold(filepath.Base(targetDirectory), folder) {
				targetDirectory = filepath.Join(targetDirectory, folder)
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
		if err := os.Mkdir(directory, 0o755); err != nil {
			err = rollbackStaged(staged, err)
			removeDirectories(createdDirectories)
			return fmt.Errorf("create target directory %q: %w", directory, err)
		}
		createdDirectories = append(createdDirectories, directory)
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
