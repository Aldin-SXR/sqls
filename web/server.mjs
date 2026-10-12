import { createServer } from 'node:http';
import { readFile, mkdtemp, writeFile, rm } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve, extname, sep } from 'node:path';
import { tmpdir } from 'node:os';
import { spawn, execFileSync } from 'node:child_process';
import { WebSocketServer, WebSocket } from 'ws';

const webRoot = dirname(fileURLToPath(import.meta.url));
const root = resolve(webRoot, '..');
const binary = resolve(process.env.SQLS_BINARY || resolve(root, 'sqls'));
if (!existsSync(binary)) throw new Error('Build sqls first: go build -o sqls .');
const temp = await mkdtemp(resolve(tmpdir(), 'sqls-web-'));
const db = resolve(temp, 'fixture.sqlite');
execFileSync('python3', ['-c',
  'import sys; sys.path.insert(0, sys.argv[1]); from lsp_smoke_test import create_database; create_database(sys.argv[2])',
  resolve(root, 'script'), db]);
const config = resolve(temp, 'config.yml');
await writeFile(config, `connections:\n  - driver: sqlite3\n    dataSourceName: ${JSON.stringify(db)}\n`);
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css',
  '.json': 'application/json', '.ttf': 'font/ttf', '.svg': 'image/svg+xml' };
const vendorRoot = resolve(webRoot, 'node_modules/monaco-editor/min/vs');
const server = createServer(async (req, res) => {
  try {
    const pathname = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
    let file;
    if (pathname.startsWith('/vendor/vs/')) {
      file = resolve(vendorRoot, pathname.slice('/vendor/vs/'.length));
      if (!file.startsWith(vendorRoot + sep)) throw new Error('Invalid path');
    } else {
      const routes = { '/': 'index.html', '/app.js': 'app.js', '/style.css': 'style.css' };
      if (!routes[pathname]) { res.writeHead(404).end(); return; }
      file = resolve(webRoot, routes[pathname]);
    }
    const content = await readFile(file);
    res.writeHead(200, { 'Content-Type': types[extname(file)] || 'application/octet-stream',
      'Cache-Control': 'no-store' });
    res.end(content);
  } catch {
    if (!res.headersSent) res.writeHead(404);
    res.end();
  }
});
const sockets = new WebSocketServer({ noServer: true, maxPayload: 2 * 1024 * 1024 });
server.on('upgrade', (req, socket, head) => {
  // This local harness starts processes and executes SQL against its fixture.
  // Only accept browser connections from the page served by this server.
  if (req.url !== '/lsp' || req.headers.origin !== `http://${req.headers.host}`) {
    socket.end('HTTP/1.1 403 Forbidden\r\n\r\n');
    return;
  }
  sockets.handleUpgrade(req, socket, head, ws => sockets.emit('connection', ws));
});
const children = new Set();
sockets.on('connection', ws => {
  const child = spawn(binary, ['-config', config], { cwd: root, stdio: ['pipe', 'pipe', 'pipe'] });
  children.add(child);
  let buffer = Buffer.alloc(0);
  const send = message => { if (ws.readyState === WebSocket.OPEN) ws.send(message); };
  child.stdout.on('data', chunk => {
    buffer = Buffer.concat([buffer, chunk]);
    while (true) {
      const headerEnd = buffer.indexOf('\r\n\r\n');
      if (headerEnd < 0) return;
      const match = /content-length:\s*(\d+)/i.exec(buffer.subarray(0, headerEnd).toString());
      if (!match) { ws.close(1011, 'Invalid LSP header'); return; }
      const length = Number(match[1]);
      const start = headerEnd + 4;
      if (buffer.length < start + length) return;
      send(buffer.subarray(start, start + length).toString());
      buffer = buffer.subarray(start + length);
    }
  });
  child.stderr.on('data', chunk => send(JSON.stringify({ jsonrpc: '2.0', method: 'sqls/stderr',
    params: { message: chunk.toString() } })));
  child.stdin.on('error', () => ws.close(1011, 'Language server input closed'));
  child.on('error', error => {
    send(JSON.stringify({ jsonrpc: '2.0', method: 'window/showMessage', params: { type: 1, message: error.message } }));
    ws.close(1011, 'Language server failed');
  });
  let killTimer;
  child.on('close', () => {
    clearTimeout(killTimer);
    children.delete(child);
    ws.close();
  });
  ws.on('message', data => {
    try {
      const message = JSON.parse(data.toString());
      if (message.jsonrpc !== '2.0' || typeof message.method !== 'string') throw new Error('Invalid request');
      const body = Buffer.from(JSON.stringify(message));
      child.stdin.write(Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`), body]));
    } catch { ws.close(1003, 'Invalid JSON-RPC'); }
  });
  ws.on('error', () => ws.close());
  ws.on('close', () => {
    child.stdin.end();
    if (child.exitCode === null && child.signalCode === null) {
      killTimer = setTimeout(() => child.kill('SIGKILL'), 2000);
      killTimer.unref();
    }
  });
});

const port = Number(process.env.PORT || 3000);
server.listen(port, '127.0.0.1', () => console.log(`SQLS playground: http://127.0.0.1:${port}`));
async function shutdown() {
  for (const ws of sockets.clients) ws.terminate();
  for (const child of children) child.kill();
  server.close();
  await rm(temp, { recursive: true, force: true });
}
process.once('SIGTERM', shutdown);
process.once('SIGINT', shutdown);
