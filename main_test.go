package main

import (
	"strings"
	"testing"
)

func TestNormalizeExplicitNoneDisablesTextAndLogo(t *testing.T) {
	cfg := defaultConfig()
	cfg.Title = " none "
	cfg.Subtitle = "NoNe"
	cfg.LogoFile = "NONE"

	normalizeExplicitNone(&cfg)

	if cfg.Title != "" {
		t.Fatalf("title = %q, want empty", cfg.Title)
	}
	if cfg.Subtitle != "" {
		t.Fatalf("subtitle = %q, want empty", cfg.Subtitle)
	}
	if cfg.LogoFile != "" {
		t.Fatalf("logo file = %q, want empty", cfg.LogoFile)
	}
	if cfg.LogoPosition != "none" {
		t.Fatalf("logo position = %q, want none", cfg.LogoPosition)
	}
}

func TestNormalizeExplicitEmptyLogoDisablesAutoFallback(t *testing.T) {
	cfg := defaultConfig()
	cfg.UserLogo = true
	cfg.LogoFile = ""

	normalizeExplicitNone(&cfg)
	applyDefaults(&cfg)

	if cfg.LogoFile != "" {
		t.Fatalf("logo file = %q, want empty", cfg.LogoFile)
	}
	if cfg.LogoPosition != "none" {
		t.Fatalf("logo position = %q, want none", cfg.LogoPosition)
	}
}

func TestApplyPresetRejectsUnknownPreset(t *testing.T) {
	cfg := defaultConfig()
	cfg.Preset = "cinema"

	if err := applyPreset(&cfg); err == nil {
		t.Fatal("applyPreset returned nil error for unknown preset")
	}
}

func TestApplyPresetKeepsExplicitDimensions(t *testing.T) {
	cfg := defaultConfig()
	cfg.Width = 1920
	cfg.Height = 1080
	cfg.Preset = "vertical"

	if err := applyPreset(&cfg); err != nil {
		t.Fatalf("applyPreset returned error: %v", err)
	}
	if cfg.Width != 1920 || cfg.Height != 1080 {
		t.Fatalf("dimensions = %dx%d, want 1920x1080", cfg.Width, cfg.Height)
	}
}

func TestValidateOutputPathRejectsDirectoryTarget(t *testing.T) {
	if err := validateOutputPath("output", t.TempDir()); err == nil {
		t.Fatal("validateOutputPath returned nil error for directory target")
	}
}

func TestValidateOutputPathRejectsMissingParent(t *testing.T) {
	path := t.TempDir() + "/missing/out.mp4"
	if err := validateOutputPath("output", path); err == nil {
		t.Fatal("validateOutputPath returned nil error for missing parent")
	}
}

func TestApplyFontFlagsUsesSingleFontForAllText(t *testing.T) {
	cfg := defaultConfig()
	cfg.Font = "/tmp/custom.ttf"
	cfg.UserFont = true

	applyFontFlags(&cfg)

	if cfg.FontTimer != cfg.Font {
		t.Fatalf("timer font = %q, want %q", cfg.FontTimer, cfg.Font)
	}
	if cfg.FontTitle != cfg.Font {
		t.Fatalf("title font = %q, want %q", cfg.FontTitle, cfg.Font)
	}
	if cfg.FontSubtitle != cfg.Font {
		t.Fatalf("subtitle font = %q, want %q", cfg.FontSubtitle, cfg.Font)
	}
}

func TestApplyFontFlagsKeepsAdvancedOverride(t *testing.T) {
	cfg := defaultConfig()
	cfg.Font = "/tmp/common.ttf"
	cfg.FontTitle = "/tmp/title.ttf"
	cfg.FontSubtitle = "/tmp/subtitle.ttf"
	cfg.UserFont = true
	cfg.UserFontTitle = true
	cfg.UserFontSubtitle = true

	applyFontFlags(&cfg)

	if cfg.FontTimer != cfg.Font {
		t.Fatalf("timer font = %q, want %q", cfg.FontTimer, cfg.Font)
	}
	if cfg.FontTitle != "/tmp/title.ttf" {
		t.Fatalf("title font = %q, want advanced override", cfg.FontTitle)
	}
	if cfg.FontSubtitle != "/tmp/subtitle.ttf" {
		t.Fatalf("subtitle font = %q, want advanced override", cfg.FontSubtitle)
	}
}

func TestValidFontPathAcceptsEmbeddedDefaults(t *testing.T) {
	if !validFontPath(defaultFontPath) {
		t.Fatalf("default font path was rejected")
	}
	if !validFontPath(embeddedFont) {
		t.Fatalf("embedded font path was rejected")
	}
}

func TestValidateAcceptsTimerColorOverride(t *testing.T) {
	cfg := defaultConfig()
	cfg.Output = "out.mp4"
	cfg.Width = 1920
	cfg.Height = 1080
	cfg.TimerColor = "#00ff88"

	if err := validate(cfg); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
}

func TestValidateRejectsInvalidTimerColorOverride(t *testing.T) {
	cfg := defaultConfig()
	cfg.Output = "out.mp4"
	cfg.Width = 1920
	cfg.Height = 1080
	cfg.TimerColor = "white:fontsize=200"

	if err := validate(cfg); err == nil {
		t.Fatal("validate returned nil error for invalid timer color")
	}
}

func TestValidatePositionalArgsRejectsUnexpectedArgument(t *testing.T) {
	err := validatePositionalArgs([]string{"extra"})
	if err == nil {
		t.Fatal("validatePositionalArgs returned nil error for unexpected argument")
	}
	if !strings.Contains(err.Error(), "countdownnow only accepts flags") {
		t.Fatalf("error = %q, want flags-only message", err)
	}
}

func TestValidatePositionalArgsHintsOutroSeconds(t *testing.T) {
	err := validatePositionalArgs([]string{"23", "--theme", "minimal"})
	if err == nil {
		t.Fatal("validatePositionalArgs returned nil error for unexpected argument")
	}
	if !strings.Contains(err.Error(), "did you mean --outro-seconds 23") {
		t.Fatalf("error = %q, want outro-seconds hint", err)
	}
}

func TestOutroTimingStagesTitleAfterControlsStartFading(t *testing.T) {
	timing := outroTiming(3)
	if timing.ControlsFade <= 0 {
		t.Fatalf("controls fade = %v, want positive", timing.ControlsFade)
	}
	if timing.TitleDelay <= 0 || timing.TitleDelay >= 3 {
		t.Fatalf("title delay = %v, want inside outro", timing.TitleDelay)
	}
	if timing.TitleDelay <= timing.ControlsFade {
		t.Fatalf("title delay = %v, want after controls fade ends at %v", timing.TitleDelay, timing.ControlsFade)
	}
	if timing.SubtitleDelay <= 0 || timing.TitleDelay+timing.SubtitleDelay >= 3 {
		t.Fatalf("subtitle delay = %v, want subtitle inside outro", timing.SubtitleDelay)
	}
	if timing.TitleFade <= 0 || timing.TitleFade > 1.4 {
		t.Fatalf("title fade = %v, want capped reveal pace", timing.TitleFade)
	}
}

func TestOutroTimingHandlesShortOutro(t *testing.T) {
	timing := outroTiming(0.25)
	if timing.ControlsFade <= 0 || timing.ControlsFade > 0.25 {
		t.Fatalf("controls fade = %v, want within short outro", timing.ControlsFade)
	}
	if timing.TitleDelay < 0 || timing.TitleDelay > 0.25 {
		t.Fatalf("title delay = %v, want within short outro", timing.TitleDelay)
	}
	if timing.TitleDelay+timing.SubtitleDelay > 0.25 {
		t.Fatalf("subtitle starts after outro: title delay %v subtitle delay %v", timing.TitleDelay, timing.SubtitleDelay)
	}
	if timing.TitleDelay+timing.TitleFade > 0.25 {
		t.Fatalf("title reveal ends after short outro: title delay %v title fade %v", timing.TitleDelay, timing.TitleFade)
	}
	if timing.TitleDelay+timing.SubtitleDelay+timing.SubtitleFade > 0.25 {
		t.Fatalf("subtitle reveal ends after short outro: title delay %v subtitle delay %v subtitle fade %v", timing.TitleDelay, timing.SubtitleDelay, timing.SubtitleFade)
	}
}

func TestOutroTimingCapsLongOutroRevealPace(t *testing.T) {
	timing := outroTiming(20)
	if timing.TitleDelay > 1.2 {
		t.Fatalf("title delay = %v, want title to start early for long outro", timing.TitleDelay)
	}
	if timing.TitleFade > 1.4 {
		t.Fatalf("title fade = %v, want capped reveal pace", timing.TitleFade)
	}
	if timing.SubtitleDelay > 0.45 {
		t.Fatalf("subtitle delay = %v, want capped subtitle delay", timing.SubtitleDelay)
	}
	if timing.SubtitleFade > 1.2 {
		t.Fatalf("subtitle fade = %v, want capped subtitle reveal pace", timing.SubtitleFade)
	}
}

func TestProgressEndTimeVanishesBeforeEnd(t *testing.T) {
	if got := progressEndTime(6); got != 5.5 {
		t.Fatalf("progressEndTime(6) = %v, want 5.5", got)
	}
}

func TestProgressEndTimeHandlesShortDuration(t *testing.T) {
	got := progressEndTime(1)
	if got <= 0 || got >= 1 {
		t.Fatalf("progressEndTime(1) = %v, want inside duration", got)
	}
}

func TestOutroOverDurationExtendsOutputOnly(t *testing.T) {
	cfg := defaultConfig()
	cfg.Duration = 10
	cfg.OutroSeconds = 6
	cfg.OutroOverDuration = true

	if got := outputDuration(cfg); got != 16 {
		t.Fatalf("outputDuration = %v, want 16", got)
	}
	if got := outroStartTime(cfg); got != 10 {
		t.Fatalf("outroStartTime = %v, want 10", got)
	}
}

func TestDefaultOutroTakesTimeFromDuration(t *testing.T) {
	cfg := defaultConfig()
	cfg.Duration = 10
	cfg.OutroSeconds = 6

	if got := outputDuration(cfg); got != 10 {
		t.Fatalf("outputDuration = %v, want 10", got)
	}
	if got := outroStartTime(cfg); got != 4 {
		t.Fatalf("outroStartTime = %v, want 4", got)
	}
}

func TestBuildFFmpegArgsUsesExtendedOutroDuration(t *testing.T) {
	cfg := defaultConfig()
	cfg.Duration = 10
	cfg.OutroSeconds = 6
	cfg.OutroOverDuration = true

	args := buildFFmpegArgs(cfg, softwareEncoder(cfg).Args, "out.mp4")
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-t" && args[i+1] == "16" {
			return
		}
	}
	t.Fatalf("ffmpeg args missing extended -t 16: %v", args)
}

func TestExtendedOutroTimerClampsAtZeroDuringFade(t *testing.T) {
	cfg := defaultConfig()
	cfg.Duration = 10
	cfg.OutroSeconds = 6
	cfg.OutroOverDuration = true

	graph := buildFilterGraph(cfg, -1)
	if !strings.Contains(graph, "max(10-t,0)") {
		t.Fatalf("timer expression should clamp remaining time at zero: %s", graph)
	}
	if !strings.Contains(graph, "enable='lt(t,10.75)'") {
		t.Fatalf("timer should remain enabled through the extended outro fade: %s", graph)
	}
}

func TestTimerColorOverrideUsesCustomFontColor(t *testing.T) {
	cfg := defaultConfig()
	cfg.TimerColor = "#00ff88"

	graph := buildFilterGraph(cfg, -1)
	if !strings.Contains(graph, "fontcolor=#00ff88") {
		t.Fatalf("timer drawtext should use custom timer color: %s", graph)
	}
}
