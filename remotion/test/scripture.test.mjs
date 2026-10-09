import {test} from 'node:test';
import assert from 'node:assert/strict';
import {wordState} from '../src/motion.mjs';

test('scripture focus reveals stay restrained, repeat exactly, and settle by the scheduled hold', () => {
  for (const fps of [24, 30, 60]) {
    for (const fade of [0.08, 1.2, 2]) {
      for (const size of [24, 42, 126]) {
        const word = {Kind: 'content', Size: size, Appear: 3.17, Fade: fade};
        const slide = {Type: 'scripture-reveal', Style: 'gentle-lift', Start: 0, End: 20};
        assert.equal(wordState(word.Appear, word, slide, fps).opacity, 0);
        let previous = wordState(word.Appear, word, slide, fps);
        for (let frame = 1; frame <= Math.ceil(fade * fps); frame++) {
          const time = word.Appear + frame / fps;
          const state = wordState(time, word, slide, fps);
          assert.deepEqual(state, wordState(time, word, slide, fps));
          assert.ok(state.y >= 0 && state.y <= 2.5, 'scripture moved too far');
          assert.ok(state.y <= previous.y && state.blur <= previous.blur, 'spring bounced or lost focus');
          assert.ok(state.opacity >= previous.opacity && state.opacity <= 1);
          if (fade >= 1.2) assert.ok(state.opacity - previous.opacity < 0.06, 'word appeared too abruptly');
          previous = state;
        }
        const hold = wordState(word.Appear + fade, word, slide, fps);
        assert.deepEqual(hold, {opacity: 1, y: 0, blur: 0});
        assert.deepEqual(hold, wordState(12, word, slide, fps));
      }
    }
  }
});

test('soft dissolve and scripture metadata keep their exact sentence positions', () => {
  const word = {Kind: 'content', Size: 42, Appear: 3, Fade: 1.2};
  const slide = {Type: 'scripture-reveal', Style: 'soft-dissolve', Start: 0, End: 20};
  assert.equal(wordState(3.4, word, slide, 30).y, 0);
  assert.ok(wordState(3.4, word, slide, 30).blur > 0);
  for (const kind of ['label', 'reference']) {
    assert.equal(wordState(3.4, {...word, Kind: kind}, {...slide, Style: 'gentle-lift'}, 30).y, 0);
  }
});
