package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepeatableRandomRevealAndNormalOrder(t *testing.T) {
	slide := testScripture()
	seed := int64(10)
	slide.RevealOrder, slide.Seed = "random", &seed
	anchors, _ := contentAnchors(slide)
	order := contentRevealOrder(slide, anchors)
	if !reflect.DeepEqual(order, contentRevealOrder(slide, anchors)) {
		t.Fatal("same seed changed the reveal order")
	}
	seen := make(map[int]bool)
	for _, i := range order {
		if anchors[i] || seen[i] {
			t.Fatal("random order contains an anchor or duplicate")
		}
		seen[i] = true
	}
	if len(seen)+len(anchors) != len(strings.Fields(slide.Text)) {
		t.Fatal("random order skipped words")
	}
	seed = 11
	if reflect.DeepEqual(order, contentRevealOrder(slide, anchors)) {
		t.Fatal("different seeds produced the same order")
	}
	slide.RevealOrder = "normal"
	last := -1
	for _, i := range contentRevealOrder(slide, anchors) {
		if i <= last {
			t.Fatal("normal order is not reading order")
		}
		last = i
	}
	slide.RevealOrder, slide.Seed = "random", nil
	if !reflect.DeepEqual(contentRevealOrder(slide, anchors), contentRevealOrder(slide, anchors)) {
		t.Fatal("implicit verse-derived seed is not stable")
	}
	item := scheduledContent{Slide: slide, Start: 2, End: 35}
	times, fade, err := revealTimes(item)
	if err != nil {
		t.Fatal(err)
	}
	lastTime := 0.0
	for i, time := range times {
		if anchors[i] {
			continue
		}
		lastTime = math.Max(lastTime, time)
		if time+fade > item.Start+slide.IntroSeconds+slide.RevealSeconds+1e-7 {
			t.Fatal("random reveal consumed reading hold")
		}
	}
	if math.Abs(lastTime+fade-(item.Start+slide.IntroSeconds+slide.RevealSeconds)) > 1e-7 {
		t.Fatal("reveal did not finish at hold boundary")
	}
}

func TestContentRevealOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "content.json")
	for _, field := range []string{`"reveal_order":"surprise"`, `"reveal_style":"bounce"`, `"seed":1.5`, `"fade_seconds":0`, `"fade_seconds":-1`} {
		data := `{"slides":[{"type":"scripture-reveal","text":"Every word is here.","reference":"Example 1:1",` + field + `}]}`
		os.WriteFile(path, []byte(data), 0600)
		if _, err := loadContentFile(path); err == nil {
			t.Fatalf("accepted invalid field %s", field)
		}
	}
	data := `{"slides":[{"type":"scripture-reveal","text":"Every word is here.","reference":"Example 1:1","seed":0},{"type":"scripture-reveal","text":"Every word is here.","reference":"Example 1:1","reveal_order":"normal","reveal_style":"soft-dissolve","fade_seconds":2}]}`
	os.WriteFile(path, []byte(data), 0600)
	slides, err := loadContentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if slides[0].RevealOrder != "random" || slides[0].RevealStyle != "gentle-lift" || slides[0].FadeSeconds != 1.2 || slides[0].Seed == nil || *slides[0].Seed != 0 {
		t.Fatalf("wrong defaults: %+v", slides[0])
	}
	if slides[1].RevealOrder != "normal" || slides[1].RevealStyle != "soft-dissolve" || slides[1].FadeSeconds != 2 {
		t.Fatal("lost per-verse options")
	}
}

func TestRemotionPlanPreservesLayoutAndExistingOptions(t *testing.T) {
	for _, preset := range []string{"landscape", "vertical", "square", "ultrawide", "4k"} {
		t.Run(preset, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Preset = preset
			applyPreset(&cfg)
			applyResponsiveLayout(&cfg)
			applyDefaults(&cfg)
			cfg.Content = []ContentSlide{testScripture()}
			cfg.OutroOverDuration, cfg.Theme, cfg.BackgroundFit = true, "warm", "contain"
			cfg.AudioFile, cfg.EndSoundFile = "music.wav", "end.wav"
			cfg.FadeAudio, cfg.AudioVolume, cfg.TimerColor = true, 0.2, "white@0.5"
			plan, err := buildRemotionPlan(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Frames != (300+3)*cfg.FPS || plan.OutroStart != 300 || !reflect.DeepEqual(plan.Config, cfg) {
				t.Fatal("renderer lost existing settings or timing")
			}
			if len(plan.Fonts) != 4 || len(plan.Slides) != 1 || plan.Slides[0].Type != "scripture-reveal" {
				t.Fatal("fonts or content missing from plan")
			}
			var words []string
			box := contentArea(cfg)
			for _, word := range plan.Slides[0].Words {
				if word.Kind != "content" {
					continue
				}
				words = append(words, word.Text)
				if word.X < box.Min.X || word.X > box.Max.X || word.Y < box.Min.Y || word.Y > box.Max.Y {
					t.Fatal("word is outside the content area")
				}
			}
			if strings.Join(words, " ") != cfg.Content[0].Text {
				t.Fatal("reveal changed the verse's wording or layout order")
			}
		})
	}
}

func TestRemotionCrossfadesPreserveReadingHoldsAndRevealTiming(t *testing.T) {
	cfg := contentConfig(t)
	cfg.Duration = 75
	tiny := testScripture()
	tiny.IntroSeconds, tiny.RevealSeconds, tiny.HoldSeconds = 0, 0.1, 0.2
	cfg.Content = []ContentSlide{testScripture(), {Type: "statement", Text: "Welcome.", DurationSeconds: 0.2}, tiny, testScripture()}
	items, err := scheduleContent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildRemotionPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i, slide := range plan.Slides {
		if i > 0 {
			previous := plan.Slides[i-1]
			if previous.End <= slide.Start || previous.ExitSeconds != slide.EnterSeconds {
				t.Fatal("crossfade has no shared overlap")
			}
		}
		if items[i].Slide.Type != "scripture-reveal" {
			continue
		}
		times, fade, _ := revealTimes(items[i])
		full := items[i].Start + items[i].Slide.IntroSeconds + items[i].Slide.RevealSeconds
		if slide.Start+slide.EnterSeconds > full+1e-7 {
			t.Fatal("entry transition consumed full-verse reading hold")
		}
		if slide.End-slide.ExitSeconds < full+items[i].Slide.HoldSeconds-1e-7 {
			t.Fatal("exit transition cut the reading hold short")
		}
		index := 0
		for _, word := range slide.Words {
			if word.Kind != "content" {
				continue
			}
			if !word.Initial && (word.Appear != times[index] || word.Fade != fade) {
				t.Fatal("crossfade changed the seeded word-reveal timing")
			}
			index++
		}
	}
	if plan.Frames != 75*cfg.FPS || plan.Slides[len(plan.Slides)-1].End != contentEndTime(cfg) {
		t.Fatal("crossfades changed countdown/outro timing")
	}
	cfg.SlideTransition = "none"
	plan, err = buildRemotionPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i, slide := range plan.Slides {
		if slide.Start != items[i].Start || slide.End != items[i].End || slide.EnterSeconds != 0 || slide.ExitSeconds != 0 {
			t.Fatal("disabled transitions still overlap or fade")
		}
	}
	cfg.SlideTransition = "fade"
	plan, err = buildRemotionPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i, slide := range plan.Slides {
		if slide.Start != items[i].Start || slide.End != items[i].End || slide.EnterSeconds != 0 || slide.ExitSeconds <= 0 {
			t.Fatal("plain fades should keep the original slide boundaries")
		}
	}
}

func TestRemotionInitialScriptureWordsFadeBeforeReadingHold(t *testing.T) {
	for _, transition := range []string{"crossfade", "fade", "none"} {
		for _, timing := range []struct {
			name        string
			intro, fade float64
			reveal      float64
		}{
			{"default", 3, 1.2, 17},
			{"no-intro", 0, 1.2, 3},
			{"long-fade", 1, 2.5, 4},
			{"brief", 0, 1.2, 0.1},
		} {
			t.Run(transition+"/"+timing.name, func(t *testing.T) {
				cfg := contentConfig(t)
				cfg.Duration, cfg.SlideTransition = 90, transition
				verse := testScripture()
				verse.IntroSeconds, verse.FadeSeconds, verse.RevealSeconds = timing.intro, timing.fade, timing.reveal
				cfg.Content = []ContentSlide{verse, {Type: "statement", Text: "Welcome.", DurationSeconds: 3}, verse}
				items, err := scheduleContent(cfg)
				if err != nil {
					t.Fatal(err)
				}
				plan, err := buildRemotionPlan(cfg)
				if err != nil {
					t.Fatal(err)
				}
				for _, i := range []int{0, 2} {
					full := items[i].Start + timing.intro + timing.reveal
					for _, word := range plan.Slides[i].Words {
						if !word.Initial {
							continue
						}
						if word.Fade <= 0 || word.Appear != plan.Slides[i].Start {
							t.Fatal("opening word appears abruptly rather than fading from the slide start")
						}
						if timing.name != "brief" && word.Fade < math.Max(2, timing.fade) {
							t.Fatal("opening fade is too short")
						}
						if word.Appear+word.Fade > full+1e-7 {
							t.Fatal("opening fade consumed full-verse reading time")
						}
					}
				}
			})
		}
	}
}

func TestRemotionScriptureFrames(t *testing.T) {
	if os.Getenv("COUNTDOWN_REMOTION_TEST") != "1" {
		t.Skip("set COUNTDOWN_REMOTION_TEST=1 to exercise the installed Remotion runtime")
	}
	cfg := contentConfig(t)
	cfg.Renderer = "remotion"
	cfg.RemotionBrowser = os.Getenv("COUNTDOWN_REMOTION_BROWSER")
	cfg.Width, cfg.Height, cfg.FPS = 640, 360, 30
	applyResponsiveLayout(&cfg)
	cfg.ProgressStyle, cfg.OutroMode = "none", "none"
	cfg.Duration = 4
	cfg.Content[0].Text = "Every 100% word: [stays] 'literal' \\ complete."
	cfg.Content[0].IntroSeconds, cfg.Content[0].RevealSeconds, cfg.Content[0].HoldSeconds = 0.3, 1.5, 1
	cfg.Content[0].RevealOrder = "random"
	seed := int64(10)
	cfg.Content[0].Seed = &seed
	dir := t.TempDir()
	background := filepath.Join(dir, "background 'quote' [brackets],;.png")
	f, err := os.Create(background)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 640, 360))); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	cfg.Background, cfg.BackgroundIsImage = background, true
	box := contentArea(cfg)
	var crops [][]byte
	for i, second := range []float64{1, 2, 2.5, 1} {
		cfg.PreviewTime = second
		cfg.Preview = filepath.Join(dir, []string{"partial.png", "full.png", "hold.png", "partial-repeat.png"}[i])
		if err := runRemotion(cfg, "render"); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(cfg.Preview)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		var pixels []byte
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				pixels = append(pixels, byte(r>>8), byte(g>>8), byte(b>>8))
			}
		}
		crops = append(crops, pixels)
	}
	if bytes.Equal(crops[0], crops[1]) {
		t.Fatal("words did not reveal")
	}
	if !bytes.Equal(crops[1], crops[2]) {
		t.Fatal("full scripture changed during reading hold")
	}
	if !bytes.Equal(crops[0], crops[3]) {
		t.Fatal("same frame rendered differently on repeat")
	}
}

func TestRemotionEncoderFallbackKeepsRenderedFrames(t *testing.T) {
	if os.Getenv("COUNTDOWN_REMOTION_TEST") != "1" {
		t.Skip("set COUNTDOWN_REMOTION_TEST=1 to exercise encoder fallback")
	}
	cfg := contentConfig(t)
	cfg.Content = nil
	cfg.Duration, cfg.OutroMode = 0.5, "none"
	cfg.Width, cfg.Height, cfg.FPS = 320, 180, 10
	cfg.RemotionBrowser = os.Getenv("COUNTDOWN_REMOTION_BROWSER")
	plan, err := buildRemotionPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	plan.Encoders = []Encoder{{Name: "unavailable", Args: []string{"-c:v", "countdown_test_missing_encoder"}}, softwareEncoder(cfg)}
	dir := t.TempDir()
	plan.Config.Output = filepath.Join(dir, "fallback.mp4")
	path := filepath.Join(dir, "plan.json")
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	renderer, err := findRemotionDir("")
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("node", filepath.Join(renderer, "render.mjs"), path).CombinedOutput()
	if err != nil {
		t.Fatalf("fallback failed: %v\n%s", err, output)
	}
	if bytes.Count(output, []byte("Rendering 5 frames")) != 1 || !bytes.Contains(output, []byte("Encoder unavailable failed")) || !bytes.Contains(output, []byte("Encoding: libx264")) || !fileExists(plan.Config.Output) {
		t.Fatalf("encoder fallback did not reuse the rendered frames: %s", output)
	}
}

func TestRemotionCrossfadeRendersBothSlides(t *testing.T) {
	if os.Getenv("COUNTDOWN_REMOTION_TEST") != "1" {
		t.Skip("set COUNTDOWN_REMOTION_TEST=1 to verify rendered crossfade layers")
	}
	cfg := contentConfig(t)
	cfg.Content = nil
	cfg.Duration, cfg.OutroMode = 4, "none"
	cfg.Width, cfg.Height, cfg.FPS = 640, 360, 30
	cfg.ProgressStyle, cfg.BackgroundMotion = "none", "none"
	cfg.RemotionBrowser = os.Getenv("COUNTDOWN_REMOTION_BROWSER")
	plan, err := buildRemotionPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	plan.Slides = []remotionSlide{
		{Start: 0, End: 2.6, Style: "soft-dissolve", ExitSeconds: 1.2, Words: []remotionWord{{Text: "Grace", X: 20, Y: 100, Size: 42, Kind: "content"}}},
		{Start: 1.4, End: 4, Style: "soft-dissolve", EnterSeconds: 1.2, Words: []remotionWord{{Text: "Peace", X: 320, Y: 200, Size: 42, Appear: 1.4, Kind: "content"}}},
	}
	dir := t.TempDir()
	plan.Config.Preview, plan.Config.PreviewTime = filepath.Join(dir, "transition.png"), 2
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	renderer, err := findRemotionDir("")
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("node", filepath.Join(renderer, "render.mjs"), path).CombinedOutput()
	if err != nil {
		t.Fatalf("crossfade render failed: %v\n%s", err, output)
	}
	f, err := os.Open(plan.Config.Preview)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	for i, box := range []image.Rectangle{image.Rect(20, 65, 160, 105), image.Rect(320, 165, 460, 205)} {
		visible := 0
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r>>8 > 75 && g>>8 > 75 && b>>8 > 50 {
					visible++
				}
			}
		}
		if visible < 100 {
			t.Fatalf("slide %d was absent at the crossfade midpoint (%d text pixels)", i+1, visible)
		}
	}
}
