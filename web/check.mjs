import { readdir, readFile, stat } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const dist = join(root, 'public');
const errors = [];
const htmlPages = [];

async function collect(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) await collect(full);
    else if (entry.name.endsWith('.html')) htmlPages.push(full);
  }
}

await collect(dist);
const pageCache = new Map();
for (const page of htmlPages) pageCache.set(page, await readFile(page, 'utf8'));

async function targetExists(value) {
  try { return (await stat(value)).isFile(); } catch { return false; }
}

for (const [page, contents] of pageCache) {
  if (!contents.includes('<title>') || !contents.includes('name="description"')) errors.push(`Missing metadata: ${page}`);
  const localIds = new Set([...contents.matchAll(/\bid="([^"]+)"/g)].map((match) => match[1]));
  for (const match of contents.matchAll(/\bhref="([^"]+)"/g)) {
    const raw = match[1].replaceAll('&amp;', '&');
    if (raw.startsWith('https:') || raw.startsWith('http:') || raw.startsWith('mailto:') || raw.startsWith('tel:')) continue;
    const url = new URL(raw, 'https://dm-dot.example' + '/' + page.slice(dist.length + 1));
    if (url.origin !== 'https://dm-dot.example') continue;
    const file = url.pathname.endsWith('/') ? join(dist, url.pathname, 'index.html') : join(dist, url.pathname);
    const targetFile = (await targetExists(file)) ? file : (await targetExists(file + '/index.html') ? file + '/index.html' : null);
    if (!targetFile) {
      errors.push(`Broken link ${raw} from ${page.slice(dist.length + 1)}`);
      continue;
    }
    if (url.hash && url.hash.length > 1 && targetFile.endsWith('.html')) {
      const ids = targetFile === page ? localIds : new Set([...(pageCache.get(targetFile) || '').matchAll(/\bid="([^"]+)"/g)].map((m) => m[1]));
      if (!ids.has(decodeURIComponent(url.hash.slice(1)))) errors.push(`Missing anchor ${raw} from ${page.slice(dist.length + 1)}`);
    }
  }
}

if (errors.length) {
  console.error(errors.join('\n'));
  process.exit(1);
}
console.log(`Validated ${htmlPages.length} HTML pages, local links and anchor targets.`);
