import {scriptureWordMotion} from './scripture.mjs';

export const clamp = (value, min = 0, max = 1) => Math.min(max, Math.max(min, value));
export const ease = (value) => {
  const t = clamp(value);
  return t * t * (3 - 2 * t);
};
export const reveal = (time, start, duration) => duration <= 0 ? Number(time >= start) : ease((time - start) / duration);
export const backgroundLights = (time, motion = 'subtle') => {
  const t = motion === 'none' ? 0 : time;
  const wave = (period, phase = 0) => Math.sin(t * Math.PI * 2 / period + phase);
  return {
    primary: {x: 22 + 5 * wave(96), y: 71 + 4 * wave(113, 0.7)},
    secondary: {x: 76 + 4 * wave(123, 1.1), y: 29 + 3 * wave(137, 2.2)},
    glow: {x: 64 + 5 * wave(151, 0.4), y: 47 + 4 * wave(163, 1.7)},
  };
};
export const wordState = (time, word, slide, fps = 30) => {
  const progress = reveal(time, word.Appear, word.Fade);
  const exitSeconds = slide.ExitSeconds ?? Math.min(0.6, (slide.End - slide.Start) / 4);
  // A restrained overlap lets the background breathe between passages.
  // Two full sentences should never compete at half opacity for readability.
  const exitPhase = exitSeconds > 0 ? clamp((time - slide.End + exitSeconds) / exitSeconds) : 0;
  const enterPhase = slide.EnterSeconds > 0 ? clamp((time - slide.Start) / slide.EnterSeconds) : 1;
  const exit = 1 - ease(slide.CrossfadeOut ? exitPhase / 0.6 : exitPhase);
  const enter = ease(slide.CrossfadeIn ? (enterPhase - 0.4) / 0.6 : enterPhase);
  const sceneOpacity = exit * enter;
  const lift = slide.Style === 'soft-dissolve' ? 0 : word.Size * 0.16;
  const transitionBlur = slide.CrossfadeIn || slide.CrossfadeOut ? word.Size * 0.02 * (1 - sceneOpacity) : 0;
  const motion = slide.Type === 'scripture-reveal' ? scriptureWordMotion(time, word, slide.Style, fps) : {
    y: lift * (1 - progress), blur: word.Size * 0.035 * (1 - progress),
  };
  return {opacity: progress * sceneOpacity, y: motion.y, blur: motion.blur + transitionBlur};
};
export const countdown = (duration, time) => {
  const seconds = Math.max(0, Math.floor(duration - time + 1e-7));
  return `${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`;
};
export const timerOpacity = (plan, time) => {
  const c = plan.Config;
  const hasContent = (plan.Slides?.length ?? 0) > 0;
  const shouldFade = c.OutroMode.toLowerCase() !== 'none';
  const outroOpacity = shouldFade ? 1 - reveal(time, plan.OutroStart, plan.Timing.ControlsFade) : 1;
  if (!hasContent) return outroOpacity;
  // The counter rounds down, so 00:02 begins just after three seconds remain.
  // Fade the label and digits together before that tick, including with no outro.
  const hideAt = Math.max(0, c.Duration - 3);
  const fadeSeconds = Math.min(1, hideAt);
  return Math.min(outroOpacity, 1 - reveal(time, hideAt - fadeSeconds, fadeSeconds));
};
export const progressState = (plan, time) => {
  const c = plan.Config;
  if (c.ProgressStyle.toLowerCase() === 'none' || plan.ProgressEnd <= 0) return null;
  const hasOutro = c.OutroMode.toLowerCase() !== 'none';
  const fadeEnd = hasOutro ? Math.min(plan.ProgressEnd, plan.OutroStart + plan.Timing.ControlsFade) : plan.ProgressEnd;
  if (time >= fadeEnd) return null;
  const fadeSeconds = Math.min(1, fadeEnd / 3);
  const opacity = reveal(time, 0, Math.min(0.8, fadeEnd / 3)) * (1 - reveal(time, fadeEnd - fadeSeconds, fadeSeconds));
  // Keep the amount linear. Only the entrance, exit and edge light are eased.
  const fillWidth = c.BarWidth * clamp(1 - time / plan.ProgressEnd);
  const thickness = Math.max(1, c.BarHeight * 0.3);
  const glowWidth = Math.max(12, thickness * 12);
  const glowOpacity = ease(fillWidth / (glowWidth / 2)) * ease((c.BarWidth - fillWidth) / (glowWidth / 2));
  return {fillWidth, thickness, glowWidth, glowHeight: c.BarHeight, glowOpacity, opacity};
};
export const cssColor = (color) => {
  const [base, alpha] = color.split('@');
  if (alpha === undefined) return base;
  return `color-mix(in srgb, ${base} ${Number(alpha) * 100}%, transparent)`;
};

// Remotion owns visual composition and audio mixing. This adapter preserves
// the CLI's existing encoder choices at the final encoding stage only.
// Versions are pinned because the renderer's generated arguments can change.
export const withEncoder = (args, encoder) => {
  const options = new Set(['-c:v', '-codec:v', '-vcodec', '-crf', '-preset', '-b:v', '-maxrate', '-bufsize', '-allow_sw']);
  const result = [];
  for (let i = 0; i < args.length; i++) {
    if (options.has(args[i])) { i++; continue; }
    result.push(args[i]);
  }
  result.splice(result.length - 1, 0, ...encoder.Args);
  return result;
};
