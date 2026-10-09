package main

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

func testScripture() ContentSlide {
	return ContentSlide{
		Type: "scripture-reveal", Text: "Every word stays in its final place until the passage is complete.",
		Reference: "Example 1:1", AnchorWords: []string{"Every", "complete"},
		IntroSeconds: 3, RevealSeconds: 17, HoldSeconds: 12,
		RevealOrder: "normal",
	}
}

func contentConfig(t *testing.T) Config {
	t.Helper()
	cfg := defaultConfig()
	cfg.LogoPosition = "none"
	cfg.Content = []ContentSlide{testScripture()}
	if err := applyPreset(&cfg); err != nil {
		t.Fatal(err)
	}
	applyResponsiveLayout(&cfg)
	applyDefaults(&cfg)
	return cfg
}

func TestContentFileDefaultsAndExplicitZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "content.json")
	data := `{"slides":[{"type":"scripture-reveal","text":"Words are supplied by the church.","reference":"Example 1:1","intro_seconds":0,"anchor_words":[]},{"type":"statement","text":"Welcome."}]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	slides, err := loadContentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if slides[0].IntroSeconds != 0 || slides[0].RevealSeconds != 17 || slides[0].HoldSeconds != 12 || slides[0].AnchorWords == nil || slides[1].DurationSeconds != 10 {
		t.Fatalf("defaults or explicit zero/empty settings were lost: %+v", slides)
	}
}

func TestContentFileRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"unknown root field":     `{"slides":[],"duration":300}`,
		"empty playlist":         `{"slides":[]}`,
		"trailing JSON":          `{"slides":[]} {}`,
		"unknown slide field":    `{"slides":[{"type":"statement","text":"Welcome","duraton_seconds":10}]}`,
		"null timing":            `{"slides":[{"type":"statement","text":"Welcome","duration_seconds":null}]}`,
		"negative timing":        `{"slides":[{"type":"statement","text":"Welcome","duration_seconds":-1}]}`,
		"blank text":             `{"slides":[{"type":"statement","text":"  "}]}`,
		"unknown type":           `{"slides":[{"type":"quiz","text":"Welcome"}]}`,
		"missing reference":      `{"slides":[{"type":"scripture-reveal","text":"A verse"}]}`,
		"zero reveal":            `{"slides":[{"type":"scripture-reveal","text":"A verse","reference":"Example","reveal_seconds":0}]}`,
		"missing anchor":         `{"slides":[{"type":"scripture-reveal","text":"A verse","reference":"Example","anchor_words":["absent"]}]}`,
		"phrase anchor":          `{"slides":[{"type":"scripture-reveal","text":"A verse","reference":"Example","anchor_words":["A verse"]}]}`,
		"wrong scripture timing": `{"slides":[{"type":"scripture-reveal","text":"A verse","reference":"Example","duration_seconds":10}]}`,
		"wrong statement timing": `{"slides":[{"type":"statement","text":"Welcome","hold_seconds":10}]}`,
		"control character":      `{"slides":[{"type":"statement","text":"Welcome\u0000"}]}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "content.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadContentFile(path); err == nil {
				t.Fatal("invalid content was accepted")
			}
		})
	}
}

func TestContentAnchorsMatchCaseAndPunctuation(t *testing.T) {
	slide := testScripture()
	slide.Text = "Know, know; KNOW. stay."
	slide.AnchorWords = []string{"know"}
	anchors, err := contentAnchors(slide)
	if err != nil || !reflect.DeepEqual(anchors, map[int]bool{0: true, 1: true, 2: true}) {
		t.Fatalf("anchors = %v, error = %v", anchors, err)
	}
	slide.AnchorWords = []string{}
	anchors, err = contentAnchors(slide)
	if err != nil || len(anchors) != 0 {
		t.Fatalf("explicit empty anchors should reveal every word: %v, %v", anchors, err)
	}
}

func TestContentScheduleReservesOutroAndRejectsOverflow(t *testing.T) {
	cfg := contentConfig(t)
	cfg.Duration = 40
	cfg.OutroSeconds = 3
	cfg.Content = append(cfg.Content, ContentSlide{Type: "statement", Text: "Welcome", DurationSeconds: 4})
	items, err := scheduleContent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(items[0].End-32.6) > 1e-8 || items[1].Start != items[0].End || items[1].End != 37 {
		t.Fatalf("unexpected schedule: %+v", items)
	}
	cfg.Duration = 39
	if _, err := scheduleContent(cfg); err == nil {
		t.Fatal("playlist that cuts off before the outro was accepted")
	}
	cfg.OutroOverDuration = true
	items, err = scheduleContent(cfg)
	if err != nil || items[len(items)-1].End != 39 {
		t.Fatalf("extended outro should leave the full countdown available: %v, %v", items, err)
	}
	cfg.OutroOverDuration = false
	cfg.OutroMode = "none"
	items, err = scheduleContent(cfg)
	if err != nil || items[len(items)-1].End != 39 {
		t.Fatalf("disabled outro should leave the full countdown available: %v, %v", items, err)
	}
	for _, invalid := range []float64{math.Inf(1), math.NaN()} {
		cfg.Duration = invalid
		if _, err := scheduleContent(cfg); err == nil {
			t.Fatal("nonfinite duration was accepted")
		}
	}
}

func TestRevealCompletesBeforeFullReadingHold(t *testing.T) {
	slide := testScripture()
	for _, reveal := range []float64{0.1, 1, 17} {
		slide.RevealSeconds = reveal
		item := scheduledContent{Slide: slide, Start: 10, End: 10 + slide.IntroSeconds + reveal + slide.HoldSeconds + contentFadeSeconds}
		times, fade, err := revealTimes(item)
		if err != nil {
			t.Fatal(err)
		}
		anchors, _ := contentAnchors(slide)
		last := item.Start
		for i, appear := range times {
			if anchors[i] {
				if appear != item.Start {
					t.Fatalf("anchor %d was not visible at the start", i)
				}
				continue
			}
			if appear < last || appear+fade > 10+slide.IntroSeconds+reveal+1e-8 {
				t.Fatalf("word %d has invalid reveal time %v", i, appear)
			}
			last = appear
		}
		if math.Abs(last+fade-(10+slide.IntroSeconds+reveal)) > 1e-8 || math.Abs((item.End-contentFadeSeconds)-(last+fade)-slide.HoldSeconds) > 1e-8 {
			t.Fatal("last word's fade reduced the full-verse reading hold")
		}
	}
}

func TestContentLayoutFitsPresetsAndPreservesWordOrder(t *testing.T) {
	parsed, err := opentype.Parse(embeddedPassionOneBold)
	if err != nil {
		t.Fatal(err)
	}
	for _, preset := range []string{"landscape", "vertical", "square", "ultrawide", "4k"} {
		t.Run(preset, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Preset = preset
			cfg.LogoPosition = "none"
			cfg.SafeArea = 40
			if err := applyPreset(&cfg); err != nil {
				t.Fatal(err)
			}
			applyResponsiveLayout(&cfg)
			box := contentArea(cfg)
			slide := testScripture()
			slide.Text += "\nEvery supplied word is kept, including punctuation and explicit line breaks."
			layout, err := fitContent(slide, parsed, parsed, box, minInt(cfg.Width, cfg.Height))
			if err != nil {
				t.Fatal(err)
			}
			face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: float64(layout.FontSize), DPI: 72, Hinting: font.HintingNone})
			if err != nil {
				t.Fatal(err)
			}
			defer face.Close()
			var actual []string
			for lineIndex, line := range layout.Lines {
				baseline := layout.Baseline + lineIndex*layout.LineHeight
				if baseline-face.Metrics().Ascent.Ceil() < box.Min.Y || baseline+face.Metrics().Descent.Ceil() > box.Max.Y {
					t.Fatalf("line extends outside content area: %+v", layout)
				}
				for _, word := range line.Words {
					if word.Index != len(actual) || layout.X+word.X < box.Min.X || layout.X+word.X+font.MeasureString(face, word.Text).Ceil() > box.Max.X {
						t.Fatalf("word reordered or outside content area: %+v", word)
					}
					actual = append(actual, word.Text)
				}
			}
			if !reflect.DeepEqual(actual, strings.Fields(slide.Text)) {
				t.Fatal("wrapping changed supplied wording")
			}
		})
	}
}

func TestContentLayoutAvoidsCenteredLogoAndRejectsNoSpace(t *testing.T) {
	cfg := contentConfig(t)
	cfg.LogoFile = embeddedLogo
	cfg.LogoPosition = "center"
	cfg.LogoWidth = 160
	box := contentArea(cfg)
	info, _, err := image.DecodeConfig(bytes.NewReader(embeddedLogoPNG))
	if err != nil {
		t.Fatal(err)
	}
	height := roundInt(float64(cfg.LogoWidth) * float64(info.Height) / float64(info.Width))
	logo := image.Rect((cfg.Width-cfg.LogoWidth)/2, (cfg.Height-height)/2, (cfg.Width+cfg.LogoWidth)/2, (cfg.Height+height)/2)
	if box.Empty() || box.Overlaps(logo) {
		t.Fatalf("content rectangle overlaps centered logo: %v, %v", box, logo)
	}
	cfg.SafeArea = cfg.Width
	if !contentArea(cfg).Empty() {
		t.Fatal("oversized safe area should leave no valid content rectangle")
	}
	parsed, _ := opentype.Parse(embeddedPassionOneBold)
	if _, err := fitContent(testScripture(), parsed, parsed, contentArea(cfg), 720); err == nil {
		t.Fatal("no-space layout was accepted")
	}
}

func TestPreparedContentUsesLiteralTextAndCleansAssets(t *testing.T) {
	cfg := contentConfig(t)
	cfg.Content[0].Text = "Every 100% word: [stays] 'literal' \\ complete."
	prepared, cleanup, err := prepareContent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	graph, err := os.ReadFile(prepared.FilterGraphFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(graph), "expansion=none") || strings.Contains(string(graph), "100%") || strings.Contains(string(graph), "[stays]") {
		t.Fatal("user text was inserted into the filtergraph instead of literal text files")
	}
	entries, err := os.ReadDir(filepath.Dir(prepared.FilterGraphFile))
	if err != nil {
		t.Fatal(err)
	}
	foundPercent := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "text-") {
			text, err := os.ReadFile(filepath.Join(filepath.Dir(prepared.FilterGraphFile), entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			foundPercent = foundPercent || string(text) == "100%"
		}
	}
	if !foundPercent {
		t.Fatal("literal punctuation was not preserved")
	}
	prepared.DryRun = true
	args := buildFFmpegArgs(prepared, softwareEncoder(prepared).Args, "out.mp4")
	if !strings.Contains(strings.Join(args, " "), "-filter_complex ") {
		t.Fatal("dry-run should print the inline filter graph")
	}
	path := prepared.FilterGraphFile
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("content assets were not cleaned up")
	}
}

func TestContentFontOverridesAndTimerCompatibility(t *testing.T) {
	cfg := defaultConfig()
	cfg.Font = "/tmp/shared.ttf"
	cfg.UserFont = true
	applyFontFlags(&cfg)
	if cfg.FontContent != cfg.Font {
		t.Fatal("--font did not apply to content")
	}
	cfg.FontContent = "/tmp/body.ttf"
	cfg.UserFontContent = true
	applyFontFlags(&cfg)
	if cfg.FontContent != "/tmp/body.ttf" {
		t.Fatal("--font overwrote an explicit --font-content")
	}
	cfg = contentConfig(t)
	cfg.TimerColor = "#00ff88"
	graph := buildFilterGraph(cfg, -1)
	if !strings.Contains(graph, "SERVICE STARTS IN") || !strings.Contains(graph, "fontcolor=#00ff88") || strings.Contains(graph, "alpha='if(lt(t,297)") {
		t.Fatal("content timer did not preserve custom color and visibility through the countdown")
	}
}

func TestFilterGraphFileCompatibility(t *testing.T) {
	for _, tc := range []struct{ help, want string }{
		{"-filter_complex_script <filename> read graph from file", "-filter_complex_script"},
		{"-filter_complex_script\t<filename> read graph from file", "-filter_complex_script"},
		{"-filter_complex <graph_description> create a complex filtergraph", "-/filter_complex"},
		{"", "-filter_complex_script"},
	} {
		if got := filterGraphFileOption(tc.help); got != tc.want {
			t.Errorf("option for %q = %q; want %q", tc.help, got, tc.want)
		}
	}
}

func TestContentPreviewOmitsAudioGraph(t *testing.T) {
	cfg := contentConfig(t)
	cfg.AudioFile = "/tmp/background-audio.wav"
	cfg.EndSoundFile = "/tmp/end-sound.wav"
	cfg.Preview = filepath.Join(t.TempDir(), "preview.png")
	prepared, cleanup, err := prepareContent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	graph, err := os.ReadFile(prepared.FilterGraphFile)
	if err != nil || bytes.Contains(graph, []byte("[aout]")) {
		t.Fatalf("preview contains an unused audio graph: %v", err)
	}
	cfg.Preview = ""
	prepared, cleanupVideo, err := prepareContent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupVideo()
	graph, err = os.ReadFile(prepared.FilterGraphFile)
	if err != nil || !bytes.Contains(graph, []byte("[aout]")) || !bytes.Contains(graph, []byte("amix=inputs=2")) {
		t.Fatalf("video did not retain background audio and end sound: %v", err)
	}
}

func TestFFmpegScriptureRevealFrames(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is not installed")
	}
	filters, err := ffmpegOutput("-hide_banner", "-filters")
	if err != nil || !ffmpegFeatureExists(filters, "drawtext") {
		t.Skip("FFmpeg build has no drawtext")
	}
	dir := t.TempDir()
	tempName := "content path 'quote' [bracket],;"
	if runtime.GOOS != "windows" {
		tempName += ":"
	}
	contentTemp := filepath.Join(dir, tempName)
	if err := os.Mkdir(contentTemp, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, contentTemp)
	}
	background := filepath.Join(dir, "black.png")
	f, err := os.Create(background)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 640, 360))); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	cfg := contentConfig(t)
	cfg.Content[0].Text = "Every 100% word: [stays] 'literal' \\ complete."
	cfg.Width, cfg.Height = 640, 360
	cfg.BarY = 330
	cfg.Background, cfg.BackgroundIsImage = background, true
	cfg.ProgressStyle, cfg.OutroMode = "none", "none"
	cfg.Duration = 4
	cfg.Content[0].IntroSeconds, cfg.Content[0].RevealSeconds, cfg.Content[0].HoldSeconds = 0.3, 1.5, 1
	cfg.Preview = filepath.Join(dir, "start.png")
	resolved, assetsCleanup, err := prepareEmbeddedAssets(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer assetsCleanup()
	prepared, cleanup, err := prepareContent(resolved)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var crops [][]byte
	box := contentArea(cfg)
	for i, second := range []float64{0, 2, 2.5} {
		prepared.PreviewTime = second
		prepared.Preview = filepath.Join(dir, []string{"start.png", "full.png", "hold.png"}[i])
		cmd := exec.Command(ffmpeg, buildPreviewArgs(prepared)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("render frame at %vs: %v\n%s", second, err, output)
		}
		f, err := os.Open(prepared.Preview)
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
		t.Fatal("revealing words did not change the content frame")
	}
	if !bytes.Equal(crops[1], crops[2]) {
		t.Fatal("complete verse moved or changed during the reading hold")
	}
}
