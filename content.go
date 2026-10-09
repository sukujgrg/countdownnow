package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const contentFadeSeconds = 0.6

// ContentSlide contains user-authored wording. Timing is in seconds. Scripture
// reveals reserve an additional short fade after the complete-verse hold.
type ContentSlide struct {
	Type            string   `json:"type"`
	Text            string   `json:"text"`
	Reference       string   `json:"reference,omitempty"`
	Translation     string   `json:"translation,omitempty"`
	Label           string   `json:"label,omitempty"`
	AnchorWords     []string `json:"anchor_words,omitempty"`
	IntroSeconds    float64  `json:"intro_seconds,omitempty"`
	RevealSeconds   float64  `json:"reveal_seconds,omitempty"`
	HoldSeconds     float64  `json:"hold_seconds,omitempty"`
	DurationSeconds float64  `json:"duration_seconds,omitempty"`
	RevealOrder     string   `json:"reveal_order,omitempty"`
	Seed            *int64   `json:"seed,omitempty"`
	RevealStyle     string   `json:"reveal_style,omitempty"`
	FadeSeconds     float64  `json:"fade_seconds,omitempty"`
}

type scheduledContent struct {
	Slide ContentSlide
	Start float64
	End   float64
}

func loadContentFile(path string) ([]ContentSlide, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("content file must be smaller than 1 MiB")
	}
	var playlist struct {
		Slides []json.RawMessage `json:"slides"`
	}
	if err := decodeContentJSON(data, &playlist); err != nil {
		return nil, fmt.Errorf("content JSON: %w", err)
	}
	if len(playlist.Slides) == 0 || len(playlist.Slides) > 100 {
		return nil, fmt.Errorf("content must contain between 1 and 100 slides")
	}
	slides := make([]ContentSlide, 0, len(playlist.Slides))
	totalWords := 0
	for i, raw := range playlist.Slides {
		slide := ContentSlide{IntroSeconds: 3, RevealSeconds: 17, HoldSeconds: 12, DurationSeconds: 10, RevealOrder: "random", RevealStyle: "gentle-lift", FadeSeconds: 1.2}
		if err := decodeContentJSON(raw, &slide); err != nil {
			return nil, fmt.Errorf("content slide %d: %w", i+1, err)
		}
		// Catch accidental timing fields for the wrong slide type instead of
		// silently accepting settings that would have no effect.
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, fmt.Errorf("content slide %d: %w", i+1, err)
		}
		for name, value := range fields {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("content slide %d: %s cannot be null", i+1, name)
			}
			if slide.Type == "statement" && name != "type" && name != "text" && name != "label" && name != "duration_seconds" {
				return nil, fmt.Errorf("content slide %d: %s is only supported on scripture-reveal slides", i+1, name)
			}
			if slide.Type == "scripture-reveal" && name == "duration_seconds" {
				return nil, fmt.Errorf("content slide %d: use intro_seconds, reveal_seconds and hold_seconds for scripture-reveal", i+1)
			}
			if name == "fade_seconds" && !finitePositive(slide.FadeSeconds) {
				return nil, fmt.Errorf("content slide %d: fade_seconds must be finite and greater than zero", i+1)
			}
		}
		if err := validateContentSlide(slide); err != nil {
			return nil, fmt.Errorf("content slide %d: %w", i+1, err)
		}
		totalWords += len(strings.Fields(slide.Text))
		if totalWords > 2000 {
			return nil, fmt.Errorf("content playlist must contain no more than 2000 words")
		}
		slides = append(slides, slide)
	}
	return slides, nil
}

func decodeContentJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected a single JSON value")
	}
	return nil
}

func validateContentSlide(slide ContentSlide) error {
	if slide.Type != "scripture-reveal" && slide.Type != "statement" {
		return fmt.Errorf("type must be scripture-reveal or statement")
	}
	if strings.TrimSpace(slide.Text) == "" {
		return fmt.Errorf("text must not be empty")
	}
	if len(strings.Fields(slide.Text)) > 400 {
		return fmt.Errorf("text is too long; split it into slides of no more than 400 words")
	}
	for _, text := range []string{slide.Text, slide.Reference, slide.Translation, slide.Label} {
		for _, r := range text {
			if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
				return fmt.Errorf("text contains unsupported control character %U", r)
			}
		}
	}
	if len([]rune(slide.Reference)) > 256 || len([]rune(slide.Translation)) > 64 || len([]rune(slide.Label)) > 100 {
		return fmt.Errorf("reference, translation or label is too long")
	}
	if slide.Type == "statement" {
		if !finitePositive(slide.DurationSeconds) {
			return fmt.Errorf("duration_seconds must be finite and greater than zero")
		}
		return nil
	}
	if strings.TrimSpace(slide.Reference) == "" {
		return fmt.Errorf("scripture-reveal requires a reference")
	}
	if slide.RevealOrder != "" && slide.RevealOrder != "normal" && slide.RevealOrder != "random" {
		return fmt.Errorf("reveal_order must be normal or random")
	}
	if slide.RevealStyle != "" && slide.RevealStyle != "gentle-lift" && slide.RevealStyle != "soft-dissolve" {
		return fmt.Errorf("reveal_style must be gentle-lift or soft-dissolve")
	}
	if math.IsNaN(slide.FadeSeconds) || math.IsInf(slide.FadeSeconds, 0) || slide.FadeSeconds < 0 {
		return fmt.Errorf("fade_seconds must be finite and greater than zero")
	}
	if math.IsNaN(slide.IntroSeconds) || math.IsInf(slide.IntroSeconds, 0) || slide.IntroSeconds < 0 {
		return fmt.Errorf("intro_seconds must be finite and zero or greater")
	}
	if !finitePositive(slide.RevealSeconds) || !finitePositive(slide.HoldSeconds) {
		return fmt.Errorf("reveal_seconds and hold_seconds must be finite and greater than zero")
	}
	_, err := contentAnchors(slide)
	return err
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func contentAnchors(slide ContentSlide) (map[int]bool, error) {
	words := strings.Fields(slide.Text)
	anchors := make(map[int]bool)
	if len(words) == 0 {
		return anchors, nil
	}
	if slide.AnchorWords == nil {
		anchors[0] = true
		if len(words) > 3 {
			anchors[1] = true
		}
		if len(words) >= 8 {
			anchors[len(words)/2] = true
			anchors[len(words)-1] = true
		}
		return anchors, nil
	}
	for _, anchor := range slide.AnchorWords {
		needle := normalizeAnchor(anchor)
		if len(strings.Fields(anchor)) != 1 || needle == "" {
			return nil, fmt.Errorf("anchor_words entries must each contain one word")
		}
		found := false
		for i, word := range words {
			if normalizeAnchor(word) == needle {
				anchors[i] = true
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("anchor word %q does not occur in the verse", anchor)
		}
	}
	return anchors, nil
}

func normalizeAnchor(word string) string {
	return strings.ToLower(strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }))
}

func contentEndTime(cfg Config) float64 {
	if strings.EqualFold(cfg.OutroMode, "none") {
		return cfg.Duration
	}
	return outroStartTime(cfg)
}

func scheduleContent(cfg Config) ([]scheduledContent, error) {
	if len(cfg.Content) == 0 {
		return nil, nil
	}
	available := contentEndTime(cfg)
	if !finitePositive(available) {
		return nil, fmt.Errorf("countdown must leave time for content before the outro")
	}
	var cursor float64
	scheduled := make([]scheduledContent, 0, len(cfg.Content))
	for i, slide := range cfg.Content {
		if err := validateContentSlide(slide); err != nil {
			return nil, fmt.Errorf("content slide %d: %w", i+1, err)
		}
		duration := slide.DurationSeconds
		if slide.Type == "scripture-reveal" {
			duration = slide.IntroSeconds + slide.RevealSeconds + slide.HoldSeconds + contentFadeSeconds
		}
		end := cursor + duration
		if !finitePositive(end) || end > available+1e-7 {
			return nil, fmt.Errorf("content slide %d ends at %ss, but only %ss is available before the outro; increase --duration or shorten the playlist", i+1, ff(end), ff(available))
		}
		scheduled = append(scheduled, scheduledContent{Slide: slide, Start: cursor, End: end})
		cursor = end
	}
	// Extra time is a quiet hold of the final slide, rather than a blank screen.
	scheduled[len(scheduled)-1].End = available
	return scheduled, nil
}

// revealTimes includes each word's fade within reveal_seconds, so the full
// verse is completely visible before the user-specified reading hold begins.
func revealTimes(item scheduledContent) ([]float64, float64, error) {
	anchors, err := contentAnchors(item.Slide)
	if err != nil {
		return nil, 0, err
	}
	words := strings.Fields(item.Slide.Text)
	times := make([]float64, len(words))
	order := contentRevealOrder(item.Slide, anchors)
	remaining := len(order)
	fade := item.Slide.FadeSeconds
	if fade == 0 {
		fade = 1.2
	}
	fade = math.Min(fade, item.Slide.RevealSeconds)
	for i := range words {
		if anchors[i] {
			times[i] = item.Start
		}
	}
	for ordinal, i := range order {
		portion := 1.0
		if remaining > 1 {
			portion = float64(ordinal) / float64(remaining-1)
		}
		times[i] = item.Start + item.Slide.IntroSeconds + portion*(item.Slide.RevealSeconds-fade)
	}
	return times, fade, nil
}

// Sort by a seeded hash rather than process-global randomness. The order is
// stable across platforms, Go versions, previews and concurrent frame renders.
func contentRevealOrder(slide ContentSlide, anchors map[int]bool) []int {
	var order []int
	for i := range strings.Fields(slide.Text) {
		if !anchors[i] {
			order = append(order, i)
		}
	}
	if slide.RevealOrder == "normal" {
		return order
	}
	seed := slide.Text + "\x00" + slide.Reference
	if slide.Seed != nil {
		seed = strconv.FormatInt(*slide.Seed, 10)
	}
	sort.Slice(order, func(i, j int) bool {
		a := sha256.Sum256([]byte(seed + ":" + strconv.Itoa(order[i])))
		b := sha256.Sum256([]byte(seed + ":" + strconv.Itoa(order[j])))
		if c := bytes.Compare(a[:], b[:]); c != 0 {
			return c < 0
		}
		return order[i] < order[j]
	})
	return order
}

type contentWord struct {
	Text  string
	Index int
	X     int
}

type contentLine struct {
	Words []contentWord
	Width int
}

func wrapContent(text string, face font.Face, width int) ([]contentLine, error) {
	if width <= 0 {
		return nil, fmt.Errorf("no space remains for content text")
	}
	space := font.MeasureString(face, " ").Ceil()
	var lines []contentLine
	index := 0
	for _, paragraph := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := contentLine{}
		for _, word := range strings.Fields(paragraph) {
			for _, r := range word {
				if _, ok := face.GlyphAdvance(r); !ok {
					return nil, fmt.Errorf("font does not contain %q (%U); select a font covering this text", r, r)
				}
			}
			bounds, advance := font.BoundString(face, word)
			wordWidth := maxInt(advance.Ceil(), bounds.Max.X.Ceil())
			if wordWidth > width {
				return nil, fmt.Errorf("word %q does not fit", word)
			}
			x := line.Width
			if len(line.Words) > 0 {
				x += space
			}
			if x+wordWidth > width {
				lines = append(lines, line)
				line = contentLine{}
				x = 0
			}
			line.Words = append(line.Words, contentWord{Text: word, Index: index, X: x})
			line.Width = x + wordWidth
			index++
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func logoAspectRatio(cfg Config) float64 {
	var reader io.Reader
	if cfg.LogoFile == embeddedLogo {
		reader = bytes.NewReader(embeddedLogoPNG)
	} else if f, err := os.Open(cfg.LogoFile); err == nil {
		reader = f
		defer f.Close()
	}
	if reader != nil {
		if info, _, err := image.DecodeConfig(reader); err == nil && info.Width > 0 && info.Height > 0 {
			return float64(info.Width) / float64(info.Height)
		}
	}
	return 1
}

func contentArea(cfg Config) image.Rectangle {
	short := minInt(cfg.Width, cfg.Height)
	margin := maxInt(cfg.SafeArea, roundInt(float64(short)*0.075))
	top := roundInt(float64(cfg.Height) * 0.19)
	left := roundInt(float64(cfg.Width) * 0.43)
	if float64(cfg.Width)/float64(cfg.Height) < 1.45 {
		top = roundInt(float64(cfg.Height) * 0.34)
		left = margin
	}
	bottom := roundInt(float64(cfg.Height) * 0.84)
	if progressStyle(cfg.ProgressStyle) != "none" {
		bottom = minInt(bottom, cfg.BarY-margin)
	}
	box := image.Rectangle{Min: image.Pt(maxInt(left, margin), maxInt(top, margin)), Max: image.Pt(cfg.Width-margin, minInt(bottom, cfg.Height-margin))}
	if box.Empty() {
		return image.Rectangle{}
	}
	if !logoEnabled(cfg) {
		return box
	}
	logoHeight := roundInt(float64(cfg.LogoWidth) / logoAspectRatio(cfg))
	x, y := cfg.Width-cfg.LogoWidth-cfg.LogoMarginX, cfg.LogoMarginY
	switch strings.ToLower(cfg.LogoPosition) {
	case "top-left":
		x = cfg.LogoMarginX
	case "bottom-left":
		x, y = cfg.LogoMarginX, cfg.Height-logoHeight-cfg.LogoMarginY
	case "bottom-right":
		y = cfg.Height - logoHeight - cfg.LogoMarginY
	case "center":
		x, y = (cfg.Width-cfg.LogoWidth)/2, (cfg.Height-logoHeight)/2
	}
	obstacle := image.Rect(x, y, x+cfg.LogoWidth, y+logoHeight).Inset(-maxInt(4, margin/3))
	if !box.Overlaps(obstacle) {
		return box
	}
	// Respect the selected logo position, including a centered logo, by using
	// the largest remaining text rectangle rather than drawing underneath it.
	candidates := []image.Rectangle{
		{Min: box.Min, Max: image.Pt(box.Max.X, minInt(box.Max.Y, obstacle.Min.Y))},
		{Min: image.Pt(box.Min.X, maxInt(box.Min.Y, obstacle.Max.Y)), Max: box.Max},
		{Min: box.Min, Max: image.Pt(minInt(box.Max.X, obstacle.Min.X), box.Max.Y)},
		{Min: image.Pt(maxInt(box.Min.X, obstacle.Max.X), box.Min.Y), Max: box.Max},
	}
	best := image.Rectangle{}
	for _, candidate := range candidates {
		if !candidate.Empty() && candidate.Dx()*candidate.Dy() > best.Dx()*best.Dy() {
			best = candidate
		}
	}
	return best
}

type contentLayout struct {
	Lines         []contentLine
	FontSize      int
	LineHeight    int
	Baseline      int
	X             int
	LabelLines    []contentLine
	MetaLines     []contentLine
	MetaSize      int
	MetaHeight    int
	LabelBaseline int
	MetaBaseline  int
}

func fitContent(slide ContentSlide, bodyFont, metaFont *opentype.Font, box image.Rectangle, shortSide int) (contentLayout, error) {
	if box.Empty() {
		return contentLayout{}, fmt.Errorf("safe area and logo leave no space for content")
	}
	metaSize := maxInt(12, roundInt(float64(shortSide)*0.027))
	metaFace, err := opentype.NewFace(metaFont, &opentype.FaceOptions{Size: float64(metaSize), DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return contentLayout{}, err
	}
	defer metaFace.Close()
	metaHeight := roundInt(float64(metaFace.Metrics().Height.Ceil()) * 1.25)
	label := slide.Label
	if label == "" {
		label = "SCRIPTURE"
		if slide.Type == "statement" {
			label = "WELCOME"
		}
	}
	labelLines, err := wrapContent(label, metaFace, box.Dx())
	if err != nil {
		return contentLayout{}, fmt.Errorf("label: %w", err)
	}
	var metaLines []contentLine
	if slide.Reference != "" {
		metadata := slide.Reference
		if slide.Translation != "" {
			metadata += " | " + slide.Translation
		}
		metaLines, err = wrapContent(metadata, metaFace, box.Dx())
		if err != nil {
			return contentLayout{}, fmt.Errorf("reference: %w", err)
		}
	}
	minSize := maxInt(12, roundInt(float64(shortSide)*0.026))
	maxSize := maxInt(minSize, roundInt(float64(shortSide)*0.058))
	var lastErr error
	for size := maxSize; size >= minSize; size-- {
		face, err := opentype.NewFace(bodyFont, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingNone})
		if err != nil {
			return contentLayout{}, err
		}
		lines, wrapErr := wrapContent(slide.Text, face, box.Dx())
		metrics := face.Metrics()
		face.Close()
		if wrapErr != nil {
			lastErr = wrapErr
			continue
		}
		lineHeight := roundInt(float64(metrics.Height.Ceil()) * 1.3)
		gap := maxInt(8, size/2)
		height := len(labelLines)*metaHeight + gap + len(lines)*lineHeight
		if len(metaLines) > 0 {
			height += gap + len(metaLines)*metaHeight
		}
		if height > box.Dy() {
			lastErr = fmt.Errorf("passage needs more reading space")
			continue
		}
		top := box.Min.Y + (box.Dy()-height)/2
		baseline := top + len(labelLines)*metaHeight + gap + metrics.Ascent.Ceil()
		return contentLayout{
			Lines: lines, FontSize: size, LineHeight: lineHeight, Baseline: baseline, X: box.Min.X,
			LabelLines: labelLines, MetaLines: metaLines, MetaSize: metaSize, MetaHeight: metaHeight,
			LabelBaseline: top + metaFace.Metrics().Ascent.Ceil(),
			MetaBaseline:  baseline - metrics.Ascent.Ceil() + len(lines)*lineHeight + gap + metaFace.Metrics().Ascent.Ceil(),
		}, nil
	}
	return contentLayout{}, fmt.Errorf("text cannot fit at a readable size; split the passage into shorter slides or reduce logo/safe-area sizing: %w", lastErr)
}

// Escape both the drawtext option parser and the outer filtergraph parser.
// Literal user text is read from files with expansion=none, never interpolated.
func escapeFilterOption(value string) string {
	inner := strings.NewReplacer("\\", "\\\\", ":", "\\:", "'", "\\'").Replace(filepath.ToSlash(value))
	return strings.NewReplacer("\\", "\\\\", "'", "\\'", ",", "\\,", ";", "\\;", "[", "\\[", "]", "\\]").Replace(inner)
}

func contentTextFilter(fontPath, textPath string, size, x, baseline int, color string, start, end, appear, fade float64) string {
	alpha := fmt.Sprintf("min(max((%s-t)/%s,0),1)", ff(end), ff(math.Min(contentFadeSeconds, (end-start)/4)))
	if fade > 0 {
		alpha += fmt.Sprintf("*min(max((t-%s)/%s,0),1)", ff(appear), ff(fade))
	}
	return fmt.Sprintf("drawtext=fontfile=%s:textfile=%s:expansion=none:fontsize=%d:ft_load_flags=no_hinting:x=%d:y='%d-ascent':fontcolor=%s:borderw=%d:bordercolor=black@0.35:shadowx=1:shadowy=1:shadowcolor=black@0.3:alpha='%s':enable='gte(t,%s)*lt(t,%s)'",
		escapeFilterOption(fontPath), escapeFilterOption(textPath), size, x, baseline, color, maxInt(1, size/40), alpha, ff(start), ff(end))
}

func prepareContent(cfg Config) (Config, func(), error) {
	if len(cfg.Content) == 0 {
		return cfg, func() {}, nil
	}
	items, err := scheduleContent(cfg)
	if err != nil {
		return cfg, func() {}, err
	}
	dir, err := os.MkdirTemp("", "countdown-content-*")
	if err != nil {
		return cfg, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	fontCache := make(map[string]*opentype.Font)
	fontPaths := make(map[string]string)
	loadFont := func(path string) (*opentype.Font, string, error) {
		if cached := fontCache[path]; cached != nil {
			return cached, fontPaths[path], nil
		}
		data, err := readFontData(path)
		if err != nil {
			return nil, "", err
		}
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, "", fmt.Errorf("content requires a TTF or OTF font: %w", err)
		}
		staged := filepath.Join(dir, fmt.Sprintf("font-%d.ttf", len(fontCache)))
		if err := os.WriteFile(staged, data, 0600); err != nil {
			return nil, "", err
		}
		fontCache[path], fontPaths[path] = parsed, staged
		return parsed, staged, nil
	}
	bodyFont, bodyPath, err := loadFont(cfg.FontContent)
	if err != nil {
		cleanup()
		return cfg, func() {}, fmt.Errorf("content font: %w", err)
	}
	metaFont, metaPath, err := loadFont(cfg.FontSubtitle)
	if err != nil {
		cleanup()
		return cfg, func() {}, fmt.Errorf("reference font: %w", err)
	}
	textFiles := make(map[string]string)
	writeText := func(text string) (string, error) {
		if path, ok := textFiles[text]; ok {
			return path, nil
		}
		path := filepath.Join(dir, fmt.Sprintf("text-%d.txt", len(textFiles)))
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			return "", err
		}
		textFiles[text] = path
		return path, nil
	}
	theme := themeByName(cfg.Theme)
	box := contentArea(cfg)
	var filters []string
	for i, item := range items {
		layout, err := fitContent(item.Slide, bodyFont, metaFont, box, minInt(cfg.Width, cfg.Height))
		if err != nil {
			cleanup()
			return cfg, func() {}, fmt.Errorf("content slide %d: %w", i+1, err)
		}
		addText := func(text, fontPath, color string, size, x, y int, appear, fade float64) error {
			path, err := writeText(text)
			if err != nil {
				return err
			}
			filters = append(filters, contentTextFilter(fontPath, path, size, x, y, color, item.Start, item.End, appear, fade))
			return nil
		}
		addLines := func(lines []contentLine, baseline int, color string) error {
			for j, line := range lines {
				var text []string
				for _, word := range line.Words {
					text = append(text, word.Text)
				}
				if len(text) > 0 {
					if err := addText(strings.Join(text, " "), metaPath, color, layout.MetaSize, layout.X, baseline+j*layout.MetaHeight, item.Start, 0); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := addLines(layout.LabelLines, layout.LabelBaseline, theme.TimerColor); err != nil {
			cleanup()
			return cfg, func() {}, err
		}
		if err := addLines(layout.MetaLines, layout.MetaBaseline, theme.SubtitleColor); err != nil {
			cleanup()
			return cfg, func() {}, err
		}
		times, fade, err := revealTimes(item)
		if err != nil {
			cleanup()
			return cfg, func() {}, err
		}
		anchors, _ := contentAnchors(item.Slide)
		for j, line := range layout.Lines {
			for _, word := range line.Words {
				appear, wordFade := times[word.Index], fade
				if anchors[word.Index] {
					wordFade = 0
				}
				if item.Slide.Type == "statement" {
					appear, wordFade = item.Start, math.Min(contentFadeSeconds, item.Slide.DurationSeconds/4)
				}
				if err := addText(word.Text, bodyPath, theme.TitleColor, layout.FontSize, layout.X+word.X, layout.Baseline+j*layout.LineHeight, appear, wordFade); err != nil {
					cleanup()
					return cfg, func() {}, err
				}
			}
		}
	}
	cfg.ContentFilters = filters
	// A script keeps long playlists below Windows/macOS command-length limits.
	_, indexes := buildInputs(cfg, outputDuration(cfg))
	graph := buildFilterGraph(cfg, indexes.Logo)
	if cfg.Preview == "" && (indexes.Audio >= 0 || indexes.EndSound >= 0) {
		graph += ";" + buildAudioGraph(cfg, indexes.Audio, indexes.EndSound)
	}
	cfg.FilterGraphFile = filepath.Join(dir, "filters.txt")
	if err := os.WriteFile(cfg.FilterGraphFile, []byte(graph), 0600); err != nil {
		cleanup()
		return cfg, func() {}, err
	}
	if !cfg.DryRun {
		// Older FFmpeg uses a dedicated script option. Newer releases load an
		// option's value from a file with the -/ prefix instead.
		help, _ := ffmpegOutput("-hide_banner", "-h", "long")
		cfg.FilterGraphOption = filterGraphFileOption(help)
	}
	return cfg, cleanup, nil
}

func filterGraphFileOption(help string) string {
	if ffmpegFeatureExists(help, "-filter_complex_script") || help == "" {
		return "-filter_complex_script"
	}
	return "-/filter_complex"
}

func appendFilterGraphArgs(args []string, cfg Config, graph string) []string {
	if cfg.FilterGraphFile != "" && !cfg.DryRun {
		option := cfg.FilterGraphOption
		if option == "" {
			option = "-filter_complex_script"
		}
		return append(args, option, cfg.FilterGraphFile)
	}
	return append(args, "-filter_complex", graph)
}

func contentTimerFilter(cfg Config, theme themeConfig, text string, end float64, outroFade string) string {
	short := minInt(cfg.Width, cfg.Height)
	margin := maxInt(cfg.SafeArea, roundInt(float64(short)*0.075))
	x := fmt.Sprintf("%d+(%d-tw)/2", margin, roundInt(float64(cfg.Width)*0.32)-margin)
	baseline := roundInt(float64(cfg.Height) * 0.58)
	labelBaseline := roundInt(float64(cfg.Height) * 0.39)
	size := minInt(cfg.TimerFontSize, roundInt(float64(cfg.Width)*0.105))
	if float64(cfg.Width)/float64(cfg.Height) < 1.45 {
		x = "(w-tw)/2"
		baseline = roundInt(float64(cfg.Height) * 0.24)
		labelBaseline = roundInt(float64(cfg.Height) * 0.13)
		size = minInt(cfg.TimerFontSize, roundInt(float64(short)*0.12))
	}
	alpha := "1"
	if cfg.OutroOverDuration {
		alpha = fmt.Sprintf("if(lt(t,%s),1,%s)", ff(cfg.Duration), outroFade)
	}
	label := fmt.Sprintf("drawtext=fontfile=%s:text='SERVICE STARTS IN':expansion=none:fontsize=%d:x='%s':y='%d-ascent':fontcolor=%s:alpha='%s':enable='lt(t,%s)'",
		escapeFilterOption(cfg.FontSubtitle), maxInt(10, short/45), x, labelBaseline, theme.SubtitleColor, alpha, ff(end))
	timer := fmt.Sprintf("drawtext=fontfile=%s:text='%s':fontsize=%d:x='%s':y='%d-ascent':fontcolor=%s:borderw=%d:bordercolor=%s:shadowx=%d:shadowy=%d:alpha='%s':enable='lt(t,%s)'",
		escapeFilterOption(cfg.FontTimer), text, maxInt(12, size), x, baseline, theme.TimerColor, maxInt(1, size/30), theme.BorderColor, maxInt(1, size/40), maxInt(1, size/40), alpha, ff(end))
	return label + "," + timer
}
