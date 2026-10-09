package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/tiff"
)

type remotionWord struct {
	Text         string
	X, Y, Size   int
	Appear, Fade float64
	Kind         string
	Initial      bool
}

type remotionSlide struct {
	Start, End                float64
	Type                      string
	Style                     string
	EnterSeconds, ExitSeconds float64
	CrossfadeIn, CrossfadeOut bool
	Words                     []remotionWord
}

type remotionPlan struct {
	Config                        Config
	Theme                         themeConfig
	Timing                        outroTransition
	Fonts                         map[string]string
	Slides                        []remotionSlide
	Frames                        int
	BackgroundFrames, AudioFrames int
	OutroStart, ProgressEnd       float64
	LogoAspectRatio               float64
	Encoders                      []Encoder
	Mode                          string
}

func useRemotion(cfg Config) bool { return cfg.Renderer == "remotion" || cfg.Renderer == "" }

func readFontData(path string) ([]byte, error) {
	switch path {
	case embeddedContentFont:
		return embeddedSourceSerif, nil
	case embeddedSansFont:
		return embeddedSourceSans, nil
	case embeddedFont:
		return embeddedPassionOneBold, nil
	case defaultFontPath:
		if !fileExists(path) {
			return embeddedPassionOneBold, nil
		}
	}
	return os.ReadFile(path)
}

// Opening words need time to settle even when the intro is zero. Finish
// their reveal before the full-verse hold, including on very brief slides.
func initialScriptureFade(slide ContentSlide, available float64) float64 {
	return math.Min(available, math.Max(2, slide.FadeSeconds))
}

func buildRemotionPlan(cfg Config) (remotionPlan, error) {
	plan := remotionPlan{
		Config: cfg, Theme: themeByName(cfg.Theme), Timing: outroTiming(cfg.OutroSeconds),
		Frames:     maxInt(1, int(math.Ceil(outputDuration(cfg)*float64(cfg.FPS)))),
		OutroStart: outroStartTime(cfg), ProgressEnd: progressEndTime(cfg.Duration),
		LogoAspectRatio: logoAspectRatio(cfg),
		Fonts:           make(map[string]string), Encoders: encoderPlan(cfg),
	}
	parsed := make(map[string]*opentype.Font)
	for _, role := range []struct{ name, path string }{
		{"timer", cfg.FontTimer}, {"title", cfg.FontTitle}, {"subtitle", cfg.FontSubtitle}, {"content", cfg.FontContent},
	} {
		data, err := readFontData(role.path)
		if err != nil {
			return plan, fmt.Errorf("%s font: %w", role.name, err)
		}
		face, err := opentype.Parse(data)
		if err != nil {
			return plan, fmt.Errorf("%s requires a TTF or OTF font: %w", role.name, err)
		}
		parsed[role.name] = face
		plan.Fonts[role.name] = "data:font/ttf;base64," + base64.StdEncoding.EncodeToString(data)
	}
	items, err := scheduleContent(cfg)
	if err != nil {
		return plan, err
	}
	for i, item := range items {
		layout, err := fitContent(item.Slide, parsed["content"], parsed["subtitle"], contentArea(cfg), minInt(cfg.Width, cfg.Height))
		if err != nil {
			return plan, fmt.Errorf("content slide %d: %w", i+1, err)
		}
		slide := remotionSlide{Start: item.Start, End: item.End, Type: item.Slide.Type, Style: item.Slide.RevealStyle}
		if slide.Style == "" {
			slide.Style = "gentle-lift"
		}
		times, fade, err := revealTimes(item)
		if err != nil {
			return plan, err
		}
		anchors, _ := contentAnchors(item.Slide)
		for j, line := range layout.Lines {
			for _, word := range line.Words {
				appear, wordFade := times[word.Index], fade
				if item.Slide.Type == "statement" {
					appear, wordFade = item.Start, math.Min(0.8, (item.End-item.Start)/4)
				} else if anchors[word.Index] {
					wordFade = initialScriptureFade(item.Slide, item.Slide.IntroSeconds+item.Slide.RevealSeconds)
				}
				slide.Words = append(slide.Words, remotionWord{
					Text: word.Text, X: layout.X + word.X, Y: layout.Baseline + j*layout.LineHeight,
					Size: layout.FontSize, Appear: appear, Fade: wordFade, Kind: "content",
					Initial: item.Slide.Type == "statement" || anchors[word.Index],
				})
			}
		}
		for _, meta := range []struct {
			lines    []contentLine
			baseline int
			kind     string
		}{
			{layout.LabelLines, layout.LabelBaseline, "label"}, {layout.MetaLines, layout.MetaBaseline, "reference"},
		} {
			for j, line := range meta.lines {
				var words []string
				for _, word := range line.Words {
					words = append(words, word.Text)
				}
				if len(words) > 0 {
					metaFade := math.Min(0.6, (item.End-item.Start)/4)
					if item.Slide.Type == "scripture-reveal" {
						metaFade = initialScriptureFade(item.Slide, item.Slide.IntroSeconds+item.Slide.RevealSeconds)
					}
					slide.Words = append(slide.Words, remotionWord{
						Text: strings.Join(words, " "), X: layout.X, Y: meta.baseline + j*layout.MetaHeight,
						Size: layout.MetaSize, Appear: item.Start, Fade: metaFade, Kind: meta.kind,
						Initial: true,
					})
				}
			}
		}
		plan.Slides = append(plan.Slides, slide)
	}
	applyRemotionTransitions(&plan, items)
	return plan, nil
}

// Center the crossfade on the existing slide boundary. Outgoing scripture
// starts fading only in its reserved fade tail, after the full reading hold.
// The next slide's initial words appear early; its word-reveal timing stays
// unchanged. Playlist timing and the countdown/outro duration do not change.
func applyRemotionTransitions(plan *remotionPlan, items []scheduledContent) {
	cfg := plan.Config
	for i := range plan.Slides {
		if cfg.SlideTransition != "none" {
			plan.Slides[i].ExitSeconds = math.Min(contentFadeSeconds, (items[i].End-items[i].Start)/4)
		}
	}
	if cfg.SlideTransition != "crossfade" || cfg.TransitionSeconds == 0 {
		return
	}
	for i := 1; i < len(plan.Slides); i++ {
		previous, next := items[i-1], items[i]
		half := math.Min(cfg.TransitionSeconds/2, math.Min(contentFadeSeconds, (previous.End-previous.Start)/4))
		half = math.Min(half, (next.End-next.Start)/4)
		if next.Slide.Type == "scripture-reveal" {
			half = math.Min(half, next.Slide.IntroSeconds+next.Slide.RevealSeconds)
		}
		duration := 2 * half
		plan.Slides[i-1].End += half
		plan.Slides[i-1].ExitSeconds = duration
		plan.Slides[i-1].CrossfadeOut = true
		plan.Slides[i].Start -= half
		plan.Slides[i].EnterSeconds = duration
		plan.Slides[i].CrossfadeIn = true
		for j := range plan.Slides[i].Words {
			if plan.Slides[i].Words[j].Initial {
				plan.Slides[i].Words[j].Appear = plan.Slides[i].Start
				if next.Slide.Type == "scripture-reveal" {
					available := next.Start + next.Slide.IntroSeconds + next.Slide.RevealSeconds - plan.Slides[i].Start
					plan.Slides[i].Words[j].Fade = initialScriptureFade(next.Slide, available)
				} else {
					plan.Slides[i].Words[j].Fade = 0
				}
			}
		}
	}
}

func findRemotionDir(requested string) (string, error) {
	var candidates []string
	if requested != "" {
		candidates = []string{requested}
	} else {
		if binary, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(binary), "remotion"))
		}
		candidates = append(candidates, "remotion")
	}
	for _, dir := range candidates {
		if fileExists(filepath.Join(dir, "render.mjs")) && fileExists(filepath.Join(dir, "package.json")) {
			return filepath.Abs(dir)
		}
	}
	return "", fmt.Errorf("Remotion renderer folder not found; keep remotion beside countdownnow or set --remotion-dir")
}

// Only media metadata is inspected. Video/audio bytes belong to Remotion's
// decoding pipeline, never to the playlist or layout code.
func probeMediaFrames(path string, fps int) (int, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("inspect media duration: %w: %s", err, strings.TrimSpace(string(out)))
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || !finitePositive(duration) {
		return 0, fmt.Errorf("could not determine a finite duration for %s", path)
	}
	// Do not sample the end of a clip beyond its last decoded frame.
	return maxInt(1, int(math.Floor(duration*float64(fps)))), nil
}

func runRemotion(cfg Config, mode string) error {
	dir, err := findRemotionDir(cfg.RemotionDir)
	if err != nil {
		return err
	}
	resolved, cleanup, err := prepareEmbeddedAssets(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	plan, err := buildRemotionPlan(resolved)
	if err != nil {
		return err
	}
	plan.Mode = mode
	if cfg.DryRun {
		fmt.Printf("Remotion: %s\nCanvas: %dx%d at %d fps, %d frames\nPlaylist: %d slides\n", dir, cfg.Width, cfg.Height, cfg.FPS, plan.Frames, len(plan.Slides))
		for i, slide := range cfg.Content {
			fmt.Printf("Slide %d: %s", i+1, slide.Type)
			if slide.Type == "scripture-reveal" {
				order, style := slide.RevealOrder, slide.RevealStyle
				if order == "" {
					order = "random"
				}
				if style == "" {
					style = "gentle-lift"
				}
				fmt.Printf(", order=%s, style=%s", order, style)
				if slide.Seed != nil {
					fmt.Printf(", seed=%d", *slide.Seed)
				}
			}
			fmt.Println()
		}
		fmt.Printf("Options: background=%q, fit=%s, motion=%s, transition=%s (%ss), theme=%s, logo=%s, logo-outro=%s, logo-shadow=%s, progress=%s, outro=%s, audio=%q, end-sound=%q\n", cfg.Background, cfg.BackgroundFit, cfg.BackgroundMotion, cfg.SlideTransition, ff(cfg.TransitionSeconds), cfg.Theme, cfg.LogoPosition, cfg.LogoOutro, cfg.LogoShadow, cfg.ProgressStyle, cfg.OutroMode, cfg.AudioFile, cfg.EndSoundFile)
		fmt.Printf("Encoders: %v\n", plan.Encoders)
		return nil
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("Remotion needs Node.js 22 or newer in PATH; install Node.js, then run npm --prefix %s ci", dir)
	}
	if !fileExists(filepath.Join(dir, "node_modules", "@remotion", "renderer", "package.json")) {
		return fmt.Errorf("Remotion dependencies are not installed; run npm --prefix %s ci", dir)
	}
	for _, path := range []*string{&plan.Config.Background, &plan.Config.LogoFile, &plan.Config.AudioFile, &plan.Config.EndSoundFile, &plan.Config.RemotionBrowser} {
		if *path != "" {
			*path, err = filepath.Abs(*path)
			if err != nil {
				return err
			}
		}
	}
	if cfg.Background != "" && !cfg.BackgroundIsImage {
		plan.BackgroundFrames, err = probeMediaFrames(cfg.Background, cfg.FPS)
		if err != nil {
			return err
		}
	}
	if cfg.AudioFile != "" && cfg.Preview == "" && mode != "check" {
		plan.AudioFrames, err = probeMediaFrames(cfg.AudioFile, cfg.FPS)
		if err != nil {
			return err
		}
	}
	work, err := os.MkdirTemp("", "countdown-remotion-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	// Chrome cannot display TIFF directly. Decode the still without using any
	// video filters; the scene's fit, theme and motion remain in Remotion.
	if cfg.BackgroundIsImage && (strings.EqualFold(filepath.Ext(cfg.Background), ".tif") || strings.EqualFold(filepath.Ext(cfg.Background), ".tiff")) {
		input, err := os.Open(cfg.Background)
		if err != nil {
			return err
		}
		img, err := tiff.Decode(input)
		input.Close()
		if err != nil {
			return fmt.Errorf("decode TIFF background: %w", err)
		}
		plan.Config.Background = filepath.Join(work, "background.png")
		output, err := os.Create(plan.Config.Background)
		if err != nil {
			return err
		}
		err = png.Encode(output, img)
		closeErr := output.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if cfg.Preview != "" {
		plan.Config.Preview, err = filepath.Abs(cfg.Preview)
		if err != nil {
			return err
		}
	}
	var temporary string
	if mode == "check" {
		plan.Config.Preview = filepath.Join(work, "check.png")
	} else if cfg.Preview == "" {
		f, err := os.CreateTemp(filepath.Dir(cfg.Output), ".countdown-remotion-*.mp4")
		if err != nil {
			return err
		}
		temporary = f.Name()
		f.Close()
		defer os.Remove(temporary)
		plan.Config.Output, err = filepath.Abs(temporary)
		if err != nil {
			return err
		}
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	path := filepath.Join(work, "plan.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	cmd := exec.Command(node, filepath.Join(dir, "render.mjs"), path)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Remotion render failed: %w", err)
	}
	if mode == "check" {
		return nil
	}
	if cfg.Preview != "" {
		fmt.Println(cfg.Preview)
		return nil
	}
	if err := os.Rename(temporary, cfg.Output); err != nil {
		// Windows cannot rename over an existing destination.
		if !fileExists(cfg.Output) {
			return err
		}
		if err := os.Remove(cfg.Output); err != nil {
			return err
		}
		if err := os.Rename(temporary, cfg.Output); err != nil {
			return err
		}
	}
	fmt.Println(cfg.Output)
	return nil
}
