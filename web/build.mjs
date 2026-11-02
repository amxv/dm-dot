import { readFile, mkdir, rm, writeFile, copyFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import MarkdownIt from 'markdown-it';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(root, 'public');
const documents = [
  { slug: 'getting-started', title: 'Getting started', group: 'Start here', description: 'Connect your dot, subscribe to events, and send the first message.' },
  { slug: 'deployment', title: 'Deployment', group: 'Start here', description: 'Deploy to Vercel with Upstash persistence and secure credentials.' },
  { slug: 'rest-api', title: 'REST API', group: 'Reference', description: 'Authenticate and send messages from any HTTP client.' },
  { slug: 'protocol', title: 'MCP & Events', group: 'Reference', description: 'Tools, event subscription, payloads, and delivery responses.' },
  { slug: 'architecture', title: 'Architecture', group: 'Reference', description: 'Data flow, components, storage, and subscription lifecycle.' },
  { slug: 'security', title: 'Security', group: 'Reference', description: 'Authentication, signatures, trust boundaries, and limitations.' },
  { slug: 'latency', title: 'Measuring latency', group: 'Guides', description: 'Measure webhook acknowledgment and dot execution separately.' },
  { slug: 'troubleshooting', title: 'Troubleshooting', group: 'Guides', description: 'Diagnose event discovery, subscriptions, and webhook failures.' },
  { slug: 'development', title: 'Development', group: 'Guides', description: 'Repository structure, tests, and contributing.' },
];

const escapeHTML = (value) => String(value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
const slugify = (value) => value.toLowerCase().replace(/<[^>]*>/g, '').replace(/[^\p{L}\p{N} -]/gu, '').trim().replace(/\s+/g, '-');

const markdown = new MarkdownIt({ html: false, linkify: true, typographer: true, breaks: false });
const defaultLinkOpen = markdown.renderer.rules.link_open;
markdown.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const hrefIndex = tokens[idx].attrIndex('href');
  if (hrefIndex >= 0) {
    const href = tokens[idx].attrs[hrefIndex][1];
    const match = href.match(/^(?:\.\/)?(?:\.\.\/)?([^/#]+)\.md(#[\w-]+)?$/);
    if (match) {
      const slug = match[1].toLowerCase();
      if (slug === 'readme' && href.startsWith('../')) {
        tokens[idx].attrs[hrefIndex][1] = '/';
      } else if (slug === 'readme') {
        tokens[idx].attrs[hrefIndex][1] = '/reference/';
      } else if (slug === 'contributing') {
        tokens[idx].attrs[hrefIndex][1] = '/reference/development/';
      } else {
        tokens[idx].attrs[hrefIndex][1] = `/reference/${slug}/${match[2] || ''}`;
      }
    }
  }
  return defaultLinkOpen ? defaultLinkOpen(tokens, idx, options, env, self) : self.renderToken(tokens, idx, options);
};

function markdownToPage(source) {
  const tokens = markdown.parse(source, {});
  const headings = [];
  const counts = new Map();
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i];
    if (token.type !== 'heading_open') continue;
    const depth = Number(token.tag.slice(1));
    const label = tokens[i + 1].content;
    const base = slugify(label) || 'section';
    const count = counts.get(base) || 0;
    counts.set(base, count + 1);
    const id = count ? `${base}-${count + 1}` : base;
    token.attrSet('id', id);
    if (depth <= 3) headings.push({ id, label, depth });
  }
  return { body: markdown.renderer.render(tokens, markdown.options, {}), headings };
}

const nav = `<a class="brand" href="/" aria-label="dm-dot home"><span class="brand-symbol" aria-hidden="true"><span></span><i></i><span></span></span><span>dm<span class="brand-separator">·</span>dot</span></a>`;
function referenceSidebar(activeSlug) {
  const groups = [...new Set(documents.map((doc) => doc.group))];
  return `<nav class="reference-nav" aria-label="Reference pages">
    <a href="/reference/" class="reference-overview ${activeSlug === 'overview' ? 'selected' : ''}">Overview <span aria-hidden="true">↗</span></a>
    ${groups.map((group) => `<div class="reference-group"><div class="reference-group-title">${escapeHTML(group)}</div>
      ${documents.filter((doc) => doc.group === group).map((doc) => `<a href="/reference/${doc.slug}/" ${activeSlug === doc.slug ? 'aria-current="page"' : ''} class="reference-link ${activeSlug === doc.slug ? 'selected' : ''}">${escapeHTML(doc.title)}</a>`).join('')}
    </div>`).join('')}
  </nav>`;
}

function pageShell({ title, description, activeSlug, article, headings = [] }) {
  const toc = headings.filter((h) => h.depth === 2 || h.depth === 3);
  return `<!doctype html>
<html lang="en"><head>
  <meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1" />
  <meta name="theme-color" content="#f7f8f4" /><meta name="description" content="${escapeHTML(description)}" />
  <title>${escapeHTML(title)} · dm-dot reference</title>
  <link rel="icon" type="image/svg+xml" href="/icon.svg" />
  <link rel="preconnect" href="https://fonts.googleapis.com" /><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=DM+Sans:wght@400;450;500;550;600;650;700&family=Instrument+Serif:ital@0;1&family=IBM+Plex+Mono:wght@400;500;600&display=swap" rel="stylesheet" />
  <link rel="stylesheet" href="/assets/site.css" /><link rel="stylesheet" href="/assets/reference.css" />
  <script src="/assets/site.js" defer></script>
</head><body class="reference-page">
  <a class="skip-link" href="#main">Skip to main content</a>
  <header class="site-header"><div class="shell header-inner">${nav}<nav class="top-nav" aria-label="Main navigation"><a href="/#how">How it works</a><a class="current" href="/reference/">Reference</a></nav><a class="header-action" href="/#start">Get started <span aria-hidden="true">↗</span></a></div></header>
  <main id="main" class="shell reference-layout">
    <aside class="reference-side"><div class="reference-side-heading"><span class="tiny-label">DOCUMENTATION</span><span class="reference-edition">v0.1</span></div>${referenceSidebar(activeSlug)}</aside>
    <div class="reference-main">
      <details class="mobile-reference-nav"><summary>Reference navigation <span aria-hidden="true">⌄</span></summary>${referenceSidebar(activeSlug)}</details>
      ${article}
      <div class="reference-bottom"><span>Built for a simpler way to connect.</span><a href="/">Back to dm-dot <span aria-hidden="true">↗</span></a></div>
    </div>
    <aside class="reference-toc">${toc.length ? `<div class="tiny-label">ON THIS PAGE</div><nav aria-label="On this page">${toc.map((h) => `<a class="toc-${h.depth}" href="#${h.id}">${escapeHTML(h.label)}</a>`).join('')}</nav>` : ''}<div class="toc-foot">One tool.<br>One dot.<br>One direct line.</div></aside>
  </main>
  <footer class="reference-footer shell"><span>dm·dot</span><span>Made for the message in motion.</span><a href="/reference/security/">Security</a></footer>
</body></html>`;
}

function referenceLanding() {
  return pageShell({ title: 'Reference', description: 'Guides and technical reference for dm-dot.', activeSlug: 'overview', article: `
    <div class="reference-hero"><div class="tiny-label"><span class="tiny-dot"></span> THE REFERENCE</div><h1>Everything you need.<br><em>Nothing you don't.</em></h1>
    <p>From your first message to the details of webhook delivery. Practical guides up front; the protocol beneath.</p></div>
    ${[...new Set(documents.map((doc) => doc.group))].map((group) => `<section class="reference-index-group"><div class="reference-section-head"><span class="tiny-label">${escapeHTML(group.toUpperCase())}</span><span class="section-rule"></span></div><div class="reference-card-grid">${documents.filter((doc) => doc.group === group).map((doc) => `<a class="reference-card" href="/reference/${doc.slug}/"><span class="reference-card-icon">↗</span><h2>${escapeHTML(doc.title)}</h2><p>${escapeHTML(doc.description)}</p></a>`).join('')}</div></section>`).join('')}
  ` });
}

async function write(relative, contents) {
  const target = join(dist, relative);
  await mkdir(dirname(target), { recursive: true });
  await writeFile(target, contents);
}

await rm(dist, { recursive: true, force: true });
await mkdir(join(dist, 'assets'), { recursive: true });
const resources = ['site.css', 'reference.css', 'site.js'];
for (const name of resources) await copyFile(join(root, 'web', name), join(dist, 'assets', name));
await copyFile(join(root, 'web/icon.svg'), join(dist, 'icon.svg'));
await write('index.html', await readFile(join(root, 'web/landing.html'), 'utf8'));
await write('reference/index.html', referenceLanding());

for (const doc of documents) {
  const source = await readFile(join(root, 'docs', `${doc.slug}.md`), 'utf8');
  const { body, headings } = markdownToPage(source);
  const article = `<div class="reference-breadcrumb"><a href="/reference/">Reference</a><span>/</span><span>${escapeHTML(doc.group)}</span></div>
    <div class="reference-article-intro"><span class="tiny-label">${escapeHTML(doc.group.toUpperCase())} / ${escapeHTML(doc.title.toUpperCase())}</span><p>${escapeHTML(doc.description)}</p></div>
    <article class="markdown-body">${body}</article>`;
  await write(`reference/${doc.slug}/index.html`, pageShell({ title: doc.title, description: doc.description, activeSlug: doc.slug, article, headings }));
}

console.log(`Built landing page and ${documents.length + 1} reference pages → public/`);
