package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chris/media-renamer/internal/media"
	"github.com/spf13/cobra"
)

type options struct {
	configPath         string
	directory          string
	patterns           []string
	format             string
	tvFormat           string
	filmFormat         string
	tmdbAPIKey         string
	tmdbLang           string
	organiseShows      bool
	showFolderFormat   string
	organiseSeasons    bool
	seasonFolderFormat string
	dryRun             bool
}

func Execute() error {
	return newRootCommand(os.Stdout, os.Stderr).Execute()
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	opts := options{}
	command := &cobra.Command{
		Use:           "media-renamer [files or patterns...]",
		Short:         "Rename TV and film files consistently",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(command *cobra.Command, args []string) error {
			return run(command, opts, args)
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	flags := command.Flags()
	flags.StringVar(&opts.configPath, "config", "", "YAML configuration file (default media-renamer.yaml when present)")
	flags.StringVarP(&opts.directory, "directory", "d", ".", `directory or directory glob containing media files, e.g. "ted-lasso/*"`)
	flags.StringSliceVarP(&opts.patterns, "pattern", "p", nil, `file glob to select, e.g. "Ted.Lasso.S04*" (repeatable or comma-separated)`)
	flags.StringVarP(&opts.format, "format", "f", "", `naming format for all media, e.g. "<Show Name> - S<Season Number>E<Episode Number> - <Episode Title>"`)
	flags.StringVar(&opts.tvFormat, "tv-format", "", "naming format for TV episodes")
	flags.StringVar(&opts.filmFormat, "film-format", "", "naming format for films")
	flags.StringVar(&opts.tmdbAPIKey, "tmdb-api-key", "", "TMDb API key used to find missing episode titles")
	flags.StringVar(&opts.tmdbLang, "tmdb-language", "en-US", "TMDb result language")
	flags.BoolVar(&opts.organiseShows, "organise-shows", false, "move TV episodes into show directories")
	flags.StringVar(&opts.showFolderFormat, "show-folder-format", media.DefaultShowFolderFormat, `show directory format, e.g. "<Show Name>" or "<Show_Name>"`)
	flags.BoolVar(&opts.organiseSeasons, "organise-seasons", false, "move TV episodes into season directories")
	flags.StringVar(&opts.seasonFolderFormat, "season-folder-format", media.DefaultSeasonFolderFormat, `season directory format, e.g. "Season <season-number>" or "s-<season-number>"`)
	flags.BoolVarP(&opts.dryRun, "dry-run", "n", false, "show planned changes without renaming files")
	command.AddCommand(newHelpCommand())
	return command
}

func newHelpCommand() *cobra.Command {
	topics := map[string]string{
		"format": strings.TrimSpace(`
The --format (-f) option controls the output filename for both TV episodes and films.
Use --tv-format or --film-format to configure each media type separately.

Available placeholders:
  <Show Name>       Parsed, path-derived, or provider-supplied TV show name
  <Show_Name>       Show name with underscores instead of spaces
  <Show-Name>       Show name with hyphens instead of spaces
  <ShowName>        Show name with spaces removed
  <Season Number>   Zero-padded season number, e.g. 04
  <Episode Number>  Zero-padded episode number, e.g. 08
  <Episode Title>   Parsed or provider-supplied episode title
  <Episode_Title>   Episode title with underscores instead of spaces
  <Episode-Title>   Episode title with hyphens instead of spaces
  <EpisodeTitle>    Episode title with spaces removed
  <Film Name>       Parsed film name
  <Film_Name>       Film name with underscores instead of spaces
  <Film-Name>       Film name with hyphens instead of spaces
  <FilmName>        Film name with spaces removed
  <Release Year>    Parsed, path-derived, or provider-supplied release year
  <Resolution>      Resolution such as 720p, 1080p, or 2160p
  <File Extension>  Original extension without a leading dot

Defaults:
  TV:   <Show Name> - S<Season Number>E<Episode Number> - <Episode Title>
  Film: <Film Name>

If <File Extension> is omitted, the original extension is appended automatically.
Files are left unchanged when the selected format requires metadata for the other media type.

Examples:
  media-renamer -f "<Show Name> - S<Season Number>E<Episode Number> - <Episode Title>"
  media-renamer --tv-format "<Show_Name>-S<Season Number>E<Episode Number>-<Episode_Title>"
  media-renamer --tv-format "<Show Name> - <Season Number>x<Episode Number> - <Episode Title>"
  media-renamer --film-format "<Film Name> (<Release Year>) - <Resolution>"
`),
		"pattern": strings.TrimSpace(`
The --pattern (-p) option selects filenames using shell-style glob patterns.
It can be repeated or supplied as a comma-separated list.

Pattern syntax:
  *      Matches any sequence of characters
  ?      Matches one character
  [abc]  Matches one character from the set

Patterns are evaluated relative to --directory. Only supported media files are selected.

Examples:
  media-renamer -p "Ted.Lasso.S04*"
  media-renamer -p "*.mkv" -p "*.mp4"
  media-renamer -p "Show.S0[34]E??.*"
`),
		"season-folder-format": strings.TrimSpace(`
The --season-folder-format option controls season directory names when
--organise-seasons is enabled.

Available placeholders:
  <season-number>  Unpadded season number, e.g. 4
  <Season Number>  Alias for <season-number>

The format must produce one directory name and cannot contain path separators.

Examples:
  media-renamer --organise-seasons
  media-renamer --organise-seasons --season-folder-format "Season <season-number>"
  media-renamer --organise-seasons --season-folder-format "s-<season-number>"
  media-renamer --organise-seasons --season-folder-format "<season-number>"
`),
		"show-folder-format": strings.TrimSpace(`
The --show-folder-format option controls show directory names when
--organise-shows is enabled.

Available placeholders:
  <Show Name>  Words separated by spaces, e.g. Ted Lasso
  <Show_Name>  Words separated by underscores, e.g. Ted_Lasso
  <Show-Name>  Words separated by hyphens, e.g. Ted-Lasso
  <ShowName>   Words joined together, e.g. TedLasso

The format must produce one directory name and cannot contain path separators.

Examples:
  media-renamer --organise-shows
  media-renamer --organise-shows --show-folder-format "<Show_Name>"
  media-renamer --organise-shows --organise-seasons
`),
	}

	command := &cobra.Command{
		Use:       "help [topic]",
		Short:     "Help about media-renamer or a configuration topic",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"format", "pattern", "season-folder-format", "show-folder-format"},
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 {
				return command.Root().Help()
			}
			description, exists := topics[args[0]]
			if !exists {
				return fmt.Errorf(
					"unknown help topic %q; available topics: format, pattern, season-folder-format, show-folder-format",
					args[0],
				)
			}
			_, err := fmt.Fprintln(command.OutOrStdout(), description)
			if err != nil {
				return err
			}
			return nil
		},
	}
	return command
}

func run(command *cobra.Command, opts options, args []string) error {
	cfg, err := loadConfig(opts.configPath, command.Flags().Changed("config"))
	if err != nil {
		return err
	}
	applyConfig(command, &opts, cfg)

	files, err := media.Select(opts.directory, args, opts.patterns)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no supported media files matched")
	}

	planner := media.Planner{
		TVFormat:           opts.tvFormat,
		FilmFormat:         opts.filmFormat,
		OrganiseShows:      opts.organiseShows,
		ShowFolderFormat:   opts.showFolderFormat,
		OrganiseSeasons:    opts.organiseSeasons,
		SeasonFolderFormat: opts.seasonFolderFormat,
	}
	if opts.tmdbAPIKey != "" {
		planner.Lookup = &media.TMDbClient{APIKey: opts.tmdbAPIKey, Language: opts.tmdbLang}
	} else {
		planner.Lookup = &media.TVmazeClient{}
	}
	plans, err := planner.Build(files)
	if err != nil {
		return err
	}

	changed := 0
	for _, plan := range plans {
		if plan.Source == plan.Target {
			_, err := fmt.Fprintf(command.OutOrStdout(), "unchanged: %s\n", filepath.Base(plan.Source))
			if err != nil {
				return err
			}
			continue
		}
		changed++
		target, relativeErr := filepath.Rel(filepath.Dir(plan.Source), plan.Target)
		if relativeErr != nil {
			target = plan.Target
		}
		_, err := fmt.Fprintf(command.OutOrStdout(), "%s -> %s\n", filepath.Base(plan.Source), target)
		if err != nil {
			return err
		}
	}
	if opts.dryRun || changed == 0 {
		return nil
	}
	return media.Apply(plans)
}

func applyConfig(command *cobra.Command, opts *options, cfg config) {
	if !command.Flags().Changed("directory") && cfg.Directory != "" {
		opts.directory = cfg.Directory
	}
	if !command.Flags().Changed("pattern") && len(cfg.Patterns) > 0 {
		opts.patterns = cfg.Patterns
	}
	if !command.Flags().Changed("tmdb-api-key") {
		opts.tmdbAPIKey = firstNonEmpty(os.Getenv("TMDB_API_KEY"), cfg.TMDbAPIKey)
	}
	if !command.Flags().Changed("tmdb-language") && cfg.TMDbLanguage != "" {
		opts.tmdbLang = cfg.TMDbLanguage
	}
	if !command.Flags().Changed("organise-seasons") {
		opts.organiseSeasons = cfg.OrganiseSeasons
	}
	if !command.Flags().Changed("organise-shows") {
		opts.organiseShows = cfg.OrganiseShows
	}
	if !command.Flags().Changed("show-folder-format") && cfg.ShowFolderFormat != "" {
		opts.showFolderFormat = cfg.ShowFolderFormat
	}
	if !command.Flags().Changed("season-folder-format") && cfg.SeasonFolderFormat != "" {
		opts.seasonFolderFormat = cfg.SeasonFolderFormat
	}

	sharedFormat := opts.format
	if !command.Flags().Changed("format") {
		sharedFormat = cfg.Format
	}
	if command.Flags().Changed("tv-format") {
		// Keep the explicit value.
	} else if command.Flags().Changed("format") {
		opts.tvFormat = sharedFormat
	} else if cfg.TVFormat != "" {
		opts.tvFormat = cfg.TVFormat
	} else {
		opts.tvFormat = sharedFormat
	}
	if command.Flags().Changed("film-format") {
		// Keep the explicit value.
	} else if command.Flags().Changed("format") {
		opts.filmFormat = sharedFormat
	} else if cfg.FilmFormat != "" {
		opts.filmFormat = cfg.FilmFormat
	} else {
		opts.filmFormat = sharedFormat
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
