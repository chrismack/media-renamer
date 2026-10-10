package media

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Select(directory string, arguments, patterns []string) ([]string, error) {
	directories, err := resolveDirectories(directory)
	if err != nil {
		return nil, err
	}

	selectors := append(append([]string{}, arguments...), patterns...)
	var candidates []string
	if len(selectors) == 0 {
		for _, base := range directories {
			entries, readErr := os.ReadDir(base)
			if readErr != nil {
				return nil, fmt.Errorf("read directory %q: %w", base, readErr)
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					candidates = append(candidates, filepath.Join(base, entry.Name()))
				}
			}
		}
	} else {
		for _, base := range directories {
			for _, selector := range selectors {
				path := selector
				if !filepath.IsAbs(path) {
					path = filepath.Join(base, path)
				}
				if hasMeta(selector) {
					matches, globErr := filepath.Glob(path)
					if globErr != nil {
						return nil, fmt.Errorf("invalid pattern %q: %w", selector, globErr)
					}
					candidates = append(candidates, matches...)
				} else {
					candidates = append(candidates, path)
				}
			}
		}
	}

	seen := make(map[string]struct{})
	files := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			return nil, fmt.Errorf("access %q: %w", candidate, err)
		}
		if info.IsDir() || !IsMediaFile(candidate) {
			continue
		}
		absolute, _ := filepath.Abs(candidate)
		key := canonicalPath(absolute)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			files = append(files, absolute)
		}
	}
	sort.Strings(files)
	return files, nil
}

func resolveDirectories(selector string) ([]string, error) {
	path, err := filepath.Abs(selector)
	if err != nil {
		return nil, fmt.Errorf("resolve directory %q: %w", selector, err)
	}
	paths := []string{path}
	if hasMeta(selector) {
		paths, err = filepath.Glob(path)
		if err != nil {
			return nil, fmt.Errorf("invalid directory pattern %q: %w", selector, err)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("directory pattern %q matched no directories", selector)
		}
	}

	directories := make([]string, 0, len(paths))
	for _, candidate := range paths {
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			return nil, fmt.Errorf("access directory %q: %w", candidate, statErr)
		}
		if !info.IsDir() {
			if hasMeta(selector) {
				continue
			}
			return nil, fmt.Errorf("%q is not a directory", selector)
		}
		directories = append(directories, candidate)
	}
	if len(directories) == 0 {
		return nil, fmt.Errorf("directory pattern %q matched no directories", selector)
	}
	sort.Strings(directories)
	return directories, nil
}

func hasMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}
