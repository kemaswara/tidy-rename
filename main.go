package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

type AudioFile struct {
	OriginalPath string
	OriginalName string
	Category     string
	SubCategory  string
	Source       string
	ID           string
	NewName      string
	Tags         []string
	AudioMeta    *AudioMetadata `json:"audio_metadata,omitempty"`
}

type Config struct {
	SourceDir      string
	OutputDir      string
	PackName       string
	DryRun         bool
	Organize       bool
	CreateManifest bool
}

var (
	version = "dev" // set at build time with -ldflags
)

func main() {
	var config Config
	var showVersion bool

	flag.StringVar(&config.SourceDir, "source", "", "Source directory containing audio files (required)")
	flag.StringVar(&config.OutputDir, "output", "", "Output directory for cleaned files (default: source directory)")
	flag.StringVar(&config.PackName, "pack", "", "Pack name identifier for UE5 naming (required)")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Preview changes without modifying files")
	flag.BoolVar(&config.Organize, "organize", true, "Organize files into category folders")
	flag.BoolVar(&config.CreateManifest, "manifest", true, "Create manifest.json with file metadata")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	flag.Parse()

	if showVersion {
		fmt.Printf("tidy-rename version %s\n", version)
		os.Exit(0)
	}

	if config.SourceDir == "" {
		fmt.Fprintf(os.Stderr, "Error: -source flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	if config.PackName == "" {
		fmt.Fprintf(os.Stderr, "Error: -pack flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	if config.OutputDir == "" {
		config.OutputDir = config.SourceDir // default to same as source
	}

	if _, err := os.Stat(config.SourceDir); os.IsNotExist(err) {
		log.Fatalf("Error: Source directory does not exist: %s", config.SourceDir)
	}

	processor := NewAudioProcessor(config)
	if err := processor.Process(); err != nil {
		log.Fatalf("Error processing files: %v", err)
	}
}
