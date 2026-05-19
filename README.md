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

Generated media files are output artifacts. Do not inspect audio/video file contents unless explicitly needed.

## Requirements

Install FFmpeg:

```sh
brew install ffmpeg-full
```

Default assets are embedded into the Go binary at build time:

```text
logo.png
fonts/Passion_One/PassionOne-Bold.ttf
fonts/Passion_One/OFL.txt
```

The CLI still accepts external logo and font paths with `--logo` and `--font`.

`fonts/Passion_One/OFL.txt` is included because the embedded default font is licensed under the SIL Open Font License.

## Quick Start

Run the default 5-minute landscape countdown:

```sh
go run .
```

Build a reusable binary:

```sh
make build
./countdownnow
```

The Makefile builds with `-trimpath -buildvcs=false` and `-ldflags="-s -w"` so the binary omits local source paths, VCS metadata, and debug symbols.

Build metadata is injected at build time. If `HEAD` is on a git tag, that tag is used as the version; otherwise the short commit SHA is used.

```sh
./countdownnow --version
./countdownnow --help
```

## Releases

GitHub Actions publishes release binaries when a tag starting with `v` is pushed:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds Linux, macOS, and Windows binaries for `amd64` and `arm64`, then attaches `.tar.gz` or `.zip` archives to the GitHub Release.

Default output:

```text
countdown_300s.mp4
```

## Short Test Renders

Use short renders while tuning layout:

```sh
go run . --duration 0.5 --output /tmp/countdown-test.mp4 --encoder software
```

Print the generated FFmpeg command without rendering:

```sh
go run . --duration 0.5 --dry-run
```

Check the runtime FFmpeg installation before rendering:

```sh
go run . --check
./countdownnow --check
```

The check verifies the FFmpeg executable, required video/audio filters, `libx264`, listed hardware encoders, and a tiny `drawtext` render.

Generate one preview frame without rendering a video:

```sh
go run . --preview preview.png --preview-time 120
```

Use a custom image or video background:

```sh
go run . --background background.png
go run . --background background.mp4
go run . --background background.mp4 --width 1920 --height 1080 --background-fit cover
go run . --background poster.png --width 1080 --height 1920 --background-fit contain
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
go run . --preset landscape
```

Vertical, 1080x1920:

```sh
go run . --preset vertical --output vertical-countdown.mp4
```

Square, 1080x1080:

```sh
go run . --preset square --output square-countdown.mp4
```

Ultrawide, 2560x1080:

```sh
go run . --preset ultrawide --output ultrawide-countdown.mp4
```

4K landscape, 3840x2160:

```sh
go run . --preset 4k --output countdown-4k.mp4
```

You can also override dimensions directly. The timer, logo, progress bar, title, subtitle, margins, and bitrate scale from the selected canvas size:

```sh
go run . --width 1920 --height 1080 --output custom.mp4
go run . --width 1080 --height 1350 --output social-4x5.mp4
go run . --width 2160 --height 3840 --output vertical-4k.mp4
go run . --width 3440 --height 1440 --output wide-monitor.mp4
```

## Text Options

Set duration:

```sh
go run . --duration 600
```

Set title:

```sh
go run . --title "Sunday Gathering"
```

Set outro subtitle:

```sh
go run . --subtitle "We are glad you are here."
```

Use a custom font for all text:

```sh
go run . --font ./fonts/Passion_One/PassionOne-Bold.ttf
```

Advanced font overrides:

```sh
go run . --font-title ./title.ttf --font-subtitle ./subtitle.ttf
go run . --font-timer ./timer.ttf
```

Disable the final title and subtitle:

```sh
go run . --title none --subtitle none
```

The timer and logo still fade out during the outro.

## Visual Options

Set a color theme:

```sh
go run . --theme midnight
go run . --theme warm
go run . --theme sunrise
go run . --theme minimal
```

Set the outro reveal style:

```sh
go run . --outro slide
go run . --outro fade
go run . --outro typewriter
go run . --outro none
```

Set the progress indicator:

```sh
go run . --progress bottom-bar
go run . --progress none
```

`bottom-bar` is the default shrinking bar. It runs through the countdown and finishes just before the end so the final moment is clean. Use `none` to hide the progress indicator.

Add extra padding for screens that crop edges:

```sh
go run . --safe-area 80
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
go run . --logo logo.png
```

Disable the logo:

```sh
go run . --logo none
go run . --logo-position none
```

Set logo position:

```sh
go run . --logo-position top-right
go run . --logo-position top-left
go run . --logo-position bottom-right
go run . --logo-position bottom-left
go run . --logo-position center
```

Resize or adjust margins:

```sh
go run . --logo-width 160 --logo-margin-x 30 --logo-margin-y 40
```

The app warns when `ffprobe` reports a low-resolution logo or a logo pixel format that may not include transparency.

## Audio Options

Add background audio:

```sh
go run . --audio worship-pad.mp3 --audio-volume 0.35
```

Fade background audio during the outro:

```sh
go run . --audio worship-pad.mp3 --fade-audio
```

Mix in a short sound effect near the end:

```sh
go run . --end-sound chime.wav
```

## Encoder Options

Default mode is `auto`:

```sh
go run . --encoder auto
```

Auto mode tries hardware encoders first for the current OS, then falls back to software:

- macOS: `h264_videotoolbox`, then `libx264`
- Windows: `h264_nvenc`, `h264_qsv`, `h264_amf`, then `libx264`
- Linux: `h264_nvenc`, `h264_qsv`, then `libx264`

Force software:

```sh
go run . --encoder software
```

Use a specific FFmpeg encoder:

```sh
go run . --encoder h264_nvenc
```

Set hardware bitrate:

```sh
go run . --bitrate 8M
```

Set software quality:

```sh
go run . --encoder software --crf 20 --preset-x264 veryfast
```
