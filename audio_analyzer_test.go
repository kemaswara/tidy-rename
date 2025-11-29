package main

import (
	"math"
	"testing"
	"time"
)

func TestGenerateFingerprint(t *testing.T) {
	aa := NewAudioAnalyzer()

	tests := []struct {
		name     string
		meta     *AudioMetadata
		expected string // we'll just check it's not empty and consistent
	}{
		{
			name: "basic_metadata",
			meta: &AudioMetadata{
				SampleRate: 44100,
				Channels:   2,
				BitDepth:   16,
				Duration:   5 * time.Second,
				Format:     "WAV",
				Title:      "Test",
			},
		},
		{
			name: "different_metadata",
			meta: &AudioMetadata{
				SampleRate: 48000,
				Channels:   1,
				BitDepth:   24,
				Duration:   10 * time.Second,
				Format:     "MP3",
				Title:      "Different",
			},
		},
		{
			name: "no_title",
			meta: &AudioMetadata{
				SampleRate: 44100,
				Channels:   2,
				BitDepth:   16,
				Duration:   5 * time.Second,
				Format:     "WAV",
				Title:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp1 := aa.generateFingerprint(tt.meta)
			fp2 := aa.generateFingerprint(tt.meta)

			// fingerprint should be consistent
			if fp1 != fp2 {
				t.Errorf("generateFingerprint() inconsistent: %q != %q", fp1, fp2)
			}

			// fingerprint should be 32 hex characters (16 bytes)
			if len(fp1) != 32 {
				t.Errorf("generateFingerprint() length = %d, want 32", len(fp1))
			}

			// fingerprint should not be empty
			if fp1 == "" {
				t.Error("generateFingerprint() returned empty string")
			}
		})
	}

	// test that different metadata produces different fingerprints
	fp1 := aa.generateFingerprint(tests[0].meta)
	fp2 := aa.generateFingerprint(tests[1].meta)
	if fp1 == fp2 {
		t.Error("generateFingerprint() should produce different fingerprints for different metadata")
	}
}

func TestCalculateSpectralFeatures(t *testing.T) {
	aa := NewAudioAnalyzer()

	tests := []struct {
		name     string
		samples  []float64
		sampleRate int
		checkFunc func(*SpectralFeatures) bool
	}{
		{
			name: "sine_wave_like",
			samples: generateSineWave(1000, 44100),
			sampleRate: 44100,
			checkFunc: func(f *SpectralFeatures) bool {
				return f.Energy > 0 && f.ZeroCrossing >= 0 && f.ZeroCrossing <= 1
			},
		},
		{
			name: "noisy_signal",
			samples: generateNoisySignal(1000),
			sampleRate: 44100,
			checkFunc: func(f *SpectralFeatures) bool {
				return f.Energy > 0 && f.ZeroCrossing >= 0 // just check it's valid
			},
		},
		{
			name: "silence",
			samples: make([]float64, 1000),
			sampleRate: 44100,
			checkFunc: func(f *SpectralFeatures) bool {
				return f.Energy == 0 && f.ZeroCrossing == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := &SpectralFeatures{}
			aa.calculateSpectralFeatures(tt.samples, tt.sampleRate, features)

			if !tt.checkFunc(features) {
				t.Errorf("calculateSpectralFeatures() failed validation for %s", tt.name)
			}

			// basic sanity checks
			if features.ZeroCrossing < 0 || features.ZeroCrossing > 1 {
				t.Errorf("ZeroCrossing = %f, should be between 0 and 1", features.ZeroCrossing)
			}
			if features.Energy < 0 {
				t.Errorf("Energy = %f, should be non-negative", features.Energy)
			}
			if features.Centroid < 0 {
				t.Errorf("Centroid = %f, should be non-negative", features.Centroid)
			}
		})
	}
}

func TestInferCategoryWithConfidence(t *testing.T) {
	aa := NewAudioAnalyzer()

	tests := []struct {
		name     string
		filename string
		meta     *AudioMetadata
		expectedCategory string
		minConfidence    float64
	}{
		{
			name:     "scream_voice",
			filename:  "scream_male.wav",
			meta:      &AudioMetadata{Duration: 2 * time.Second, Channels: 1},
			expectedCategory: "SFX_Voice",
			minConfidence:    0.5,
		},
		{
			name:     "creature_roar",
			filename:  "creature_roar.wav",
			meta:      &AudioMetadata{Duration: 3 * time.Second, Channels: 2},
			expectedCategory: "SFX_Creature",
			minConfidence:    0.5,
		},
		{
			name:     "short_ui",
			filename:  "button_click.wav",
			meta:      &AudioMetadata{Duration: 500 * time.Millisecond, Channels: 1},
			expectedCategory: "SFX_UI",
			minConfidence:    0.5,
		},
		{
			name:     "long_ambient",
			filename:  "wind_ambient.wav",
			meta:      &AudioMetadata{Duration: 60 * time.Second, Channels: 2},
			expectedCategory: "Ambient",
			minConfidence:    0.4,
		},
		{
			name:     "weapon_gun",
			filename:  "gun_shot.wav",
			meta:      &AudioMetadata{Duration: 1 * time.Second, Channels: 1},
			expectedCategory: "SFX_Weapon",
			minConfidence:    0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aa.InferCategoryWithConfidence(tt.meta, tt.filename)

			if result.Category != tt.expectedCategory {
				t.Errorf("InferCategoryWithConfidence() Category = %q, want %q", result.Category, tt.expectedCategory)
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("InferCategoryWithConfidence() Confidence = %f, want >= %f", result.Confidence, tt.minConfidence)
			}

			if result.Confidence < 0 || result.Confidence > 1 {
				t.Errorf("InferCategoryWithConfidence() Confidence = %f, should be between 0 and 1", result.Confidence)
			}
		})
	}
}

func TestGenerateAudioTags(t *testing.T) {
	aa := NewAudioAnalyzer()

	tests := []struct {
		name     string
		meta     *AudioMetadata
		expectedTags []string
	}{
		{
			name: "short_mono",
			meta: &AudioMetadata{
				Duration: 500 * time.Millisecond,
				Channels: 1,
				SampleRate: 44100,
			},
			expectedTags: []string{"short", "<1s", "mono"},
		},
		{
			name: "long_stereo",
			meta: &AudioMetadata{
				Duration: 60 * time.Second,
				Channels: 2,
				SampleRate: 48000,
			},
			expectedTags: []string{"long", ">30s", "stereo", "hq", "48kHz"},
		},
		{
			name: "high_quality",
			meta: &AudioMetadata{
				Duration: 5 * time.Second,
				Channels: 2,
				SampleRate: 96000,
				BitDepth: 24,
			},
			expectedTags: []string{"medium", "5-30s", "stereo", "hq", "96kHz", "hq", "24bit"}, // 5 seconds is medium, not short
		},
		{
			name: "with_genre",
			meta: &AudioMetadata{
				Duration: 10 * time.Second,
				Channels: 2,
				HasEmbeddedTags: true,
				Genre: "Horror",
			},
			expectedTags: []string{"medium", "5-30s", "stereo", "tagged", "genre:horror"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := aa.GenerateAudioTags(tt.meta)

			for _, expectedTag := range tt.expectedTags {
				if !containsTag(tags, expectedTag) {
					t.Errorf("GenerateAudioTags() missing tag %q, got %v", expectedTag, tags)
				}
			}
		})
	}
}

// Helper functions for generating test data

func generateSineWave(length int, sampleRate int) []float64 {
	samples := make([]float64, length)
	freq := 440.0 // A4 note
	for i := 0; i < length; i++ {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
	}
	return samples
}

func generateNoisySignal(length int) []float64 {
	samples := make([]float64, length)
	for i := 0; i < length; i++ {
		// generate random-like signal
		samples[i] = math.Sin(float64(i)*0.1) * 0.5 + math.Sin(float64(i)*0.3)*0.3 + math.Sin(float64(i)*0.7)*0.2
	}
	return samples
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

