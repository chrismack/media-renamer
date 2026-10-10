package main

import (
	"os"

	"github.com/chris/media-renamer/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
