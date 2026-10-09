import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp, writeFile, rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {serveAssets} from '../assets.mjs';

test('asset server serves only allowed paths, supports seeking, and handles punctuation in paths', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'countdown-assets-test-'));
  const path = join(dir, "an image 'with' [spaces],;.png");
  await writeFile(path, '0123456789');
  const server = await serveAssets({Background: path, AudioFile: '', BackgroundFit: 'contain'});
  try {
    assert.equal(server.config.BackgroundFit, 'contain');
    assert.equal(server.config.AudioFile, '');
    let response = await fetch(server.config.Background, {headers: {Range: 'bytes=2-5'}});
    assert.equal(response.status, 206);
    assert.equal(response.headers.get('content-range'), 'bytes 2-5/10');
    assert.equal(await response.text(), '2345');
    response = await fetch(server.config.Background, {headers: {Range: 'bytes=-3'}});
    assert.equal(await response.text(), '789');
    assert.equal((await fetch(server.config.Background, {headers: {Range: 'bytes=99-'}})).status, 416);
    const origin = new URL(server.config.Background).origin;
    assert.equal((await fetch(origin + '/etc/passwd')).status, 404);
    assert.equal((await fetch(server.config.Background, {method: 'POST'})).status, 404);
    response = await fetch(server.config.Background, {method: 'HEAD'});
    assert.equal(response.headers.get('content-length'), '10');
    assert.equal(await response.text(), '');
  } finally { await server.close(); await rm(dir, {recursive: true, force: true}); }
});
