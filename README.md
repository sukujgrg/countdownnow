# Countdown Video Generator

This project generates countdown videos with FFmpeg. The primary tool is the Go CLI in `main.go`, which builds the FFmpeg filter graph from presets and command-line flags.

The video includes:

- Static radial gradient background, or a custom image/video background
- Large `MM:SS` countdown timer
- Shrinking bottom progress bar
- Optional logo overlay with configurable position
- Final outro where the timer/logo fade and the title/subtitle appear
- Hardware encoder attempts when available, with automatic `libx264` fallback
- Responsive layout sizing for landscape, vertical, square, ultrawide, 4K, and custom dimensions
- Themes, outro modes, progress styles, preview frames, and optional audio

## Requirements

Install a full FFmpeg build before running CountdownNow.

macOS:

```sh
brew install ffmpeg-full
```

Windows and Linux users should install a full FFmpeg package from their package manager or from the official FFmpeg builds. After installing FFmpeg, verify the runtime with:

```sh
./countdownnow --check
```

## Quick Start

Download the release archive for your platform from the [latest GitHub Release](https://github.com/sukujgrg/countdownnow/releases/latest), extract it, and run the binary.

Run the default 5-minute landscape countdown:

```sh
./countdownnow
```

```sh
./countdownnow --version
./countdownnow --help
```

To run an unsigned or unnotarized macOS download, remove the quarantine attribute after extracting the archive:

```sh
xattr -dr com.apple.quarantine ./countdownnow-macos-universal-unsigned
./countdownnow-macos-universal-unsigned/countdownnow --help
```

Default output:

```text
countdown_300s.mp4
```

## Short Test Renders

Use short renders while tuning layout:

```sh
./countdownnow --duration 0.5 --output /tmp/countdown-test.mp4 --encoder software
```

Print the generated FFmpeg command without rendering:

```sh
./countdownnow --duration 0.5 --dry-run
```

Check the runtime FFmpeg installation before rendering:

```sh
./countdownnow --check
```

The check verifies the FFmpeg executable, required video/audio filters, `libx264`, listed hardware encoders, and a tiny `drawtext` render.

Generate one preview frame without rendering a video:

```sh
./countdownnow --preview preview.png --preview-time 120
```

Use a custom image or video background:

```sh
./countdownnow --background background.png
./countdownnow --background background.mp4
./countdownnow --background background.mp4 --width 1920 --height 1080 --background-fit cover
./countdownnow --background poster.png --width 1080 --height 1920 --background-fit contain
```

When `--background` is provided without `--width` and `--height`, the output video uses the background's first video stream dimensions. If you provide dimensions, pass both `--width` and `--height`; those values override the background size.

Image backgrounds are looped for the full countdown. Video backgrounds are looped and trimmed to `--duration`.

Background fit modes:

- `cover`: fill the output canvas and crop overflow. This is the default.
- `contain`: show the whole background with black padding when aspect ratios differ.
- `stretch`: resize exactly to the output size, even if that distorts the background.

## Presets And Sizes

Landscape, 1280x720:

```sh
./countdownnow --preset landscape
```

Vertical, 1080x1920:

```sh
./countdownnow --preset vertical --output vertical-countdown.mp4
```

Square, 1080x1080:

```sh
./countdownnow --preset square --output square-countdown.mp4
```

Ultrawide, 2560x1080:

```sh
./countdownnow --preset ultrawide --output ultrawide-countdown.mp4
```

4K landscape, 3840x2160:

```sh
./countdownnow --preset 4k --output countdown-4k.mp4
```

You can also override dimensions directly. The timer, logo, progress bar, title, subtitle, margins, and bitrate scale from the selected canvas size:

```sh
./countdownnow --width 1920 --height 1080 --output custom.mp4
./countdownnow --width 1080 --height 1350 --output social-4x5.mp4
./countdownnow --width 2160 --height 3840 --output vertical-4k.mp4
./countdownnow --width 3440 --height 1440 --output wide-monitor.mp4
```

## Text Options

Set duration:

```sh
./countdownnow --duration 600
```

Set title:

```sh
./countdownnow --title "Sunday Gathering"
```

Set outro subtitle:

```sh
./countdownnow --subtitle "We are glad you are here."
```

Use a custom font file for all text:

```sh
./countdownnow --font ./fonts/Passion_One/PassionOne-Bold.ttf
```

Advanced font overrides:

```sh
./countdownnow --font-title ./title.ttf --font-subtitle ./subtitle.ttf
./countdownnow --font-timer ./timer.ttf
```

Disable the final title and subtitle:

```sh
./countdownnow --title none --subtitle none
```

The timer and logo still fade out during the outro.

## Visual Options

Set a color theme:

```sh
./countdownnow --theme midnight
./countdownnow --theme warm
./countdownnow --theme sunrise
./countdownnow --theme minimal
```

Set the outro reveal style:

```sh
./countdownnow --outro slide
./countdownnow --outro fade
./countdownnow --outro typewriter
./countdownnow --outro none
```

Set the progress indicator:

```sh
./countdownnow --progress bottom-bar
./countdownnow --progress none
```

`bottom-bar` is the default shrinking bar. It runs through the countdown and finishes just before the end so the final moment is clean. Use `none` to hide the progress indicator.

Add extra padding for screens that crop edges:

```sh
./countdownnow --safe-area 80
```

## Logo Options

Default logo path is auto-detected from:

```text
logo.png
default.png
Round Icon Design.png
```

If no local logo file is available, the Go binary uses its embedded `logo.png`.

Use a specific logo:

```sh
./countdownnow --logo logo.png
```

Disable the logo:

```sh
./countdownnow --logo none
./countdownnow --logo-position none
```

Set logo position:

```sh
./countdownnow --logo-position top-right
./countdownnow --logo-position top-left
./countdownnow --logo-position bottom-right
./countdownnow --logo-position bottom-left
./countdownnow --logo-position center
```

Resize or adjust margins:

```sh
./countdownnow --logo-width 160 --logo-margin-x 30 --logo-margin-y 40
```

The app warns when `ffprobe` reports a low-resolution logo or a logo pixel format that may not include transparency.

## Audio Options

Add background audio:

```sh
./countdownnow --audio worship-pad.mp3 --audio-volume 0.35
```

Fade background audio during the outro:

```sh
./countdownnow --audio worship-pad.mp3 --fade-audio
```

Mix in a short sound effect near the end:

```sh
./countdownnow --end-sound chime.wav
```

## Encoder Options

Default mode is `auto`:

```sh
./countdownnow --encoder auto
```

Auto mode tries hardware encoders first for the current OS, then falls back to software:

- macOS: `h264_videotoolbox`, then `libx264`
- Windows: `h264_nvenc`, `h264_qsv`, `h264_amf`, then `libx264`
- Linux: `h264_nvenc`, `h264_qsv`, then `libx264`

Force software:

```sh
./countdownnow --encoder software
```

Use a specific FFmpeg encoder:

```sh
./countdownnow --encoder h264_nvenc
```

Set hardware bitrate:

```sh
./countdownnow --bitrate 8M
```

Set software quality:

```sh
./countdownnow --encoder software --crf 20 --preset-x264 veryfast
```

## Developer Build

Build from source:

```sh
make build
./countdownnow
```

The Makefile builds with `-trimpath -buildvcs=false` and `-ldflags="-s -w"` so the binary omits local source paths, VCS metadata, and debug symbols.

Build metadata is injected at build time. If `HEAD` is on a git tag, that tag is used as the version; otherwise the short commit SHA is used.
