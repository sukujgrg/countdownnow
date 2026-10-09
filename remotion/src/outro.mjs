import {Easing, interpolate, spring} from 'remotion';

const springConfig = {damping: 26, stiffness: 90, mass: 1, overshootClamping: true};
const opacityEase = Easing.bezier(0.25, 0.1, 0.25, 1);
const interpolation = {extrapolateLeft: 'clamp', extrapolateRight: 'clamp'};

export const outroScene = (plan, fps) => {
  const c = plan.Config;
  if (!(plan.Slides?.length > 0) || c.OutroMode.toLowerCase() === 'none') return null;
  const landscape = c.Width / c.Height >= 1.45;
  const start = Math.ceil((plan.OutroStart + plan.Timing.TitleDelay) * fps);
  const end = plan.Frames;
  // Finish every entrance together, leaving the final third as a still hold.
  const budget = Math.max(1, Math.floor((end - start - 1) * 0.65));
  const titleDuration = Math.min(Math.round(1.15 * fps), budget);
  const wordCount = Math.max(1, c.Title.match(/\S+/g)?.length ?? 0);
  const wordDelay = wordCount > 1 ? Math.min(Math.round(0.07 * fps), Math.floor((budget - titleDuration) / (wordCount - 1))) : 0;
  const subtitleDelay = Math.min(Math.round(0.24 * fps), Math.floor(budget * 0.15));
  const ruleDelay = Math.min(Math.round(0.12 * fps), Math.floor(budget * 0.1));
  return {
    start, titleDuration, wordDelay,
    subtitleStart: start + subtitleDelay,
    subtitleDuration: Math.min(Math.round(0.9 * fps), budget - subtitleDelay),
    ruleStart: start + ruleDelay,
    ruleDuration: Math.min(Math.round(0.85 * fps), budget - ruleDelay),
    padding: Math.max(c.SafeArea, c.Width * 0.075),
    titleSize: c.OutroTitleSize * (landscape ? 1.65 : 1.25),
    subtitleSize: c.OutroSubtitleSize * (landscape ? 1.25 : 1.1),
    ruleWidth: Math.min(c.Width * 0.12, Math.min(c.Width, c.Height) * 0.18),
    ruleHeight: Math.max(1, Math.min(c.Width, c.Height) * 0.002),
  };
};

export const outroElementState = (frame, fps, start, duration, size, mode = 'fade') => {
  const localFrame = frame - start;
  const progress = localFrame <= 0 ? 0 : localFrame >= duration ? 1 : spring({
    frame: localFrame, fps, durationInFrames: duration,
    durationRestThreshold: 0.001, config: springConfig,
  });
  const opacity = interpolate(localFrame, [0, Math.max(1, Math.min(duration, Math.round(0.65 * fps)))], [0, 1], {
    ...interpolation, easing: opacityEase,
  });
  return {
    progress, opacity,
    y: interpolate(progress, [0, 1], [size * (mode === 'slide' ? 0.42 : 0.12), 0], interpolation),
    blur: interpolate(progress, [0, 1], [Math.min(4, size * 0.045), 0], interpolation),
    scale: interpolate(progress, [0, 1], [0.985, 1], interpolation),
  };
};
