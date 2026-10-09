import {interpolate, spring} from 'remotion';

const springConfig = {damping: 30, stiffness: 60, mass: 1.2, overshootClamping: true};
const interpolation = {extrapolateLeft: 'clamp', extrapolateRight: 'clamp'};

export const scriptureWordMotion = (time, word, style, fps) => {
  const elapsed = time - word.Appear;
  // Use the existing scheduled fade exactly: never extend an entrance into
  // the full-verse reading hold or change the seeded reveal order.
  let progress = 0;
  if (elapsed >= 0) {
    if (word.Fade <= 0 || elapsed >= word.Fade - 1e-7) {
      progress = 1;
    } else if (elapsed > 0) {
      progress = spring({frame: elapsed * fps, fps, durationInFrames: word.Fade * fps,
        durationRestThreshold: 0.001, config: springConfig});
    }
  }
  const lift = style === 'soft-dissolve' || word.Kind !== 'content' ? 0 : Math.min(2.5, word.Size * 0.045);
  return {
    y: interpolate(progress, [0, 1], [lift, 0], interpolation),
    blur: interpolate(progress, [0, 1], [word.Size * 0.022, 0], interpolation),
  };
};
