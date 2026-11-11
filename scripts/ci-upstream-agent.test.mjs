import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';

import {
  AGENT_TOOLS,
  DEFAULT_MAX_TURNS,
  DEFAULT_REQUEST_TIMEOUT_MS,
  DEFAULT_RETRY_DELAYS_MS,
  DEFAULT_TOOL_EXECUTOR,
  FORK_AUTHOR,
  FORK_REPOSITORY,
  UPSTREAM_REPO,
  cliMain,
  fetchLatestUpstreamRelease,
  runAgenticRepairLoop,
  runVerification,
  setGithubOutput,
  syncRegistryVersion
} from './ci-upstream-agent.mjs';

test('defaults: exports expected 30-turn and cold-start resilience constants', () => {
  assert.equal(UPSTREAM_REPO, 'jianli9158/cpa-plugin-aistudio2api');
  assert.equal(DEFAULT_MAX_TURNS, 30);
  assert.deepEqual(DEFAULT_RETRY_DELAYS_MS, [10000, 20000, 30000, 45000]);
  assert.equal(DEFAULT_REQUEST_TIMEOUT_MS, 180000);
});

test('fetchLatestUpstreamRelease: extracts latest semver release tag from default repo', async () => {
  const mockFetch = async (url) => {
    assert.equal(url, 'https://api.github.com/repos/jianli9158/cpa-plugin-aistudio2api/releases/latest');
    return {
      ok: true,
      status: 200,
      json: async () => ({
        tag_name: 'v1.2.1',
        name: 'v1.2.1',
        published_at: '2026-11-10T10:00:00Z',
        html_url: 'https://github.com/jianli9158/cpa-plugin-aistudio2api/releases/tag/v1.2.1'
      })
    };
  };

  const rel = await fetchLatestUpstreamRelease({ fetchFn: mockFetch });
  assert.equal(rel.tagName, 'v1.2.1');
  assert.equal(rel.publishedAt, '2026-11-10T10:00:00Z');
});

test('fetchLatestUpstreamRelease: falls back to /releases when /releases/latest returns non-semver tag', async () => {
  const requestedUrls = [];
  const mockFetch = async (url) => {
    requestedUrls.push(url);
    if (url.endsWith('/releases/latest')) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          tag_name: 'latest',
          name: 'Rolling latest'
        })
      };
    }
    if (url.endsWith('/releases')) {
      return {
        ok: true,
        status: 200,
        json: async () => ([
          { tag_name: 'v1.3.0', draft: true, name: 'Draft release' },
          { tag_name: 'nightly', draft: false, name: 'Nightly' },
          {
            tag_name: 'v1.2.5',
            draft: false,
            name: 'v1.2.5',
            published_at: '2026-11-11T00:00:00Z',
            html_url: 'https://github.com/jianli9158/cpa-plugin-aistudio2api/releases/tag/v1.2.5'
          },
          { tag_name: 'v1.2.4', draft: false, name: 'v1.2.4' }
        ])
      };
    }
    throw new Error(`Unexpected URL: ${url}`);
  };

  const rel = await fetchLatestUpstreamRelease({ fetchFn: mockFetch });
  assert.deepEqual(requestedUrls, [
    'https://api.github.com/repos/jianli9158/cpa-plugin-aistudio2api/releases/latest',
    'https://api.github.com/repos/jianli9158/cpa-plugin-aistudio2api/releases'
  ]);
  assert.equal(rel.tagName, 'v1.2.5');
  assert.equal(rel.publishedAt, '2026-11-11T00:00:00Z');
});

test('fetchLatestUpstreamRelease: throws on HTTP error, missing tag_name, or missing semver fallback', async () => {
  await assert.rejects(
    async () => fetchLatestUpstreamRelease({
      fetchFn: async () => ({ ok: false, status: 404, statusText: 'Not Found' })
    }),
    /Failed to fetch latest release/
  );
  await assert.rejects(
    async () => fetchLatestUpstreamRelease({
      fetchFn: async () => ({ ok: true, status: 200, json: async () => ({}) })
    }),
    /did not contain a valid tag_name/
  );
  await assert.rejects(
    async () => fetchLatestUpstreamRelease({
      fetchFn: async (url) => {
        if (url.endsWith('/releases/latest')) {
          return { ok: true, status: 200, json: async () => ({ tag_name: 'latest' }) };
        }
        return { ok: true, status: 200, json: async () => ([{ tag_name: 'v1.2.0', draft: true }]) };
      }
    }),
    /No formal semver release found/
  );
});

test('syncRegistryVersion: updates version, author, and repository from internal/cpa/config.go', () => {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'registry-sync-test-'));
  try {
    fs.mkdirSync(path.join(tempDir, 'internal', 'cpa'), { recursive: true });
    fs.writeFileSync(
      path.join(tempDir, 'internal', 'cpa', 'config.go'),
      'package cpa\n\nconst (\n\tPluginID = "aistudio2api"\n\tPluginVersion = "1.4.2"\n)\n',
      'utf8'
    );
    fs.writeFileSync(
      path.join(tempDir, 'registry.json'),
      JSON.stringify({
        schema_version: 1,
        plugins: [
          {
            id: 'aistudio2api',
            name: 'Google AI Studio (WAA)',
            description: 'Test',
            author: 'jianli9158',
            version: '1.2.1',
            repository: 'https://github.com/jianli9158/cpa-plugin-aistudio2api',
            license: 'MIT',
            tags: ['Provider', 'OAuth']
          }
        ]
      }, null, 2),
      'utf8'
    );

    const first = syncRegistryVersion({ cwd: tempDir });
    assert.deepEqual(first, { updated: true, version: '1.4.2' });

    const synced = JSON.parse(fs.readFileSync(path.join(tempDir, 'registry.json'), 'utf8'));
    assert.equal(synced.plugins[0].version, '1.4.2');
    assert.equal(synced.plugins[0].author, FORK_AUTHOR);
    assert.equal(synced.plugins[0].repository, FORK_REPOSITORY);
    assert.deepEqual(synced.plugins[0].tags, ['Provider', 'OAuth']);

    const second = syncRegistryVersion({ cwd: tempDir });
    assert.deepEqual(second, { updated: false, version: '1.4.2' });
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('syncRegistryVersion: throws when PluginVersion or plugins[0] is missing', () => {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'registry-sync-err-'));
  try {
    fs.mkdirSync(path.join(tempDir, 'internal', 'cpa'), { recursive: true });
    fs.writeFileSync(path.join(tempDir, 'internal', 'cpa', 'config.go'), 'package cpa\n', 'utf8');
    fs.writeFileSync(path.join(tempDir, 'registry.json'), '{"plugins":[]}', 'utf8');

    assert.throws(() => syncRegistryVersion({ cwd: tempDir }), /Could not find PluginVersion/);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('runVerification: returns ok=false when workspace verification fails', async () => {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'verify-fail-test-'));
  try {
    const res = await runVerification({ cwd: tempDir });
    assert.equal(res.ok, false);
    assert.ok(res.output.length > 0);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('AGENT_TOOLS: contains expected tool definitions and schemas', () => {
  const toolNames = AGENT_TOOLS.map(t => t.function.name);
  assert.deepEqual(toolNames.sort(), ['git_status', 'read_file', 'run_command', 'write_file']);

  for (const tool of AGENT_TOOLS) {
    assert.equal(tool.type, 'function');
    assert.ok(tool.function.description);
    assert.equal(tool.function.parameters.type, 'object');
  }
});

test('DEFAULT_TOOL_EXECUTOR: read_file and write_file operations', async () => {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tool-test-'));
  try {
    const testFile = path.join(tempDir, 'sub', 'test.txt');
    const content = 'hello world from tool executor';

    const writeRes = await DEFAULT_TOOL_EXECUTOR.write_file({ path: testFile, content });
    assert.match(writeRes, /Successfully wrote \d+ bytes/);
    assert.equal(fs.readFileSync(testFile, 'utf8'), content);

    const readRes = await DEFAULT_TOOL_EXECUTOR.read_file({ path: testFile });
    assert.equal(readRes, content);

    const nonExistentRes = await DEFAULT_TOOL_EXECUTOR.read_file({ path: path.join(tempDir, 'missing.txt') });
    assert.match(nonExistentRes, /Error reading file/);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('DEFAULT_TOOL_EXECUTOR: run_command executes basic shell commands', async () => {
  const isWindows = process.platform === 'win32';
  const cmd = isWindows ? 'cmd /c echo tool_command_success' : 'echo tool_command_success';
  const out = await DEFAULT_TOOL_EXECUTOR.run_command({ command: cmd });
  assert.match(out, /tool_command_success/);
});

test('setGithubOutput: appends output variables or handles unset GITHUB_OUTPUT', () => {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'gh-output-test-'));
  try {
    const outputFile = path.join(tempDir, 'output.txt');
    fs.writeFileSync(outputFile, '', 'utf8');

    setGithubOutput({ changed: 'true', verified: 'true', summary: 'all good' }, { GITHUB_OUTPUT: outputFile });

    const content = fs.readFileSync(outputFile, 'utf8');
    assert.match(content, /changed=true/);
    assert.match(content, /verified=true/);
    assert.match(content, /summary=all good/);

    assert.doesNotThrow(() => setGithubOutput({ changed: 'false' }, {}));
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('cliMain: prints usage when invoked with empty or unrecognized args', async () => {
  const logs = [];
  const originalLog = console.log;
  console.log = (...args) => logs.push(args.join(' '));
  try {
    await cliMain([]);
    assert.ok(logs.some(l => l.includes('Usage: node scripts/ci-upstream-agent.mjs')));
  } finally {
    console.log = originalLog;
  }
});

test('runAgenticRepairLoop: 1-turn repair stops on green verification and enforces ADR constraints in prompt', async () => {
  const calls = [];
  const mockFetch = async (url, opts) => {
    calls.push({ url, opts, body: JSON.parse(opts.body) });
    return {
      ok: true,
      status: 200,
      json: async () => ({
        choices: [
          {
            message: {
              role: 'assistant',
              content: 'Fixing the build error by writing code',
              tool_calls: [
                {
                  id: 'call_1',
                  type: 'function',
                  function: {
                    name: 'write_file',
                    arguments: JSON.stringify({ path: 'fixed.txt', content: 'resolved' })
                  }
                }
              ]
            }
          }
        ]
      })
    };
  };

  let fileWritten = false;
  const mockToolExecutor = {
    write_file: async () => {
      fileWritten = true;
      return 'ok';
    }
  };

  let verifyCallCount = 0;
  const verifyFn = async () => {
    verifyCallCount++;
    return { ok: fileWritten };
  };

  const result = await runAgenticRepairLoop({
    baseUrl: 'https://test-agent.example.com',
    apiKey: 'secret-key',
    model: 'gemini-3.8-flash-high',
    maxTurns: 5,
    contextSummary: 'Build broken on line 12',
    verifyFn,
    toolExecutor: mockToolExecutor,
    fetchFn: mockFetch
  });

  assert.equal(result.resolved, true);
  assert.equal(result.turns, 1);
  assert.equal(fileWritten, true);
  assert.equal(verifyCallCount, 1);
  assert.equal(calls[0].url, 'https://test-agent.example.com/v1/chat/completions');
  assert.equal(calls[0].opts.headers['Authorization'], 'Bearer secret-key');

  const systemPrompt = calls[0].body.messages[0].content;
  assert.match(systemPrompt, /0001-native-channel-waterfall-and-host-sticky-hedging\.md/);
  assert.match(systemPrompt, /0002-automated-upstream-convergence-and-dual-target-release\.md/);
  assert.match(systemPrompt, /PreferredAccountID/);
  assert.match(systemPrompt, /ApplyConfig/);
  assert.match(systemPrompt, /linux\/amd64/);
  assert.match(systemPrompt, /windows\/amd64/);
});

test('runAgenticRepairLoop: completion message with no tool calls and passing verifyFn', async () => {
  const mockFetch = async () => ({
    ok: true,
    status: 200,
    json: async () => ({
      choices: [
        {
          message: {
            role: 'assistant',
            content: 'The workspace is already in order.'
          }
        }
      ]
    })
  });

  const verifyFn = async () => ({ ok: true, output: 'passed' });

  const result = await runAgenticRepairLoop({
    baseUrl: 'https://test-agent.example.com/v1',
    maxTurns: 5,
    verifyFn,
    fetchFn: mockFetch
  });

  assert.equal(result.resolved, true);
  assert.equal(result.turns, 1);
});

test('runAgenticRepairLoop: multi-turn repair reaches resolution on turn 2', async () => {
  let turnCounter = 0;
  const mockFetch = async () => {
    turnCounter++;
    if (turnCounter === 1) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          choices: [
            {
              message: {
                role: 'assistant',
                tool_calls: [
                  {
                    id: 'call_turn1',
                    type: 'function',
                    function: {
                      name: 'read_file',
                      arguments: JSON.stringify({ path: 'main.go' })
                    }
                  }
                ]
              }
            }
          ]
        })
      };
    }
    return {
      ok: true,
      status: 200,
      json: async () => ({
        choices: [
          {
            message: {
              role: 'assistant',
              tool_calls: [
                {
                  id: 'call_turn2',
                  type: 'function',
                  function: {
                    name: 'write_file',
                    arguments: JSON.stringify({ path: 'main.go', content: 'package main' })
                  }
                }
              ]
            }
          }
        ]
      })
    };
  };

  let isFixed = false;
  const mockToolExecutor = {
    read_file: async () => 'broken content',
    write_file: async () => {
      isFixed = true;
      return 'written';
    }
  };

  const verifyFn = async () => {
    return isFixed ? { ok: true } : { ok: false, output: 'syntax error' };
  };

  const result = await runAgenticRepairLoop({
    baseUrl: 'https://test-agent.example.com/v1',
    apiKey: 'test-key',
    maxTurns: 5,
    verifyFn,
    toolExecutor: mockToolExecutor,
    fetchFn: mockFetch
  });

  assert.equal(result.resolved, true);
  assert.equal(result.turns, 2);
});

test('runAgenticRepairLoop: defaults maxTurns to 30 when omitted', async () => {
  let fetchCalls = 0;
  const mockFetch = async () => {
    fetchCalls++;
    return {
      ok: true,
      status: 200,
      json: async () => ({
        choices: [
          {
            message: {
              role: 'assistant',
              content: 'Continuing repair...',
              tool_calls: [
                {
                  id: `call_${fetchCalls}`,
                  type: 'function',
                  function: {
                    name: 'run_command',
                    arguments: JSON.stringify({ command: 'echo check' })
                  }
                }
              ]
            }
          }
        ]
      })
    };
  };

  const result = await runAgenticRepairLoop({
    baseUrl: 'https://test-agent.example.com/v1',
    verifyFn: async () => ({ ok: false, output: 'still failing' }),
    toolExecutor: { run_command: async () => 'checked' },
    fetchFn: mockFetch
  });

  assert.equal(result.resolved, false);
  assert.equal(result.turns, 30);
  assert.equal(fetchCalls, 30);
  assert.equal(result.lastError, 'still failing');
});

test('runAgenticRepairLoop: cold-start resilience retries retryable HTTP statuses with [10000, 20000, 30000, 45000] backoff', async () => {
  const retryStatuses = [408, 429, 502, 503];
  const recordedSleeps = [];
  let attempt = 0;

  const mockFetch = async () => {
    attempt++;
    if (attempt <= retryStatuses.length) {
      const status = retryStatuses[attempt - 1];
      return {
        ok: false,
        status,
        statusText: `HTTP ${status}`,
        text: async () => 'cold start warming'
      };
    }
    return {
      ok: true,
      status: 200,
      json: async () => ({
        choices: [
          {
            message: {
              role: 'assistant',
              content: 'Recovered after cold start'
            }
          }
        ]
      })
    };
  };

  const result = await runAgenticRepairLoop({
    baseUrl: 'https://test-agent.example.com/v1',
    maxTurns: 2,
    verifyFn: async () => ({ ok: true }),
    fetchFn: mockFetch,
    sleepFn: async (ms) => {
      recordedSleeps.push(ms);
    }
  });

  assert.equal(result.resolved, true);
  assert.equal(attempt, 5);
  assert.deepEqual(recordedSleeps, [10000, 20000, 30000, 45000]);
});

test('runAgenticRepairLoop: cold-start resilience retries 500, 504, and network errors', async () => {
  const recordedSleeps = [];
  let attempt = 0;

  const mockFetch = async () => {
    attempt++;
    if (attempt === 1) {
      throw new Error('ECONNRESET');
    }
    if (attempt === 2) {
      return { ok: false, status: 500, statusText: 'Internal Server Error', text: async () => 'booting' };
    }
    if (attempt === 3) {
      return { ok: false, status: 504, statusText: 'Gateway Timeout', text: async () => 'timeout' };
    }
    return {
      ok: true,
      status: 200,
      json: async () => ({
        choices: [{ message: { role: 'assistant', content: 'ok' } }]
      })
    };
  };

  const result = await runAgenticRepairLoop({
    baseUrl: 'https://test-agent.example.com/v1',
    maxTurns: 2,
    verifyFn: async () => ({ ok: true }),
    fetchFn: mockFetch,
    sleepFn: async (ms) => {
      recordedSleeps.push(ms);
    }
  });

  assert.equal(result.resolved, true);
  assert.equal(attempt, 4);
  assert.deepEqual(recordedSleeps, [10000, 20000, 30000]);
});

test('runAgenticRepairLoop: aborts stalled request after requestTimeoutMs and retries', async () => {
  const recordedSleeps = [];
  let attempt = 0;

  const mockFetch = async (_url, opts) => {
    attempt++;
    if (attempt === 1) {
      return new Promise((_, reject) => {
        opts.signal.addEventListener('abort', () => {
          reject(opts.signal.reason || new Error('Aborted'));
        });
      });
    }
    return {
      ok: true,
      status: 200,
      json: async () => ({
        choices: [{ message: { role: 'assistant', content: 'ok after timeout retry' } }]
      })
    };
  };

  const result = await runAgenticRepairLoop({
    baseUrl: 'https://test-agent.example.com/v1',
    maxTurns: 2,
    verifyFn: async () => ({ ok: true }),
    fetchFn: mockFetch,
    requestTimeoutMs: 25,
    retryDelaysMs: [5, 10],
    sleepFn: async (ms) => {
      recordedSleeps.push(ms);
    }
  });

  assert.equal(result.resolved, true);
  assert.equal(attempt, 2);
  assert.deepEqual(recordedSleeps, [5]);
});

test('runAgenticRepairLoop: does not retry non-retryable HTTP errors (e.g. 401)', async () => {
  let attempt = 0;
  const mockFetch = async () => {
    attempt++;
    return {
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      text: async () => 'invalid key'
    };
  };

  await assert.rejects(
    async () => runAgenticRepairLoop({
      baseUrl: 'https://test-agent.example.com/v1',
      maxTurns: 2,
      verifyFn: async () => ({ ok: false }),
      fetchFn: mockFetch,
      sleepFn: async () => {
        throw new Error('Should not sleep on 401');
      }
    }),
    /Agent repair API request failed \(401 Unauthorized\): invalid key/
  );
  assert.equal(attempt, 1);
});

test('runAgenticRepairLoop: throws after exhausting retries on persistent 500 error', async () => {
  let attempt = 0;
  const recordedSleeps = [];
  const mockFetch = async () => {
    attempt++;
    return {
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      text: async () => 'upstream crashed'
    };
  };

  await assert.rejects(
    async () => runAgenticRepairLoop({
      baseUrl: 'https://test-agent.example.com/v1',
      maxTurns: 2,
      verifyFn: async () => ({ ok: false }),
      fetchFn: mockFetch,
      sleepFn: async (ms) => {
        recordedSleeps.push(ms);
      }
    }),
    /Agent repair API request failed \(500 Internal Server Error\): upstream crashed/
  );
  assert.equal(attempt, 5);
  assert.deepEqual(recordedSleeps, [10000, 20000, 30000, 45000]);
});
