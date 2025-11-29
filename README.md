<div align="center">
  <img src="assets/tidy_rename_logo.png" alt="Tidy Rename Logo" width="200">
</div>

# Tidy Rename

A simple Go tool to clean up and organize audio files for Unreal Engine 5 projects. It renames files to follow UE5 conventions, extracts metadata from the actual audio files (not just filenames), and organizes everything into a structure that makes sense.

## What it does

- Renames files to UE5 format (starts with `A_`)
- Analyzes actual audio files to get duration, sample rate, channels, bit depth, etc.
- Reads embedded tags (ID3, Vorbis comments) if they exist
- **Spectral analysis** - analyzes frequency characteristics (low/mid/high energy bands, zero crossing rate, spectral centroid) for better categorization
- **Audio fingerprinting** - detects duplicate files with identical audio content
- **Confidence scoring** - combines filename patterns, metadata, and spectral features for smarter categorization
- Automatically categorizes files based on filename patterns and audio properties
- Organizes files into folders by category
- Generates a manifest.json with all the metadata
- Lets you preview changes before applying them (dry-run mode)
- **Fast parallel processing** - analyzes multiple files simultaneously
- **Progress bars** - see real-time status as files are processed

## Installation

### Download pre-built binaries

Check the [Releases](https://github.com/kemaswara/tidy-rename/releases) page for pre-built binaries for Windows, Linux, and macOS. Just download the archive for your platform, extract it, and you're ready to go.

### Build from source

Just build it:

```bash
go build -o tidy-rename
```

Or build for different platforms:

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o tidy-rename.exe

# Linux
GOOS=linux GOARCH=amd64 go build -o tidy-rename

# macOS
GOOS=darwin GOARCH=amd64 go build -o tidy-rename
```

## Usage

You need to provide a source directory and a pack name. The pack name is used in the UE5 naming convention.

```bash
# Preview what it will do (recommended first step)
./tidy-rename -source ./audio_files -pack "HorrorPack" -dry-run

# Actually do it
./tidy-rename -source ./audio_files -pack "HorrorPack"

# Output to a different directory
./tidy-rename -source ./audio_files -pack "HorrorPack" -output ./cleaned_audio
```

### Options

- `-source <path>` - Where your audio files are (required)
- `-output <path>` - Where to put the cleaned files (defaults to source directory)
- `-pack <name>` - Pack name for UE5 naming, like "HorrorPack" or "MyGameSFX" (required)
- `-dry-run` - Preview changes without modifying anything
- `-organize` - Put files in category folders (default: true)
- `-manifest` - Create manifest.json file (default: true)

## How naming works

The tool converts messy filenames into clean UE5 format. Here are some examples:

**Before → After (with pack name "HorrorPack"):**
- `thunder_crash_heavy_STUDIO.9876.wav` → `A_HorrorPack_Sfx_Thunder_Crash_Heavy.wav`
- `thunder_crash_heavy_STUDIO.9877.wav` → `A_HorrorPack_Sfx_Thunder_Crash_Heavy_01.wav` (duplicate gets numbered)
- `scream_female_pain_VOICE.5432.wav` → `A_HorrorPack_Voice_Scream_Female_Pain.wav`
- `monster_roar_deep_CREATURE.2109.wav` → `A_HorrorPack_Creature_Monster_Roar_Deep.wav`
- `wind_howling_desert_AMBIENT.8765.wav` → `A_HorrorPack_Ambient_Wind_Howling_Desert.wav`

**Format:** `A_<PackName>_<Category>_<SubCategory>[_<Number>].<ext>`

The tool removes variant IDs and source codes to keep names clean. If you have duplicate names, it automatically numbers them (_01, _02, etc.).

## Categories

Files get automatically sorted into categories:

- `SFX` - General sound effects
- `SFX_Percussion` - Percussion and impact sounds
- `SFX_Voice` - Voice, scream, and dialogue effects
- `SFX_Creature` - Creature and monster sounds
- `SFX_Alarm` - Alarm and siren sounds
- `Ambient` - Ambient and environmental sounds
- `Music` - Music tracks
- `UI` - User interface sounds
- `Dialogue` - Dialogue files

The categorization is based on filename patterns and audio properties (like duration). Short sounds (< 2s) often get categorized as UI, longer ones (> 30s) might be ambient or music.

## Output structure

When you use `-organize` (which is the default), files get sorted into folders:

```
output/
├── SFX/
│   ├── A_HorrorPack_Sfx_Thunder_Crash_Heavy.wav
│   ├── A_HorrorPack_Sfx_Thunder_Crash_Heavy_01.wav
│   └── A_HorrorPack_Sfx_Door_Creak_Wooden.wav
├── SFX_Voice/
│   ├── A_HorrorPack_Voice_Scream_Female_Pain.wav
│   └── A_HorrorPack_Voice_Groan_Male.wav
├── SFX_Creature/
│   └── A_HorrorPack_Creature_Monster_Roar_Deep.wav
├── Ambient/
│   └── A_HorrorPack_Ambient_Wind_Howling_Desert.wav
└── manifest.json
```

## Manifest file

The tool creates a `manifest.json` file with all the metadata it collected:

- Total file count and category breakdown
- For each file:
  - Original and new file paths
  - Categories and tags
  - Source information (if found in filename)
  - Audio properties: duration, sample rate, channels, bit depth, bitrate
  - Embedded tags: title, artist, album, genre, year (if the file has them)

This is useful for keeping track of what you have and for importing into other tools.

## Supported formats

Works with:
- WAV
- MP3
- OGG
- FLAC
- AAC
- M4A
- WMA

The tool actually reads the audio files to extract metadata, not just the filenames. For WAV files, it gets accurate duration, sample rate, and channel info. For compressed formats, it relies on embedded tags and file size estimates.

## Usage Examples

### Basic Workflow

```bash
# Step 1: Preview what will happen (always do this first!)
./tidy-rename -source ./my_audio_files -pack "MyGamePack" -dry-run

# Step 2: Review the preview output, then run for real
./tidy-rename -source ./my_audio_files -pack "MyGamePack"
```

### Different Scenarios

**Processing a large library with subdirectories:**
```bash
# The tool recursively scans subdirectories automatically
./tidy-rename -source ./audio_library -pack "GameSFX" -output ./organized_audio
```

**Keeping files flat (no category folders):**
```bash
# Disable folder organization
./tidy-rename -source ./audio_files -pack "HorrorPack" -organize=false
```

**Just renaming, no manifest:**
```bash
# Skip manifest generation if you don't need it
./tidy-rename -source ./audio_files -pack "ActionPack" -manifest=false
```

**Processing specific file types only:**
The tool automatically filters to supported audio formats. If you have mixed content, it will only process:
- `.wav`, `.mp3`, `.ogg`, `.flac`, `.aac`, `.m4a`, `.wma`

**Working with existing UE5 projects:**
```bash
# If your files are already in a UE5 project structure
./tidy-rename -source ./Content/Audio -pack "HorrorPack" -output ./Content/Audio_Cleaned
```

### Advanced Workflow

**Batch processing multiple packs:**
```bash
# Process different categories separately
./tidy-rename -source ./weapons -pack "WeaponPack" -output ./cleaned/weapons
./tidy-rename -source ./voices -pack "VoicePack" -output ./cleaned/voices
./tidy-rename -source ./ambient -pack "AmbientPack" -output ./cleaned/ambient
```

**Using with version control:**
```bash
# Always use dry-run first when files are in git
./tidy-rename -source ./audio -pack "MyPack" -dry-run

# Review changes, then commit before running
git add .
git commit -m "Backup before renaming"

# Now run for real
./tidy-rename -source ./audio -pack "MyPack"
```

## Tips

- **Always use `-dry-run` first** to see what it will do before making changes
- Files are **moved, not copied** - make sure you have backups if needed
- The tool analyzes actual audio properties, so it takes a moment to process each file
- Duplicate names get numbered automatically, so you don't have to worry about conflicts
- If a file can't be analyzed (corrupted, unsupported format, etc.), it still gets processed but you'll see a warning
- Large directories (1000+ files) will take a while - the progress bar shows you what's happening

## Limitations & Known Issues

This tool works pretty well for most cases, but there are some areas where it's still "dumb" or could be better:

**Category Detection:**
- The filename-based categorization is pretty basic - it just looks for keywords. If your files have weird naming conventions, it might not categorize them correctly
- The audio property-based categorization (using duration/channels) is a rough heuristic. Short files might not always be UI sounds, etc.
- Some edge cases might get miscategorized - you might need to manually fix a few files after running

**Naming:**
- The name cleaning is pretty aggressive - it strips out a lot of special characters. If you have important info in weird characters, it might get lost
- Pack name casing tries to be smart but might not always work perfectly if you give it something unusual
- Duplicate detection only works on the final cleaned name, so if two very different files end up with the same name after cleaning, one will get numbered

**Audio Analysis:**
- WAV file analysis is pretty accurate, but compressed formats (MP3, OGG, etc.) rely on embedded tags which might not always be there
- Duration estimates for compressed files are rough - they're based on file size and bitrate, which isn't always accurate
- Bit depth detection for WAV files just assumes 16-bit (most common case) - it doesn't actually read it from the file
- Spectral analysis only works on WAV files (compressed formats skip this step)
- Audio fingerprinting uses metadata-based hashing - it's good for detecting exact duplicates but won't catch similar-sounding files
- Confidence scoring combines multiple signals but is still heuristic-based, not ML-powered

**Error Handling:**
- If a file can't be analyzed, it just skips it and continues. You might not notice until you check the output
- Cross-device file moves (copy+delete) could fail partway through and leave you with duplicates

If you run into issues or have ideas for improvements, feel free to open an issue or submit a PR!

## Troubleshooting

### Common Issues

**"Error: -source flag is required"**
- Make sure you're providing the `-source` flag with a path
- Check that the path exists and is accessible
- On Windows, use quotes if the path has spaces: `-source "C:\My Audio Files"`

**"Error: -pack flag is required"**
- You must provide a pack name for UE5 naming
- Use quotes if the pack name has spaces: `-pack "My Game Pack"`

**"Error: Source directory does not exist"**
- Verify the path is correct
- Use absolute paths if relative paths aren't working
- On Windows, check for typos in drive letters (C: vs D:)

**Files aren't being renamed correctly**
- Check the preview output with `-dry-run` first
- Some special characters in filenames might get stripped
- Very unusual naming patterns might not categorize correctly

**"No audio files found"**
- Make sure your files have one of the supported extensions: `.wav`, `.mp3`, `.ogg`, `.flac`, `.aac`, `.m4a`, `.wma`
- Check that files aren't in a subdirectory that's being skipped
- Verify file permissions allow reading

**Processing is very slow**
- This is normal for large directories - the tool analyzes each file
- WAV files take longer because of spectral analysis
- Progress bars show you it's working - be patient!

**Files are being moved but I can't find them**
- Check the output directory (defaults to source if not specified)
- If using `-organize`, files are in category subfolders
- Check the manifest.json to see where each file ended up

**Duplicate detection found files that aren't duplicates**
- The fingerprinting uses metadata, so files with identical properties will match
- Check the manifest.json to see which files were flagged
- You can manually rename files after processing if needed

**Category is wrong for some files**
- The categorization is heuristic-based and not perfect
- Check the preview with `-dry-run` first
- You can manually move/rename files after processing
- Consider improving filenames before processing for better results

**"Failed to analyze" warnings**
- Some files might be corrupted or in an unsupported format variant
- The tool will still rename them, just without full metadata
- Check the manifest.json to see which files had issues

**Cross-platform path issues**
- On Windows, use backslashes or forward slashes (both work)
- On Linux/macOS, use forward slashes
- Avoid special characters in paths if possible

### Getting Help

If you're still stuck:
1. Run with `-dry-run` and check the preview output
2. Check the `manifest.json` to see what metadata was extracted
3. Look at the error messages - they usually tell you what's wrong
4. Open an issue on GitHub with:
   - The command you ran
   - The error message (if any)
   - Your operating system
   - A sample of problematic filenames (if relevant)

## FAQ

**Q: Will this modify my original files?**  
A: Yes, by default files are moved (not copied). Always use `-dry-run` first to preview changes. If you want to keep originals, copy them to a different location first.

**Q: Can I undo the changes?**  
A: The tool doesn't have an undo feature. The `manifest.json` file contains the original paths, so you could write a script to reverse the changes if needed. Always backup first!

**Q: Does it work with files already in UE5 format?**  
A: Yes, but it will rename them again according to the pack name you provide. If your files are already properly named, you might not need this tool.

**Q: What if I don't want category folders?**  
A: Use `-organize=false` to keep all files in a single directory.

**Q: Can I process files from multiple directories?**  
A: The tool processes one directory at a time (recursively). Process each directory separately, or combine them first.

**Q: How does duplicate detection work?**  
A: It creates a fingerprint based on audio metadata (sample rate, channels, duration, format, title). Files with identical fingerprints are flagged as duplicates.

**Q: Why are some files taking so long to process?**  
A: WAV files undergo spectral analysis which reads audio samples. Large WAV files or many files will take longer. Compressed formats (MP3, OGG) are faster.

**Q: Can I customize the naming format?**  
A: Not currently - it follows UE5 conventions (`A_<Pack>_<Category>_<Name>`). You could modify the code or request this as a feature.

**Q: What happens to files that can't be categorized?**  
A: They default to the `SFX` category and still get renamed. Check the preview to see what category was assigned.

**Q: Does it preserve audio quality?**  
A: Yes! The tool only renames and moves files - it doesn't re-encode or modify the audio data itself.

**Q: Can I use this on files that are currently in use by UE5?**  
A: Not recommended. Close UE5 first, or the file moves might fail. Always backup first.

**Q: What's the difference between `-organize` and `-manifest`?**  
A: `-organize` creates category folders (SFX/, Voice/, etc.). `-manifest` creates a JSON file with all metadata. Both are enabled by default.

**Q: How do I know if a file was analyzed correctly?**  
A: Check the `manifest.json` file. Files with full metadata have duration, sample rate, channels, etc. Files with issues will have missing or estimated values.

**Q: Can I process only specific file types?**  
A: The tool automatically filters to supported formats. If you want only WAV files, you'd need to filter the directory first (or modify the code).

**Q: What if my pack name has special characters?**  
A: The tool will clean the pack name to be UE5-compliant. Use simple alphanumeric names for best results (e.g., "HorrorPack" not "Horror Pack!").

**Q: Does it work with audio files from asset stores?**  
A: Yes! It's designed to clean up messy asset store filenames. The duplicate detection is especially useful for asset packs with many variations.

## License

MIT License - do whatever you want with it. See the [LICENSE](LICENSE) file for details.
