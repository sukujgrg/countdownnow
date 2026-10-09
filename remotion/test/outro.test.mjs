import {test} from 'node:test';
import assert from 'node:assert/strict';
import {outroElementState, outroScene} from '../src/outro.mjs';
import {timerOpacity} from '../src/motion.mjs';

test('closing springs remain restrained, repeat exactly and come fully to rest', () => {
  for (const fps of [24, 30, 60]) {
    for (const mode of ['fade', 'slide', 'typewriter']) {
      const duration = Math.round(1.15 * fps);
      let previous = outroElementState(0, fps, 0, duration, 80, mode);
      assert.equal(previous.opacity, 0);
      for (let frame = 1; frame <= duration; frame++) {
        const state = outroElementState(frame, fps, 0, duration, 80, mode);
        assert.deepEqual(state, outroElementState(frame, fps, 0, duration, 80, mode));
        assert.ok(state.progress >= previous.progress && state.progress <= 1, 'spring bounced or overshot');
        assert.ok(state.opacity >= previous.opacity && state.opacity - previous.opacity < 0.15, 'title popped into view');
        assert.ok(state.scale >= 0.985 && state.scale <= 1, 'greeting zoomed too far');
        assert.ok(state.y <= previous.y && state.blur <= previous.blur);
        previous = state;
      }
      assert.deepEqual(outroElementState(duration, fps, 0, duration, 80, mode), {progress: 1, opacity: 1, y: 0, blur: 0, scale: 1});
      assert.deepEqual(outroElementState(duration + 5, fps, 0, duration, 80, mode), previous);
    }
  }
});

test('title, accent and subtitle finish before the closing hold in every format', () => {
  for (const [width, height] of [[1280, 720], [1080, 1920], [1080, 1080], [2560, 1080]]) {
    for (const mode of ['fade', 'slide', 'typewriter']) {
      for (const extended of [false, true]) {
        const fps = 30;
        const plan = {
          Config: {Width: width, Height: height, SafeArea: 40, Duration: 33, OutroSeconds: 3, OutroTitleSize: 46, OutroSubtitleSize: 28, Title: 'Welcome to our church family', OutroMode: mode, OutroOverDuration: extended},
          Slides: [{}], Frames: (extended ? 36 : 33) * fps, OutroStart: extended ? 33 : 30,
          Timing: {TitleDelay: 0.66, ControlsFade: 0.54},
        };
        const scene = outroScene(plan, fps);
        assert.equal(timerOpacity(plan, scene.start / fps), 0, 'timer competed with the closing scene');
        assert.ok(scene.subtitleStart > scene.start);
        assert.ok(scene.wordDelay > 0);
        for (const [start, duration] of [
          [scene.start + 4 * scene.wordDelay, scene.titleDuration],
          [scene.subtitleStart, scene.subtitleDuration], [scene.ruleStart, scene.ruleDuration],
        ]) {
          assert.ok(start + duration < plan.Frames - 10, 'entrance used up the quiet reading hold');
          assert.equal(outroElementState(plan.Frames - 10, fps, start, duration, 80, mode).progress, 1);
        }
        assert.ok(scene.padding >= 40 && scene.titleSize > 46);
      }
    }
  }
});

test('short and long endings preserve timing, and disabled outros stay disabled', () => {
  const fps = 30;
  for (const outro of [0.2, 1, 10]) {
    const plan = {
      Config: {Width: 1280, Height: 720, SafeArea: 0, Duration: 33, OutroSeconds: outro, OutroTitleSize: 46, OutroSubtitleSize: 28, Title: 'Welcome', OutroMode: 'fade'},
      Slides: [{}], Frames: 33 * fps, OutroStart: 33 - outro,
      Timing: {TitleDelay: outro * 0.2, ControlsFade: outro * 0.18},
    };
    const scene = outroScene(plan, fps);
    assert.equal(timerOpacity(plan, scene.start / fps), 0);
    assert.ok(scene.start + scene.titleDuration <= plan.Frames - 1);
    assert.ok(scene.subtitleStart + scene.subtitleDuration <= plan.Frames - 1);
    assert.equal(outroScene({...plan, Slides: []}, fps), null);
    assert.equal(outroScene({...plan, Config: {...plan.Config, OutroMode: 'none'}}, fps), null);
  }
});
