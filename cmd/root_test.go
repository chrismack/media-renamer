package cmd

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("CLI", func() {
	It("includes examples for configurable string flags in help", func() {
		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{"--help"})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring(`Ted.Lasso.S04*`))
		Expect(output.String()).To(ContainSubstring(`<Show Name> - S<Season Number>E<Episode Number> - <Episode Title>`))
		Expect(output.String()).To(ContainSubstring(`Season <season-number>`))
		Expect(output.String()).To(ContainSubstring(`s-<season-number>`))
	})

	DescribeTable("shows detailed flag help topics",
		func(topic string, expected ...string) {
			var output bytes.Buffer
			command := newRootCommand(&output, &output)
			command.SetArgs([]string{"help", topic})
			Expect(command.Execute()).To(Succeed())
			for _, text := range expected {
				Expect(output.String()).To(ContainSubstring(text))
			}
		},
		Entry(
			"format",
			"format",
			"<Show Name>",
			"<Season Number>",
			"<Episode Number>",
			"<Episode Title>",
			"<Film Name>",
			"<Release Year>",
			"<Resolution>",
			"<File Extension>",
		),
		Entry("pattern", "pattern", "Matches any sequence", "Show.S0[34]E??.*"),
		Entry(
			"season folder format",
			"season-folder-format",
			"<season-number>",
			"cannot contain path separators",
		),
	)

	It("reports the available topics for unknown detailed help", func() {
		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{"help", "unknown"})
		err := command.Execute()
		Expect(err).To(MatchError(ContainSubstring(
			"available topics: format, pattern, season-folder-format",
		)))
	})

	It("shows root help when no detailed topic is supplied", func() {
		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{"help"})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Rename TV and film files consistently"))
	})

	It("uses a format from the configuration file during a dry run", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "arrival.2016.1080p.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		configPath := filepath.Join(directory, "config.yaml")
		Expect(os.WriteFile(
			configPath,
			[]byte("film_format: \"<Film Name> (<Release Year>) - <Resolution>\"\n"),
			0o600,
		)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{"--directory", directory, "--config", configPath, "--dry-run"})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Arrival (2016) - 1080p.mkv"))
		Expect(source).To(BeAnExistingFile())
	})

	It("gives a command-line format precedence over configuration", func() {
		directory := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(directory, "arrival.2016.mkv"), nil, 0o600)).To(Succeed())
		configPath := filepath.Join(directory, "config.yaml")
		Expect(os.WriteFile(configPath, []byte("format: \"Configured\"\n"), 0o600)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{
			"--directory", directory,
			"--config", configPath,
			"--format", "<Film Name> <Release Year>",
			"--dry-run",
		})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Arrival 2016.mkv"))
	})

	It("gives a shared command-line format precedence over type-specific configuration", func() {
		directory := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(directory, "arrival.2016.mkv"), nil, 0o600)).To(Succeed())
		configPath := filepath.Join(directory, "config.yaml")
		Expect(os.WriteFile(configPath, []byte(
			"film_format: \"Configured <Film Name>\"\n",
		), 0o600)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{
			"--directory", directory,
			"--config", configPath,
			"--format", "<Film Name> <Release Year>",
			"--dry-run",
		})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Arrival 2016.mkv"))
		Expect(output.String()).NotTo(ContainSubstring("Configured Arrival.mkv"))
	})

	It("reports malformed and explicitly missing configuration files", func() {
		directory := GinkgoT().TempDir()
		malformed := filepath.Join(directory, "malformed.yaml")
		Expect(os.WriteFile(malformed, []byte("format: ["), 0o600)).To(Succeed())

		for _, path := range []string{malformed, filepath.Join(directory, "missing.yaml")} {
			var output bytes.Buffer
			command := newRootCommand(&output, &output)
			command.SetArgs([]string{"--config", path, "--dry-run"})
			Expect(command.Execute()).To(HaveOccurred())
		}
	})

	It("reports when no supported media files match", func() {
		directory := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(directory, "notes.txt"), nil, 0o600)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{"--directory", directory})
		Expect(command.Execute()).To(MatchError("no supported media files matched"))
	})

	It("uses directory and pattern options with test_data fixtures", func() {
		directory := GinkgoT().TempDir()
		for _, fixture := range loadCommandFixtures() {
			if fixture.Group == "doctor-who" {
				Expect(os.WriteFile(filepath.Join(directory, fixture.Filename), nil, 0o600)).To(Succeed())
			}
		}

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{
			"--directory", directory,
			"--pattern", "*S01E01*",
			"--dry-run",
		})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring(
			"Doctor Who (2005) - S01E01 - Rose.mkv",
		))
		Expect(output.String()).NotTo(ContainSubstring("S04E10"))
	})

	It("organises shows using a custom season folder format", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s03e02.an.episode.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{
			"--directory", directory,
			"--organise-seasons",
			"--season-folder-format", "s-<season-number>",
		})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring(
			"show.s03e02.an.episode.mkv -> s-3/Show - S03E02 - An Episode.mkv",
		))
		Expect(filepath.Join(directory, "s-3", "Show - S03E02 - An Episode.mkv")).To(BeAnExistingFile())
		Expect(source).NotTo(BeAnExistingFile())
	})

	It("reads season organisation options from configuration", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s04e01.premiere.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		configPath := filepath.Join(directory, "config.yaml")
		Expect(os.WriteFile(configPath, []byte(
			"organise_seasons: true\nseason_folder_format: \"Season <season-number>\"\n",
		), 0o600)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{
			"--directory", directory,
			"--config", configPath,
			"--dry-run",
		})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring(
			"Season 4/Show - S04E01 - Premiere.mkv",
		))
		Expect(source).To(BeAnExistingFile())
	})

	It("allows command-line season options to override configuration", func() {
		directory := GinkgoT().TempDir()
		source := filepath.Join(directory, "show.s04e01.premiere.mkv")
		Expect(os.WriteFile(source, nil, 0o600)).To(Succeed())
		configPath := filepath.Join(directory, "config.yaml")
		Expect(os.WriteFile(configPath, []byte(
			"organise_seasons: true\nseason_folder_format: \"Configured <season-number>\"\n",
		), 0o600)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{
			"--directory", directory,
			"--config", configPath,
			"--organise-seasons=false",
			"--season-folder-format", "CLI <season-number>",
			"--dry-run",
		})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).NotTo(ContainSubstring("Configured 4"))
		Expect(output.String()).NotTo(ContainSubstring("CLI 4"))
		Expect(output.String()).To(ContainSubstring("Show - S04E01 - Premiere.mkv"))
	})

	It("processes media in child directories selected by a directory glob", func() {
		directory := GinkgoT().TempDir()
		season3 := filepath.Join(directory, "Season 3")
		season4 := filepath.Join(directory, "Season 4")
		Expect(os.Mkdir(season3, 0o755)).To(Succeed())
		Expect(os.Mkdir(season4, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(season3, "show.s03e01.first.mkv"), nil, 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(season4, "show.s04e01.second.mkv"), nil, 0o600)).To(Succeed())

		var output bytes.Buffer
		command := newRootCommand(&output, &output)
		command.SetArgs([]string{
			"--directory", filepath.Join(directory, "*"),
			"--dry-run",
		})
		Expect(command.Execute()).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Show - S03E01 - First.mkv"))
		Expect(output.String()).To(ContainSubstring("Show - S04E01 - Second.mkv"))
	})
})

type commandFixture struct {
	Group    string `yaml:"group"`
	Filename string `yaml:"filename"`
}

func loadCommandFixtures() []commandFixture {
	content, err := os.ReadFile(filepath.Join("..", "test_data", "fixtures.yaml"))
	Expect(err).NotTo(HaveOccurred())
	var catalogue struct {
		Fixtures []commandFixture `yaml:"fixtures"`
	}
	Expect(yaml.Unmarshal(content, &catalogue)).To(Succeed())
	return catalogue.Fixtures
}
