import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const site = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const repo = path.resolve(site, '..');

// Tests run against the committed workflow and Vercel project configuration.
test('filtered GitHub pushes trigger deployment only for actual site inputs', () => {
  const workflow = readFileSync(path.join(repo, '.github/workflows/docs-deploy.yml'), 'utf8');
  const block = workflow.match(/^  push:\n([\s\S]*?)(?=^  workflow_dispatch:|^permissions:)/m)?.[1];
  assert.ok(block, 'missing push event');
  assert.match(block, /branches: \[main\]/);
  const found = [...block.matchAll(/^      - "([^"]+)"$/gm)].map(m => m[1]);
  assert.deepEqual(found, ["docs/**", ".github/workflows/docs-deploy.yml"]);
  assert.match(workflow, /secrets\.DM_DOT_DOCS_VERCEL_TOKEN/);
  assert.match(workflow, /vars\.DM_DOT_VERCEL_ORG_ID/);
  assert.match(workflow, /vars\.DM_DOT_DOCS_VERCEL_PROJECT_ID/);
  assert.match(workflow, /vercel@65\.1\.0 deploy --prebuilt --prod/);
  assert.match(workflow, /working-directory: \./);
  assert.match(workflow, /prepare-prebuilt\.mjs/);
});

test('the API deploy has its own path filter and project-scoped token', () => {
  const workflow = readFileSync(path.join(repo, '.github/workflows/api-deploy.yml'), 'utf8');
  const block = workflow.match(/^  push:\n([\s\S]*?)(?=^  workflow_dispatch:|^permissions:)/m)?.[1];
  assert.ok(block, 'missing API push event');
  assert.match(block, /branches: \[main\]/);
  const paths = [...block.matchAll(/^      - "([^"]+)"$/gm)].map(m => m[1]);
  assert.ok(paths.includes('api/**'));
  assert.ok(paths.includes('pkg/**'));
  assert.ok(paths.includes('vercel.json'));
  assert.ok(!paths.some(p => p.startsWith('docs/')));
  assert.match(workflow, /secrets\.DM_DOT_API_VERCEL_TOKEN/);
  assert.match(workflow, /vars\.DM_DOT_API_VERCEL_PROJECT_ID/);
  const config = JSON.parse(readFileSync(path.join(repo, 'vercel.json'), 'utf8'));
  assert.equal(config.git.deploymentEnabled, false);
  assert.ok(!Object.hasOwn(config, 'outputDirectory'));
  assert.ok(!Object.hasOwn(config, 'buildCommand'));
});

test('only Actions creates Vercel deployments', () => {
  const config = JSON.parse(readFileSync(path.join(repo, 'docs/vercel.json'), 'utf8'));
  assert.equal(config.git.deploymentEnabled, false);
  assert.equal(Object.hasOwn(config, 'ignoreCommand'), false);
});
