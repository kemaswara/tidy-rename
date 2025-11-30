package main

import (
	"strings"
	"time"
)

// CategoryRule defines how to match a category based on filename patterns
type CategoryRule struct {
	Category   string   // The category name (e.g., "SFX_Voice", "Ambient")
	Keywords   []string // Keywords that match this category
	Exclusions []string // Keywords that exclude this category (e.g., "atmos" excludes vehicles)
	Priority   int      // Higher priority = checked first (important for ambiguous cases)
	Confidence float64  // Default confidence score when matched
}

// CategoryRules defines all category matching rules
// Order matters - rules are checked in order, so put more specific rules first
var CategoryRules = []CategoryRule{
	// Drones (check early, specific)
	{
		Category:   "SFX_Drone",
		Keywords:   []string{"drone"},
		Priority:   10,
		Confidence: 0.8,
	},
	// Loops (check early, specific)
	{
		Category:   "Music",
		Keywords:   []string{"loop"},
		Priority:   10,
		Confidence: 0.8,
	},
	// Risers (check early, specific)
	{
		Category:   "SFX_Riser",
		Keywords:   []string{"riser"},
		Priority:   10,
		Confidence: 0.8,
	},
	// Slowmotion/Timelapse (check early, specific)
	{
		Category:   "SFX_Time",
		Keywords:   []string{"slowmotion", "slow motion", "slow-motion", "timelapse", "time lapse", "time-lapse"},
		Priority:   10,
		Confidence: 0.8,
	},
	// Transitions (check early, specific)
	{
		Category:   "SFX_Transition",
		Keywords:   []string{"transition"},
		Priority:   10,
		Confidence: 0.7,
	},
	// Whooshes (check early, specific)
	{
		Category:   "SFX_Whoosh",
		Keywords:   []string{"whoosh"},
		Priority:   10,
		Confidence: 0.8,
	},
	// Voice/Dialogue
	{
		Category:   "SFX_Voice",
		Keywords:   []string{"scream", "voice", "dialogue", "speech", "male", "female", "grunt", "groan"},
		Priority:   8,
		Confidence: 0.8,
	},
	// Creatures/Animals
	{
		Category:   "SFX_Creature",
		Keywords:   []string{"creature", "monster", "animal", "beast", "roar", "growl", "howl", "moan", "cat", "dog", "cow", "rooster", "monkey", "meow", "bark", "moo", "pur"},
		Priority:   8,
		Confidence: 0.8,
	},
	// Ambient/Environment (check before vehicles to catch "atmos")
	{
		Category:   "Ambient",
		Keywords:   []string{"wind", "rain", "thunder", "storm", "water", "ocean", "forest", "nature", "atmos", "atmosphere", "ambient", "ambience", "flame", "flames", "burning", "ember", "campfire", "bonfire", "jungle", "rainforest", "insect", "cicada", "cricket", "frog", "waterfall", "river", "stream", "wave", "beach", "underwater", "monsoon", "downpour", "raindrop", "lightning", "wind chime", "windchime", "city", "urban", "traffic", "crowd", "market", "construction", "airport", "station", "restaurant", "kitchen", "street", "highway", "freeway", "intersection", "walla", "room tone", "roomtone"},
		Priority:   9,
		Confidence: 0.8,
		// Special handling for standalone "fire" - handled separately
	},
	// Weapons/Combat (with special fire handling)
	{
		Category:   "SFX_Weapon",
		Keywords:   []string{"gun", "weapon", "shot", "bullet", "sword", "slash", "punch", "combat", "gunfire", "firearm", "samurai", "kung fu", "karate"},
		Priority:   7,
		Confidence: 0.8,
		// Special: "fire" only matches if combined with weapon keywords
	},
	// Impacts/Explosions
	{
		Category:   "SFX_Impact",
		Keywords:   []string{"explosion", "explode", "impact", "crash", "slam", "thud", "bang", "boom", "gong", "hit"},
		Priority:   7,
		Confidence: 0.8,
	},
	// Footsteps/Movement
	{
		Category:   "SFX_Footstep",
		Keywords:   []string{"footstep", "step", "walk", "run", "jump", "land"},
		Priority:   7,
		Confidence: 0.8,
	},
	// Vehicles (with exclusions for ambient-related)
	{
		Category:   "SFX_Vehicle",
		Keywords:   []string{"vehicle", "car", "engine", "motor", "tire", "wheel", "drive", "bus", "train", "truck", "motorbike", "motorcycle", "tuktuk", "aeroplane", "airplane", "ferry", "boat", "driveby", "drive-by", "pass by", "passby", "hoot", "honk", "horn"},
		Exclusions: []string{"atmos", "atmosphere", "ambient", "ambience", "room tone", "roomtone"},
		Priority:   6,
		Confidence: 0.8,
	},
	// UI/Interface
	{
		Category:   "SFX_UI",
		Keywords:   []string{"ui", "interface", "button", "click", "select", "hover", "menu", "notification"},
		Priority:   7,
		Confidence: 0.9,
	},
	// Alarms/Alerts
	{
		Category:   "SFX_Alarm",
		Keywords:   []string{"alarm", "alert", "siren", "warning", "beep", "buzz"},
		Priority:   7,
		Confidence: 0.8,
	},
	// Mechanical
	{
		Category:   "SFX_Mechanical",
		Keywords:   []string{"mechanical", "machine", "gear", "whir", "clank", "robot"},
		Priority:   6,
		Confidence: 0.8,
	},
	// Doors/Objects
	{
		Category:   "SFX_Object",
		Keywords:   []string{"door", "open", "close", "creak", "squeak", "hinge", "object", "item", "pickup", "drop"},
		Priority:   6,
		Confidence: 0.8,
	},
	// Percussion/Drums
	{
		Category:   "SFX_Percussion",
		Keywords:   []string{"drum", "percussion", "beat", "kick", "snare", "cymbal", "gong", "tambourine", "bell", "clap"},
		Priority:   7,
		Confidence: 0.8,
	},
	// Traditional/Ceremonial
	{
		Category:   "SFX_Traditional",
		Keywords:   []string{"traditional", "ceremony", "ceremonial", "temple", "chant", "chanting", "pray", "praying", "royal", "emperor", "ancient"},
		Priority:   7,
		Confidence: 0.8,
	},
	// String Instruments
	{
		Category:   "SFX_String",
		Keywords:   []string{"pluck", "string", "guitar", "harp", "sitar", "pipa"},
		Priority:   7,
		Confidence: 0.8,
	},
	// Music
	{
		Category:   "Music",
		Keywords:   []string{"music", "song", "track", "score", "melody", "theme"},
		Priority:   6,
		Confidence: 0.8,
	},
}

// CategoryNormalization maps various category name formats to standardized names
var CategoryNormalization = map[string]string{
	"PE":          "SFX_Percussion",
	"PERCUSSION":  "SFX_Percussion",
	"SFX":         "SFX",
	"VOICE":       "SFX_Voice",
	"CREATURE":    "SFX_Creature",
	"WEAPON":      "SFX_Weapon",
	"IMPACT":      "SFX_Impact",
	"FOOTSTEP":    "SFX_Footstep",
	"VEHICLE":     "SFX_Vehicle",
	"ALARM":       "SFX_Alarm",
	"MECHANICAL":  "SFX_Mechanical",
	"OBJECT":      "SFX_Object",
	"AMBIENT":     "Ambient",
	"MUSIC":       "Music",
	"UI":          "UI",
	"DIALOGUE":    "Dialogue",
	"DRONE":       "SFX_Drone",
	"LOOP":        "Music",
	"RISER":       "SFX_Riser",
	"SLOWMOTION":  "SFX_Time",
	"SLOW_MOTION": "SFX_Time",
	"TIMELAPSE":   "SFX_Time",
	"TIME_LAPSE":  "SFX_Time",
	"TRANSITION":  "SFX_Transition",
	"WHOOSH":      "SFX_Whoosh",
	"TRADITIONAL": "SFX_Traditional",
	"CEREMONIAL":  "SFX_Traditional",
	"STRING":      "SFX_String",
	"CITY":        "Ambient",
	"URBAN":       "Ambient",
}

// matchCategoryRule checks if a filename matches a category rule
func matchCategoryRule(nameLower string, rule CategoryRule) bool {
	// Check exclusions first
	for _, exclusion := range rule.Exclusions {
		if strings.Contains(nameLower, exclusion) {
			return false
		}
	}

	// Check keywords
	for _, keyword := range rule.Keywords {
		if strings.Contains(nameLower, keyword) {
			// Special handling for "fire" in weapon category
			if rule.Category == "SFX_Weapon" && keyword == "fire" {
				// Only match "fire" if it's clearly weapon-related
				if strings.Contains(nameLower, "gunfire") || strings.Contains(nameLower, "firearm") ||
					strings.Contains(nameLower, "fire_") || strings.Contains(nameLower, "_fire") ||
					(strings.Contains(nameLower, "gun") || strings.Contains(nameLower, "weapon") || strings.Contains(nameLower, "shot")) {
					return true
				}
				return false
			}
			return true
		}
	}

	// Special handling for standalone "fire" -> Ambient
	if rule.Category == "Ambient" {
		if nameLower == "fire" || strings.HasPrefix(nameLower, "fire ") || strings.HasSuffix(nameLower, " fire") {
			// Make sure it's not weapon-related
			if !strings.Contains(nameLower, "gun") && !strings.Contains(nameLower, "weapon") &&
				!strings.Contains(nameLower, "shot") && !strings.Contains(nameLower, "gunfire") &&
				!strings.Contains(nameLower, "firearm") {
				return true
			}
		}
	}

	return false
}

// InferCategory matches filename against category rules and returns the best match
func InferCategory(filename string) string {
	nameLower := strings.ToLower(filename)

	// Sort rules by priority (higher first)
	rules := make([]CategoryRule, len(CategoryRules))
	copy(rules, CategoryRules)

	// Check rules in priority order
	for _, rule := range rules {
		if matchCategoryRule(nameLower, rule) {
			return rule.Category
		}
	}

	return "SFX" // default fallback
}

// InferCategoryWithConfidenceScores matches filename and returns confidence scores for all matching categories
func InferCategoryWithConfidenceScores(filename string) map[string]float64 {
	nameLower := strings.ToLower(filename)
	scores := make(map[string]float64)

	// Check all rules and accumulate scores
	for _, rule := range CategoryRules {
		if matchCategoryRule(nameLower, rule) {
			scores[rule.Category] += rule.Confidence
		}
	}

	return scores
}

// NormalizeCategory converts various category name formats to standardized names
func NormalizeCategory(cat string) string {
	catUpper := strings.ToUpper(cat)

	if normalized, ok := CategoryNormalization[catUpper]; ok {
		return normalized
	}

	// Auto-prefix with SFX_ if it's not already categorized
	if !strings.Contains(catUpper, "_") && !strings.Contains(catUpper, "MUSIC") && !strings.Contains(catUpper, "AMBIENT") {
		return "SFX_" + catUpper
	}

	return cat
}

// ApplyMetadataScoring adds confidence scores based on audio metadata
func ApplyMetadataScoring(scores map[string]float64, meta *AudioMetadata, filenameLower string) {
	if meta == nil {
		return
	}

	// Duration-based scoring
	if meta.Duration > 0 {
		if meta.Duration < 2*time.Second {
			scores["SFX_UI"] += 0.6
		} else if meta.Duration < 5*time.Second {
			scores["SFX"] += 0.4
		} else if meta.Duration > 30*time.Second {
			scores["Ambient"] += 0.5
			// Long files with "fire" are likely ambient fire sounds, not weapon fire
			if strings.Contains(filenameLower, "fire") && !strings.Contains(filenameLower, "gun") &&
				!strings.Contains(filenameLower, "weapon") && !strings.Contains(filenameLower, "shot") &&
				!strings.Contains(filenameLower, "gunfire") && !strings.Contains(filenameLower, "firearm") {
				scores["Ambient"] += 0.4
				if scores["SFX_Weapon"] > 0 {
					scores["SFX_Weapon"] -= 0.3
				}
			}
			if meta.HasEmbeddedTags && meta.Genre != "" {
				genreLower := strings.ToLower(meta.Genre)
				if strings.Contains(genreLower, "music") {
					scores["Music"] += 0.6
				}
			}
		}
	}

	// Channel-based scoring
	if meta.Channels == 1 {
		scores["SFX"] += 0.3 // mono = usually focused SFX
	} else if meta.Channels >= 5 {
		scores["Ambient"] += 0.4 // surround = probably ambient
	}

	// Genre-based scoring
	if meta.HasEmbeddedTags && meta.Genre != "" {
		genreLower := strings.ToLower(meta.Genre)
		if strings.Contains(genreLower, "voice") || strings.Contains(genreLower, "dialogue") {
			scores["SFX_Voice"] += 0.7
		}
		if strings.Contains(genreLower, "music") {
			scores["Music"] += 0.7
		}
		if strings.Contains(genreLower, "ambient") {
			scores["Ambient"] += 0.7
		}
	}
}
