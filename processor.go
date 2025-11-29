package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

type AudioProcessor struct {
	config        Config
	audioFiles    []AudioFile
	extensions    map[string]bool
	audioAnalyzer *AudioAnalyzer
	fingerprints  map[string][]int // fingerprint -> list of file indices (for duplicate detection)
}

func NewAudioProcessor(config Config) *AudioProcessor {
	return &AudioProcessor{
		config:        config,
		audioFiles:    make([]AudioFile, 0),
		audioAnalyzer: NewAudioAnalyzer(),
		fingerprints:  make(map[string][]int),
		extensions: map[string]bool{
			".wav": true, ".mp3": true, ".ogg": true, ".flac": true,
			".aac": true, ".m4a": true, ".wma": true, // common formats
		},
	}
}

func (ap *AudioProcessor) Process() error {
	fmt.Printf("Scanning directory: %s\n", ap.config.SourceDir)

	if err := ap.scanFiles(); err != nil {
		return fmt.Errorf("failed to scan files: %w", err)
	}

	fmt.Printf("Found %d audio files\n", len(ap.audioFiles))

	if err := ap.analyzeAudioFiles(); err != nil {
		return fmt.Errorf("failed to analyze audio files: %w", err)
	}

	ap.parseFiles()
	ap.generateNewNames()
	ap.displayPreview()

	if ap.config.DryRun {
		fmt.Println("\n[DRY RUN] No files were modified. Remove -dry-run to apply changes.")
		return nil // bail out early if dry run
	}

	if err := ap.applyChanges(); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	if ap.config.CreateManifest {
		if err := ap.createManifest(); err != nil {
			return fmt.Errorf("failed to create manifest: %w", err)
		}
	}

	fmt.Println("\n✓ Processing complete!")
	return nil
}

func (ap *AudioProcessor) scanFiles() error {
	return filepath.WalkDir(ap.config.SourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// skip output dir to avoid processing files we just created
			if ap.config.OutputDir != ap.config.SourceDir && path == ap.config.OutputDir {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ap.extensions[ext] {
			ap.audioFiles = append(ap.audioFiles, AudioFile{
				OriginalPath: path,
				OriginalName: filepath.Base(path),
			})
		}

		return nil
	})
}

func (ap *AudioProcessor) analyzeAudioFiles() error {
	total := len(ap.audioFiles)
	if total == 0 {
		return nil
	}

	// create progress bar
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetDescription("Analyzing audio files"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("files"),
	)

	// use worker pool for parallel processing
	numWorkers := 8
	if total < numWorkers {
		numWorkers = total
	}

	type job struct {
		index int
		file  *AudioFile
	}

	jobs := make(chan job, total)
	results := make(chan struct {
		index int
		meta  *AudioMetadata
		tags  []string
		cat   string
		err   error
	}, total)

	// start workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				meta, err := ap.audioAnalyzer.AnalyzeFile(j.file.OriginalPath)
				if err != nil {
					results <- struct {
						index int
						meta  *AudioMetadata
						tags  []string
						cat   string
						err   error
					}{index: j.index, err: err}
					continue
				}

				var audioTags []string
				var audioCat string
				if meta != nil {
					audioTags = ap.audioAnalyzer.GenerateAudioTags(meta)
					// use confidence-based categorization
					catResult := ap.audioAnalyzer.InferCategoryWithConfidence(meta, j.file.OriginalName)
					audioCat = catResult.Category
				}

				results <- struct {
					index int
					meta  *AudioMetadata
					tags  []string
					cat   string
					err   error
				}{index: j.index, meta: meta, tags: audioTags, cat: audioCat}
			}
		}()
	}

	// send jobs
	go func() {
		for i := range ap.audioFiles {
			jobs <- job{index: i, file: &ap.audioFiles[i]}
		}
		close(jobs)
	}()

	// collect results with progress
	go func() {
		wg.Wait()
		close(results)
	}()

	processed := 0
	for result := range results {
		af := &ap.audioFiles[result.index]

		if result.err != nil {
			// skip if we can't analyze it
			bar.Add(1)
			processed++
			continue
		}

		af.AudioMeta = result.meta

		// track fingerprints for duplicate detection
		if result.meta != nil && result.meta.Fingerprint != "" {
			ap.fingerprints[result.meta.Fingerprint] = append(ap.fingerprints[result.meta.Fingerprint], result.index)
		}

		// use audio properties to help categorize if filename didn't give us much
		if result.cat != "" {
			if af.Category == "" || af.Category == "SFX" {
				af.Category = result.cat
			}
		}

		af.Tags = append(af.Tags, result.tags...)

		bar.Add(1)
		processed++
	}

	bar.Finish()
	fmt.Println()

	// detect and report duplicates
	ap.detectDuplicates()

	return nil
}

// detectDuplicates finds files with matching fingerprints and tags them
func (ap *AudioProcessor) detectDuplicates() {
	duplicateCount := 0
	for _, indices := range ap.fingerprints {
		if len(indices) > 1 {
			duplicateCount++
			// tag all duplicates
			for _, idx := range indices {
				ap.audioFiles[idx].Tags = append(ap.audioFiles[idx].Tags, "duplicate")
				if len(indices) > 1 {
					ap.audioFiles[idx].Tags = append(ap.audioFiles[idx].Tags, fmt.Sprintf("duplicate-group-%d", duplicateCount))
				}
			}
		}
	}
	if duplicateCount > 0 {
		fmt.Printf("⚠ Found %d duplicate file groups (same audio content)\n", duplicateCount)
	}
}

func (ap *AudioProcessor) parseFiles() {
	for i := range ap.audioFiles {
		ap.parseFile(&ap.audioFiles[i])
	}
}

func (ap *AudioProcessor) parseFile(af *AudioFile) {
	name := strings.TrimSuffix(af.OriginalName, filepath.Ext(af.OriginalName))

	// grab the ID (usually at the end like .12345)
	idPattern := regexp.MustCompile(`\.(\d+)$`)
	if matches := idPattern.FindStringSubmatch(name); len(matches) > 1 {
		af.ID = matches[1]
		name = strings.TrimSuffix(name, "."+af.ID)
	}

	// last underscore segment is usually the source/library code
	parts := strings.Split(name, "_")
	if len(parts) > 1 {
		af.Source = parts[len(parts)-1]
		name = strings.Join(parts[:len(parts)-1], "_")
	}

	// check for dash-separated category (e.g., "FX-Impact")
	if strings.Contains(name, "-") {
		catParts := strings.SplitN(name, "-", 2)
		af.Category = catParts[0]
		if len(catParts) > 1 {
			af.SubCategory = catParts[1]
		}
	} else {
		// no dash, try to guess from the name
		af.Category = ap.inferCategory(name)
		af.SubCategory = name
	}

	af.Category = ap.normalizeCategory(af.Category)
	af.Tags = ap.generateTags(af)
}

func (ap *AudioProcessor) inferCategory(name string) string {
	nameLower := strings.ToLower(name)

	// voice/dialogue
	if strings.Contains(nameLower, "scream") || strings.Contains(nameLower, "voice") ||
		strings.Contains(nameLower, "dialogue") || strings.Contains(nameLower, "speech") ||
		strings.Contains(nameLower, "male") || strings.Contains(nameLower, "female") ||
		strings.Contains(nameLower, "grunt") || strings.Contains(nameLower, "groan") {
		return "SFX_Voice"
	}

	// creatures/animals
	if strings.Contains(nameLower, "creature") || strings.Contains(nameLower, "monster") ||
		strings.Contains(nameLower, "animal") || strings.Contains(nameLower, "beast") ||
		strings.Contains(nameLower, "roar") || strings.Contains(nameLower, "growl") ||
		strings.Contains(nameLower, "howl") || strings.Contains(nameLower, "moan") {
		return "SFX_Creature"
	}

	// weapons/combat (exclude standalone "fire" which is usually flame/ambient)
	// only match "fire" if it's clearly weapon-related (gunfire, firearm, etc.)
	if strings.Contains(nameLower, "gun") || strings.Contains(nameLower, "weapon") ||
		strings.Contains(nameLower, "shot") || strings.Contains(nameLower, "bullet") ||
		strings.Contains(nameLower, "sword") || strings.Contains(nameLower, "slash") ||
		strings.Contains(nameLower, "hit") || strings.Contains(nameLower, "punch") ||
		strings.Contains(nameLower, "combat") ||
		strings.Contains(nameLower, "gunfire") || strings.Contains(nameLower, "firearm") {
		return "SFX_Weapon"
	}
	// match "fire" only if it's combined with weapon keywords
	if strings.Contains(nameLower, "fire") {
		if strings.Contains(nameLower, "gun") || strings.Contains(nameLower, "weapon") ||
			strings.Contains(nameLower, "shot") || strings.Contains(nameLower, "gunfire") ||
			strings.Contains(nameLower, "firearm") {
			return "SFX_Weapon"
		}
		// standalone "fire" will be caught by ambient check below
	}

	// impacts/explosions
	if strings.Contains(nameLower, "explosion") || strings.Contains(nameLower, "explode") ||
		strings.Contains(nameLower, "impact") || strings.Contains(nameLower, "crash") ||
		strings.Contains(nameLower, "slam") || strings.Contains(nameLower, "thud") ||
		strings.Contains(nameLower, "bang") || strings.Contains(nameLower, "boom") {
		return "SFX_Impact"
	}

	// footsteps/movement
	if strings.Contains(nameLower, "footstep") || strings.Contains(nameLower, "step") ||
		strings.Contains(nameLower, "walk") || strings.Contains(nameLower, "run") ||
		strings.Contains(nameLower, "jump") || strings.Contains(nameLower, "land") {
		return "SFX_Footstep"
	}

	// nature/environment (including fire/flame as ambient) - check this BEFORE vehicle to catch "atmos"
	if strings.Contains(nameLower, "wind") || strings.Contains(nameLower, "rain") ||
		strings.Contains(nameLower, "thunder") || strings.Contains(nameLower, "storm") ||
		strings.Contains(nameLower, "water") || strings.Contains(nameLower, "ocean") ||
		strings.Contains(nameLower, "forest") || strings.Contains(nameLower, "nature") ||
		strings.Contains(nameLower, "atmos") || strings.Contains(nameLower, "atmosphere") ||
		strings.Contains(nameLower, "ambient") || strings.Contains(nameLower, "ambience") ||
		strings.Contains(nameLower, "flame") || strings.Contains(nameLower, "flames") ||
		strings.Contains(nameLower, "burning") || strings.Contains(nameLower, "ember") ||
		strings.Contains(nameLower, "campfire") || strings.Contains(nameLower, "bonfire") ||
		// standalone "fire" (not gunfire) is usually ambient
		(nameLower == "fire" || strings.HasPrefix(nameLower, "fire ") || strings.HasSuffix(nameLower, " fire")) {
		return "Ambient"
	}

	// vehicles
	if strings.Contains(nameLower, "vehicle") || strings.Contains(nameLower, "car") ||
		strings.Contains(nameLower, "engine") || strings.Contains(nameLower, "motor") ||
		strings.Contains(nameLower, "tire") || strings.Contains(nameLower, "wheel") ||
		strings.Contains(nameLower, "drive") || strings.Contains(nameLower, "vehicle") {
		return "SFX_Vehicle"
	}

	// UI/interface
	if strings.Contains(nameLower, "ui") || strings.Contains(nameLower, "interface") ||
		strings.Contains(nameLower, "button") || strings.Contains(nameLower, "click") ||
		strings.Contains(nameLower, "select") || strings.Contains(nameLower, "hover") ||
		strings.Contains(nameLower, "menu") || strings.Contains(nameLower, "notification") {
		return "SFX_UI"
	}

	// alarms/alerts
	if strings.Contains(nameLower, "alarm") || strings.Contains(nameLower, "alert") ||
		strings.Contains(nameLower, "siren") || strings.Contains(nameLower, "warning") ||
		strings.Contains(nameLower, "beep") || strings.Contains(nameLower, "buzz") {
		return "SFX_Alarm"
	}

	// mechanical
	if strings.Contains(nameLower, "mechanical") || strings.Contains(nameLower, "machine") ||
		strings.Contains(nameLower, "gear") || strings.Contains(nameLower, "motor") ||
		strings.Contains(nameLower, "whir") || strings.Contains(nameLower, "clank") {
		return "SFX_Mechanical"
	}

	// doors/objects
	if strings.Contains(nameLower, "door") || strings.Contains(nameLower, "open") ||
		strings.Contains(nameLower, "close") || strings.Contains(nameLower, "creak") ||
		strings.Contains(nameLower, "squeak") || strings.Contains(nameLower, "hinge") {
		return "SFX_Object"
	}

	// percussion/drums
	if strings.Contains(nameLower, "drum") || strings.Contains(nameLower, "percussion") ||
		strings.Contains(nameLower, "beat") || strings.Contains(nameLower, "kick") ||
		strings.Contains(nameLower, "snare") || strings.Contains(nameLower, "cymbal") {
		return "SFX_Percussion"
	}

	return "SFX" // default fallback
}

func (ap *AudioProcessor) normalizeCategory(cat string) string {
	cat = strings.ToUpper(cat)

	categoryMap := map[string]string{
		"PE":         "SFX_Percussion",
		"PERCUSSION": "SFX_Percussion",
		"SFX":        "SFX",
		"VOICE":      "SFX_Voice",
		"CREATURE":   "SFX_Creature",
		"WEAPON":     "SFX_Weapon",
		"IMPACT":     "SFX_Impact",
		"FOOTSTEP":   "SFX_Footstep",
		"VEHICLE":    "SFX_Vehicle",
		"ALARM":      "SFX_Alarm",
		"MECHANICAL": "SFX_Mechanical",
		"OBJECT":     "SFX_Object",
		"AMBIENT":    "Ambient",
		"MUSIC":      "Music",
		"UI":         "UI",
		"DIALOGUE":   "Dialogue",
	}

	if normalized, ok := categoryMap[cat]; ok {
		return normalized
	}

	// auto-prefix with SFX_ if it's not already categorized
	if !strings.Contains(cat, "_") && !strings.Contains(cat, "Music") && !strings.Contains(cat, "Ambient") {
		return "SFX_" + cat
	}

	return cat
}

func (ap *AudioProcessor) generateTags(af *AudioFile) []string {
	tags := []string{}

	if af.Category != "" {
		tags = append(tags, af.Category)
	}

	if af.SubCategory != "" {
		subCatLower := strings.ToLower(af.SubCategory)
		words := strings.Fields(strings.ReplaceAll(subCatLower, "_", " "))
		for _, word := range words {
			if len(word) > 2 {
				tags = append(tags, word)
			}
		}
	}

	if af.Source != "" {
		tags = append(tags, "src:"+af.Source)
	}

	nameLower := strings.ToLower(af.OriginalName)
	if strings.Contains(nameLower, "lfe") {
		tags = append(tags, "lfe", "low-frequency")
	}
	if strings.Contains(nameLower, "processed") {
		tags = append(tags, "processed", "fx")
	}
	if strings.Contains(nameLower, "attacked") || strings.Contains(nameLower, "pain") {
		tags = append(tags, "combat", "damage")
	}

	return tags
}

func (ap *AudioProcessor) generateNewNames() {
	nameCounts := make(map[string]int)

	// first pass: generate all the base names
	for i := range ap.audioFiles {
		af := &ap.audioFiles[i]
		af.NewName = ap.generateUE5Name(af)
	}

	// second pass: handle duplicates by adding numbers
	for i := range ap.audioFiles {
		af := &ap.audioFiles[i]
		baseName := strings.TrimSuffix(af.NewName, filepath.Ext(af.NewName))
		count := nameCounts[baseName]
		nameCounts[baseName]++

		if count > 0 {
			ext := filepath.Ext(af.NewName)
			af.NewName = fmt.Sprintf("%s_%02d%s", baseName, count, ext) // _01, _02, etc.
		}
	}
}

func (ap *AudioProcessor) generateUE5Name(af *AudioFile) string {
	var parts []string

	parts = append(parts, "A") // UE5 convention

	if ap.config.PackName != "" {
		packName := ap.cleanNameWithCase(ap.config.PackName)
		if packName != "" {
			parts = append(parts, packName)
		}
	}

	// strip SFX_ prefix since it's implied
	category := strings.TrimPrefix(af.Category, "SFX_")
	if category != "" {
		category = ap.cleanNamePart(category)
		parts = append(parts, category)
	}

	if af.SubCategory != "" {
		subCat := ap.cleanNamePart(af.SubCategory)
		if subCat != "" {
			parts = append(parts, subCat)
		}
	}

	newName := strings.Join(parts, "_")

	// make sure it starts with A_ (just in case)
	if !strings.HasPrefix(newName, "A_") {
		newName = "A_" + strings.TrimPrefix(newName, "A")
	}

	ext := filepath.Ext(af.OriginalName)
	return newName + ext
}

func (ap *AudioProcessor) cleanName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")

	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = reg.ReplaceAllString(name, "")

	reg = regexp.MustCompile(`_+`)
	name = reg.ReplaceAllString(name, "_")

	name = strings.Trim(name, "_")

	words := strings.Split(name, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, "_")
}

func (ap *AudioProcessor) cleanNamePart(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")

	// strip out anything that's not alphanumeric or underscore
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = reg.ReplaceAllString(name, "")

	// collapse multiple underscores
	reg = regexp.MustCompile(`_+`)
	name = reg.ReplaceAllString(name, "_")

	name = strings.Trim(name, "_")

	words := strings.Split(name, "_")
	for i, word := range words {
		if len(word) > 0 {
			// keep numbers as-is, capitalize words
			if word[0] >= '0' && word[0] <= '9' {
				words[i] = word
			} else {
				words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			}
		}
	}

	return strings.Join(words, "_")
}

func (ap *AudioProcessor) cleanNameWithCase(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s\-_]`)
	name = reg.ReplaceAllString(name, "")

	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	wordBoundaryRegex := regexp.MustCompile(`([a-z])([A-Z])`)
	name = wordBoundaryRegex.ReplaceAllString(name, `$1 $2`)

	words := strings.Fields(name)

	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, "")
}

func (ap *AudioProcessor) displayPreview() {
	fmt.Println("\n=== Preview of Changes ===")

	// Group by category
	categoryGroups := make(map[string][]*AudioFile)
	for i := range ap.audioFiles {
		cat := ap.audioFiles[i].Category
		if cat == "" {
			cat = "Uncategorized"
		}
		categoryGroups[cat] = append(categoryGroups[cat], &ap.audioFiles[i])
	}

	// Sort categories
	categories := make([]string, 0, len(categoryGroups))
	for cat := range categoryGroups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		files := categoryGroups[cat]
		fmt.Printf("\n[%s] (%d files)\n", cat, len(files))
		for _, af := range files {
			fmt.Printf("  %s\n", af.OriginalName)
			fmt.Printf("  → %s\n", af.NewName)
			if af.AudioMeta != nil {
				if af.AudioMeta.Duration > 0 {
					fmt.Printf("    Duration: %v", af.AudioMeta.Duration.Round(time.Millisecond))
				}
				if af.AudioMeta.SampleRate > 0 {
					fmt.Printf(" | %dHz", af.AudioMeta.SampleRate)
				}
				if af.AudioMeta.Channels > 0 {
					fmt.Printf(" | %dch", af.AudioMeta.Channels)
				}
				if af.AudioMeta.BitDepth > 0 {
					fmt.Printf(" | %dbit", af.AudioMeta.BitDepth)
				}
				fmt.Println()
			}
			if len(af.Tags) > 0 {
				fmt.Printf("    Tags: %s\n", strings.Join(af.Tags, ", "))
			}
		}
	}
}

func (ap *AudioProcessor) applyChanges() error {
	fmt.Println("\n=== Applying Changes ===")

	total := len(ap.audioFiles)
	if total == 0 {
		return nil
	}

	bar := progressbar.NewOptions(total,
		progressbar.OptionSetDescription("Moving files"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("files"),
	)

	for i := range ap.audioFiles {
		af := &ap.audioFiles[i]

		// Determine output path
		var outputPath string
		if ap.config.Organize {
			// Organize by category
			categoryDir := ap.cleanName(af.Category)
			if categoryDir == "" {
				categoryDir = "Uncategorized"
			}
			outputPath = filepath.Join(ap.config.OutputDir, categoryDir, af.NewName)
		} else {
			// Keep in same structure
			relPath, err := filepath.Rel(ap.config.SourceDir, af.OriginalPath)
			if err != nil {
				relPath = af.NewName
			}
			outputPath = filepath.Join(ap.config.OutputDir, filepath.Dir(relPath), af.NewName)
		}

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			bar.Finish()
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Skip if source and destination are the same
		if af.OriginalPath == outputPath {
			bar.Add(1)
			continue
		}

		// Rename/move file
		if err := os.Rename(af.OriginalPath, outputPath); err != nil {
			// If rename fails (cross-device), try copy + delete
			if err := ap.moveFile(af.OriginalPath, outputPath); err != nil {
				bar.Finish()
				return fmt.Errorf("failed to move file %s: %w", af.OriginalName, err)
			}
		}

		bar.Add(1)
	}

	bar.Finish()
	fmt.Println()

	return nil
}

func (ap *AudioProcessor) moveFile(src, dst string) error {
	// cross-device move: copy then delete (os.Rename fails across drives)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}

	return os.Remove(src)
}

func (ap *AudioProcessor) createManifest() error {
	manifestPath := filepath.Join(ap.config.OutputDir, "manifest.json")

	manifest := map[string]interface{}{
		"total_files": len(ap.audioFiles),
		"categories":  ap.getCategoryStats(),
		"files":       ap.audioFiles,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return err
	}

	fmt.Printf("\n✓ Created manifest: %s\n", manifestPath)
	return nil
}

func (ap *AudioProcessor) getCategoryStats() map[string]int {
	stats := make(map[string]int)
	for _, af := range ap.audioFiles {
		cat := af.Category
		if cat == "" {
			cat = "Uncategorized"
		}
		stats[cat]++
	}
	return stats
}
