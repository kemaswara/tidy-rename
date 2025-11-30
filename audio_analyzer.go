package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

type AudioMetadata struct {
	Duration        time.Duration
	SampleRate      int
	Channels        int
	BitDepth        int
	Bitrate         int
	Format          string
	Title           string
	Artist          string
	Album           string
	Genre           string
	Year            int
	Comment         string
	HasEmbeddedTags bool

	// Spectral analysis features
	SpectralFeatures *SpectralFeatures `json:"spectral_features,omitempty"`

	// Audio fingerprint for duplicate detection
	Fingerprint string `json:"fingerprint,omitempty"`
}

type SpectralFeatures struct {
	LowEnergy    float64 // 0-200 Hz
	MidEnergy    float64 // 200-2000 Hz
	HighEnergy   float64 // 2000+ Hz
	ZeroCrossing float64 // zero crossing rate
	Centroid     float64 // spectral centroid (Hz)
	Energy       float64 // total energy
}

type AudioAnalyzer struct {
}

func NewAudioAnalyzer() *AudioAnalyzer {
	return &AudioAnalyzer{}
}

func (aa *AudioAnalyzer) AnalyzeFile(filePath string) (*AudioMetadata, error) {
	meta := &AudioMetadata{}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if err := aa.readEmbeddedTags(file, meta); err != nil {
		// no embedded tags, that's fine
	}

	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".wav":
		if err := aa.analyzeWAV(file, meta); err != nil {
			return nil, fmt.Errorf("failed to analyze WAV: %w", err)
		}
		// perform spectral analysis on WAV files
		if _, err := file.Seek(0, 0); err == nil {
			if err := aa.analyzeSpectral(file, meta); err != nil {
				// spectral analysis failed, but that's okay - continue without it
			}
		}
	case ".mp3", ".ogg", ".flac", ".aac", ".m4a", ".wma":
		if err := aa.analyzeCompressed(file, meta); err != nil {
			meta.Format = ext[1:]
		}
	default:
		meta.Format = ext[1:]
	}

	return meta, nil
}

func (aa *AudioAnalyzer) readEmbeddedTags(file *os.File, meta *AudioMetadata) error {
	m, err := tag.ReadFrom(file)
	if err != nil {
		return err
	}

	meta.HasEmbeddedTags = true
	meta.Title = m.Title()
	meta.Artist = m.Artist()
	meta.Album = m.Album()
	meta.Genre = m.Genre()
	meta.Year = m.Year()
	meta.Comment = m.Comment()

	format := m.Format()
	meta.Format = string(format)

	return nil
}

func (aa *AudioAnalyzer) analyzeWAV(file *os.File, meta *AudioMetadata) error {
	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return fmt.Errorf("invalid WAV file")
	}

	meta.Format = "WAV"

	format := decoder.Format()
	if format != nil {
		meta.SampleRate = int(format.SampleRate)
		meta.Channels = int(format.NumChannels)
		meta.BitDepth = 16 // most WAVs are 16-bit, decoder doesn't expose this directly
	}

	if format != nil && format.SampleRate > 0 {
		duration, err := decoder.Duration()
		if err == nil && duration > 0 {
			meta.Duration = duration
		} else {
			// fallback: estimate from file size (44 bytes is typical WAV header)
			fileInfo, err := file.Stat()
			if err == nil {
				bytesPerSample := int64(meta.BitDepth / 8)
				if bytesPerSample > 0 {
					dataSize := fileInfo.Size() - 44
					if dataSize > 0 {
						totalSamples := dataSize / (int64(format.NumChannels) * bytesPerSample)
						if totalSamples > 0 {
							durationSeconds := float64(totalSamples) / float64(format.SampleRate)
							meta.Duration = time.Duration(durationSeconds * float64(time.Second))
						}
					}
				}
			}
		}
	}

	if meta.SampleRate > 0 && meta.Channels > 0 && meta.BitDepth > 0 {
		meta.Bitrate = meta.SampleRate * meta.Channels * meta.BitDepth
	}

	// generate fingerprint after we have all metadata
	meta.Fingerprint = aa.generateFingerprint(meta)

	return nil
}

func (aa *AudioAnalyzer) analyzeCompressed(file *os.File, meta *AudioMetadata) error {
	m, err := tag.ReadFrom(file)
	if err != nil {
		return err
	}

	format := m.Format()
	if format != "" {
		meta.Format = string(format)
	}

	// rough duration estimate for compressed formats
	if meta.Bitrate > 0 {
		fileInfo, err := file.Stat()
		if err == nil {
			fileSizeBits := fileInfo.Size() * 8
			durationSeconds := float64(fileSizeBits) / float64(meta.Bitrate)
			meta.Duration = time.Duration(durationSeconds * float64(time.Second))
		}
	}

	return nil
}

func (aa *AudioAnalyzer) InferCategoryFromAudio(meta *AudioMetadata, filename string) string {
	// use duration as a hint
	if meta.Duration > 0 {
		if meta.Duration < 2*time.Second {
			return "SFX_UI" // very short = probably UI sound
		} else if meta.Duration < 5*time.Second {
			return "SFX"
		} else if meta.Duration > 30*time.Second {
			// long file, check genre tag if available
			if meta.HasEmbeddedTags && meta.Genre != "" {
				genreLower := strings.ToLower(meta.Genre)
				if strings.Contains(genreLower, "music") || strings.Contains(genreLower, "song") {
					return "Music"
				}
			}
			return "Ambient"
		}
	}

	// channel count can also be a hint
	if meta.Channels == 1 {
		return "SFX" // mono = usually focused SFX
	} else if meta.Channels >= 5 {
		return "Ambient" // surround = probably ambient
	}

	if meta.HasEmbeddedTags {
		if meta.Genre != "" {
			genreLower := strings.ToLower(meta.Genre)
			if strings.Contains(genreLower, "voice") || strings.Contains(genreLower, "dialogue") {
				return "SFX_Voice"
			}
			if strings.Contains(genreLower, "music") {
				return "Music"
			}
			if strings.Contains(genreLower, "ambient") {
				return "Ambient"
			}
		}
	}

	return ""
}

func (aa *AudioAnalyzer) GenerateAudioTags(meta *AudioMetadata) []string {
	tags := []string{}

	if meta.Duration > 0 {
		if meta.Duration < 1*time.Second {
			tags = append(tags, "short", "<1s")
		} else if meta.Duration < 5*time.Second {
			tags = append(tags, "short", "1-5s")
		} else if meta.Duration < 30*time.Second {
			tags = append(tags, "medium", "5-30s")
		} else {
			tags = append(tags, "long", ">30s")
		}
	}

	if meta.Channels == 1 {
		tags = append(tags, "mono")
	} else if meta.Channels == 2 {
		tags = append(tags, "stereo")
	} else if meta.Channels > 2 {
		tags = append(tags, "multichannel", fmt.Sprintf("%dch", meta.Channels))
	}

	if meta.SampleRate > 0 {
		if meta.SampleRate >= 48000 {
			tags = append(tags, "hq", fmt.Sprintf("%dkHz", meta.SampleRate/1000)) // 48kHz+ is high quality
		} else {
			tags = append(tags, fmt.Sprintf("%dkHz", meta.SampleRate/1000))
		}
	}

	if meta.BitDepth >= 24 {
		tags = append(tags, "hq", fmt.Sprintf("%dbit", meta.BitDepth))
	}

	if meta.Bitrate > 0 {
		if meta.Bitrate >= 320000 {
			tags = append(tags, "hq", "high-bitrate")
		}
	}

	if meta.HasEmbeddedTags {
		tags = append(tags, "tagged")
		if meta.Genre != "" {
			tags = append(tags, "genre:"+strings.ToLower(meta.Genre))
		}
	}

	return tags
}

// analyzeSpectral performs basic spectral analysis on WAV files
// extracts frequency characteristics to help with categorization
func (aa *AudioAnalyzer) analyzeSpectral(file *os.File, meta *AudioMetadata) error {
	if meta.SampleRate == 0 || meta.Channels == 0 {
		return fmt.Errorf("missing audio format info")
	}

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return fmt.Errorf("invalid WAV file")
	}

	// read a sample of audio data (first 2 seconds or up to 8192 samples, whichever is smaller)
	// this gives us enough data for basic analysis without loading huge files
	maxSamples := 8192
	if meta.SampleRate > 0 {
		maxSamples = meta.SampleRate * 2 // 2 seconds
	}
	if maxSamples > 8192 {
		maxSamples = 8192
	}

	var samples []float64
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: meta.Channels,
			SampleRate:  meta.SampleRate,
		},
		Data: make([]int, maxSamples*meta.Channels),
	}

	// read samples using PCMBuffer
	samplesRead := 0
	for samplesRead < maxSamples {
		n, err := decoder.PCMBuffer(buf)
		if err != nil || n == 0 {
			break
		}

		// convert to float64 and take first channel (or average for stereo)
		// n is the number of frames read, each frame has Channels samples
		for i := 0; i < n && samplesRead < maxSamples; i++ {
			idx := i * meta.Channels
			if idx >= len(buf.Data) {
				break
			}

			if meta.Channels == 1 {
				samples = append(samples, float64(buf.Data[idx])/32768.0)
			} else {
				// average channels for stereo
				val := float64(buf.Data[idx])
				if idx+1 < len(buf.Data) {
					val = (val + float64(buf.Data[idx+1])) / 2.0
				}
				samples = append(samples, val/32768.0)
			}
			samplesRead++
		}
	}

	if len(samples) < 100 {
		return fmt.Errorf("not enough samples for analysis")
	}

	features := &SpectralFeatures{}
	aa.calculateSpectralFeatures(samples, meta.SampleRate, features)
	meta.SpectralFeatures = features

	return nil
}

// calculateSpectralFeatures computes frequency band energies, zero crossing rate, and spectral centroid
func (aa *AudioAnalyzer) calculateSpectralFeatures(samples []float64, sampleRate int, features *SpectralFeatures) {
	// calculate zero crossing rate
	zeroCrossings := 0
	for i := 1; i < len(samples); i++ {
		if (samples[i-1] >= 0 && samples[i] < 0) || (samples[i-1] < 0 && samples[i] >= 0) {
			zeroCrossings++
		}
	}
	features.ZeroCrossing = float64(zeroCrossings) / float64(len(samples))

	// simple frequency band analysis using a basic FFT approximation
	// we'll use a simplified approach: calculate energy in different frequency ranges
	// by looking at sample variations and using a simple high-pass/low-pass concept

	// calculate total energy
	totalEnergy := 0.0
	for _, s := range samples {
		totalEnergy += s * s
	}
	features.Energy = totalEnergy / float64(len(samples))

	// frequency band analysis using simple differentiation
	// high frequencies = rapid changes, low frequencies = slow changes
	lowFreqEnergy := 0.0
	midFreqEnergy := 0.0
	highFreqEnergy := 0.0

	// use different window sizes to approximate frequency bands
	// low: large window (slow changes)
	// high: small window (fast changes)
	windowLow := 100
	windowMid := 20
	windowHigh := 5

	if len(samples) > windowLow {
		// low frequency energy (0-200 Hz approximation)
		for i := windowLow; i < len(samples); i++ {
			diff := samples[i] - samples[i-windowLow]
			lowFreqEnergy += diff * diff
		}
		lowFreqEnergy /= float64(len(samples) - windowLow)
	}

	if len(samples) > windowMid {
		// mid frequency energy (200-2000 Hz approximation)
		for i := windowMid; i < len(samples); i++ {
			diff := samples[i] - samples[i-windowMid]
			midFreqEnergy += diff * diff
		}
		midFreqEnergy /= float64(len(samples) - windowMid)
	}

	if len(samples) > windowHigh {
		// high frequency energy (2000+ Hz approximation)
		for i := windowHigh; i < len(samples); i++ {
			diff := samples[i] - samples[i-windowHigh]
			highFreqEnergy += diff * diff
		}
		highFreqEnergy /= float64(len(samples) - windowHigh)
	}

	features.LowEnergy = lowFreqEnergy
	features.MidEnergy = midFreqEnergy
	features.HighEnergy = highFreqEnergy

	// spectral centroid approximation
	// weighted average frequency - higher = brighter sound
	totalWeighted := 0.0
	totalWeight := 0.0
	for i := 1; i < len(samples); i++ {
		// use sample index as frequency proxy
		freq := float64(i) * float64(sampleRate) / float64(len(samples))
		magnitude := math.Abs(samples[i] - samples[i-1])
		totalWeighted += freq * magnitude
		totalWeight += magnitude
	}
	if totalWeight > 0 {
		features.Centroid = totalWeighted / totalWeight
	} else {
		features.Centroid = float64(sampleRate) / 4 // default to mid-range
	}
}

// generateFingerprint creates a hash-based fingerprint for duplicate detection
func (aa *AudioAnalyzer) generateFingerprint(meta *AudioMetadata) string {
	// combine key characteristics into a fingerprint
	fpData := fmt.Sprintf("%d|%d|%d|%d|%s|%s",
		meta.SampleRate,
		meta.Channels,
		meta.BitDepth,
		int(meta.Duration.Seconds()),
		meta.Format,
		meta.Title, // include title if available
	)

	hash := sha256.Sum256([]byte(fpData))
	return hex.EncodeToString(hash[:16]) // use first 16 bytes (32 hex chars)
}

// InferCategoryWithConfidence returns category with confidence score (0.0-1.0)
// combines filename patterns, metadata, and spectral features
type CategoryResult struct {
	Category   string
	Confidence float64
}

func (aa *AudioAnalyzer) InferCategoryWithConfidence(meta *AudioMetadata, filename string) CategoryResult {
	filenameLower := strings.ToLower(filename)

	// Start with filename-based category matching
	scores := InferCategoryWithConfidenceScores(filename)

	// Apply metadata-based scoring
	ApplyMetadataScoring(scores, meta, filenameLower)

	// spectral analysis scoring (low-medium confidence)
	if meta.SpectralFeatures != nil {
		sf := meta.SpectralFeatures

		// high zero crossing rate = noisy/percussive sounds (impacts, weapons)
		if sf.ZeroCrossing > 0.15 {
			scores["SFX_Impact"] += 0.3
			scores["SFX_Weapon"] += 0.3
		}

		// high energy in low frequencies = impacts, explosions, bass
		if sf.LowEnergy > 0.1 && sf.LowEnergy > sf.MidEnergy && sf.LowEnergy > sf.HighEnergy {
			scores["SFX_Impact"] += 0.4
		}

		// high energy in high frequencies = UI sounds, clicks, sharp impacts
		if sf.HighEnergy > 0.05 && sf.HighEnergy > sf.MidEnergy {
			scores["SFX_UI"] += 0.3
			scores["SFX_Impact"] += 0.2
		}

		// balanced energy across bands = ambient/music
		if sf.LowEnergy > 0.01 && sf.MidEnergy > 0.01 && sf.HighEnergy > 0.01 {
			balance := math.Min(sf.LowEnergy, math.Min(sf.MidEnergy, sf.HighEnergy)) /
				math.Max(sf.LowEnergy, math.Max(sf.MidEnergy, sf.HighEnergy))
			if balance > 0.3 {
				scores["Ambient"] += 0.3
				scores["Music"] += 0.2
			}
		}

		// low spectral centroid = dark/ambient, high = bright/UI
		if sf.Centroid < 500 {
			scores["Ambient"] += 0.2
		} else if sf.Centroid > 2000 {
			scores["SFX_UI"] += 0.2
		}
	}

	// find best category
	bestCategory := "SFX"
	bestScore := 0.0
	for cat, score := range scores {
		if score > bestScore {
			bestScore = score
			bestCategory = cat
		}
	}

	// normalize confidence to 0.0-1.0
	confidence := math.Min(bestScore/1.5, 1.0) // cap at reasonable max
	if confidence < 0.3 {
		confidence = 0.3 // minimum confidence floor
	}

	return CategoryResult{
		Category:   bestCategory,
		Confidence: confidence,
	}
}
