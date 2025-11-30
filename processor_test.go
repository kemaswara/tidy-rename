package main

import (
	"testing"
)

func TestCleanName(t *testing.T) {
	ap := NewAudioProcessor(Config{PackName: "TestPack"})

	tests := []struct {
		input    string
		expected string
	}{
		{"hello-world", "Hello_World"},
		{"test_file.wav", "Test_Filewav"},           // cleanName doesn't preserve dots
		{"PE-Horror_BW.28968", "Pe_Horror_Bw28968"}, // dots removed
		{"scream_male_123", "Scream_Male_123"},
		{"test___multiple___underscores", "Test_Multiple_Underscores"},
		{"test-with-dashes", "Test_With_Dashes"},
		{"test@#$%special", "Testspecial"}, // special chars removed but keeps alphanumeric
		{"", ""},
		{"123", "123"},
		{"UPPERCASE", "Uppercase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ap.cleanName(tt.input)
			if result != tt.expected {
				t.Errorf("cleanName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanNamePart(t *testing.T) {
	ap := NewAudioProcessor(Config{PackName: "TestPack"})

	tests := []struct {
		input    string
		expected string
	}{
		{"hello-world", "Hello_World"},
		{"hello world", "Hello_World"},
		{"test123", "Test123"},
		{"123test", "123test"}, // numbers at start stay as-is
		{"test_123_file", "Test_123_File"},
		{"PE-Horror", "Pe_Horror"},
		{"test___multiple", "Test_Multiple"},
		{"test@#$%special", "Testspecial"}, // cleanNamePart keeps alphanumeric after removing special chars
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ap.cleanNamePart(tt.input)
			if result != tt.expected {
				t.Errorf("cleanNamePart(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInferCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"scream_male", "SFX_Voice"},
		{"voice_dialogue", "SFX_Voice"},
		{"creature_roar", "SFX_Creature"},
		{"monster_growl", "SFX_Creature"},
		{"gun_shot", "SFX_Weapon"},
		{"weapon_fire", "SFX_Weapon"},
		{"explosion_impact", "SFX_Impact"},
		{"footstep_walk", "SFX_Footstep"},
		{"car_engine", "SFX_Vehicle"},
		{"door_creak", "SFX_Object"},
		{"button_click", "SFX_UI"},
		{"wind_ambient", "Ambient"},
		{"music_track", "Music"}, // music and track keywords now supported
		{"siren_alarm", "SFX_Alarm"},
		{"random_sound", "SFX"}, // default fallback
		{"", "SFX"},
		{"drone_sustained", "SFX_Drone"},
		{"loop_music", "Music"},
		{"riser_tension", "SFX_Riser"},
		{"whoosh_wind", "SFX_Whoosh"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := InferCategory(tt.input)
			if result != tt.expected {
				t.Errorf("InferCategory(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SFX_Voice", "SFX_Voice"}, // NormalizeCategory uses map lookup, preserves case
		{"SFX_Creature", "SFX_Creature"},
		{"SFX_Weapon", "SFX_Weapon"},
		{"SFX_Impact", "SFX_Impact"},
		{"SFX_Footstep", "SFX_Footstep"},
		{"SFX_Vehicle", "SFX_Vehicle"},
		{"SFX_Mechanical", "SFX_Mechanical"},
		{"SFX_Object", "SFX_Object"},
		{"SFX_UI", "SFX_UI"}, // SFX_UI not in map, already has "_", so returned as-is
		{"UI", "UI"},         // UI maps to "UI" in CategoryNormalization
		{"SFX_Alarm", "SFX_Alarm"},
		{"Ambient", "Ambient"},
		{"Music", "Music"},
		{"SFX", "SFX"},
		{"PE", "SFX_Percussion"}, // PE maps to SFX_Percussion
		{"DRONE", "SFX_Drone"},
		{"LOOP", "Music"},
		{"unknown", "SFX_UNKNOWN"}, // unknown gets SFX_ prefix and uppercased
		{"", "SFX_"},               // empty: "" -> "" -> not in map -> !contains("_") -> "SFX_" + "" = "SFX_"
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeCategory(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeCategory(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateUE5Name(t *testing.T) {
	ap := NewAudioProcessor(Config{PackName: "TestPack"})

	tests := []struct {
		name     string
		file     AudioFile
		expected string
	}{
		{
			name: "basic_sfx",
			file: AudioFile{
				OriginalName: "test.wav",
				Category:     "SFX",
				SubCategory:  "test",
			},
			expected: "A_TestPack_Sfx_Test.wav",
		},
		{
			name: "voice_category",
			file: AudioFile{
				OriginalName: "scream.wav",
				Category:     "SFX_Voice",
				SubCategory:  "scream",
			},
			expected: "A_TestPack_Voice_Scream.wav",
		},
		{
			name: "creature_category",
			file: AudioFile{
				OriginalName: "roar.wav",
				Category:     "SFX_Creature",
				SubCategory:  "roar",
			},
			expected: "A_TestPack_Creature_Roar.wav",
		},
		{
			name: "with_numbers",
			file: AudioFile{
				OriginalName: "test_123.wav",
				Category:     "SFX",
				SubCategory:  "test_123",
			},
			expected: "A_TestPack_Sfx_Test_123.wav",
		},
		{
			name: "mp3_format",
			file: AudioFile{
				OriginalName: "test.mp3",
				Category:     "SFX",
				SubCategory:  "test",
			},
			expected: "A_TestPack_Sfx_Test.mp3",
		},
		{
			name: "no_subcategory",
			file: AudioFile{
				OriginalName: "test.wav",
				Category:     "SFX",
				SubCategory:  "",
			},
			expected: "A_TestPack_Sfx.wav",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ap.generateUE5Name(&tt.file)
			if result != tt.expected {
				t.Errorf("generateUE5Name() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	ap := NewAudioProcessor(Config{PackName: "TestPack"})

	tests := []struct {
		name           string
		originalName   string
		expectedID     string
		expectedSource string
		expectedCat    string
	}{
		{
			name:           "with_id",
			originalName:   "PE-Horror_BW.28968.wav",
			expectedID:     "28968",
			expectedSource: "BW",
			expectedCat:    "SFX_Percussion", // PE- prefix should infer percussion
		},
		{
			name:           "with_source",
			originalName:   "Scream_SFXB.1471.wav",
			expectedID:     "1471",
			expectedSource: "SFXB",
			expectedCat:    "SFX_Voice", // NormalizeCategory preserves case
		},
		{
			name:           "dash_category",
			originalName:   "FX-Impact.wav",
			expectedID:     "",
			expectedSource: "",
			expectedCat:    "SFX_FX", // FX becomes category, Impact becomes subcategory
		},
		{
			name:           "no_id_or_source",
			originalName:   "test_sound.wav",
			expectedID:     "",
			expectedSource: "sound", // last underscore segment is treated as source
			expectedCat:    "SFX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			af := AudioFile{
				OriginalName: tt.originalName,
			}
			ap.parseFile(&af)

			if af.ID != tt.expectedID {
				t.Errorf("parseFile() ID = %q, want %q", af.ID, tt.expectedID)
			}
			if af.Source != tt.expectedSource {
				t.Errorf("parseFile() Source = %q, want %q", af.Source, tt.expectedSource)
			}
			if af.Category != tt.expectedCat {
				t.Errorf("parseFile() Category = %q, want %q", af.Category, tt.expectedCat)
			}
		})
	}
}

func TestDetectDuplicates(t *testing.T) {
	ap := NewAudioProcessor(Config{PackName: "TestPack"})

	// create test files with same fingerprint
	fingerprint := "test_fingerprint_123"
	ap.audioFiles = []AudioFile{
		{OriginalName: "file1.wav", AudioMeta: &AudioMetadata{Fingerprint: fingerprint}},
		{OriginalName: "file2.wav", AudioMeta: &AudioMetadata{Fingerprint: fingerprint}},
		{OriginalName: "file3.wav", AudioMeta: &AudioMetadata{Fingerprint: "different_fp"}},
	}
	ap.fingerprints[fingerprint] = []int{0, 1}

	ap.detectDuplicates()

	// check that duplicates are tagged
	if !contains(ap.audioFiles[0].Tags, "duplicate") {
		t.Error("file1 should be tagged as duplicate")
	}
	if !contains(ap.audioFiles[1].Tags, "duplicate") {
		t.Error("file2 should be tagged as duplicate")
	}
	if contains(ap.audioFiles[2].Tags, "duplicate") {
		t.Error("file3 should not be tagged as duplicate")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
