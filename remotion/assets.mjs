import {createServer} from 'node:http';
import {createReadStream} from 'node:fs';
import {stat} from 'node:fs/promises';
import {extname} from 'node:path';
import {randomUUID} from 'node:crypto';

const mime = {'.png': 'image/png', '.apng': 'image/png', '.avif': 'image/avif', '.bmp': 'image/bmp', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.webp': 'image/webp', '.gif': 'image/gif', '.svg': 'image/svg+xml', '.mp4': 'video/mp4', '.mov': 'video/quicktime', '.webm': 'video/webm', '.mp3': 'audio/mpeg', '.wav': 'audio/wav', '.m4a': 'audio/mp4', '.aac': 'audio/aac', '.ogg': 'audio/ogg', '.flac': 'audio/flac'};

// Serve only the explicitly configured assets, with range requests for media.
// Files stay local and are neither copied into a bundle nor sent to a service.
export const serveAssets = async (config) => {
  const routes = new Map();
  const mapped = {...config};
  const keys = ['Background', 'LogoFile', 'AudioFile', 'EndSoundFile'];
  for (const key of keys) {
    if (!config[key]) continue;
    const route = `/${randomUUID()}${extname(config[key]).toLowerCase()}`;
    routes.set(route, config[key]);
    mapped[key] = route;
  }
  const server = createServer(async (req, res) => {
    const path = routes.get(req.url);
    if (!path || !['GET', 'HEAD'].includes(req.method)) { res.writeHead(404).end(); return; }
    try {
      const info = await stat(path);
      const headers = {'Content-Type': mime[extname(path).toLowerCase()] || 'application/octet-stream', 'Access-Control-Allow-Origin': '*', 'Accept-Ranges': 'bytes'};
      let start = 0;
      let end = info.size - 1;
      if (req.headers.range) {
        const match = /^bytes=(\d*)-(\d*)$/.exec(req.headers.range);
        if (!match || (!match[1] && !match[2])) { res.writeHead(416, {'Content-Range': `bytes */${info.size}`}).end(); return; }
        if (!match[1]) start = Math.max(0, info.size - Number(match[2]));
        else { start = Number(match[1]); if (match[2]) end = Math.min(end, Number(match[2])); }
        if (start > end || start >= info.size) { res.writeHead(416, {'Content-Range': `bytes */${info.size}`}).end(); return; }
        headers['Content-Range'] = `bytes ${start}-${end}/${info.size}`;
      }
      headers['Content-Length'] = end - start + 1;
      res.writeHead(req.headers.range ? 206 : 200, headers);
      if (req.method === 'HEAD') { res.end(); return; }
      const stream = createReadStream(path, {start, end});
      stream.on('error', () => res.destroy());
      res.on('close', () => stream.destroy());
      stream.pipe(res);
    } catch { res.writeHead(404).end(); }
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  for (const key of keys) if (mapped[key]) mapped[key] = origin + mapped[key];
  return {config: mapped, close: () => new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); })};
};
