import React, {useEffect, useState} from 'react';
import {AbsoluteFill, AnimatedImage, Audio, Composition, Img, Loop, OffthreadVideo, Sequence, cancelRender, continueRender, delayRender, registerRoot, useCurrentFrame, useVideoConfig} from 'remotion';
import {backgroundLights, clamp, countdown, cssColor, progressState, reveal, timerOpacity, wordState} from './motion.mjs';
import {outroElementState, outroScene} from './outro.mjs';
import {closingLogoLayout, closingLogoOpacity, cornerLogoOpacity, logoEnabled, logoShadow} from './logo.mjs';

const Fonts = ({fonts, children}) => {
  const [handle] = useState(() => delayRender('Loading countdown fonts'));
  useEffect(() => {
    let active = true;
    Promise.all(Object.entries(fonts).map(async ([role, src]) => {
      const face = new FontFace(`Countdown-${role}`, `url("${src}")`);
      await face.load();
      document.fonts.add(face);
    })).then(() => { if (active) continueRender(handle); }).catch(cancelRender);
    return () => { active = false; };
  }, [fonts, handle]);
  return children;
};

const Background = ({plan, time}) => {
  const c = plan.Config;
  const theme = plan.Theme;
  const fitMode = c.BackgroundFit.toLowerCase();
  const fit = fitMode === 'stretch' ? 'fill' : fitMode;
  const animated = /\.(gif|apng|png|avif|webp)$/i.test(c.Background);
  const lights = backgroundLights(time, c.BackgroundMotion);
  const style = {width: '100%', height: '100%', objectFit: fit,
    filter: `brightness(${1 + Number(theme.Brightness)}) saturate(${theme.Saturation}) contrast(${theme.Contrast})`};
  return <AbsoluteFill style={{background: c.Background ? '#000000' : '#050914'}}>
    {c.Background ? c.BackgroundIsImage ? animated ?
      <AnimatedImage src={c.Background} width={c.Width} height={c.Height} fit={fit} style={{filter: style.filter}}/> :
      <Img src={c.Background} style={style}/> :
      <Loop durationInFrames={plan.BackgroundFrames}>
        <OffthreadVideo src={c.Background} muted style={style}/>
      </Loop> :
      <AbsoluteFill style={{background: `radial-gradient(ellipse at ${lights.glow.x}% ${lights.glow.y}%, ${cssColor(theme.ProgressColor + '@0.055')} 0%, transparent 65%), radial-gradient(ellipse at ${lights.secondary.x}% ${lights.secondary.y}%, ${theme.Gradient[2]} 0%, transparent 62%), radial-gradient(ellipse at ${lights.primary.x}% ${lights.primary.y}%, ${theme.Gradient[1]} 0%, transparent 68%), linear-gradient(125deg, ${theme.Gradient[0]}, ${theme.Gradient[4]})`}}/>}
    <AbsoluteFill style={{background: 'radial-gradient(ellipse at center, transparent 30%, rgba(0,0,0,0.45) 100%)'}}/>
    <div style={{position: 'absolute', top: 0, width: '100%', height: '22%', background: `linear-gradient(180deg, ${cssColor(theme.HeaderColor)}, transparent)`}}/>
  </AbsoluteFill>;
};

const Timer = ({plan, time}) => {
  const c = plan.Config;
  const hasContent = (plan.Slides?.length ?? 0) > 0;
  const landscape = c.Width / c.Height >= 1.45;
  const contentSize = Math.round(Math.min(c.Width, c.Height) * (landscape ? 0.16 : 0.14));
  const size = hasContent ? contentSize : c.TimerFontSize;
  const centerX = hasContent && landscape ? c.Width * 0.23 : c.Width / 2;
  const centerY = hasContent ? c.Height * (landscape ? 0.50 : 0.19) : c.Height / 2 + c.TimerYOffset;
  const opacity = timerOpacity(plan, time);
  return <div style={{position: 'absolute', left: centerX, top: centerY, transform: 'translate(-50%, -50%)', textAlign: 'center', opacity}}>
    {hasContent && <div style={{fontFamily: 'Countdown-subtitle', fontSize: Math.max(12, Math.round(Math.min(c.Width, c.Height) * 0.022)), letterSpacing: '0.12em', color: plan.Theme.SubtitleColor, marginBottom: size * 0.12, whiteSpace: 'nowrap'}}>SERVICE STARTS IN</div>}
    <div style={{fontFamily: 'Countdown-timer', fontSize: size, lineHeight: 1, fontVariantNumeric: 'tabular-nums lining-nums', letterSpacing: '0.015em', color: cssColor(c.TimerColor || plan.Theme.TimerColor), whiteSpace: 'nowrap', textShadow: '0 3px 20px rgba(0,0,0,0.25)'}}>{countdown(c.Duration, time).split(':').map((part, i) => <React.Fragment key={i}>{i > 0 && <span style={{opacity: 0.65}}>:</span>}{part}</React.Fragment>)}</div>
  </div>;
};

const Logo = ({plan, time}) => {
  const c = plan.Config;
  if (!logoEnabled(plan)) return null;
  const style = {position: 'absolute', width: c.LogoWidth, opacity: cornerLogoOpacity(plan, time), filter: logoShadow(plan)};
  switch (c.LogoPosition.toLowerCase()) {
    case 'top-left': Object.assign(style, {left: c.LogoMarginX, top: c.LogoMarginY}); break;
    case 'bottom-left': Object.assign(style, {left: c.LogoMarginX, bottom: c.LogoMarginY}); break;
    case 'bottom-right': Object.assign(style, {right: c.LogoMarginX, bottom: c.LogoMarginY}); break;
    case 'center': Object.assign(style, {left: '50%', top: '50%', transform: 'translate(-50%, -50%)'}); break;
    default: Object.assign(style, {right: c.LogoMarginX, top: c.LogoMarginY});
  }
  return <Img src={c.LogoFile} style={style}/>;
};

const ClosingLogo = ({plan, time, start, duration, width, height, above = false}) => {
  const layout = closingLogoLayout(plan, width, height);
  if (!layout) return null;
  const style = {width: layout.width, height: layout.height, margin: `0 auto ${layout.gap}px`, opacity: closingLogoOpacity(time, start, duration)};
  if (above) Object.assign(style, {position: 'absolute', bottom: '100%', left: '50%', transform: 'translateX(-50%)'});
  return <div style={style}><Img src={plan.Config.LogoFile} style={{width: '100%', height: '100%', objectFit: 'contain', filter: logoShadow(plan)}}/></div>;
};

const Scripture = ({plan, time}) => {
  const {fps} = useVideoConfig();
  const slides = plan.Slides?.filter((s) => time >= s.Start && time < s.End) ?? [];
  if (!slides.length) return null;
  return <svg width={plan.Config.Width} height={plan.Config.Height} style={{position: 'absolute', overflow: 'visible'}}>
    {slides.map((slide) => <g key={slide.Start}>{slide.Words.map((word, i) => {
      const state = wordState(time, word, slide, fps);
      const body = word.Kind === 'content';
      const color = word.Kind === 'label' ? plan.Theme.TimerColor : body ? plan.Theme.TitleColor : plan.Theme.SubtitleColor;
      return <text key={i} x={word.X} y={word.Y + state.y} fontSize={word.Size} fontFamily={`Countdown-${body ? 'content' : 'subtitle'}`} fill={color} opacity={state.opacity}
        style={{filter: state.blur > 0.001 ? `blur(${state.blur}px)` : undefined, whiteSpace: 'pre', fontVariantLigatures: 'none', fontKerning: 'normal', textShadow: '0 2px 12px rgba(0,0,0,0.25)'}}>{word.Text}</text>;
    })}</g>)}
  </svg>;
};

const Progress = ({plan, time}) => {
  const c = plan.Config;
  const state = progressState(plan, time);
  if (!state) return null;
  const top = (c.BarHeight - state.thickness) / 2;
  const color = plan.Theme.ProgressColor;
  const line = {position: 'absolute', top, height: state.thickness, borderRadius: state.thickness / 2};
  return <div style={{position: 'absolute', left: c.BarX, top: c.BarY, width: c.BarWidth, height: c.BarHeight, overflow: 'hidden', opacity: state.opacity}}>
    <div style={{...line, width: '100%', background: cssColor(plan.Theme.ProgressBackColor), opacity: 0.4}}/>
    <div style={{...line, width: state.fillWidth, background: `linear-gradient(90deg, ${cssColor(color + '@0.72')}, ${color})`}}/>
    <div style={{position: 'absolute', left: state.fillWidth, top: '50%', width: state.glowWidth, height: state.glowHeight, transform: 'translate(-50%, -50%)', opacity: state.glowOpacity, background: `radial-gradient(ellipse, ${cssColor(color + '@0.32')} 0%, ${cssColor(color + '@0.1')} 40%, transparent 72%)`}}/>
  </div>;
};

const ClosingGreeting = ({plan}) => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();
  const c = plan.Config;
  const scene = outroScene(plan, fps);
  const mode = c.OutroMode.toLowerCase();
  const titleState = outroElementState(frame, fps, scene.start, scene.titleDuration, scene.titleSize, mode);
  const subtitle = outroElementState(frame, fps, scene.subtitleStart, scene.subtitleDuration, scene.subtitleSize);
  const rule = outroElementState(frame, fps, scene.ruleStart, scene.ruleDuration, 1);
  const letters = Math.floor(clamp((frame - scene.start) / scene.titleDuration) * Array.from(c.Title).length);
  let wordIndex = 0;
  const textStyle = (state) => ({
    display: 'inline-block', maxWidth: '100%', opacity: state.opacity,
    transform: `translateY(${state.y}px) scale(${state.scale})`,
    filter: state.blur > 0.001 ? `blur(${state.blur}px)` : undefined,
  });
  return <AbsoluteFill style={{justifyContent: 'center', alignItems: 'center', padding: scene.padding, boxSizing: 'border-box', textAlign: 'center'}}>
    <div style={{width: '100%'}}>
      <ClosingLogo plan={plan} time={frame / fps} start={scene.start / fps} duration={scene.titleDuration / fps} width={c.Width - scene.padding * 2} height={Math.max(0, c.Height - scene.padding * 2) * 0.25}/>
      {c.Title && <div style={{fontFamily: 'Countdown-title', fontSize: scene.titleSize, lineHeight: 1.12, color: plan.Theme.TitleColor, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', textShadow: '0 2px 16px rgba(0,0,0,0.22)'}}>
        {mode === 'typewriter' ? <div style={{position: 'relative', ...textStyle(titleState), display: 'block'}}>
          {Array.from(c.Title).map((character, i) => <span key={i} style={{visibility: i < letters ? 'visible' : 'hidden'}}>{character}</span>)}
        </div> : c.Title.split(/(\s+)/).map((token, i) => {
          if (/^\s*$/.test(token)) return <React.Fragment key={i}>{token}</React.Fragment>;
          const state = outroElementState(frame, fps, scene.start + wordIndex++ * scene.wordDelay, scene.titleDuration, scene.titleSize, mode);
          return <span key={i} style={{display: 'inline-block', maxWidth: '100%', verticalAlign: 'bottom', overflow: 'hidden', padding: '0.12em 0.06em', margin: '-0.12em -0.06em'}}>
            <span style={textStyle(state)}>{token}</span>
          </span>;
        })}
      </div>}
      {c.Title && <div style={{width: scene.ruleWidth, height: scene.ruleHeight, margin: `${scene.subtitleSize * 0.72}px auto ${scene.subtitleSize * 0.65}px`, background: `linear-gradient(90deg, transparent, ${plan.Theme.ProgressColor}, transparent)`, transform: `scaleX(${rule.progress})`, transformOrigin: 'center', opacity: rule.opacity * 0.5}}/>}
      {c.Subtitle && <div style={{fontFamily: 'Countdown-subtitle', fontSize: scene.subtitleSize, lineHeight: 1.3, color: plan.Theme.SubtitleColor, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', opacity: subtitle.opacity, transform: `translateY(${subtitle.y}px)`, filter: subtitle.blur > 0.001 ? `blur(${subtitle.blur}px)` : undefined, textShadow: '0 2px 12px rgba(0,0,0,0.2)'}}>{c.Subtitle}</div>}
    </div>
  </AbsoluteFill>;
};

const Outro = ({plan, time}) => {
  const c = plan.Config;
  if (c.OutroMode.toLowerCase() === 'none' || time < plan.OutroStart) return null;
  if ((plan.Slides?.length ?? 0) > 0) return <ClosingGreeting plan={plan}/>;
  const titleStart = plan.OutroStart + plan.Timing.TitleDelay;
  const subtitleStart = titleStart + plan.Timing.SubtitleDelay;
  const titleProgress = reveal(time, titleStart, plan.Timing.TitleFade);
  const subtitleProgress = reveal(time, subtitleStart, plan.Timing.SubtitleFade);
  const left = Math.max(c.SafeArea, c.Width * 0.075);
  const right = Math.max(c.SafeArea, c.Width * 0.075);
  const width = c.Width - left - right;
  const top = c.Height * 0.38;
  const lift = c.OutroMode.toLowerCase() === 'slide' ? (1 - titleProgress) * c.OutroTitleSize * 0.4 : 0;
  const title = Array.from(c.Title);
  const letters = c.OutroMode.toLowerCase() === 'typewriter' ? Math.floor(clamp((time - titleStart) / plan.Timing.TitleFade) * title.length) : title.length;
  return <div style={{position: 'absolute', left, top, width, textAlign: 'center'}}>
    <ClosingLogo plan={plan} time={time} start={titleStart} duration={plan.Timing.TitleFade} width={width} height={Math.max(0, top - Math.max(c.SafeArea, c.Height * 0.075))} above/>
    {c.Title && <div style={{position: 'relative', fontFamily: 'Countdown-title', fontSize: c.OutroTitleSize, lineHeight: 1.05, color: plan.Theme.TitleColor, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', opacity: titleProgress, transform: `translateY(${lift}px)`, textShadow: '0 3px 20px rgba(0,0,0,0.35)'}}>
      {/* Keep the final text in the layout so typing does not recenter each frame. */}
      <span style={{visibility: 'hidden'}}>{c.Title}</span>
      <span style={{position: 'absolute', inset: 0}}>{title.slice(0, letters).join('')}</span>
    </div>}
    {c.Subtitle && <div style={{marginTop: c.OutroSubtitleSize * 0.7, fontFamily: 'Countdown-subtitle', fontSize: c.OutroSubtitleSize, lineHeight: 1.3, color: plan.Theme.SubtitleColor, opacity: subtitleProgress, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', textShadow: '0 2px 12px rgba(0,0,0,0.35)'}}>{c.Subtitle}</div>}
  </div>;
};

const Sound = ({plan}) => {
  const c = plan.Config;
  const {fps} = useVideoConfig();
  const frame = useCurrentFrame();
  if (c.Preview || plan.Mode === 'check') return null;
  const endStart = Math.max(0, Math.round((c.Duration - 1) * fps));
  return <>
    {c.AudioFile && <Loop durationInFrames={plan.AudioFrames}>
      <Audio src={c.AudioFile} allowAmplificationDuringRender volume={c.AudioVolume * (c.FadeAudio ? 1 - clamp((frame / fps - plan.OutroStart) / c.OutroSeconds) : 1)}/>
    </Loop>}
    {c.EndSoundFile && endStart < plan.Frames && <Sequence from={endStart} durationInFrames={Math.min(Math.round(3 * fps), plan.Frames - endStart)} layout="none">
      <Audio src={c.EndSoundFile}/>
    </Sequence>}
  </>;
};

const Countdown = ({plan}) => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();
  const time = frame / fps;
  return <Fonts fonts={plan.Fonts}><AbsoluteFill style={{overflow: 'hidden'}}>
    <Background plan={plan} time={time}/>
    <Logo plan={plan} time={time}/>
    <Timer plan={plan} time={time}/>
    <Scripture plan={plan} time={time}/>
    <Progress plan={plan} time={time}/>
    <Outro plan={plan} time={time}/>
    <Sound plan={plan}/>
  </AbsoluteFill></Fonts>;
};

const Root = () => <Composition id="Countdown" component={Countdown} width={1280} height={720} fps={30} durationInFrames={9000}
  calculateMetadata={({props}) => ({width: props.plan.Config.Width, height: props.plan.Config.Height, fps: props.plan.Config.FPS, durationInFrames: props.plan.Frames})}/>;
registerRoot(Root);
