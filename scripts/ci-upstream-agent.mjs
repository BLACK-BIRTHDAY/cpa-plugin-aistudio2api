#!/usr/bin/env node

/**
 * ci-upstream-agent.mjs
 *
 * Automated Upstream Convergence & Verification Agent for:
 * cpa-plugin-aistudio2api (BLACK-BIRTHDAY/cpa-plugin-aistudio2api)
 *
 * Features:
 * - Upstream Git fetch and merge from jianli9158/cpa-plugin-aistudio2api with conflict detection
 * - Agentic Conflict Resolution repair loop (30-turn default) with Cloud Run cold-start resilience
 * - Registry version synchronization (registry.json <-> internal/cpa/config.go PluginVersion)
 * - Verification gate: syncRegistryVersion, gofmt -w ., go vet ./..., CGO -buildmode=c-shared build, go test
 * - CI synchronization runner for GitHub Actions
 *
 * Zero-dependency: Node.js >= 20 built-ins only.
 */

import fs from 'node:fs';
import path from 'node:path';
import cp from 'node:child_process';
import process from 'node:process';
import { pathToFileURL } from 'node:url';

export const UPSTREAM_REPO = 'jianli9158/cpa-plugin-aistudio2api';
export const FORK_AUTHOR = 'BLACK-BIRTHDAY';
export const FORK_REPOSITORY = 'https://github.com/BLACK-BIRTHDAY/cpa-plugin-aistudio2api';
export const DEFAULT_BASE_URL = 'https://antigravity-manager-963915716511.asia-northeast2.run.app/v1';
export const DEFAULT_MODEL = 'gemini-3.8-flash-high';
export const DEFAULT_MAX_TURNS = 30;
export const DEFAULT_RETRY_DELAYS_MS = [10000, 20000, 30000, 45000];
export const DEFAULT_REQUEST_TIMEOUT_MS = 180000;
export const RETRYABLE_HTTP_STATUSES = new Set([408, 429, 500, 502, 503, 504]);

const TEST_PACKAGES = [
  './internal/aistudio',
  './internal/api',
  './internal/config',
  './internal/cpa',
  './internal/imagecodec/...',
  './internal/proxydial',
  './internal/resample',
  './internal/waa'
];

/**
 * Standard OpenAI tools exposed to the agent.
 */
export const AGENT_TOOLS = [
  {
    type: 'function',
    function: {
      name: 'read_file',
      description: 'Read contents of a file in the workspace',
      parameters: {
        type: 'object',
        properties: {
          path: { type: 'string', description: 'Relative or absolute file path to read' }
        },
        required: ['path']
      }
    }
  },
  {
    type: 'function',
    function: {
      name: 'write_file',
      description: 'Write or overwrite a file in the workspace',
      parameters: {
        type: 'object',
        properties: {
          path: { type: 'string', description: 'Relative or absolute file path to write' },
          content: { type: 'string', description: 'Full file content to write' }
        },
        required: ['path', 'content']
      }
    }
  },
  {
    type: 'function',
    function: {
      name: 'run_command',
      description: 'Execute a shell command in the workspace',
      parameters: {
        type: 'object',
        properties: {
          command: { type: 'string', description: 'Shell command to execute' }
        },
        required: ['command']
      }
    }
  },
  {
    type: 'function',
    function: {
      name: 'git_status',
      description: 'Inspect git status and uncommitted/conflicted files',
      parameters: {
        type: 'object',
        properties: {}
      }
    }
  }
];

/**
 * Default tool executor for local workspace operations.
 */
export const DEFAULT_TOOL_EXECUTOR = {
  async read_file({ path: filePath }) {
    const fullPath = path.isAbsolute(filePath) ? filePath : path.resolve(process.cwd(), filePath);
    try {
      return fs.readFileSync(fullPath, 'utf8');
    } catch (err) {
      return `Error reading file ${filePath}: ${err.message}`;
    }
  },
  async write_file({ path: filePath, content }) {
    const fullPath = path.isAbsolute(filePath) ? filePath : path.resolve(process.cwd(), filePath);
    try {
      fs.mkdirSync(path.dirname(fullPath), { recursive: true });
      fs.writeFileSync(fullPath, content, 'utf8');
      return `Successfully wrote ${Buffer.byteLength(content, 'utf8')} bytes to ${filePath}`;
    } catch (err) {
      return `Error writing file ${filePath}: ${err.message}`;
    }
  },
  async run_command({ command }) {
    try {
      const stdout = cp.execSync(command, {
        cwd: process.cwd(),
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'pipe'],
        maxBuffer: 10 * 1024 * 1024
      });
      return stdout || '(command completed with no output)';
    } catch (err) {
      return `Command failed (exit code ${err.status}):\nSTDOUT:\n${err.stdout || ''}\nSTDERR:\n${err.stderr || ''}`;
    }
  },
  async git_status() {
    try {
      const status = cp.execSync('git status --porcelain', { cwd: process.cwd(), encoding: 'utf8' });
      const diffStat = cp.execSync('git diff --stat', { cwd: process.cwd(), encoding: 'utf8' });
      return `--- git status --porcelain ---\n${status || '(clean)'}\n--- git diff --stat ---\n${diffStat || '(none)'}`;
    } catch (err) {
      return `Git command failed: ${err.message}`;
    }
  }
};

/**
 * Synchronize registry.json metadata with PluginVersion in internal/cpa/config.go.
 *
 * @param {{ cwd?: string }} [options]
 * @returns {{ updated: boolean, version: string }}
 */
export function syncRegistryVersion({ cwd = process.cwd() } = {}) {
  const configPath = path.join(cwd, 'internal', 'cpa', 'config.go');
  const registryPath = path.join(cwd, 'registry.json');

  const configSource = fs.readFileSync(configPath, 'utf8');
  const match = configSource.match(/\bPluginVersion\s*=\s*"([^"]+)"/);
  if (!match || !match[1]) {
    throw new Error('Could not find PluginVersion in internal/cpa/config.go');
  }
  const pluginVersion = match[1].trim();

  const registryRaw = fs.readFileSync(registryPath, 'utf8');
  const registry = JSON.parse(registryRaw);
  if (!registry || !Array.isArray(registry.plugins) || registry.plugins.length === 0) {
    throw new Error('registry.json must contain a non-empty plugins array');
  }

  const plugin = registry.plugins[0];
  let updated = false;
  if (plugin.version !== pluginVersion) {
    plugin.version = pluginVersion;
    updated = true;
  }
  if (plugin.author !== FORK_AUTHOR) {
    plugin.author = FORK_AUTHOR;
    updated = true;
  }
  if (plugin.repository !== FORK_REPOSITORY) {
    plugin.repository = FORK_REPOSITORY;
    updated = true;
  }

  if (updated) {
    fs.writeFileSync(registryPath, `${JSON.stringify(registry, null, 2)}\n`, 'utf8');
  }

  return { updated, version: pluginVersion };
}

/**
 * Call OpenAI-compatible /chat/completions with Cloud Run cold-start retry & timeout handling.
 */
async function postChatCompletionWithRetry({
  completionsUrl,
  headers,
  payload,
  fetchFn,
  retryDelaysMs = DEFAULT_RETRY_DELAYS_MS,
  requestTimeoutMs = DEFAULT_REQUEST_TIMEOUT_MS,
  sleepFn = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
}) {
  const delays = Array.isArray(retryDelaysMs) ? retryDelaysMs : [];
  const maxAttempts = 1 + delays.length;

  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const useTimeout = Number.isFinite(requestTimeoutMs) && requestTimeoutMs > 0;
    const controller = useTimeout ? new AbortController() : null;
    const timeoutId = controller
      ? setTimeout(() => {
          controller.abort(new Error(`Request timed out after ${requestTimeoutMs}ms`));
        }, requestTimeoutMs)
      : null;

    try {
      const fetchOpts = {
        method: 'POST',
        headers,
        body: JSON.stringify(payload)
      };
      if (controller) {
        fetchOpts.signal = controller.signal;
      }

      const res = await fetchFn(completionsUrl, fetchOpts);
      if (!res.ok) {
        const errText = await res.text().catch(() => '');
        const httpErr = new Error(`Agent repair API request failed (${res.status} ${res.statusText}): ${errText}`);
        if (RETRYABLE_HTTP_STATUSES.has(res.status) && attempt < maxAttempts - 1) {
          const delayMs = delays[attempt];
          console.warn(
            `[ci-upstream-agent] Retryable HTTP ${res.status} from ${completionsUrl} ` +
            `(attempt ${attempt + 1}/${maxAttempts}); retrying in ${delayMs}ms...`
          );
          if (timeoutId) clearTimeout(timeoutId);
          await sleepFn(delayMs);
          continue;
        }
        throw httpErr;
      }

      return await res.json();
    } catch (err) {
      if (err && typeof err.message === 'string' && err.message.startsWith('Agent repair API request failed (')) {
        throw err;
      }

      const effectiveErr = controller?.signal?.aborted && controller.signal.reason instanceof Error
        ? controller.signal.reason
        : err;

      if (attempt < maxAttempts - 1) {
        const delayMs = delays[attempt];
        console.warn(
          `[ci-upstream-agent] Request error calling ${completionsUrl} ` +
          `(attempt ${attempt + 1}/${maxAttempts}): ${effectiveErr.message}; retrying in ${delayMs}ms...`
        );
        if (timeoutId) clearTimeout(timeoutId);
        await sleepFn(delayMs);
        continue;
      }

      throw effectiveErr;
    } finally {
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
    }
  }
}

/**
 * Run the agentic repair loop against an OpenAI-compatible endpoint.
 *
 * @param {{
 *   baseUrl: string,
 *   apiKey?: string,
 *   model?: string,
 *   maxTurns?: number,
 *   contextSummary?: string,
 *   verifyFn: () => Promise<{ ok: boolean, output?: string }>,
 *   toolExecutor?: Record<string, Function>,
 *   fetchFn?: typeof globalThis.fetch,
 *   retryDelaysMs?: number[],
 *   requestTimeoutMs?: number,
 *   sleepFn?: (ms: number) => Promise<void>
 * }} options
 * @returns {Promise<{ resolved: boolean, turns: number, lastError?: string }>}
 */
export async function runAgenticRepairLoop({
  baseUrl,
  apiKey,
  model = DEFAULT_MODEL,
  maxTurns = DEFAULT_MAX_TURNS,
  contextSummary = '',
  verifyFn,
  toolExecutor,
  fetchFn = globalThis.fetch,
  retryDelaysMs = DEFAULT_RETRY_DELAYS_MS,
  requestTimeoutMs = DEFAULT_REQUEST_TIMEOUT_MS,
  sleepFn = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
}) {
  let normalizedBase = (baseUrl || DEFAULT_BASE_URL).replace(/\/+$/, '');
  if (!normalizedBase.endsWith('/v1')) {
    normalizedBase += '/v1';
  }
  const completionsUrl = `${normalizedBase}/chat/completions`;

  const systemPrompt = `You are an automated upstream convergence repair agent for cpa-plugin-aistudio2api.
You must obey GLOSSARY.md, docs/adr/0001-native-channel-waterfall-and-host-sticky-hedging.md, and docs/adr/0002-automated-upstream-convergence-and-dual-target-release.md:
- Keep changes small and simple (KISS), and write comments in English only.
- Preserve the native Playground-to-Assistant (-build / CodeAssistantOffline) waterfall within a single plugin execution hop; never reintroduce loopback HTTP self-proxying.
- Preserve Prefixed Model Host Affinity Routing (ModelRouteTargetProvider Target: "aistudio2api") and PreferredAccountID host-sticky cooldown HTTP 429 propagation (with Retry-After) when sibling pool accounts are healthy, cascading to -build only when all eligible Playground accounts are simultaneously cooling.
- Preserve StandbyReady preferred-account promotion in acquireWarmLease and same-account hedging ([same]) for session-bound requests.
- Preserve PluginRuntime.ApplyConfig synchronization (activeManagement, ApplyCapacity, SetFirstEventHedge) under lock across trackedService, worker manager, and account pool.
- Keep .github/workflows/release.yml restricted strictly to linux/amd64 and windows/amd64 targets and in-place v<PluginVersion> releases aligned with internal/cpa/config.go (PluginVersion) and registry.json.
- Format all Go files with gofmt -w .
- Resolve any git conflict markers (<<<<<<<, =======, >>>>>>>).
- Fix compilation, vet, CGO shared-library build, or unit test errors until the verification suite passes.`;

  const initialUserPrompt = `Automated upstream convergence verification failed or encountered conflicts.
Context Summary:
${contextSummary}

Please inspect the workspace, resolve all conflicts and compilation/test failures, and fix any broken code.`;

  const messages = [
    { role: 'system', content: systemPrompt },
    { role: 'user', content: initialUserPrompt }
  ];

  let lastError = '';

  for (let turn = 1; turn <= maxTurns; turn++) {
    const headers = {
      'Content-Type': 'application/json'
    };
    if (apiKey) {
      headers['Authorization'] = `Bearer ${apiKey}`;
    }

    const payload = {
      model,
      messages,
      tools: AGENT_TOOLS,
      tool_choice: 'auto'
    };

    const data = await postChatCompletionWithRetry({
      completionsUrl,
      headers,
      payload,
      fetchFn,
      retryDelaysMs,
      requestTimeoutMs,
      sleepFn
    });

    const choice = data.choices?.[0];
    if (!choice || !choice.message) {
      throw new Error('Invalid response from agent API: no message choice');
    }

    const message = choice.message;
    messages.push(message);

    const toolCalls = message.tool_calls;
    if (Array.isArray(toolCalls) && toolCalls.length > 0) {
      for (const call of toolCalls) {
        const fnName = call.function?.name;
        let fnArgs = {};
        try {
          fnArgs = typeof call.function?.arguments === 'string'
            ? JSON.parse(call.function.arguments)
            : (call.function?.arguments || {});
        } catch {
          fnArgs = { raw: call.function?.arguments };
        }

        const executor = (toolExecutor && toolExecutor[fnName])
          ? toolExecutor[fnName]
          : DEFAULT_TOOL_EXECUTOR[fnName];

        let result = '';
        if (typeof executor === 'function') {
          try {
            result = await executor(fnArgs);
          } catch (execErr) {
            result = `Tool execution error: ${execErr.message}`;
          }
        } else {
          result = `Unknown tool: ${fnName}`;
        }

        messages.push({
          role: 'tool',
          tool_call_id: call.id,
          name: fnName,
          content: typeof result === 'string' ? result : JSON.stringify(result)
        });
      }

      // Verify state after tool execution turn
      const verifyResult = await verifyFn();
      if (verifyResult.ok) {
        return { resolved: true, turns: turn };
      }
      lastError = verifyResult.output || '';
    } else {
      // Agent finished with plain message
      const verifyResult = await verifyFn();
      if (verifyResult.ok) {
        return { resolved: true, turns: turn };
      }
      lastError = verifyResult.output || '';
      messages.push({
        role: 'user',
        content: `Verification gate still failed:\n${lastError}\nPlease fix the remaining errors.`
      });
    }
  }

  return { resolved: false, turns: maxTurns, lastError };
}

/**
 * Standard plugin verification gate:
 * syncRegistryVersion -> gofmt -w . -> go vet ./... -> c-shared build -> go test
 */
export async function runVerification({ cwd = process.cwd() } = {}) {
  try {
    // 1. Sync registry.json with internal/cpa/config.go PluginVersion
    syncRegistryVersion({ cwd });

    // 2. gofmt -w .
    cp.execSync('gofmt -w .', {
      cwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      maxBuffer: 10 * 1024 * 1024
    });

    // 3. Configure CGO env (using existing env or ${cwd}/spidermonkey & ${cwd}/libwebp)
    const verifyEnv = { ...process.env };
    delete verifyEnv.GITHUB_TOKEN;
    delete verifyEnv.GH_TOKEN;
    verifyEnv.CGO_ENABLED = '1';

    if (!verifyEnv.CGO_CPPFLAGS) {
      const smInclude = path.join(cwd, 'spidermonkey', 'include');
      const webpInclude = path.join(cwd, 'libwebp', 'include');
      verifyEnv.CGO_CPPFLAGS = `-I${smInclude} -I${webpInclude}`;
    }
    if (!verifyEnv.CGO_LDFLAGS) {
      const smLib = path.join(cwd, 'spidermonkey', 'lib');
      const webpLib = path.join(cwd, 'libwebp', 'lib');
      verifyEnv.CGO_LDFLAGS = `-L${smLib} -L${webpLib}`;
    }

    // 4. go vet ./...
    cp.execSync('go vet ./...', {
      cwd,
      env: verifyEnv,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      maxBuffer: 20 * 1024 * 1024
    });

    // 5. Build -buildmode=c-shared plugin library
    const verifyLibPath = path.join(cwd, 'verify-plugin.so');
    const verifyHeaderPath = path.join(cwd, 'verify-plugin.h');
    try {
      cp.execSync('go build -trimpath -ldflags="-s -w" -buildmode=c-shared -o verify-plugin.so ./cmd/aistudio2api-plugin', {
        cwd,
        env: verifyEnv,
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'pipe'],
        maxBuffer: 20 * 1024 * 1024
      });
    } finally {
      try {
        fs.rmSync(verifyLibPath, { force: true });
      } catch {
        // ignore removal error
      }
      try {
        fs.rmSync(verifyHeaderPath, { force: true });
      } catch {
        // ignore removal error
      }
    }

    // 6. Run unit tests
    const testOut = cp.execSync(`go test ${TEST_PACKAGES.join(' ')}`, {
      cwd,
      env: verifyEnv,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      maxBuffer: 20 * 1024 * 1024
    });

    return { ok: true, output: testOut };
  } catch (err) {
    return {
      ok: false,
      output: `${err.message}\nSTDOUT:\n${err.stdout || ''}\nSTDERR:\n${err.stderr || ''}`
    };
  }
}

/**
 * Write key-value pairs to GITHUB_OUTPUT if set.
 */
export function setGithubOutput(entries, env = process.env) {
  const githubOutput = env.GITHUB_OUTPUT;
  if (!githubOutput) return;

  const content = Object.entries(entries)
    .map(([k, v]) => `${k}=${String(v)}`)
    .join('\n') + '\n';

  fs.appendFileSync(githubOutput, content, 'utf8');
}

/**
 * Fetch the latest formal release tag of jianli9158/cpa-plugin-aistudio2api from GitHub API.
 * Falls back to /releases if /releases/latest returns a non-semver tag (not matching /^v?[0-9]/).
 */
export async function fetchLatestUpstreamRelease({
  repo = UPSTREAM_REPO,
  githubToken = process.env.GITHUB_TOKEN,
  fetchFn = globalThis.fetch
} = {}) {
  const headers = {
    'User-Agent': 'ci-upstream-agent',
    'Accept': 'application/vnd.github.v3+json'
  };
  if (githubToken) {
    headers['Authorization'] = `Bearer ${githubToken}`;
  }

  const latestUrl = `https://api.github.com/repos/${repo}/releases/latest`;
  const res = await fetchFn(latestUrl, { headers });
  if (!res.ok) {
    throw new Error(`Failed to fetch latest release from ${repo} (${res.status} ${res.statusText})`);
  }

  const data = await res.json();
  const tagName = (data?.tag_name || '').trim();
  if (!tagName) {
    throw new Error(`Latest release from ${repo} did not contain a valid tag_name`);
  }

  if (/^v?[0-9]/.test(tagName)) {
    return {
      tagName,
      name: data.name || tagName,
      publishedAt: data.published_at || '',
      htmlUrl: data.html_url || ''
    };
  }

  // Fallback to /releases when /releases/latest points to a non-semver rolling tag (e.g. "latest")
  const listUrl = `https://api.github.com/repos/${repo}/releases`;
  const listRes = await fetchFn(listUrl, { headers });
  if (!listRes.ok) {
    throw new Error(`Failed to fetch releases list from ${repo} (${listRes.status} ${listRes.statusText})`);
  }

  const releases = await listRes.json();
  const matched = Array.isArray(releases)
    ? releases.find(
        (rel) => rel && !rel.draft && typeof rel.tag_name === 'string' && /^v[0-9]/.test(rel.tag_name.trim())
      )
    : null;

  if (!matched) {
    throw new Error(`No formal semver release found in ${repo}/releases (latest tag was "${tagName}")`);
  }

  const matchedTag = matched.tag_name.trim();
  return {
    tagName: matchedTag,
    name: matched.name || matchedTag,
    publishedAt: matched.published_at || '',
    htmlUrl: matched.html_url || ''
  };
}

/**
 * Main CLI entrypoint.
 */
export async function cliMain(args = process.argv.slice(2)) {
  const isSyncCi = args.includes('--sync-ci');

  if (isSyncCi) {
    console.log(`[sync-ci] Starting Upstream Release Sync Cycle for ${UPSTREAM_REPO}...`);
    const repoRoot = process.cwd();
    let changed = false;
    let verified = false;
    let summary = '';
    let releaseTag = '';

    // Step 1: Query latest formal GitHub Release of jianli9158/cpa-plugin-aistudio2api
    let mergeHadConflicts = false;
    let gitMerged = false;
    let isAlreadySynced = false;
    try {
      const latestRelease = await fetchLatestUpstreamRelease();
      releaseTag = latestRelease.tagName;
      console.log(`[sync-ci] Latest upstream formal release is ${releaseTag} (${latestRelease.publishedAt})`);

      console.log(`[sync-ci] Fetching upstream release tag ${releaseTag} from ${UPSTREAM_REPO}...`);
      cp.execSync(`git fetch https://github.com/${UPSTREAM_REPO}.git tag ${releaseTag} --no-tags`, { stdio: 'inherit' });

      // Check if the release tag commit (FETCH_HEAD) is already an ancestor of HEAD
      let isAncestor = false;
      try {
        cp.execSync('git merge-base --is-ancestor FETCH_HEAD HEAD');
        isAncestor = true;
      } catch {
        isAncestor = false;
      }

      if (isAncestor) {
        console.log(`[sync-ci] Upstream release ${releaseTag} is already merged into HEAD. Skipping sync and build.`);
        isAlreadySynced = true;
        verified = true;
        changed = false;
        summary = `Upstream release ${releaseTag} is already merged.`;
      } else {
        console.log(`[sync-ci] Merging upstream release ${releaseTag} with --no-commit --no-ff...`);
        try {
          cp.execSync('git merge --no-commit --no-ff FETCH_HEAD', { stdio: 'inherit' });
          gitMerged = true;
        } catch {
          console.warn(`[sync-ci] Merge of ${releaseTag} produced conflicts.`);
          mergeHadConflicts = true;
        }
      }
    } catch (err) {
      verified = false;
      summary = `Failed to check or fetch upstream release: ${err.message}`;
      console.error(`[sync-ci] ${summary}`);
      setGithubOutput({
        changed: 'false',
        verified: 'false',
        release_tag: releaseTag,
        summary: summary.replace(/\r?\n/g, ' ')
      });
      return;
    }

    if (isAlreadySynced) {
      setGithubOutput({
        changed: 'false',
        verified: 'true',
        release_tag: releaseTag,
        summary: summary.replace(/\r?\n/g, ' ')
      });
      console.log(`[sync-ci] Result: changed=false, verified=true, release_tag=${releaseTag}`);
      return;
    }

    // Step 2: Run initial verification
    console.log('[sync-ci] Running initial verification gate...');
    const initialVerify = await runVerification({ cwd: repoRoot });

    if (!mergeHadConflicts && initialVerify.ok) {
      console.log(`[sync-ci] All verification checks passed cleanly for ${releaseTag} without repair.`);
      verified = true;
      summary = `Upstream release ${releaseTag} verified cleanly without conflicts.`;
    } else {
      console.log(`[sync-ci] Verification failed or merge had conflicts for ${releaseTag}:`);
      console.log(initialVerify.output || '(no verification output)');
      console.log('[sync-ci] Invoking Agentic Conflict Resolution...');
      const repairContext = [
        `Upstream release tag being merged: ${releaseTag}`,
        `Merge conflicts encountered: ${mergeHadConflicts}`,
        `Verification output:\n${initialVerify.output || '(none)'}`
      ].filter(Boolean).join('\n\n');

      try {
        const repairResult = await runAgenticRepairLoop({
          baseUrl: process.env.UPSTREAM_AGENT_BASE_URL || DEFAULT_BASE_URL,
          apiKey: process.env.UPSTREAM_AGENT_API_KEY,
          model: process.env.UPSTREAM_AGENT_MODEL || DEFAULT_MODEL,
          maxTurns: DEFAULT_MAX_TURNS,
          contextSummary: repairContext,
          verifyFn: () => runVerification({ cwd: repoRoot })
        });

        verified = repairResult.resolved;
        if (verified) {
          summary = `Agentic repair for ${releaseTag} succeeded in ${repairResult.turns} turns.`;
          console.log(`[sync-ci] ${summary}`);
        } else {
          summary = `Agentic repair for ${releaseTag} failed after ${repairResult.turns} turns. Last error: ${repairResult.lastError}`;
          console.error(`[sync-ci] ${summary}`);
        }
      } catch (agentErr) {
        verified = false;
        summary = `Agentic repair for ${releaseTag} encountered an unexpected error: ${agentErr.message}`;
        console.error(`[sync-ci] ${summary}`);
      }
    }

    // Step 3: Detect any uncommitted or changed files
    try {
      const gitStatus = cp.execSync('git status --porcelain', { encoding: 'utf8', cwd: repoRoot }).trim();
      changed = gitStatus.length > 0 || gitMerged;
    } catch {
      changed = gitMerged;
    }

    setGithubOutput({
      changed: changed ? 'true' : 'false',
      verified: verified ? 'true' : 'false',
      release_tag: releaseTag,
      summary: summary.replace(/\r?\n/g, ' ')
    });

    console.log(`[sync-ci] Result: changed=${changed}, verified=${verified}, release_tag=${releaseTag}`);
    return;
  }

  console.log(`Usage: node scripts/ci-upstream-agent.mjs [options]
Options:
  --sync-ci        Execute Upstream Sync Cycle for CI
`);
}

// Auto-run if executed directly
const isDirectRun = Boolean(
  process.argv[1] &&
  (import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href ||
   import.meta.url === pathToFileURL(process.argv[1]).href)
);

if (isDirectRun) {
  cliMain().catch(err => {
    console.error(`[ci-upstream-agent] Fatal error: ${err.message}`);
    process.exit(1);
  });
}
