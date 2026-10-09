# Countdown Video Generator

CountdownNow generates countdown videos with Remotion. The Go CLI handles settings, playlist validation and text layout; React composes the entire scene, including backgrounds, timer, logo, progress, scripture, audio and outro. FFmpeg handles decoding, audio mixing and encoding underneath Remotion. A legacy FFmpeg renderer remains available with `--renderer ffmpeg`.

Watch the demo: [CountdownNow on YouTube](https://www.youtube.com/watch?v=nxPPpFHKHBw)

The video includes:

- Slowly drifting light over a radial gradient, or a custom image/video background
- Large `MM:SS` countdown timer
- Shrinking bottom progress bar
- Optional logo overlay with configurable position
- Final outro where the timer/logo fade and the title/subtitle appear
- Hardware encoder attempts when available, with automatic `libx264` fallback
- Responsive layout sizing for landscape, vertical, square, ultrawide, 4K, and custom dimensions
- Themes, outro modes, progress styles, preview frames, and optional audio
- User-supplied scripture reveals and statement slides while the countdown runs
- Gentle eased reveals with a per-verse choice of reading order or repeatable random order
- Soft crossfades between verses and statements, with quiet sans serif timer digits and serif reading text

## Requirements

Install **Node.js 22 or newer** and keep the `remotion` folder beside the executable. Install the pinned renderer dependencies once:

```sh
npm --prefix remotion ci
```

Remotion downloads its compatible Chrome Headless Shell on the first render/check. To use an existing Chrome installation, pass `--remotion-browser /path/to/chrome`. Renders run locally. Fonts are embedded; no online font service or scripture lookup is used.

Install FFmpeg/ffprobe for inspecting custom media dimensions and durations. Remotion includes its own encoding binaries and does not require system `drawtext` or gradient filters. The legacy renderer needs a full FFmpeg build.

macOS:

```sh
brew install ffmpeg-full
```

Windows and Linux users should install a full FFmpeg package from their package manager or from the official FFmpeg builds. After installing FFmpeg, verify the runtime with:

```sh
./countdownnow --check
```

## Quick Start

Download the release archive for your platform from the [latest GitHub Release](https://github.com/sukujgrg/countdownnow/releases/latest), extract it, install the renderer dependencies above, and run the binary.

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

Print the render plan without rendering or downloading a browser:

```sh
./countdownnow --duration 0.5 --dry-run
```

Check the selected renderer before rendering:

```sh
./countdownnow --check
```

The Remotion check validates playlist timing and layouts, loads all fonts in Chrome, renders a still, and encodes two test frames with `libx264`. The legacy `--renderer ffmpeg --check` checks system filters, encoders and a tiny `drawtext` render.

Use `--remotion-dir` if the renderer folder lives elsewhere, and `--remotion-concurrency` to change the number of frames rendered together (default 2). Remotion retains temporary frames until encoding succeeds so a failed hardware encoder can fall back without rendering the scene again. Long or high-resolution exports need temporary disk space; files are cleaned up after success or a handled error.

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

Remotion synchronizes animated GIF, PNG, AVIF and WebP backgrounds to the frame timeline. TIFF backgrounds are decoded into a temporary PNG for Chrome. Preview exports support PNG, JPEG and WebP directly; other still formats use system FFmpeg for image encoding.

Background fit modes:

- `cover`: fill the output canvas and crop overflow. This is the default.
- `contain`: show the whole background with black padding when aspect ratios differ.
- `stretch`: resize exactly to the output size, even if that distorts the background.

## Scripture Reveals And Statements

Use the included church playlist as a starting point:

```sh
./countdownnow --content examples/church-content.json --duration 90 --output church-countdown.mp4
```

A few selected words appear first. The remaining words ease into view with a small upward movement and a soft blur that clears as they settle. They appear in a repeatable random order by default, with an optional normal reading order for each verse. Every word keeps its final sentence position; the layout stays still during the full-passage reading hold. The countdown remains visible alongside the text in landscape, or above it in portrait and square layouts. Statement slides show their complete text together.

Create a JSON file with your own wording and timing:

```json
{
  "slides": [
    {
      "type": "scripture-reveal",
      "text": "The LORD is my shepherd; I shall not want.",
      "reference": "Psalm 23:1",
      "translation": "KJV",
      "anchor_words": ["LORD", "shepherd"],
      "reveal_order": "random",
      "seed": 10,
      "reveal_style": "gentle-lift",
      "fade_seconds": 1.2,
      "intro_seconds": 3,
      "reveal_seconds": 17,
      "hold_seconds": 12
    },
    {
      "type": "statement",
      "label": "WELCOME",
      "text": "We are glad you are here.\nOur service begins shortly.",
      "duration_seconds": 10
    }
  ]
}
```

Supply the exact passage, reference, and translation yourself; the app does not fetch or rewrite scripture. Use `\n` for intentional line breaks. Text also wraps automatically and keeps the same positions throughout the reveal.

Scripture settings:

- `text` and `reference` are required. `translation` is optional.
- `anchor_words` selects the words revealed at the start. In Remotion, these words, the label and reference ease in over at least 2 seconds, with subtle softening and the selected reveal style. Longer `fade_seconds` values also apply to this opening. Brief slides shorten the entrance so it finishes before the reading hold; an intro of zero still fades in gently. Matching ignores capitalization and surrounding punctuation; every occurrence of a selected word appears. Omit this field to select a few words automatically, or use `[]` to start with no words.
- `reveal_order`: `random` (default) or `normal`. This changes which word appears next, while leaving sentence positions unchanged.
- `seed`: optional signed integer. The same seed and verse produce the same order on every render. Omit it for a stable seed derived from the verse text and reference. Change it for a different arrangement. Seed `0` is valid. It has no effect in normal order.
- `reveal_style`: `gentle-lift` (default) or `soft-dissolve` (no movement). Scripture words use an overdamped Remotion spring to come softly into focus; `gentle-lift` adds a very small settle of at most 2.5 pixels. There is no bounce or word zoom, and words keep their final sentence positions. Opacity eases across the full `fade_seconds`; all motion ends before the reading hold. This works with either reveal order, including the first anchor words. The legacy renderer uses opacity fades and supports the same order/seed settings.
- `fade_seconds`: each word's reveal length, default 1.2 seconds. Reveals can overlap to keep them soft. This is capped by `reveal_seconds` so all words finish before the reading hold.
- `intro_seconds` defaults to 3 and may be zero. `reveal_seconds` defaults to 17 and `hold_seconds` to 12; both must be greater than zero. The last word finishes fading before the reading hold begins.
- Each scripture slide adds a 0.6-second fade after its hold. The default total is 32.6 seconds.
- `label` is optional and defaults to `SCRIPTURE`.

Statements require `text`. Their optional `duration_seconds` defaults to 10, including the fade in and fade out, and their optional `label` defaults to `WELCOME`.

Slides play once in the order supplied. The playlist must fit before the selected outro; an oversized playlist produces an error before rendering. Any remaining time extends the last slide's reading hold. With `--outro-over-duration`, the playlist can use the whole countdown; with `--outro none`, no outro time is reserved.

All existing background, music, logo, theme, progress, size, font, preview, and encoder options can be combined with `--content`. Text sizes adapt to the canvas and avoid the selected logo area. Passages that cannot fit at a readable size produce an error; split them into shorter slides or adjust logo/padding settings. Use a TTF or OTF font with the characters your text needs:

```sh
./countdownnow --content examples/church-content.json --duration 90 --font-content ./reading-font.ttf
./countdownnow --content examples/church-content.json --duration 90 --preset vertical --theme warm
./countdownnow --content examples/church-content.json --duration 90 --preview scripture-preview.png --preview-time 22
./countdownnow --content examples/church-content.json --duration 90 --check
```

`--font` applies to content as well as existing text. `--font-content` overrides only the scripture/statement body; labels and references use `--font-subtitle`. Scripture and welcome titles default to embedded Source Serif 4; the timer, labels and supporting text use embedded Source Sans 3 Regular. Timer digits use fixed-width numerals to avoid shifting as seconds change. The original Passion One font is still available through a custom font override. Without `--content`, the timer stays centered. The Remotion design uses softer shadows and eased outro motion; `--outro typewriter` reveals actual characters. With content, the service caption and timer fade out together over one second and are fully hidden before the digits reach `00:02`, leaving a calm ending without a zero countdown. This also applies to extended outros and `--outro none`. Without content, the timer keeps its existing outro behavior; `--outro none` keeps it visible.

Timing, reveal order and motion depend on the frame number, with no wall-clock timers or unseeded randomness. Keep the Remotion lockfile, font files, rendering options and browser version fixed for reproducible frames. Browser or encoder upgrades can change output bytes even when the reveal sequence stays the same.

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

By default, `--outro-seconds` is part of the duration. To add the outro after
the countdown, keep the timer duration unchanged and extend the final video:

```sh
./countdownnow --duration 10 --outro-seconds 6 --outro-over-duration
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
./countdownnow --font-content ./reading-font.ttf
```

Disable the final title and subtitle:

```sh
./countdownnow --title none --subtitle none
```

The timer and corner logo still fade out during the outro. When both title and subtitle are disabled, no centered closing logo is shown.

## Visual Options

Generated Remotion backgrounds have low-contrast pools of light that drift over roughly 1–3 minute cycles. They have no pulsing or flashing. Custom image/video backgrounds retain their supplied appearance and motion. Freeze the generated light with:

```sh
./countdownnow --background-motion none
```

Playlist slides use a gentle crossfade by default. The outgoing verse stays fully visible for its complete reading hold, then clears before the next slide becomes readable. The overlap is faint, with a slight softening of fading text, so two sentences do not compete. The background and timer remain continuous. Remaining scripture words retain their original reveal timing. These transitions do not extend the video or shorten reading holds. The default crossfade is 1.2 seconds and is shortened when a brief slide needs more reading space:

```sh
./countdownnow --content examples/church-content.json --duration 90 --slide-transition crossfade --transition-seconds 1.2
./countdownnow --content examples/church-content.json --duration 90 --slide-transition fade
./countdownnow --content examples/church-content.json --duration 90 --slide-transition none
```

`fade` uses the existing 0.6-second fade tails without overlapping slides; `none` switches slides immediately while keeping individual word reveals, including the gentle scripture opening. The existing fade tail limits crossfades to 1.2 seconds so the reading hold stays intact. A crossfade duration of zero uses the plain fade behavior. Background motion and slide transitions are Remotion settings; the legacy renderer retains its static gradient and plain fade tails.

Render the short church example, with both reveal orders and a quiet statement between verses:

```sh
./countdownnow --content examples/calm-church-demo.json --duration 33 --theme warm --outro fade --output calm-church-countdown.mp4
```

Compare the calmer scripture effect with normal order first and a repeatable random order second:

```sh
./countdownnow --content examples/scripture-soft-focus-demo.json --duration 23.2 --theme warm --outro fade --output scripture-soft-focus.mp4
```

Set a color theme:

```sh
./countdownnow --theme midnight
./countdownnow --theme warm
./countdownnow --theme sunrise
./countdownnow --theme minimal
```

Override the timer font color:

```sh
./countdownnow --timer-color "#ffcc00"
./countdownnow --timer-color white
```

Set the outro reveal style:

In Remotion countdowns with content, the ending becomes a centered closing scene with a larger serif title and a smaller supporting subtitle. The timer clears before this scene appears. Remotion springs give the title a soft focus reveal with restrained movement; multiword titles enter with a slight stagger. A thin accent line opens from the center, followed by the subtitle. Motion settles for the final reading hold, without shifting the greeting across the screen. Fade and slide keep their different movement strengths, and typewriter still reveals literal characters. Extended outros use the same closing scene; `none` omits it.

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

`bottom-bar` is the default shrinking progress indicator. Remotion renders a thin line with a muted track, a gentle color gradient and a soft light at the moving edge. Its length decreases steadily with time, without pulses or bouncing. The line eases in at the start and fades out before the greeting; with no outro it fades just before the countdown ends. Colors follow the selected theme, and sizing and placement follow the canvas and safe-area settings. Use `none` to hide the progress indicator. The legacy renderer retains its original bar.

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

Remotion defaults to a smaller corner logo aligned with the reading area's edge margins. The logo fades in gently over 1.8 seconds (shortened for brief countdowns), stays still during the countdown, and fades out before the greeting. A separate centered logo softly appears above the final title/subtitle; wide and tall logos keep their aspect ratio and fit within a bounded closing area. Explicit widths, margins and positions still apply during the countdown. Disable the centered closing treatment or its subtle transparency-preserving shadow with:

```sh
./countdownnow --logo-outro fade
./countdownnow --logo-shadow none
```

`--logo-outro center` is the default. Disabling the logo or the outro also disables the closing logo. These treatments apply to Remotion; the legacy renderer retains its existing logo layout and fade.

The legacy renderer warns when `ffprobe` reports a low-resolution logo or a logo pixel format that may not include transparency.

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
npm --prefix remotion ci
./countdownnow
```

The Makefile builds with `-trimpath -buildvcs=false` and `-ldflags="-s -w"` so the binary omits local source paths, VCS metadata, and debug symbols.

Run `make test` for Go checks and `npm --prefix remotion test` for motion, encoding-adapter and local asset-server checks. For actual Chrome frame tests, set `COUNTDOWN_REMOTION_TEST=1` before running Go tests; optionally set `COUNTDOWN_REMOTION_BROWSER` to an existing Chrome executable. These tests verify a repeat render, changing words during the reveal, literal punctuation, and an unchanged full-verse reading hold.

Remotion and React versions are pinned together in `remotion/package-lock.json`. Release archives include the renderer sources and lockfile; Node and dependencies are installed on the destination machine. License notices are in `THIRD-PARTY-NOTICES.txt` and the included font licenses. Remotion has its own [license terms](https://www.remotion.dev/license).

Build metadata is injected at build time. If `HEAD` is on a git tag, that tag is used as the version; otherwise the short commit SHA is used.
