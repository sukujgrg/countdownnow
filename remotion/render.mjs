import {readFile, mkdtemp, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {dirname, extname, join} from 'node:path';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {fileURLToPath} from 'node:url';
import {bundle} from '@remotion/bundler';
import {ensureBrowser, openBrowser, renderFrames, renderStill, selectComposition, stitchFramesToVideo} from '@remotion/renderer';
import {serveAssets} from './assets.mjs';
import {withEncoder} from './src/motion.mjs';

const run = async () => {
  if (Number(process.versions.node.split('.')[0]) < 22) throw new Error('Countdown rendering requires Node.js 22 or newer.');
  if (!process.argv[2]) throw new Error('Expected a countdown render plan. Use the countdownnow command.');
  const plan = JSON.parse(await readFile(process.argv[2], 'utf8'));
  const dir = dirname(fileURLToPath(import.meta.url));
  const work = await mkdtemp(join(tmpdir(), 'countdown-frames-'));
  let assets, browser;
  try {
    assets = await serveAssets(plan.Config);
    plan.Config = assets.config;
    const browserExecutable = plan.Config.RemotionBrowser || undefined;
    console.error('Preparing Remotion scene…');
    await ensureBrowser({browserExecutable});
    const serveUrl = await bundle({entryPoint: join(dir, 'src/index.jsx'), rootDir: dir, publicDir: null, outDir: join(work, 'bundle'), enableCaching: false});
    browser = await openBrowser('chrome', {browserExecutable});
    const common = {serveUrl, inputProps: {plan}, puppeteerInstance: browser};
    const composition = await selectComposition({...common, id: 'Countdown'});
    if (plan.Config.Preview || plan.Mode === 'check') {
      const frame = Math.min(plan.Frames - 1, Math.max(0, Math.round(plan.Config.PreviewTime * composition.fps)));
      const format = {'.png': 'png', '.jpg': 'jpeg', '.jpeg': 'jpeg', '.webp': 'webp'}[extname(plan.Config.Preview).toLowerCase()];
      const output = format ? plan.Config.Preview : join(work, 'preview.png');
      await renderStill({...common, composition, frame, imageFormat: format || 'png', ...(format === 'jpeg' ? {jpegQuality: 95} : {}), output});
      // Preserve the legacy CLI's other still formats through image encoding.
      if (!format) await promisify(execFile)('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-i', output, '-frames:v', '1', plan.Config.Preview]);
      if (plan.Mode !== 'check') return;
    }
    const check = plan.Mode === 'check';
    let lastPercent = -10;
    const {assetsInfo} = await renderFrames({...common, composition, outputDir: join(work, 'frames'), imageFormat: 'jpeg', jpegQuality: 95,
      concurrency: plan.Config.RemotionConcurrency, frameRange: check ? [0, Math.min(1, plan.Frames - 1)] : null,
      onStart: ({frameCount}) => console.error(`Rendering ${frameCount} frames…`),
      onFrameUpdate: (count) => {
        const percent = Math.floor(count / plan.Frames * 100);
        if (!check && (percent >= lastPercent + 10 || count === plan.Frames)) { console.error(`Frames: ${percent}%`); lastPercent = percent; }
      }});
    const encoders = check ? plan.Encoders.filter((e) => e.Name === 'libx264') : plan.Encoders;
    // A user-selected hardware encoder still gets a software capability check.
    if (check && !encoders.length) encoders.push({Name: 'libx264', Args: ['-c:v', 'libx264', '-preset', plan.Config.SoftwareSpeed, '-crf', String(plan.Config.SoftwareCRF)]});
    let lastError;
    for (const encoder of encoders) {
      try {
        console.error(`Encoding: ${encoder.Name}`);
        await stitchFramesToVideo({assetsInfo, fps: composition.fps, width: composition.width, height: composition.height,
          codec: 'h264', pixelFormat: 'yuv420p', audioCodec: 'aac', audioBitrate: '192k', force: true,
          outputLocation: check ? join(work, 'check.mp4') : plan.Config.Output,
          ffmpegOverride: ({args}) => withEncoder(args, encoder)});
        if (check) console.log('Remotion: fonts, layouts, browser rendering and software encoding passed.');
        return;
      } catch (error) {
        lastError = error;
        console.error(`Encoder ${encoder.Name} failed: ${error.message}`);
        // Frames are retained while trying the next encoder; no second render.
      }
    }
    throw lastError || new Error('No encoders configured.');
  } finally {
    try { if (browser) await browser.close({silent: true}); }
    finally {
      try { if (assets) await assets.close(); }
      finally { await rm(work, {recursive: true, force: true}); }
    }
  }
};

run().catch((error) => { console.error(error.stack || error.message); process.exitCode = 1; });
