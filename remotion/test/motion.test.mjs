import {test} from 'node:test';
import assert from 'node:assert/strict';
import {backgroundLights, countdown, cssColor, ease, timerOpacity, wordState, withEncoder} from '../src/motion.mjs';

test('background lights repeat exactly, remain restrained, and can be frozen', () => {
  assert.deepEqual(backgroundLights(128), backgroundLights(128));
  assert.notDeepEqual(backgroundLights(0), backgroundLights(60));
  assert.deepEqual(backgroundLights(128, 'none'), backgroundLights(0, 'none'));
  for (let time = 0; time <= 600; time += 0.5) {
    const now = backgroundLights(time);
    const next = backgroundLights(time + 1 / 30);
    for (const key of ['primary', 'secondary', 'glow']) {
      for (const axis of ['x', 'y']) {
        assert.ok(now[key][axis] >= 15 && now[key][axis] <= 85);
        assert.ok(Math.abs(next[key][axis] - now[key][axis]) < 0.02, 'light movement was too fast');
      }
    }
  }
});

test('crossfade renders both layers without a jump at a verse/statement boundary', () => {
  const outgoing = {Start: 0, End: 30.6, ExitSeconds: 1.2, EnterSeconds: 0, Style: 'soft-dissolve'};
  const incoming = {Start: 29.4, End: 40, ExitSeconds: 0.6, EnterSeconds: 1.2, Style: 'soft-dissolve'};
  const oldWord = {Size: 42, Appear: 3, Fade: 1.2};
  const newWord = {Size: 42, Appear: 29.4, Fade: 0};
  for (const time of [29.4, 29.7, 30, 30.3, 30.6]) {
    const previous = wordState(time, oldWord, outgoing);
    const next = wordState(time, newWord, incoming);
    assert.ok(Math.abs(previous.opacity + next.opacity - 1) < 1e-10);
    assert.equal(previous.y, 0);
    assert.equal(next.y, 0);
  }
  assert.ok(Math.abs(wordState(30, oldWord, outgoing).opacity - 0.5) < 1e-10);
  assert.deepEqual(wordState(39.9, newWord, {...incoming, ExitSeconds: 0}), {opacity: 1, y: 0, blur: 0});
});

test('calm dissolve avoids competing sentences and settles completely before reading', () => {
  const outgoing = {Start: 0, End: 30.6, ExitSeconds: 1.2, CrossfadeOut: true, Style: 'soft-dissolve'};
  const incoming = {Start: 29.4, End: 40, ExitSeconds: 0.6, EnterSeconds: 1.2, CrossfadeIn: true, Style: 'soft-dissolve'};
  const oldWord = {Size: 42, Appear: 3, Fade: 1.2};
  const newWord = {Size: 42, Appear: 29.4, Fade: 0};
  for (let i = 0; i <= 120; i++) {
    const time = 29.4 + i / 100;
    const previous = wordState(time, oldWord, outgoing);
    const next = wordState(time, newWord, incoming);
    assert.ok(previous.opacity * next.opacity < 0.01, 'two sentences competed during the dissolve');
    assert.ok(previous.opacity + next.opacity > 0.14, 'dissolve had a hard blank gap');
  }
  assert.deepEqual(wordState(31, newWord, incoming), {opacity: 1, y: 0, blur: 0});
  assert.deepEqual(wordState(32, newWord, incoming), wordState(35, newWord, incoming));
});

test('word motion is frame-derived, soft, and completely still during the reading hold', () => {
  const word = {Size: 42, Appear: 3, Fade: 1.2};
  const slide = {Start: 0, End: 30, Style: 'gentle-lift'};
  assert.deepEqual(wordState(2, word, slide), {opacity: 0, y: 6.72, blur: 1.4700000000000002});
  const middle = wordState(3.6, word, slide);
  assert.ok(Math.abs(middle.opacity - 0.5) < 1e-10);
  assert.ok(middle.y > 0 && middle.y < 6.72);
  assert.deepEqual(wordState(20, word, slide), {opacity: 1, y: 0, blur: 0});
  assert.deepEqual(wordState(20, word, slide), wordState(24, word, slide));
  assert.equal(wordState(3.6, word, {...slide, Style: 'soft-dissolve'}).y, 0);
  assert.equal(wordState(30, word, slide).opacity, 0);
  // The ease has zero velocity at both ends, avoiding abrupt starts/stops.
  assert.ok(ease(0.001) < 0.00001);
  assert.ok(1 - ease(0.999) < 0.00001);
});

test('opening scripture words settle gently on first and subsequent slides', () => {
  for (const slide of [
    {Start: 0, End: 10, Type: 'scripture-reveal', Style: 'gentle-lift'},
    {Start: 9.4, End: 20, EnterSeconds: 1.2, CrossfadeIn: true, Type: 'scripture-reveal', Style: 'gentle-lift'},
  ]) {
    const word = {Size: 42, Appear: slide.Start, Fade: 2, Initial: true, Kind: 'content'};
    assert.equal(wordState(slide.Start, word, slide).opacity, 0);
    let previous = wordState(slide.Start, word, slide);
    for (let frame = 1; frame <= 60; frame++) {
      const state = wordState(slide.Start + frame / 30, word, slide);
      assert.ok(state.opacity >= previous.opacity && state.opacity - previous.opacity < 0.055, 'opening words popped into view');
      assert.ok(state.y <= previous.y && state.blur <= previous.blur, 'opening motion did not settle smoothly');
      previous = state;
    }
    assert.deepEqual(wordState(slide.Start + 2.1, word, slide), {opacity: 1, y: 0, blur: 0});
    assert.deepEqual(wordState(slide.Start + 3, word, slide), wordState(slide.Start + 5, word, slide));
  }
});

test('timer handles boundaries without negative values', () => {
  assert.equal(countdown(300, 0), '05:00');
  assert.equal(countdown(300, 0.5), '04:59');
  assert.equal(countdown(60, 1 / 30), '00:59');
  assert.equal(countdown(1, 2), '00:00');
});

test('service timer fades out before 00:02 in every outro mode', () => {
  for (const mode of ['fade', 'slide', 'typewriter', 'none']) {
    for (const extended of [false, true]) {
      const plan = {
        Config: {Duration: 33, OutroMode: mode, OutroOverDuration: extended},
        Slides: [{}], OutroStart: extended ? 33 : 30, Timing: {ControlsFade: 0.6},
      };
      assert.equal(timerOpacity(plan, 29), 1);
      assert.equal(timerOpacity(plan, 29.5), 0.5);
      assert.equal(timerOpacity(plan, 30), 0);
      for (const fps of [24, 30, 60]) {
        let previous = 1;
        for (let frame = 29 * fps; frame <= 36 * fps; frame++) {
          const time = frame / fps;
          const opacity = timerOpacity(plan, time);
          assert.ok(opacity <= previous && previous - opacity < 0.07, 'timer vanished abruptly');
          if (['00:02', '00:01', '00:00'].includes(countdown(33, time))) {
            assert.equal(opacity, 0, 'service timer remained visible in the ending');
          }
          previous = opacity;
        }
      }
    }
  }
  const short = {Config: {Duration: 2.5, OutroMode: 'none'}, Slides: [{}]};
  assert.equal(timerOpacity(short, 0), 0);
});

test('plain countdowns retain their existing outro timing', () => {
  const plan = {Config: {Duration: 33, OutroMode: 'none'}, Slides: [], OutroStart: 30, Timing: {ControlsFade: 0.6}};
  assert.equal(timerOpacity(plan, 32.5), 1);
  plan.Config.OutroMode = 'fade';
  assert.equal(timerOpacity(plan, 30), 1);
  assert.ok(Math.abs(timerOpacity(plan, 30.3) - 0.5) < 1e-10);
  assert.equal(timerOpacity(plan, 31), 0);
});

test('encoder adapter preserves inputs, audio mixing and output while replacing video settings', () => {
  const args = ['-y', '-framerate', '30', '-i', 'frames-%05d.jpeg', '-filter_complex', 'audio graph', '-c:v', 'libx264', '-crf', '18', '-preset', 'medium', '-c:a', 'aac', '-b:a', '192k', '-pix_fmt', 'yuv420p', 'out.mp4'];
  const hardware = withEncoder(args, {Args: ['-c:v', 'h264_videotoolbox', '-allow_sw', '1', '-b:v', '2M']});
  assert.equal(hardware.at(-1), 'out.mp4');
  assert.ok(hardware.includes('audio graph') && hardware.includes('aac'));
  assert.ok(!hardware.includes('libx264') && !hardware.includes('-crf') && !hardware.includes('-preset'));
  assert.equal(hardware.filter((v) => v === '-c:v').length, 1);
  const software = withEncoder(hardware, {Args: ['-c:v', 'libx264', '-preset', 'veryfast', '-crf', '20']});
  assert.ok(!software.includes('-b:v') && !software.includes('-allow_sw'));
  assert.equal(software.at(-1), 'out.mp4');
});

test('timer color alpha survives the FFmpeg-style color input', () => {
  assert.equal(cssColor('#ffdd77'), '#ffdd77');
  assert.equal(cssColor('white@0.5'), 'color-mix(in srgb, white 50%, transparent)');
});
