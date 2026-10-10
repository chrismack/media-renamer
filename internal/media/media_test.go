package media

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

type fixtureExpectation struct {
	Group      string `yaml:"group"`
	Path       string `yaml:"path"`
	Filename   string `yaml:"filename"`
	Kind       Kind   `yaml:"kind"`
	Name       string `yaml:"name"`
	Season     int    `yaml:"season"`
	Episode    int    `yaml:"episode"`
	Title      string `yaml:"title"`
	Year       string `yaml:"year"`
	Resolution string `yaml:"resolution"`
	Formatted  string `yaml:"formatted"`
}

var fixtureExpectations = mustLoadFixtures()

var _ = Describe("Media filenames", func() {
	DescribeTable("parses and formats test_data fixtures", fixtureTableArgs()...)

	It("contains no more than five shows and five films", func() {
		showCollections := make(map[string]struct{})
		filmNames := make(map[string]struct{})
		for _, expected := range fixtureExpectations {
			if expected.Kind == TV {
				showCollections[expected.Group] = struct{}{}
			} else {
				filmNames[expected.Name] = struct{}{}
			}
		}
		Expect(showCollections).To(HaveLen(5))
		Expect(filmNames).To(HaveLen(5))
	})

	DescribeTable("recognises supported media extensions",
		func(filename string, expected bool) {
			Expect(IsMediaFile(filename)).To(Equal(expected))
		},
		Entry("uppercase extension", "episode.MKV", true),
		Entry("transport stream", "episode.m2ts", true),
		Entry("unsupported subtitle", "episode.srt", false),
		Entry("no extension", "episode", false),
	)

	It("rejects unsupported files and filenames without a media name", func() {
		_, err := Parse("notes.txt")
		Expect(err).To(MatchError(ContainSubstring("not a supported media file")))
		_, err = Parse(".mkv")
		Expect(err).To(MatchError(ContainSubstring("could not determine film name")))
	})

	DescribeTable("splits compact show names into words",
		func(filename, expected string) {
			info, err := Parse(filename)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Kind).To(Equal(TV))
			Expect(info.ShowName).To(Equal(expected))
		},
		Entry("camel case", "TedLasso-s04e11.mkv", "Ted Lasso"),
		Entry("acronym boundary", "BBCNews-s01e01.mkv", "BBC News"),
		Entry("numeric boundary", "3BodyProblem-s01e01.mkv", "3 Body Problem"),
	)
})

var _ = Describe("Formatting and renaming", func() {
	It("supports custom formats and preserves an omitted extension", func() {
		info := Info{
			Kind: TV, ShowName: "Doctor Who", Season: 4, Episode: 10,
			EpisodeTitle: "Midnight", ReleaseYear: "2008", Resolution: "1080p", Extension: "mkv",
		}
		name, err := Format(info, "<Show Name> - S<Season Number>E<Episode Number> - <Episode Title> - <Resolution>")
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("Doctor Who - S04E10 - Midnight - 1080p.mkv"))
	})

	It("supports explicit extensions and sanitises filename separators", func() {
		info := Info{Kind: Film, FilmName: "A/B: Story", ReleaseYear: "2024", Extension: "mkv"}
		name, err := Format(info, "<Film Name> (<Release Year>).<File Extension>")
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("A-B - Story (2024).mkv"))
	})

	DescribeTable("formats text placeholder separators in filenames",
		func(format, expected string) {
			info := Info{
				Kind: TV, ShowName: "Ted Lasso", Season: 4, Episode: 11,
				EpisodeTitle: "No Weddings and a Funeral", Extension: "mkv",
			}
			name, err := Format(info, format)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal(expected))
		},
		Entry(
			"underscores",
			"<Show_Name>-S<Season Number>E<Episode Number>-<Episode_Title>",
			"Ted_Lasso-S04E11-No_Weddings_and_a_Funeral.mkv",
		),
		Entry(
			"hyphens",
			"<Show-Name>-S<Season Number>E<Episode Number>-<Episode-Title>",
			"Ted-Lasso-S04E11-No-Weddings-and-a-Funeral.mkv",
		),
		Entry(
			"compact",
			"<ShowName>-S<Season Number>E<Episode Number>-<EpisodeTitle>",
			"TedLasso-S04E11-NoWeddingsandaFuneral.mkv",
		),
		Entry(
			"spaces",
			"<Show Name>-S<Season Number>E<Episode Number>-<Episode Title>",
			"Ted Lasso-S04E11-No Weddings and a Funeral.mkv",
		),
	)

	It("formats film name separators in filenames", func() {
		info := Info{Kind: Film, FilmName: "Dune Part Two", Extension: "mkv"}
		name, err := Format(info, "<Film_Name>")
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("Dune_Part_Two.mkv"))
	})

	It("rejects unknown placeholders and empty output", func() {
		_, err := Format(Info{Kind: Film, FilmName: "Film", Extension: "mkv"}, "<Unknown>")
		Expect(err).To(MatchError(ContainSubstring("unknown placeholder")))
		_, err = Format(Info{Kind: Film, Extension: "mkv"}, "<Film Name>")
		Expect(err).To(MatchError("format produced an empty file name"))
	})

	DescribeTable("normalises resolution aliases",
		func(filename, expected string) {
			info, err := Parse(filename)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Resolution).To(Equal(expected))
		},
		Entry("4K", "Film.2024.4K.mkv", "2160p"),
		Entry("8K", "Film.2024.8k.mkv", "4320p"),
	)

	DescribeTable("formats season directory names",
		func(format string, season int, expected string) {
			folder, err := FormatSeasonFolder(season, format)
			Expect(err).NotTo(HaveOccurred())
			Expect(folder).To(Equal(expected))
		},
		Entry("default", "", 3, "Season 3"),
		Entry("lowercase placeholder", "season-<season-number>", 4, "season-4"),
		Entry("legacy placeholder", "s-<Season Number>", 12, "s-12"),
		Entry("number only", "<season-number>", 2, "2"),
	)

	It("rejects season directory formats containing paths", func() {
		_, err := FormatSeasonFolder(3, "shows/<season-number>")
		Expect(err).To(MatchError(ContainSubstring("single directory name")))
	})

	It("rejects invalid season directory placeholders and empty names", func() {
		_, err := FormatSeasonFolder(3, "<episode-number>")
		Expect(err).To(MatchError(ContainSubstring("unknown placeholder")))
		_, err = FormatSeasonFolder(3, "---")
		Expect(err).To(MatchError(ContainSubstring("empty directory name")))
	})

	DescribeTable("formats show directory names",
		func(format, expected string) {
			folder, err := FormatShowFolder("Ted Lasso", format)
			Expect(err).NotTo(HaveOccurred())
			Expect(folder).To(Equal(expected))
		},
		Entry("default", "", "Ted Lasso"),
		Entry("spaces", "<Show Name>", "Ted Lasso"),
		Entry("underscores", "<Show_Name>", "Ted_Lasso"),
		Entry("hyphens", "<Show-Name>", "Ted-Lasso"),
		Entry("compact", "<ShowName>", "TedLasso"),
	)

	It("rejects invalid show directory formats", func() {
		_, err := FormatShowFolder("Ted Lasso", "shows/<Show Name>")
		Expect(err).To(MatchError(ContainSubstring("single directory name")))
		_, err = FormatShowFolder("Ted Lasso", "<Unknown>")
		Expect(err).To(MatchError(ContainSubstring("unknown placeholder")))
	})

	It("rejects a target that already exists", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s01e01.pilot.mkv")
		target := filepath.Join(directory, "Show - S01E01 - Pilot.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(target, nil, 0o600)).To(Succeed())

		_, err := (Planner{}).Build([]string{source})
		Expect(err).To(MatchError(ContainSubstring("target already exists")))
	})

	It("does not treat case-only distinct paths as unchanged", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "film.mkv")
		target := filepath.Join(directory, "Film.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(target, nil, 0o600)).To(Succeed())
		sourceInfo, err := os.Stat(source)
		Expect(err).NotTo(HaveOccurred())
		targetInfo, err := os.Stat(target)
		Expect(err).NotTo(HaveOccurred())
		if os.SameFile(sourceInfo, targetInfo) {
			Skip("filesystem is case-insensitive")
		}

		_, err = (Planner{}).Build([]string{source})
		Expect(err).To(MatchError(ContainSubstring("target already exists")))
	})

	It("rejects duplicate targets within a batch", func() {
		directory := GinkgoT().TempDir()
		first := filepath.Join(directory, "show.s01e01.pilot.mkv")
		second := filepath.Join(directory, "Show.S1E1.Pilot.mkv")
		Expect(os.WriteFile(first, nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(second, nil, 0o600)).To(Succeed())

		_, err := (Planner{}).Build([]string{first, second})
		Expect(err).To(MatchError(ContainSubstring("would both become")))
	})

	It("leaves film-classified videos unchanged when the format requires TV metadata", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "WC139.mp4")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{
			FilmFormat: "S<Season Number>E<Episode Number>",
		}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(plans).To(HaveLen(1))
		Expect(plans[0].Info.Kind).To(Equal(Film))
		Expect(plans[0].Target).To(Equal(source))
		Expect(Apply(plans)).To(Succeed())
		Expect(source).To(BeAnExistingFile())
		Expect(filepath.Join(directory, "S00E00.mp4")).NotTo(BeAnExistingFile())
	})

	It("leaves TV episodes unchanged when the format requires film metadata", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s01e01.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{TVFormat: "<Film Name>"}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(plans[0].Target).To(Equal(source))
	})

	It("uses a title lookup when an episode title is missing", func() {
		directory := GinkgoT().TempDir()
		seasonDirectory := filepath.Join(directory, "show", "Season 1")
		Expect(os.MkdirAll(seasonDirectory, 0o755)).To(Succeed())
		source := filepath.Join(seasonDirectory, "E02.1080p.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		lookup := metadataLookupFunc(func(show string, season, episode int) (EpisodeMetadata, error) {
			Expect(show).To(Equal("Show"))
			Expect(season).To(Equal(1))
			Expect(episode).To(Equal(2))
			return EpisodeMetadata{
				ShowName:     "The Show",
				EpisodeTitle: "Fetched Title",
				ReleaseYear:  "2020",
			}, nil
		})

		plans, err := (Planner{
			Lookup:   lookup,
			TVFormat: "<Show Name> - <Release Year> - S<Season Number>E<Episode Number> - <Episode Title>",
		}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Base(plans[0].Target)).To(Equal("Show - 2020 - S01E02 - Fetched Title.mkv"))
	})

	It("uses a title lookup for separator variants when an episode title is missing", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s01e02.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		lookup := metadataLookupFunc(func(string, int, int) (EpisodeMetadata, error) {
			return EpisodeMetadata{EpisodeTitle: "Fetched Title"}, nil
		})

		plans, err := (Planner{
			Lookup:   lookup,
			TVFormat: "<Show_Name>-S<Season Number>E<Episode Number>-<Episode_Title>",
		}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Base(plans[0].Target)).To(Equal("Show-S01E02-Fetched_Title.mkv"))
	})

	It("surfaces title lookup errors", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s01e02.1080p.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		lookup := metadataLookupFunc(func(string, int, int) (EpisodeMetadata, error) {
			return EpisodeMetadata{}, fmt.Errorf("provider unavailable")
		})

		_, err := (Planner{Lookup: lookup}).Build([]string{source})
		Expect(err).To(MatchError(ContainSubstring("provider unavailable")))
	})

	It("fetches a release year requested by the format without replacing an existing title", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s01e02.existing.title.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		lookup := metadataLookupFunc(func(string, int, int) (EpisodeMetadata, error) {
			return EpisodeMetadata{
				ShowName:     "Canonical Show",
				EpisodeTitle: "Provider Title",
				ReleaseYear:  "2021",
			}, nil
		})

		plans, err := (Planner{
			Lookup:   lookup,
			TVFormat: "<Show Name> - <Release Year> - <Episode Title>",
		}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Base(plans[0].Target)).To(Equal("Show - 2021 - Existing Title.mkv"))
	})

	It("does not query metadata that is not required by the format", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s01e02.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		called := false
		lookup := metadataLookupFunc(func(string, int, int) (EpisodeMetadata, error) {
			called = true
			return EpisodeMetadata{}, nil
		})

		_, err := (Planner{
			Lookup:   lookup,
			TVFormat: "<Show Name> - S<Season Number>E<Episode Number>",
		}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(called).To(BeFalse())
	})

	It("applies a batch rename", func() {
		directory := GinkgoT().TempDir()
		paths := []string{
			filepath.Join(directory, "show.s01e01.pilot.mkv"),
			filepath.Join(directory, "show.s01e02.second.part.mkv"),
		}
		for _, path := range paths {
			Expect(os.WriteFile(path, []byte(filepath.Base(path)), 0o600)).To(Succeed())
		}

		plans, err := (Planner{}).Build(paths)
		Expect(err).NotTo(HaveOccurred())
		Expect(Apply(plans)).To(Succeed())
		Expect(filepath.Join(directory, "Show - S01E01 - Pilot.mkv")).To(BeAnExistingFile())
		Expect(filepath.Join(directory, "Show - S01E02 - Second Part.mkv")).To(BeAnExistingFile())
	})

	It("organises TV episodes into season directories", func() {
		directory := GinkgoT().TempDir()
		paths := []string{
			filepath.Join(directory, "show.s03e01.first.mkv"),
			filepath.Join(directory, "show.s04e01.second.mkv"),
		}
		for _, path := range paths {
			Expect(os.WriteFile(path, nil, 0o600)).To(Succeed())
		}

		planner := Planner{
			OrganiseSeasons:    true,
			SeasonFolderFormat: "season-<season-number>",
		}
		plans, err := planner.Build(paths)
		Expect(err).NotTo(HaveOccurred())
		Expect(plans[0].Target).To(Equal(filepath.Join(directory, "season-3", "Show - S03E01 - First.mkv")))
		Expect(plans[1].Target).To(Equal(filepath.Join(directory, "season-4", "Show - S04E01 - Second.mkv")))
		Expect(Apply(plans)).To(Succeed())
		Expect(plans[0].Target).To(BeAnExistingFile())
		Expect(plans[1].Target).To(BeAnExistingFile())
	})

	It("organises TV episodes into formatted show and season directories", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "Ted_Lasso-s04e11.rajbfakjbdjk.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{
			OrganiseShows:      true,
			ShowFolderFormat:   "<Show_Name>",
			OrganiseSeasons:    true,
			SeasonFolderFormat: "Season <season-number>",
		}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(plans[0].Target).To(Equal(filepath.Join(
			directory,
			"Ted_Lasso",
			"Season 4",
			"Ted Lasso - S04E11 - Rajbfakjbdjk.mkv",
		)))
		Expect(Apply(plans)).To(Succeed())
		Expect(plans[0].Target).To(BeAnExistingFile())
	})

	It("organises shows without creating season directories when season organisation is disabled", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "ted.lasso.s04e11.episode.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{OrganiseShows: true}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(plans[0].Target).To(Equal(filepath.Join(
			directory,
			"Ted Lasso",
			"Ted Lasso - S04E11 - Episode.mkv",
		)))
	})

	It("does not nest an episode already inside its show and season directories", func() {
		directory := GinkgoT().TempDir()
		seasonDirectory := filepath.Join(directory, "Ted Lasso", "Season 4")
		Expect(os.MkdirAll(seasonDirectory, 0o755)).To(Succeed())
		source := filepath.Join(seasonDirectory, "Ted Lasso - S04E11 - Episode.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{OrganiseShows: true, OrganiseSeasons: true}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(plans[0].Target).To(Equal(source))
	})

	It("does nothing when an episode is already correctly named in its season directory", func() {
		directory := GinkgoT().TempDir()
		seasonDirectory := filepath.Join(directory, "Season 3")
		Expect(os.Mkdir(seasonDirectory, 0o755)).To(Succeed())
		source := filepath.Join(seasonDirectory, "Show - S03E01 - First.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{OrganiseSeasons: true}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(plans).To(HaveLen(1))
		Expect(plans[0].Target).To(Equal(source))
		Expect(Apply(plans)).To(Succeed())
		Expect(source).To(BeAnExistingFile())
		Expect(filepath.Join(seasonDirectory, "Season 3")).NotTo(BeADirectory())
	})

	It("renames without nesting when already inside the configured season directory", func() {
		directory := GinkgoT().TempDir()
		seasonDirectory := filepath.Join(directory, "s-4")
		Expect(os.Mkdir(seasonDirectory, 0o755)).To(Succeed())
		source := filepath.Join(seasonDirectory, "show.s04e02.second.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{
			OrganiseSeasons:    true,
			SeasonFolderFormat: "s-<season-number>",
		}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(plans[0].Target).To(Equal(filepath.Join(seasonDirectory, "Show - S04E02 - Second.mkv")))
		Expect(Apply(plans)).To(Succeed())
		Expect(plans[0].Target).To(BeAnExistingFile())
		Expect(filepath.Join(seasonDirectory, "s-4")).NotTo(BeADirectory())
	})

	It("does not organise films into season directories", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "arrival.2016.1080p.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		plans, err := (Planner{OrganiseSeasons: true}).Build([]string{source})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Dir(plans[0].Target)).To(Equal(directory))
	})

	It("selects patterns and removes duplicates", func() {
		directory := GinkgoT().TempDir()
		for _, name := range []string{"show.s01e01.mkv", "show.s02e01.mp4", "notes.txt"} {
			Expect(os.WriteFile(filepath.Join(directory, name), nil, 0o600)).To(Succeed())
		}

		files, err := Select(directory, []string{"show.s01*"}, []string{"*.mkv"})
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf(filepath.Join(directory, "show.s01e01.mkv")))
	})

	It("selects media from every directory matched by a directory glob", func() {
		directory := GinkgoT().TempDir()
		season3 := filepath.Join(directory, "Season 3")
		season4 := filepath.Join(directory, "Season 4")
		Expect(os.Mkdir(season3, 0o755)).To(Succeed())
		Expect(os.Mkdir(season4, 0o755)).To(Succeed())
		episode3 := filepath.Join(season3, "show.s03e01.first.mkv")
		episode4 := filepath.Join(season4, "show.s04e01.second.mp4")
		Expect(os.WriteFile(episode3, nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(episode4, nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(directory, "not-a-directory.txt"), nil, 0o600)).To(Succeed())

		files, err := Select(filepath.Join(directory, "*"), nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf(episode3, episode4))
	})

	DescribeTable("reports invalid selection inputs",
		func(setup func(string) string, expected string) {
			directory := GinkgoT().TempDir()
			selector := setup(directory)
			_, err := Select(selector, nil, nil)
			Expect(err).To(MatchError(ContainSubstring(expected)))
		},
		Entry("missing directory", func(directory string) string {
			return filepath.Join(directory, "missing")
		}, "access directory"),
		Entry("file instead of directory", func(directory string) string {
			path := filepath.Join(directory, "file.mkv")
			Expect(os.WriteFile(path, nil, 0o600)).To(Succeed())
			return path
		}, "is not a directory"),
		Entry("unmatched directory glob", func(directory string) string {
			return filepath.Join(directory, "Season *")
		}, "matched no directories"),
	)

	It("reports a missing explicit file", func() {
		directory := GinkgoT().TempDir()
		_, err := Select(directory, []string{"missing.mkv"}, nil)
		Expect(err).To(MatchError(ContainSubstring("access")))
	})

	It("rolls back staged files when a later temporary file conflicts", func() {
		directory := GinkgoT().TempDir()
		first := filepath.Join(directory, "first.mkv")
		second := filepath.Join(directory, "second.mkv")
		conflict := filepath.Join(directory, ".media-renamer-1-second.mkv")
		Expect(os.WriteFile(first, nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(second, nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(conflict, nil, 0o600)).To(Succeed())

		err := Apply([]Plan{
			{Source: first, Target: filepath.Join(directory, "renamed-first.mkv")},
			{Source: second, Target: filepath.Join(directory, "renamed-second.mkv")},
		})
		Expect(err).To(MatchError(ContainSubstring("temporary path already exists")))
		Expect(first).To(BeAnExistingFile())
		Expect(second).To(BeAnExistingFile())
	})

	It("creates nested target directories", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "episode.mkv")
		target := filepath.Join(directory, "show", "season", "episode.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		Expect(Apply([]Plan{{Source: source, Target: target}})).To(Succeed())
		Expect(target).To(BeAnExistingFile())
	})
})

var _ = Describe("Metadata providers", func() {
	It("retrieves an episode title from TMDb", func() {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/search/tv":
				_, err := fmt.Fprint(writer, `{"results":[{"id":95396,"name":"Severance","first_air_date":"2022-02-18"}]}`)
				if err != nil {
					return
				}
			case "/tv/95396/season/1/episode/2":
				_, err := fmt.Fprint(writer, `{"name":"Half Loop"}`)
				if err != nil {
					return
				}
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()

		client := TMDbClient{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()}
		metadata, err := client.EpisodeMetadata("Severance", 1, 2)
		Expect(err).NotTo(HaveOccurred())
		Expect(metadata.ShowName).To(Equal("Severance"))
		Expect(metadata.EpisodeTitle).To(Equal("Half Loop"))
		Expect(metadata.ReleaseYear).To(Equal("2022"))
	})

	It("requires a TMDb API key", func() {
		_, err := (&TMDbClient{}).EpisodeMetadata("Severance", 1, 2)
		Expect(err).To(MatchError("TMDb API key is required"))
	})

	DescribeTable("reports invalid TMDb responses",
		func(searchResponse string, searchStatus int, episodeResponse string, expected string) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/search/tv" {
					writer.WriteHeader(searchStatus)
					_, err := fmt.Fprint(writer, searchResponse)
					if err != nil {
						return
					}
					return
				}
				_, err := fmt.Fprint(writer, episodeResponse)
				if err != nil {
					return
				}
			}))
			defer server.Close()

			client := TMDbClient{APIKey: "key", BaseURL: server.URL, HTTPClient: server.Client()}
			_, err := client.EpisodeMetadata("Show", 1, 1)
			Expect(err).To(MatchError(ContainSubstring(expected)))
		},
		Entry("HTTP error", `{}`, http.StatusServiceUnavailable, `{}`, "HTTP 503"),
		Entry("malformed search", `{`, http.StatusOK, `{}`, "decode TMDb response"),
		Entry("show not found", `{"results":[]}`, http.StatusOK, `{}`, "was not found"),
		Entry("empty episode title", `{"results":[{"id":1}]}`, http.StatusOK, `{"name":""}`, "returned no title"),
	)

	It("retrieves TVmaze titles and caches the show ID", func() {
		searchCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/search/shows":
				searchCalls++
				_, err := fmt.Fprint(writer, `[
					{"score":2,"show":{"id":1,"name":"Ted Lasso UK","premiered":"2019-01-01"}},
					{"score":1,"show":{"id":44458,"name":"Ted Lasso","premiered":"2020-08-14"}}
				]`)
				if err != nil {
					return
				}
			case "/shows/44458/episodebynumber":
				titles := map[string]string{"4": "Greyhounds' Day Off", "8": "Follow the Anger"}
				title, exists := titles[request.URL.Query().Get("number")]
				if !exists {
					http.NotFound(writer, request)
					return
				}
				_, err := fmt.Fprintf(writer, `{"name":%q}`, title)
				if err != nil {
					return
				}
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()

		client := TVmazeClient{BaseURL: server.URL, HTTPClient: server.Client()}
		metadata, err := client.EpisodeMetadata("Ted Lasso", 4, 4)
		Expect(err).NotTo(HaveOccurred())
		Expect(metadata.ShowName).To(Equal("Ted Lasso"))
		Expect(metadata.EpisodeTitle).To(Equal("Greyhounds' Day Off"))
		Expect(metadata.ReleaseYear).To(Equal("2020"))
		metadata, err = client.EpisodeMetadata("Ted Lasso", 4, 8)
		Expect(err).NotTo(HaveOccurred())
		Expect(metadata.EpisodeTitle).To(Equal("Follow the Anger"))
		Expect(searchCalls).To(Equal(1))
	})

	DescribeTable("reports invalid TVmaze responses",
		func(searchResponse string, searchStatus int, episodeResponse string, expected string) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/search/shows" {
					writer.WriteHeader(searchStatus)
					_, err := fmt.Fprint(writer, searchResponse)
					if err != nil {
						return
					}
					return
				}
				_, err := fmt.Fprint(writer, episodeResponse)
				if err != nil {
					return
				}
			}))
			defer server.Close()

			client := TVmazeClient{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
				UserAgent:  "media-renamer-tests",
			}
			_, err := client.EpisodeMetadata("Show", 1, 1)
			Expect(err).To(MatchError(ContainSubstring(expected)))
		},
		Entry("HTTP error", `[]`, http.StatusTooManyRequests, `{}`, "HTTP 429"),
		Entry("malformed search", `[`, http.StatusOK, `{}`, "decode TVmaze response"),
		Entry("show not found", `[]`, http.StatusOK, `{}`, "was not found"),
		Entry("empty episode title", `[{"show":{"id":1,"name":"Show"}}]`, http.StatusOK, `{"name":""}`, "returned no title"),
	)
})

func fixtureTableArgs() []any {
	args := []any{func(expected fixtureExpectation) {
		source := expected.Filename
		if expected.Path != "" {
			source = filepath.FromSlash(expected.Path)
		}
		info, err := Parse(source)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Kind).To(Equal(expected.Kind))
		Expect(info.Season).To(Equal(expected.Season))
		Expect(info.Episode).To(Equal(expected.Episode))
		Expect(info.EpisodeTitle).To(Equal(expected.Title))
		Expect(info.ReleaseYear).To(Equal(expected.Year))
		Expect(info.Resolution).To(Equal(expected.Resolution))
		if expected.Kind == TV {
			Expect(info.ShowName).To(Equal(expected.Name))
		} else {
			Expect(info.FilmName).To(Equal(expected.Name))
		}

		name, err := Format(info, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal(expected.Formatted))
	}}
	for _, expected := range fixtureExpectations {
		args = append(args, Entry(expected.Group+": "+expected.Filename, expected))
	}
	return args
}

func mustLoadFixtures() []fixtureExpectation {
	content, err := os.ReadFile(filepath.Join("..", "..", "test_data", "fixtures.yaml"))
	if err != nil {
		panic(err)
	}
	var catalogue struct {
		Fixtures []fixtureExpectation `yaml:"fixtures"`
	}
	if err := yaml.Unmarshal(content, &catalogue); err != nil {
		panic(err)
	}
	return catalogue.Fixtures
}

type metadataLookupFunc func(showName string, season, episode int) (EpisodeMetadata, error)

func (f metadataLookupFunc) EpisodeMetadata(showName string, season, episode int) (EpisodeMetadata, error) {
	return f(showName, season, episode)
}
