import {reveal} from './motion.mjs';

export const logoEnabled = (plan) => Boolean(plan.Config.LogoFile) && plan.Config.LogoPosition.toLowerCase() !== 'none';

export const logoShadow = (plan) => {
  if (plan.Config.LogoShadow?.toLowerCase() === 'none') return undefined;
  const short = Math.min(plan.Config.Width, plan.Config.Height);
  return `drop-shadow(0 ${short * 0.002}px ${short * 0.006}px rgba(0,0,0,0.28))`;
};

export const cornerLogoOpacity = (plan, time) => {
  const fadeIn = Math.min(1.8, Math.max(0.01, plan.OutroStart * 0.35));
  const fadeOut = plan.Config.OutroMode.toLowerCase() === 'none' ? 1 : 1 - reveal(time, plan.OutroStart, plan.Timing.ControlsFade);
  return reveal(time, 0, fadeIn) * fadeOut;
};

export const closingLogoLayout = (plan, availableWidth, availableHeight) => {
  const c = plan.Config;
  if (!logoEnabled(plan) || c.LogoOutro?.toLowerCase() === 'fade' || c.OutroMode.toLowerCase() === 'none' || !(c.Title || c.Subtitle)) return null;
  const short = Math.min(c.Width, c.Height);
  const ratio = plan.LogoAspectRatio > 0 ? plan.LogoAspectRatio : 1;
  const width = Math.min(c.LogoWidth * 1.18, short * 0.145, availableWidth);
  const height = Math.min(width / ratio, short * 0.12, availableHeight * 0.6);
  const gap = Math.min(short * 0.025, availableHeight - height);
  if (width <= 0 || height <= 0 || gap < 0) return null;
  return {width: Math.min(width, height * ratio), height, gap};
};

export const closingLogoOpacity = (time, start, duration) => reveal(time, start, Math.min(1.2, duration));
