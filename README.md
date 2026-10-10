# Media Renamer

[![CI](https://github.com/chrismack/media-renamer/actions/workflows/ci.yml/badge.svg)](https://github.com/chrismack/media-renamer/actions/workflows/ci.yml)

`media-renamer` standardises TV episode and film filenames while preserving their file extensions.

## Usage

```console
go build .

# Preview all supported media files in the current directory.
./media-renamer --dry-run

# Rename files selected from another directory.
./media-renamer --directory /media/tv --pattern 'show.name.s04*'

# Process files directly inside every matched child directory.
./media-renamer --directory '/media/tv/ted-lasso/*'

# Select explicit files and use a custom format.
./media-renamer episode.mkv --format '<Show Name> - S<Season Number>E<Episode Number> - <Episode Title> - <Resolution>.<File Extension>'

# Move TV episodes into directories such as "Season 3".
./media-renamer --organise-seasons

# Customise the season directory name.
./media-renamer --organise-seasons --season-folder-format 's-<season-number>'
```

By default, TV episodes become `<Show Name> - S<Season Number>E<Episode Number> - <Episode Title>` and films become
`<Film Name>`. The original extension is appended automatically when `<File Extension>` is not in the format.

Supported placeholders are `<Show Name>`, `<Season Number>`, `<Episode Number>`, `<Episode Title>`, `<Film Name>`,
`<Release Year>`, `<Resolution>`, and `<File Extension>`.

Detailed help for complex options is available through focused help topics:

```console
media-renamer help format
media-renamer help pattern
media-renamer help season-folder-format
```

## Configuration

Use `--config path/to/config.yaml`. If present, `media-renamer.yaml` in the current directory is loaded automatically.
Command-line options take precedence.

```yaml
directory: /media
patterns:
  - "*.mkv"
tv_format: "<Show Name> - S<Season Number>E<Episode Number> - <Episode Title>"
film_format: "<Film Name> (<Release Year>) - <Resolution>"
tmdb_api_key: "your-api-key"
tmdb_language: "en-US"
organise_seasons: true
season_folder_format: "Season <season-number>"
```

Season organisation only affects TV episodes. The default directory format is `Season <season-number>`, and the
placeholder is not zero-padded: season `S03` becomes `Season 3`. Both `<season-number>` and `<Season Number>` are
accepted in season directory formats. Files already inside the resulting season directory are not nested again. Use
`--dry-run` to preview the directory structure before moving files.

The `--directory` option accepts glob patterns. Quote them so the application, rather than the shell, expands the
pattern. Each matching directory is scanned for media files; scanning is not recursive beyond those matched directories.

## Path-derived metadata

Missing TV metadata can be inferred from the directory path. For example:

```text
ted-lasso/Season 4/E01.mkv
```

provides the show name `Ted Lasso`, season `4`, and episode `1`. Season directories may use `Season 4`, `S04`, `s-4`, or
just `4`; episode-only filenames may use forms such as `E01`, `EP01`, or `Episode 1`. A release year in the show
directory, such as `Doctor Who (2005)/Season 4/E10.mkv`, is also available to `<Release Year>`.

When an episode title or release year required by the selected format is missing, TVmaze is queried by default. Its
episode title and show premiere year fill fields that are still missing. Configuring a TMDb key uses TMDb for the same
enrichment. The key can be supplied with `--tmdb-api-key`, `TMDB_API_KEY`, or `tmdb_api_key` in the configuration file.
Metadata already present in the filename or path is retained, and fields not used by the format do not trigger a lookup.

TMDb is The Movie Database and is a separate service from IMDb, so TMDb options deliberately use the `tmdb` name.
TVmaze data is available under the [CC BY-SA 4.0 licence](https://www.tvmaze.com/api) and is attributed to TVmaze.

## Tests

The test suites use [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/):

```console
go test ./...
```

Fixture-driven coverage is stored in `test_data/fixtures.yaml`. Each entry contains the source filename and its expected
parsed metadata and formatted filename, keeping test inputs and expectations together. The catalogue includes five TV
shows and five films with varied separators, episode-number formats, numeric titles, parenthetical qualifiers,
resolutions, release tags, and missing metadata.
