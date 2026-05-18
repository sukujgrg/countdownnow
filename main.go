package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	defaultFontPath = "./fonts/Passion_One/PassionOne-Bold.ttf"
	embeddedLogo    = "embedded:logo.png"
	embeddedFont    = "embedded:PassionOne-Bold.ttf"
)

var (
	version    = "dev"
	projectURL = "https://github.com/sukujgrg/countdownnow"
)

//go:embed logo.png
var embeddedLogoPNG []byte

//go:embed fonts/Passion_One/PassionOne-Bold.ttf
var embeddedPassionOneBold []byte

//go:embed fonts/Passion_One/OFL.txt
var embeddedPassionOneLicense []byte

type Config struct {
	Duration          float64
	Output            string
	Preset            string
	Background        string
	BackgroundIsImage bool
	BackgroundFit     string
	Width             int
	Height            int
	FPS               int
	Title             string
	Subtitle          string
	LogoFile          string
	LogoPosition      string
	LogoWidth         int
	LogoMarginX       int
	LogoMarginY       int
	Font              string
	FontTimer         string
	FontTitle         string
	FontSubtitle      string
	OutroSeconds      float64
	Theme             string
	OutroMode         string
	ProgressStyle     string
	SafeArea          int
	Preview           string
	PreviewTime       float64
	AudioFile         string
	AudioVolume       float64
	FadeAudio         bool
	EndSoundFile      string
	EncoderMode       string
	VideoBitrate      string
	SoftwareCRF       int
	SoftwareSpeed     string
	DryRun            bool
	Check             bool
	ShowVersion       bool
	UserWidth         bool
	UserHeight        bool
	UserLogo          bool
	UserFont          bool
	UserFontTimer     bool
	UserFontTitle     bool
	UserFontSubtitle  bool

	BarX              int
	BarY              int
	BarWidth          int
	BarHeight         int
	TimerFontSize     int
	OutroTitleSize    int
	OutroSubtitleSize int
	TimerYOffset      int
}

type Encoder struct {
	Name string
	Args []string
}

func main() {
	cfg := defaultConfig()
	parseFlags(&cfg)
	if cfg.ShowVersion {
		printVersion()
		return
	}
	normalizeExplicitNone(&cfg)
	applyFontFlags(&cfg)
	if err := applyBackgroundDimensions(&cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if err := applyPreset(&cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	applyResponsiveLayout(&cfg)
	applyDefaults(&cfg)

	if err := validate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	if cfg.Check {
		if err := checkFFmpeg(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if err := render(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func defaultConfig() Config {
	return Config{
		Duration:      300,
		Preset:        "landscape",
		Title:         "Welcome",
		Subtitle:      "We are glad you are here.",
		LogoPosition:  "top-right",
		Font:          defaultFontPath,
		FontTimer:     defaultFontPath,
		FontTitle:     defaultFontPath,
		FontSubtitle:  defaultFontPath,
		OutroSeconds:  3,
		Theme:         "midnight",
		OutroMode:     "slide",
		ProgressStyle: "bottom-bar",
		BackgroundFit: "cover",
		AudioVolume:   0.35,
		EncoderMode:   "auto",
		VideoBitrate:  "2M",
		SoftwareCRF:   20,
		SoftwareSpeed: "veryfast",
		FPS:           30,
	}
}

func parseFlags(cfg *Config) {
	flag.Usage = printUsage
	flag.Float64Var(&cfg.Duration, "duration", cfg.Duration, "countdown duration in seconds")
	flag.StringVar(&cfg.Output, "output", cfg.Output, "output mp4 path")
	flag.StringVar(&cfg.Preset, "preset", cfg.Preset, "layout preset: landscape, vertical, square, ultrawide, 4k")
	flag.StringVar(&cfg.Background, "background", cfg.Background, "optional image or video background path")
	flag.StringVar(&cfg.BackgroundFit, "background-fit", cfg.BackgroundFit, "background fit mode: cover, contain, stretch")
	flag.IntVar(&cfg.Width, "width", cfg.Width, "override video width")
	flag.IntVar(&cfg.Height, "height", cfg.Height, "override video height")
	flag.IntVar(&cfg.FPS, "fps", cfg.FPS, "frames per second")
	flag.StringVar(&cfg.Title, "title", cfg.Title, "main title text; use none to disable")
	flag.StringVar(&cfg.Subtitle, "subtitle", cfg.Subtitle, "outro subtitle text; use none to disable")
	flag.StringVar(&cfg.LogoFile, "logo", cfg.LogoFile, "logo PNG path; use none to disable")
	flag.StringVar(&cfg.LogoPosition, "logo-position", cfg.LogoPosition, "logo position: top-right, top-left, bottom-right, bottom-left, center, none")
	flag.IntVar(&cfg.LogoWidth, "logo-width", cfg.LogoWidth, "logo width in pixels")
	flag.IntVar(&cfg.LogoMarginX, "logo-margin-x", cfg.LogoMarginX, "logo horizontal margin in pixels")
	flag.IntVar(&cfg.LogoMarginY, "logo-margin-y", cfg.LogoMarginY, "logo vertical margin in pixels")
	flag.StringVar(&cfg.Font, "font", cfg.Font, "font file for all text")
	flag.StringVar(&cfg.FontTimer, "font-timer", cfg.FontTimer, "font file for countdown timer")
	flag.StringVar(&cfg.FontTitle, "font-title", cfg.FontTitle, "font file for outro title")
	flag.StringVar(&cfg.FontSubtitle, "font-subtitle", cfg.FontSubtitle, "font file for outro subtitle")
	flag.Float64Var(&cfg.OutroSeconds, "outro-seconds", cfg.OutroSeconds, "final title reveal duration in seconds")
	flag.StringVar(&cfg.Theme, "theme", cfg.Theme, "visual theme: midnight, warm, sunrise, minimal")
	flag.StringVar(&cfg.OutroMode, "outro", cfg.OutroMode, "outro mode: slide, fade, typewriter, none")
	flag.StringVar(&cfg.ProgressStyle, "progress", cfg.ProgressStyle, "progress style: bottom-bar, none")
	flag.IntVar(&cfg.SafeArea, "safe-area", cfg.SafeArea, "extra edge padding in pixels for logo and progress")
	flag.StringVar(&cfg.Preview, "preview", cfg.Preview, "write a single preview frame image and exit")
	flag.Float64Var(&cfg.PreviewTime, "preview-time", cfg.PreviewTime, "preview timestamp in seconds")
	flag.StringVar(&cfg.AudioFile, "audio", cfg.AudioFile, "optional background audio file")
	flag.Float64Var(&cfg.AudioVolume, "audio-volume", cfg.AudioVolume, "background audio volume multiplier")
	flag.BoolVar(&cfg.FadeAudio, "fade-audio", cfg.FadeAudio, "fade background audio out during the outro")
	flag.StringVar(&cfg.EndSoundFile, "end-sound", cfg.EndSoundFile, "optional sound effect mixed into the final second")
	flag.StringVar(&cfg.EncoderMode, "encoder", cfg.EncoderMode, "encoder mode: auto, software, or a specific ffmpeg encoder")
	flag.StringVar(&cfg.VideoBitrate, "bitrate", cfg.VideoBitrate, "hardware encoder target bitrate")
	flag.IntVar(&cfg.SoftwareCRF, "crf", cfg.SoftwareCRF, "libx264 CRF")
	flag.StringVar(&cfg.SoftwareSpeed, "preset-x264", cfg.SoftwareSpeed, "libx264 preset")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "print ffmpeg command instead of running it")
	flag.BoolVar(&cfg.Check, "check", cfg.Check, "check FFmpeg capabilities and exit without rendering")
	flag.BoolVar(&cfg.ShowVersion, "version", cfg.ShowVersion, "print version information and exit")
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "width":
			cfg.UserWidth = true
		case "height":
			cfg.UserHeight = true
		case "logo":
			cfg.UserLogo = true
		case "font":
			cfg.UserFont = true
		case "font-timer":
			cfg.UserFontTimer = true
		case "font-title":
			cfg.UserFontTitle = true
		case "font-subtitle":
			cfg.UserFontSubtitle = true
		}
	})
}

func printUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), "CountdownNow generates customizable countdown videos with FFmpeg.\n\n")
	fmt.Fprintf(flag.CommandLine.Output(), "URL: %s\n", projectURL)
	fmt.Fprintf(flag.CommandLine.Output(), "Version: %s\n\n", version)
	fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  %s [options]\n\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(flag.CommandLine.Output(), "Examples:\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  %s --duration 300 --preset landscape\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(flag.CommandLine.Output(), "  %s --preset vertical --output vertical-countdown.mp4\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(flag.CommandLine.Output(), "  %s --check\n\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
	flag.PrintDefaults()
}

func printVersion() {
	fmt.Printf("CountdownNow %s\n", version)
	fmt.Printf("%s\n", projectURL)
}

func applyPreset(cfg *Config) error {
	switch strings.ToLower(cfg.Preset) {
	case "landscape", "horizontal", "":
		setIfZero(&cfg.Width, 1280)
		setIfZero(&cfg.Height, 720)
	case "vertical", "portrait":
		setIfZero(&cfg.Width, 1080)
		setIfZero(&cfg.Height, 1920)
	case "square":
		setIfZero(&cfg.Width, 1080)
		setIfZero(&cfg.Height, 1080)
	case "ultrawide", "wide":
		setIfZero(&cfg.Width, 2560)
		setIfZero(&cfg.Height, 1080)
	case "4k", "uhd":
		setIfZero(&cfg.Width, 3840)
		setIfZero(&cfg.Height, 2160)
		if cfg.VideoBitrate == "2M" {
			cfg.VideoBitrate = "16M"
		}
	default:
		return fmt.Errorf("unknown preset %q", cfg.Preset)
	}
	return nil
}

func applyResponsiveLayout(cfg *Config) {
	shortSide := minInt(cfg.Width, cfg.Height)
	landscape := cfg.Width >= cfg.Height
	aspect := float64(cfg.Width) / float64(cfg.Height)

	barWidthRatio := 0.78
	barYRatio := 0.92
	timerScale := 0.19
	titleScale := 0.065
	subtitleScale := 0.039
	logoScale := 0.105
	marginXRatio := 0.025
	marginYRatio := 0.055
	timerYOffsetRatio := 0.0

	switch {
	case aspect >= 2.0:
		barWidthRatio = 0.72
		timerScale = 0.26
		titleScale = 0.08
		subtitleScale = 0.048
		logoScale = 0.13
		timerYOffsetRatio = 0.0
	case !landscape:
		barWidthRatio = 0.78
		barYRatio = 0.917
		timerScale = 0.205
		titleScale = 0.07
		subtitleScale = 0.041
		logoScale = 0.148
		marginXRatio = 0.046
		marginYRatio = 0.036
		timerYOffsetRatio = 0.0
	case aspect < 1.2:
		barWidthRatio = 0.76
		barYRatio = 0.90
		timerScale = 0.22
		titleScale = 0.073
		subtitleScale = 0.044
		logoScale = 0.13
		timerYOffsetRatio = 0.0
	}

	setIfZero(&cfg.BarWidth, evenInt(clampInt(roundInt(float64(cfg.Width)*barWidthRatio), 1, cfg.Width)))
	setIfZero(&cfg.BarHeight, maxInt(2, roundInt(float64(shortSide)*0.014)))
	setIfZero(&cfg.BarX, (cfg.Width-cfg.BarWidth)/2)
	setIfZero(&cfg.BarY, clampInt(roundInt(float64(cfg.Height)*barYRatio), 0, maxInt(0, cfg.Height-cfg.BarHeight)))
	setIfZero(&cfg.LogoWidth, maxInt(24, roundInt(float64(shortSide)*logoScale)))
	setIfZero(&cfg.LogoMarginX, maxInt(8, roundInt(float64(cfg.Width)*marginXRatio)))
	setIfZero(&cfg.LogoMarginY, maxInt(8, roundInt(float64(cfg.Height)*marginYRatio)))
	setIfZero(&cfg.TimerFontSize, maxInt(24, roundInt(float64(shortSide)*timerScale)))
	setIfZero(&cfg.OutroTitleSize, maxInt(18, roundInt(float64(shortSide)*titleScale)))
	setIfZero(&cfg.OutroSubtitleSize, maxInt(14, roundInt(float64(shortSide)*subtitleScale)))
	if cfg.TimerYOffset == 0 {
		cfg.TimerYOffset = roundInt(float64(shortSide) * timerYOffsetRatio)
	}
	if cfg.VideoBitrate == "2M" {
		cfg.VideoBitrate = responsiveBitrate(cfg.Width, cfg.Height)
	}
	if cfg.SafeArea > 0 {
		cfg.LogoMarginX = maxInt(cfg.LogoMarginX, cfg.SafeArea)
		cfg.LogoMarginY = maxInt(cfg.LogoMarginY, cfg.SafeArea)
		cfg.BarY = minInt(cfg.BarY, maxInt(0, cfg.Height-cfg.SafeArea-cfg.BarHeight))
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Output == "" {
		cfg.Output = fmt.Sprintf("countdown_%ss.mp4", trimFloat(cfg.Duration))
	}
	if cfg.LogoFile == "" && strings.ToLower(cfg.LogoPosition) != "none" {
		for _, candidate := range []string{"logo.png", "default.png", "Round Icon Design.png"} {
			if fileExists(candidate) {
				cfg.LogoFile = candidate
				break
			}
		}
		if cfg.LogoFile == "" && len(embeddedLogoPNG) > 0 {
			cfg.LogoFile = embeddedLogo
		}
	}
}

func normalizeExplicitNone(cfg *Config) {
	if isNoneValue(cfg.Title) {
		cfg.Title = ""
	}
	if isNoneValue(cfg.Subtitle) {
		cfg.Subtitle = ""
	}
	if isNoneValue(cfg.LogoFile) || (cfg.UserLogo && strings.TrimSpace(cfg.LogoFile) == "") {
		cfg.LogoFile = ""
		cfg.LogoPosition = "none"
	}
}

func isNoneValue(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "none")
}

func applyFontFlags(cfg *Config) {
	if !cfg.UserFont {
		return
	}
	if !cfg.UserFontTimer {
		cfg.FontTimer = cfg.Font
	}
	if !cfg.UserFontTitle {
		cfg.FontTitle = cfg.Font
	}
	if !cfg.UserFontSubtitle {
		cfg.FontSubtitle = cfg.Font
	}
}

func applyBackgroundDimensions(cfg *Config) error {
	if cfg.Background == "" {
		return nil
	}
	if !fileExists(cfg.Background) {
		return fmt.Errorf("background not found: %s", cfg.Background)
	}
	width, height, err := probeVideoDimensions(cfg.Background)
	if err != nil {
		return fmt.Errorf("could not inspect background dimensions: %w", err)
	}
	if !cfg.UserWidth && !cfg.UserHeight {
		cfg.Width = width
		cfg.Height = height
	} else if cfg.UserWidth != cfg.UserHeight {
		return errors.New("when overriding background dimensions, provide both --width and --height")
	}
	cfg.BackgroundIsImage = isImagePath(cfg.Background)
	return nil
}

func probeVideoDimensions(path string) (int, int, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x",
		path,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "x")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected ffprobe output %q", strings.TrimSpace(string(out)))
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("invalid background dimensions %dx%d", width, height)
	}
	return width, height, nil
}

func isImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".apng", ".avif", ".bmp", ".gif", ".jpeg", ".jpg", ".png", ".tif", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func setIfZero(target *int, value int) {
	if *target == 0 {
		*target = value
	}
}

func validate(cfg Config) error {
	if cfg.Duration <= 0 {
		return errors.New("duration must be greater than zero")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return errors.New("width and height must be greater than zero")
	}
	if cfg.FPS <= 0 {
		return errors.New("fps must be greater than zero")
	}
	if cfg.AudioVolume < 0 {
		return errors.New("audio volume must be zero or greater")
	}
	if cfg.PreviewTime < 0 {
		return errors.New("preview time must be zero or greater")
	}
	if cfg.OutroSeconds <= 0 {
		return errors.New("outro seconds must be greater than zero")
	}
	if cfg.SoftwareCRF < 0 || cfg.SoftwareCRF > 51 {
		return errors.New("crf must be between 0 and 51")
	}
	if cfg.LogoWidth < 0 || cfg.LogoMarginX < 0 || cfg.LogoMarginY < 0 {
		return errors.New("logo width and margins must be zero or greater")
	}
	if cfg.SafeArea < 0 {
		return errors.New("safe area must be zero or greater")
	}
	if err := validateOutputPath("output", cfg.Output); err != nil {
		return err
	}
	if cfg.Preview != "" {
		if err := validateOutputPath("preview", cfg.Preview); err != nil {
			return err
		}
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errors.New("ffmpeg not found in PATH")
	}
	if cfg.UserFont && !validFontPath(cfg.Font) {
		return fmt.Errorf("font not found: %s", cfg.Font)
	}
	if !validFontPath(cfg.FontTimer) {
		return fmt.Errorf("timer font not found: %s", cfg.FontTimer)
	}
	if !validFontPath(cfg.FontTitle) {
		return fmt.Errorf("title font not found: %s", cfg.FontTitle)
	}
	if !validFontPath(cfg.FontSubtitle) {
		return fmt.Errorf("subtitle font not found: %s", cfg.FontSubtitle)
	}
	if strings.ToLower(cfg.LogoPosition) != "none" && cfg.LogoFile != "" && cfg.LogoFile != embeddedLogo && !fileExists(cfg.LogoFile) {
		return fmt.Errorf("logo not found: %s", cfg.LogoFile)
	}
	if cfg.Background != "" && !fileExists(cfg.Background) {
		return fmt.Errorf("background not found: %s", cfg.Background)
	}
	if cfg.AudioFile != "" && !fileExists(cfg.AudioFile) {
		return fmt.Errorf("audio file not found: %s", cfg.AudioFile)
	}
	if cfg.EndSoundFile != "" && !fileExists(cfg.EndSoundFile) {
		return fmt.Errorf("end sound file not found: %s", cfg.EndSoundFile)
	}
	switch strings.ToLower(cfg.Theme) {
	case "midnight", "warm", "sunrise", "minimal":
	default:
		return fmt.Errorf("unknown theme %q", cfg.Theme)
	}
	switch strings.ToLower(cfg.OutroMode) {
	case "slide", "fade", "typewriter", "none":
	default:
		return fmt.Errorf("unknown outro mode %q", cfg.OutroMode)
	}
	switch strings.ToLower(cfg.ProgressStyle) {
	case "bottom-bar", "bar", "none":
	default:
		return fmt.Errorf("unknown progress style %q", cfg.ProgressStyle)
	}
	switch strings.ToLower(cfg.LogoPosition) {
	case "top-right", "top-left", "bottom-right", "bottom-left", "center", "none", "":
	default:
		return fmt.Errorf("unknown logo position %q", cfg.LogoPosition)
	}
	switch strings.ToLower(cfg.BackgroundFit) {
	case "cover", "contain", "stretch":
	default:
		return fmt.Errorf("unknown background fit mode %q", cfg.BackgroundFit)
	}
	return nil
}

func validateOutputPath(label string, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%s path must not be empty", label)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("%s path is a directory: %s", label, path)
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("%s parent directory does not exist: %s", label, dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s parent path is not a directory: %s", label, dir)
	}
	return nil
}

func validFontPath(path string) bool {
	return fileExists(path) || path == defaultFontPath || path == embeddedFont
}

func prepareEmbeddedAssets(cfg Config) (Config, func(), error) {
	var tempDir string
	cleanup := func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}

	writeAsset := func(name string, data []byte) (string, error) {
		if tempDir == "" {
			dir, err := os.MkdirTemp("", "countdown-assets-*")
			if err != nil {
				return "", err
			}
			tempDir = dir
		}
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			return "", err
		}
		return path, nil
	}

	var embeddedFontPath string
	resolveFont := func(path string) (string, error) {
		if fileExists(path) && path != embeddedFont {
			return path, nil
		}
		if path != defaultFontPath && path != embeddedFont {
			return path, nil
		}
		if embeddedFontPath != "" {
			return embeddedFontPath, nil
		}
		written, err := writeAsset("PassionOne-Bold.ttf", embeddedPassionOneBold)
		if err != nil {
			return "", err
		}
		embeddedFontPath = written
		return written, nil
	}

	var err error
	cfg.FontTimer, err = resolveFont(cfg.FontTimer)
	if err != nil {
		cleanup()
		return cfg, func() {}, err
	}
	cfg.FontTitle, err = resolveFont(cfg.FontTitle)
	if err != nil {
		cleanup()
		return cfg, func() {}, err
	}
	cfg.FontSubtitle, err = resolveFont(cfg.FontSubtitle)
	if err != nil {
		cleanup()
		return cfg, func() {}, err
	}
	if cfg.LogoFile == embeddedLogo {
		cfg.LogoFile, err = writeAsset("logo.png", embeddedLogoPNG)
		if err != nil {
			cleanup()
			return cfg, func() {}, err
		}
	}

	return cfg, cleanup, nil
}

func render(cfg Config) error {
	resolved, cleanup, err := prepareEmbeddedAssets(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	cfg = resolved
	warnLogoIfNeeded(cfg)

	if cfg.Preview != "" {
		args := buildPreviewArgs(cfg)
		if cfg.DryRun {
			fmt.Println("ffmpeg", shellQuoteArgs(args))
			return nil
		}
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		fmt.Println(cfg.Preview)
		return nil
	}

	tmpOutput := cfg.Output + ".tmp.mp4"
	_ = os.Remove(tmpOutput)

	encoders := encoderPlan(cfg)
	var lastErr error
	for _, enc := range encoders {
		args := buildFFmpegArgs(cfg, enc.Args, tmpOutput)
		if cfg.DryRun {
			fmt.Println("ffmpeg", shellQuoteArgs(args))
			return nil
		}
		fmt.Fprintf(os.Stderr, "Trying encoder: %s\n", enc.Name)
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			lastErr = err
			_ = os.Remove(tmpOutput)
			fmt.Fprintf(os.Stderr, "Encoder %s failed; trying next option.\n", enc.Name)
			continue
		}
		if fileExists(cfg.Output) {
			if err := os.Remove(cfg.Output); err != nil {
				_ = os.Remove(tmpOutput)
				return err
			}
		}
		if err := os.Rename(tmpOutput, cfg.Output); err != nil {
			return err
		}
		fmt.Println(cfg.Output)
		return nil
	}
	return fmt.Errorf("all encoders failed: %w", lastErr)
}

func checkFFmpeg(cfg Config) error {
	resolved, cleanup, err := prepareEmbeddedAssets(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	cfg = resolved
	warnLogoIfNeeded(cfg)

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return errors.New("ffmpeg not found in PATH")
	}
	fmt.Printf("FFmpeg: ok %s\n", ffmpegPath)

	filtersOutput, err := ffmpegOutput("-hide_banner", "-filters")
	if err != nil {
		return fmt.Errorf("could not inspect FFmpeg filters: %w", err)
	}
	requiredFilters := []string{
		"adelay",
		"afade",
		"amix",
		"anull",
		"asetpts",
		"atrim",
		"color",
		"crop",
		"drawbox",
		"drawtext",
		"eq",
		"fade",
		"format",
		"gradients",
		"overlay",
		"pad",
		"scale",
		"setparams",
		"vignette",
		"volume",
	}
	if missing := missingFFmpegFeatures(filtersOutput, requiredFilters); len(missing) > 0 {
		return fmt.Errorf("ffmpeg is missing required filters: %s", strings.Join(missing, ", "))
	}
	fmt.Printf("Filters: ok %s\n", strings.Join(requiredFilters, ", "))

	encodersOutput, err := ffmpegOutput("-hide_banner", "-encoders")
	if err != nil {
		return fmt.Errorf("could not inspect FFmpeg encoders: %w", err)
	}
	if !ffmpegFeatureExists(encodersOutput, "libx264") {
		return errors.New("ffmpeg is missing required software encoder: libx264")
	}
	fmt.Println("Software encoder: ok libx264")

	availableHardware := availableHardwareEncoders(encodersOutput, encoderPlan(cfg))
	if len(availableHardware) > 0 {
		fmt.Printf("Hardware encoders listed: %s\n", strings.Join(availableHardware, ", "))
	} else {
		fmt.Println("Hardware encoders listed: none; software fallback is available")
	}

	if err := runTinyDrawtextProbe(cfg.FontTimer); err != nil {
		return fmt.Errorf("tiny drawtext render failed: %w", err)
	}
	fmt.Println("Tiny drawtext render: ok")
	fmt.Println("FFmpeg preflight: ok")
	return nil
}

func ffmpegOutput(args ...string) (string, error) {
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func missingFFmpegFeatures(output string, required []string) []string {
	var missing []string
	for _, name := range required {
		if !ffmpegFeatureExists(output, name) {
			missing = append(missing, name)
		}
	}
	return missing
}

func ffmpegFeatureExists(output string, name string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			if field == name || strings.HasPrefix(field, name+"=") {
				return true
			}
		}
	}
	return false
}

func availableHardwareEncoders(encodersOutput string, plan []Encoder) []string {
	var available []string
	seen := map[string]bool{}
	for _, enc := range plan {
		if enc.Name == "libx264" || seen[enc.Name] {
			continue
		}
		if ffmpegFeatureExists(encodersOutput, enc.Name) {
			available = append(available, enc.Name)
			seen[enc.Name] = true
		}
	}
	return available
}

func runTinyDrawtextProbe(fontPath string) error {
	filter := fmt.Sprintf("drawtext=fontfile=%s:text='ok':fontsize=8:x=0:y=0", escPath(fontPath))
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", "color=s=16x16:d=0.1",
		"-vf", filter,
		"-frames:v", "1",
		"-f", "null",
		"-",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func warnLogoIfNeeded(cfg Config) {
	if !logoEnabled(cfg) || cfg.LogoFile == "" {
		return
	}
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,pix_fmt",
		"-of", "csv=p=0:s=x",
		cfg.LogoFile,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not inspect logo metadata with ffprobe: %s\n", strings.TrimSpace(string(out)))
		return
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "x")
	if len(parts) < 3 {
		return
	}
	width, _ := strconv.Atoi(parts[0])
	height, _ := strconv.Atoi(parts[1])
	pixFmt := parts[2]
	if width > 0 && height > 0 && minInt(width, height) < cfg.LogoWidth*2 {
		fmt.Fprintf(os.Stderr, "warning: logo is %dx%d; consider a larger PNG for %dpx display width\n", width, height, cfg.LogoWidth)
	}
	if !strings.Contains(strings.ToLower(pixFmt), "a") {
		fmt.Fprintf(os.Stderr, "warning: logo pixel format %q may not include transparency; use an RGBA PNG for clean overlays\n", pixFmt)
	}
}

func encoderPlan(cfg Config) []Encoder {
	if cfg.EncoderMode == "software" {
		return []Encoder{softwareEncoder(cfg)}
	}
	if cfg.EncoderMode != "auto" {
		return []Encoder{{Name: cfg.EncoderMode, Args: []string{"-c:v", cfg.EncoderMode, "-b:v", cfg.VideoBitrate}}}
	}
	var encoders []Encoder
	switch runtime.GOOS {
	case "darwin":
		encoders = append(encoders, Encoder{Name: "h264_videotoolbox", Args: []string{"-c:v", "h264_videotoolbox", "-allow_sw", "1", "-b:v", cfg.VideoBitrate}})
	case "windows":
		encoders = append(encoders,
			Encoder{Name: "h264_nvenc", Args: []string{"-c:v", "h264_nvenc", "-b:v", cfg.VideoBitrate}},
			Encoder{Name: "h264_qsv", Args: []string{"-c:v", "h264_qsv", "-b:v", cfg.VideoBitrate}},
			Encoder{Name: "h264_amf", Args: []string{"-c:v", "h264_amf", "-b:v", cfg.VideoBitrate}},
		)
	default:
		encoders = append(encoders,
			Encoder{Name: "h264_nvenc", Args: []string{"-c:v", "h264_nvenc", "-b:v", cfg.VideoBitrate}},
			Encoder{Name: "h264_qsv", Args: []string{"-c:v", "h264_qsv", "-b:v", cfg.VideoBitrate}},
		)
	}
	encoders = append(encoders, softwareEncoder(cfg))
	return encoders
}

func softwareEncoder(cfg Config) Encoder {
	return Encoder{
		Name: "libx264",
		Args: []string{"-c:v", "libx264", "-preset", cfg.SoftwareSpeed, "-crf", strconv.Itoa(cfg.SoftwareCRF)},
	}
}

func buildFFmpegArgs(cfg Config, encoderArgs []string, tmpOutput string) []string {
	args, inputs := buildInputs(cfg, cfg.Duration)
	filterGraph := buildFilterGraph(cfg, inputs.Logo)
	if inputs.Audio >= 0 || inputs.EndSound >= 0 {
		filterGraph += ";" + buildAudioGraph(cfg, inputs.Audio, inputs.EndSound)
	}

	args = append(args, "-filter_complex", filterGraph)
	args = append(args, "-map", "[vout]")
	if inputs.Audio >= 0 || inputs.EndSound >= 0 {
		args = append(args, "-map", "[aout]", "-shortest")
	}
	args = append(args, encoderArgs...)
	if inputs.Audio >= 0 || inputs.EndSound >= 0 {
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	args = append(args, "-r", strconv.Itoa(cfg.FPS), "-t", ff(cfg.Duration), "-fps_mode", "cfr", tmpOutput)
	return args
}

func buildPreviewArgs(cfg Config) []string {
	previewTime := clampFloat(cfg.PreviewTime, 0, cfg.Duration)
	args, inputs := buildInputs(cfg, previewTime+1)
	args = append(args,
		"-filter_complex", buildFilterGraph(cfg, inputs.Logo),
		"-map", "[vout]",
		"-ss", ff(previewTime),
		"-frames:v", "1",
		"-update", "1",
		cfg.Preview,
	)
	return args
}

type inputIndexes struct {
	Logo     int
	Audio    int
	EndSound int
}

func buildInputs(cfg Config, duration float64) ([]string, inputIndexes) {
	theme := themeByName(cfg.Theme)
	background := fmt.Sprintf("gradients=s=%dx%d:r=%d:d=%s:type=radial:speed=0:nb_colors=5:c0=%s:c1=%s:c2=%s:c3=%s:c4=%s",
		cfg.Width, cfg.Height, cfg.FPS, ff(duration), theme.Gradient[0], theme.Gradient[1], theme.Gradient[2], theme.Gradient[3], theme.Gradient[4])
	barInput := fmt.Sprintf("color=c=%s:s=%dx%d:r=%d:d=%s", theme.ProgressColor, cfg.BarWidth, cfg.BarHeight, cfg.FPS, ff(cfg.Duration))

	args := []string{
		"-y",
	}
	if cfg.Background != "" {
		if cfg.BackgroundIsImage {
			args = append(args, "-loop", "1", "-framerate", strconv.Itoa(cfg.FPS), "-i", cfg.Background)
		} else {
			args = append(args, "-stream_loop", "-1", "-i", cfg.Background)
		}
	} else {
		args = append(args, "-f", "lavfi", "-i", background)
	}
	args = append(args, "-f", "lavfi", "-i", barInput)
	inputs := inputIndexes{Logo: -1, Audio: -1, EndSound: -1}
	nextIndex := 2
	if logoEnabled(cfg) {
		args = append(args, "-loop", "1", "-framerate", strconv.Itoa(cfg.FPS), "-i", cfg.LogoFile)
		inputs.Logo = nextIndex
		nextIndex++
	}
	if cfg.AudioFile != "" {
		args = append(args, "-stream_loop", "-1", "-i", cfg.AudioFile)
		inputs.Audio = nextIndex
		nextIndex++
	}
	if cfg.EndSoundFile != "" {
		args = append(args, "-i", cfg.EndSoundFile)
		inputs.EndSound = nextIndex
	}
	return args, inputs
}

func buildFilterGraph(cfg Config, logoIndex int) string {
	theme := themeByName(cfg.Theme)
	outroStart := math.Max(0, cfg.Duration-cfg.OutroSeconds)
	transition := outroTiming(cfg.OutroSeconds)
	controlsFadeEnd := outroStart + transition.ControlsFade
	titleStart := outroStart + transition.TitleDelay
	subtitleStart := titleStart + transition.SubtitleDelay
	outroFade := fmt.Sprintf("min(max((%s-t)/%s,0),1)", ff(controlsFadeEnd), ff(transition.ControlsFade))
	outroReveal := fmt.Sprintf("min(max((t-%s)/%s,0),1)", ff(titleStart), ff(transition.TitleFade))
	subtitleReveal := fmt.Sprintf("min(max((t-%s)/%s,0),1)", ff(subtitleStart), ff(transition.SubtitleFade))

	timerText := fmt.Sprintf("%%{eif\\:floor((%s-t)/60)\\:d\\:2}\\:%%{eif\\:mod(floor(%s-t),60)\\:d\\:2}", ff(cfg.Duration), ff(cfg.Duration))

	baseFilters := []string{
		fmt.Sprintf("fps=%d", cfg.FPS),
		backgroundScaleFilter(cfg),
		"format=yuv420p",
		"setparams=range=tv",
		fmt.Sprintf("eq=brightness=%s:saturation=%s:contrast=%s", theme.Brightness, theme.Saturation, theme.Contrast),
		"vignette=PI/5",
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=%d:color=%s:t=fill", cfg.Width, headerHeight(cfg), theme.HeaderColor),
		fmt.Sprintf("drawtext=fontfile=%s:text='%s':fontsize=%d:x=(w-tw)/2:y=(h-th)/2%+d:fontcolor=%s:borderw=%d:bordercolor=%s:shadowx=%d:shadowy=%d:alpha='if(lt(t,%s)\\,1\\,%s)':enable='lt(t,%s)'",
			escPath(cfg.FontTimer), timerText, cfg.TimerFontSize, cfg.TimerYOffset, theme.TimerColor, scaleInt(cfg.TimerFontSize, 0.08), theme.BorderColor, scaleInt(cfg.TimerFontSize, 0.05), scaleInt(cfg.TimerFontSize, 0.05), ff(outroStart), outroFade, ff(cfg.Duration)),
	}
	if strings.ToLower(cfg.OutroMode) != "none" {
		baseFilters = append(baseFilters, outroTextFilters(cfg, theme, titleStart, subtitleStart, outroReveal, subtitleReveal)...)
	}
	chains := []string{
		"[0:v]" + strings.Join(baseFilters, ",") + "[base]",
	}
	current := "base"
	switch progressStyle(cfg.ProgressStyle) {
	case "bottom-bar", "bar":
		chains = append(chains,
			fmt.Sprintf("[base]drawbox=x=%d:y=%d:w=%d:h=%d:color=%s:t=fill:enable='lt(t,%s)'[bar_bg]", cfg.BarX, cfg.BarY, cfg.BarWidth, cfg.BarHeight, theme.ProgressBackColor, ff(outroStart)),
			fmt.Sprintf("[1:v]scale=w='max(1,%d*max(%s-t,0)/%s)':h=%d:eval=frame[bar]", cfg.BarWidth, ff(cfg.Duration), ff(cfg.Duration), cfg.BarHeight),
			fmt.Sprintf("[bar_bg][bar]overlay=x=%d:y=%d:format=auto:enable='lt(t,%s)'[with_progress]", cfg.BarX, cfg.BarY, ff(outroStart)),
		)
		current = "with_progress"
	case "none":
	}

	if logoEnabled(cfg) {
		posX, posY := logoPositionExpr(cfg)
		chains = append(chains,
			fmt.Sprintf("[%d:v]scale=%d:-1,format=yuva420p,fade=t=out:st=%s:d=%s:alpha=1[logo]", logoIndex, cfg.LogoWidth, ff(outroStart), ff(transition.ControlsFade)),
			fmt.Sprintf("[%s][logo]overlay=x=%s:y=%s:format=yuv420,format=yuv420p[vout]", current, posX, posY),
		)
	} else {
		chains = append(chains, fmt.Sprintf("[%s]format=yuv420p[vout]", current))
	}
	return strings.Join(chains, ";")
}

type themeConfig struct {
	Gradient          [5]string
	Brightness        string
	Saturation        string
	Contrast          string
	HeaderColor       string
	TimerColor        string
	TitleColor        string
	SubtitleColor     string
	BorderColor       string
	ProgressColor     string
	ProgressBackColor string
}

func themeByName(name string) themeConfig {
	switch strings.ToLower(name) {
	case "warm":
		return themeConfig{
			Gradient:          [5]string{"#170b08", "#3b160e", "#6c3218", "#9b632c", "#130908"},
			Brightness:        "-0.08",
			Saturation:        "1.22",
			Contrast:          "1.08",
			HeaderColor:       "#090303@0.28",
			TimerColor:        "#ffd27a",
			TitleColor:        "#f3dbc4",
			SubtitleColor:     "#e9caa5",
			BorderColor:       "#000000",
			ProgressColor:     "#ffd27a",
			ProgressBackColor: "#ffffff@0.16",
		}
	case "sunrise":
		return themeConfig{
			Gradient:          [5]string{"#130a1c", "#452052", "#b04445", "#e7934f", "#120814"},
			Brightness:        "-0.07",
			Saturation:        "1.28",
			Contrast:          "1.06",
			HeaderColor:       "#160817@0.26",
			TimerColor:        "#ffe08a",
			TitleColor:        "#ffe2cf",
			SubtitleColor:     "#f1cfb4",
			BorderColor:       "#000000",
			ProgressColor:     "#ffe08a",
			ProgressBackColor: "#ffffff@0.16",
		}
	case "minimal":
		return themeConfig{
			Gradient:          [5]string{"#101010", "#202020", "#2c2c2c", "#151515", "#070707"},
			Brightness:        "-0.02",
			Saturation:        "0.35",
			Contrast:          "1.05",
			HeaderColor:       "#000000@0.24",
			TimerColor:        "#f2f2f2",
			TitleColor:        "#ffffff",
			SubtitleColor:     "#d8d8d8",
			BorderColor:       "#000000",
			ProgressColor:     "#ffffff",
			ProgressBackColor: "#ffffff@0.14",
		}
	default:
		return themeConfig{
			Gradient:          [5]string{"#06101d", "#102647", "#1b1730", "#2a1d12", "#050914"},
			Brightness:        "-0.10",
			Saturation:        "1.30",
			Contrast:          "1.08",
			HeaderColor:       "#02040a@0.30",
			TimerColor:        "#ffdd77",
			TitleColor:        "#e0d5bd",
			SubtitleColor:     "#e0d5bd",
			BorderColor:       "#000000",
			ProgressColor:     "#ffdd77",
			ProgressBackColor: "#ffffff@0.18",
		}
	}
}

func backgroundScaleFilter(cfg Config) string {
	switch strings.ToLower(cfg.BackgroundFit) {
	case "contain":
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black", cfg.Width, cfg.Height, cfg.Width, cfg.Height)
	case "stretch":
		return fmt.Sprintf("scale=%d:%d", cfg.Width, cfg.Height)
	default:
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", cfg.Width, cfg.Height, cfg.Width, cfg.Height)
	}
}

type outroTransition struct {
	ControlsFade  float64
	TitleDelay    float64
	TitleFade     float64
	SubtitleDelay float64
	SubtitleFade  float64
}

func outroTiming(outroSeconds float64) outroTransition {
	controlsFade := clampFloat(outroSeconds*0.25, 0.2, 0.75)
	controlsFade = math.Min(controlsFade, outroSeconds*0.45)
	beat := clampFloat(outroSeconds*0.05, 0.08, 0.18)
	beat = math.Min(beat, math.Max(0, outroSeconds*0.6-controlsFade))
	titleDelay := controlsFade + beat
	titleFade := math.Max(0.2, outroSeconds-titleDelay)
	subtitleDelay := math.Min(0.45, titleFade*0.32)
	subtitleDelay = math.Min(subtitleDelay, math.Max(0, (outroSeconds-titleDelay)*0.5))
	subtitleFade := math.Max(0.2, outroSeconds-titleDelay-subtitleDelay)
	return outroTransition{
		ControlsFade:  controlsFade,
		TitleDelay:    titleDelay,
		TitleFade:     titleFade,
		SubtitleDelay: subtitleDelay,
		SubtitleFade:  subtitleFade,
	}
}

func outroTextFilters(cfg Config, theme themeConfig, titleStart float64, subtitleStart float64, outroReveal string, subtitleReveal string) []string {
	var filters []string
	mode := strings.ToLower(cfg.OutroMode)
	if cfg.Title != "" {
		titleY := fmt.Sprintf("(h-th)/2 - %d", scaleInt(cfg.OutroTitleSize, 0.34))
		alpha := outroReveal
		drawExpr := ""
		if mode == "slide" {
			titleY = fmt.Sprintf("(h-th)/2 - %d - %d*%s", scaleInt(cfg.OutroTitleSize, 0.34), scaleInt(cfg.OutroTitleSize, 0.40), outroReveal)
		}
		if mode == "typewriter" {
			drawExpr = fmt.Sprintf("drawtext=fontfile=%s:text='%s':fontsize=%d:x=(w-tw)/2:y='%s':fontcolor=%s:borderw=%d:bordercolor=%s:shadowx=%d:shadowy=%d:alpha='%s':enable='gte(t,%s)'",
				escPath(cfg.FontTitle), escDrawtext(cfg.Title), cfg.OutroTitleSize, titleY, theme.TitleColor, scaleInt(cfg.OutroTitleSize, 0.11), theme.BorderColor, scaleInt(cfg.OutroTitleSize, 0.07), scaleInt(cfg.OutroTitleSize, 0.07), alpha, ff(titleStart))
			cursorWidth := maxInt(3, scaleInt(cfg.OutroTitleSize, 0.08))
			cursorHeight := maxInt(12, scaleInt(cfg.OutroTitleSize, 0.95))
			approxTitleWidth := maxInt(cfg.OutroTitleSize, roundInt(float64(len([]rune(cfg.Title)))*float64(cfg.OutroTitleSize)*0.52))
			cursorX := fmt.Sprintf("(w-%d)/2 + %d*%s", approxTitleWidth, approxTitleWidth, outroReveal)
			cursorY := fmt.Sprintf("(h-%d)/2 - %d + %d", cfg.OutroTitleSize, scaleInt(cfg.OutroTitleSize, 0.34), scaleInt(cfg.OutroTitleSize, 0.06))
			filters = append(filters, drawExpr)
			filters = append(filters, fmt.Sprintf("drawbox=x='%s':y='%s':w=%d:h=%d:color=%s:t=fill:enable='gte(t,%s)'", cursorX, cursorY, cursorWidth, cursorHeight, theme.TitleColor, ff(titleStart)))
		} else {
			filters = append(filters, fmt.Sprintf("drawtext=fontfile=%s:text='%s':fontsize=%d:x=(w-tw)/2:y='%s':fontcolor=%s:borderw=%d:bordercolor=%s:shadowx=%d:shadowy=%d:alpha='%s':enable='gte(t,%s)'",
				escPath(cfg.FontTitle), escDrawtext(cfg.Title), cfg.OutroTitleSize, titleY, theme.TitleColor, scaleInt(cfg.OutroTitleSize, 0.11), theme.BorderColor, scaleInt(cfg.OutroTitleSize, 0.07), scaleInt(cfg.OutroTitleSize, 0.07), alpha, ff(titleStart)))
		}
	}
	if cfg.Subtitle != "" {
		filters = append(filters, fmt.Sprintf("drawtext=fontfile=%s:text='%s':fontsize=%d:x=(w-tw)/2:y='(h-th)/2 + %d':fontcolor=%s:borderw=%d:bordercolor=%s:shadowx=%d:shadowy=%d:alpha='%s':enable='gte(t,%s)'",
			escPath(cfg.FontSubtitle), escDrawtext(cfg.Subtitle), cfg.OutroSubtitleSize, scaleInt(cfg.OutroSubtitleSize, 1.7), theme.SubtitleColor, scaleInt(cfg.OutroSubtitleSize, 0.12), theme.BorderColor, scaleInt(cfg.OutroSubtitleSize, 0.07), scaleInt(cfg.OutroSubtitleSize, 0.07), subtitleReveal, ff(subtitleStart)))
	}
	return filters
}

func buildAudioGraph(cfg Config, audioIndex int, endSoundIndex int) string {
	var chains []string
	var labels []string
	if audioIndex >= 0 {
		filters := []string{
			fmt.Sprintf("[%d:a]atrim=0:%s", audioIndex, ff(cfg.Duration)),
			"asetpts=PTS-STARTPTS",
			fmt.Sprintf("volume=%s", ff(cfg.AudioVolume)),
		}
		if cfg.FadeAudio {
			filters = append(filters, fmt.Sprintf("afade=t=out:st=%s:d=%s", ff(math.Max(0, cfg.Duration-cfg.OutroSeconds)), ff(cfg.OutroSeconds)))
		}
		chains = append(chains, strings.Join(filters, ",")+"[audio_bg]")
		labels = append(labels, "[audio_bg]")
	}
	if endSoundIndex >= 0 {
		delayMS := maxInt(0, roundInt(math.Max(0, cfg.Duration-1)*1000))
		chains = append(chains, fmt.Sprintf("[%d:a]atrim=0:3,asetpts=PTS-STARTPTS,adelay=%d|%d[end_sound]", endSoundIndex, delayMS, delayMS))
		labels = append(labels, "[end_sound]")
	}
	if len(labels) == 1 {
		chains = append(chains, labels[0]+"anull[aout]")
	} else {
		chains = append(chains, strings.Join(labels, "")+fmt.Sprintf("amix=inputs=%d:duration=longest:dropout_transition=0[aout]", len(labels)))
	}
	return strings.Join(chains, ";")
}

func logoEnabled(cfg Config) bool {
	return cfg.LogoFile != "" && strings.ToLower(cfg.LogoPosition) != "none"
}

func progressStyle(s string) string {
	if strings.ToLower(s) == "bar" {
		return "bottom-bar"
	}
	return strings.ToLower(s)
}

func logoPositionExpr(cfg Config) (string, string) {
	switch strings.ToLower(cfg.LogoPosition) {
	case "top-left":
		return strconv.Itoa(cfg.LogoMarginX), strconv.Itoa(cfg.LogoMarginY)
	case "bottom-right":
		return fmt.Sprintf("main_w-overlay_w-%d", cfg.LogoMarginX), fmt.Sprintf("main_h-overlay_h-%d", cfg.LogoMarginY)
	case "bottom-left":
		return strconv.Itoa(cfg.LogoMarginX), fmt.Sprintf("main_h-overlay_h-%d", cfg.LogoMarginY)
	case "center":
		return "(main_w-overlay_w)/2", "(main_h-overlay_h)/2"
	case "top-right", "":
		return fmt.Sprintf("main_w-overlay_w-%d", cfg.LogoMarginX), strconv.Itoa(cfg.LogoMarginY)
	default:
		return fmt.Sprintf("main_w-overlay_w-%d", cfg.LogoMarginX), strconv.Itoa(cfg.LogoMarginY)
	}
}

func scaleInt(base int, multiplier float64) int {
	n := int(math.Round(float64(base) * multiplier))
	if n < 1 {
		return 1
	}
	return n
}

func responsiveBitrate(width int, height int) string {
	pixels := width * height
	switch {
	case pixels >= 3840*2160:
		return "16M"
	case pixels >= 2560*1440:
		return "10M"
	case pixels >= 1920*1080:
		return "6M"
	default:
		return "3M"
	}
}

func headerHeight(cfg Config) int {
	return maxInt(32, roundInt(float64(cfg.Height)*0.05))
}

func roundInt(v float64) int {
	return int(math.Round(v))
}

func evenInt(v int) int {
	if v <= 2 {
		return v
	}
	if v%2 == 0 {
		return v
	}
	return v - 1
}

func clampInt(v int, min int, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampFloat(v float64, min float64, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func escPath(s string) string {
	return strings.ReplaceAll(s, ":", "\\:")
}

func escDrawtext(s string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		":", "\\:",
		"'", "\\'",
		",", "\\,",
		"[", "\\[",
		"]", "\\]",
		"%", "\\%",
	)
	return replacer.Replace(s)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ff(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func trimFloat(v float64) string {
	return strings.ReplaceAll(ff(v), ".", "_")
}

func shellQuoteArgs(args []string) string {
	var quoted []string
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n'\"$&|;()<>*?[]{}") {
			quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
		} else {
			quoted = append(quoted, arg)
		}
	}
	return strings.Join(quoted, " ")
}

func init() {
	// Keep outputs relative to the current working directory, but make sure any
	// parent directory requested by --output already exists. FFmpeg errors are
	// easier to understand when path validation happens before rendering.
	flag.CommandLine.Init(filepath.Base(os.Args[0]), flag.ExitOnError)
}
