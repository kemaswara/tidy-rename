# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.2] - 2025-11-30

### Fixed
- Fixed GitHub Actions release workflow to properly attach all platform binaries (Windows, Linux, macOS) to releases
- Added artifact flattening step to ensure all build artifacts are included in releases

## [1.0.1] - 2025-11-30

### Fixed
- Fixed Windows archive creation in GitHub Actions workflow - now uses PowerShell's `Compress-Archive` instead of missing `zip` command

## [1.0.0] - 2025-11-30

### Added
- Initial release of tidy-rename tool
- UE5 naming convention support with `A_` prefix
- Audio file analysis extracting actual metadata (duration, sample rate, channels, bit depth, bitrate)
- Embedded tag reading (ID3, Vorbis comments) from audio files
- Automatic file categorization based on filename patterns and audio properties
- Folder organization by category
- Pack name support for UE5 naming convention
- Duplicate name handling with sequential numbering
- Manifest.json generation with complete file metadata
- Dry-run mode for previewing changes before applying
- Support for multiple audio formats: WAV, MP3, OGG, FLAC, AAC, M4A, WMA
- Word boundary preservation in filenames
- Variant ID removal for cleaner naming
- Audio property-based tagging (duration, quality, channels)
- Cross-platform support (Windows, Linux, macOS)
- Parallel processing for audio file analysis using worker pools (8 workers by default)
- Progress bars showing real-time status during analysis and file operations
- Detailed progress information (files processed, percentage, rate)
- **Spectral analysis** - extracts frequency characteristics (low/mid/high energy bands, zero crossing rate, spectral centroid) from WAV files to improve categorization
- **Audio fingerprinting** - generates content-based fingerprints to detect duplicate files
- **Confidence scoring** - combines filename patterns, metadata, and spectral features with weighted scoring for smarter category inference
- Duplicate detection reporting - automatically identifies and tags files with identical audio content

### Changed
- N/A (initial release)

### Deprecated
- N/A (initial release)

### Removed
- N/A (initial release)

### Fixed
- N/A (initial release)

### Security
- N/A (initial release)

[Unreleased]: https://github.com/kemaswara/tidy-rename/compare/v1.0.2...HEAD
[1.0.2]: https://github.com/kemaswara/tidy-rename/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/kemaswara/tidy-rename/compare/v1.0.0...v1.0.1

