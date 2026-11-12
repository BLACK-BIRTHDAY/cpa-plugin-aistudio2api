# cpa-plugin-aistudio2api

A native CLIProxyAPI C ABI plugin that bridges pooled Google AI Studio accounts into unified inference endpoints across Playground and Assistant Applet channels.

## Language

### Build and Release Automation

**Gateway Target (`linux/amd64`)**:
The Linux x86_64 shared library artifact (`aistudio2api.so` packaged in `aistudio2api_<version>_linux_amd64.zip`) linked against GNU SpiderMonkey and libwebp static archives for deployment inside the GCP Cloud Run CLIProxyAPI container.
_Avoid_: Server build, Linux generic build, Docker plugin

**Local Desktop Target (`windows/amd64`)**:
The Windows x86_64 dynamic-link library artifact (`aistudio2api.dll` packaged in `aistudio2api_<version>_windows_amd64.zip`) cross-compiled via `llvm-mingw` for local Windows CLIProxyAPI installations.
_Avoid_: Desktop EXE, Windows installer, client binary

**In-Place Version Release**:
The rolling GitHub Release named `v<version>` (matching `PluginVersion` and `registry.json`) that is deleted, re-tagged at `HEAD`, and recreated on each `main` build so `CLIProxyAPI` plugin store clients resolve valid semver tags and matching asset filenames (`aistudio2api_<version>_<goos>_<goarch>.zip` and `checksums.txt`).
_Avoid_: `latest`-tag release, unversioned release, draft release

**Core Upstream (`Mag1cFall/AIStudio2API`)**:
The canonical standalone AI Studio proxy repository that owns `internal/aistudio`, `internal/api`, `internal/app`, `internal/config`, and `internal/waa`.
_Avoid_: Standalone repo, base repo, generic upstream

**Plugin Upstream (`jianli9158/cpa-plugin-aistudio2api`)**:
The upstream CLIProxyAPI C-ABI plugin repository that wraps Core Upstream with `internal/cpa` and `internal/app/plugin_bridge.go` and periodically synchronizes with Core Upstream releases.
_Avoid_: Upstream fork, plugin base

**Upstream-First Convergence**:
The governance model where all general protocol, scheduling, and recovery capabilities are contributed directly to Core Upstream (`Mag1cFall/AIStudio2API`) and C-ABI host bridge adaptations are contributed to Plugin Upstream (`jianli9158/cpa-plugin-aistudio2api`), eliminating local feature divergence in `BLACK-BIRTHDAY/cpa-plugin-aistudio2api`.
_Avoid_: Local patch maintenance, long-lived feature fork

**Upstream Sync Cycle**:
A scheduled or manually dispatched reconciliation workflow that synchronizes `BLACK-BIRTHDAY/cpa-plugin-aistudio2api` with Plugin Upstream (`jianli9158/cpa-plugin-aistudio2api`), runs CGO build and unit test verification gates, and chains into an In-Place Version Release.
_Avoid_: Manual cherry-pick, blind force-merge

### Stream Recovery and Tool Calling

**In-Stream Continuation Recovery**:
The mechanism that resumes a streaming generation in-place with `<partial_reply>` context when upstream terminates with `malformed_function_call`, `unexpected_tool_call`, or `malformed_response` after preamble plain text has already been committed to the client.
_Avoid_: Client re-prompt, silent stop, dropped stream

**Continuation Prefix Stripper**:
A multi-target streaming deduplicator that holds and strips echoed introductory text across continuation rounds while preserving thought signatures and tolerating interleaved non-text events.
_Avoid_: Dumb string trim, discarded signatures, duplicated text

**Abnormal Finish Error (`*AbnormalFinishError`)**:
An explicit protocol error surfaced when retries on `malformed_function_call`, `unexpected_tool_call`, or `malformed_response` exhaust, preventing malformed turns from falsely projecting as `stop` or `end_turn`.
_Avoid_: Masked stop, silent completion, fake success

**Forbidden Workspace Tool Interception**:
The detection in `AssistantStreamDecoder` that halts processing of bogus remote applet tools (`run_command`, `view_file`, etc.) and triggers in-applet follow-up prompts for client function calls.
_Avoid_: Ignored workspace steps, hanging tool waits

**Build Pre-Commit Stall Timeout**:
The 2-minute inactivity deadline applied to Build and Assistant (`-build`) attempts during both first-event waiting and pre-commit streaming (when only thinking or metadata has arrived and no visible text, media, or tool call has been committed), after which the stalled attempt is cancelled and the request fails over to the next eligible account on the same stream.
_Avoid_: Global request timeout, first-event hedge, account cooldown

**Sticky Session Rebind on Failover**:
The failover transition that clears sticky account affinity (`preferredAccountID`) once the bound account fails or times out within a request, dispatches the retry to a healthy sibling account, and re-anchors the conversation's `assistantSessions` tip to the newly succeeded account.
_Avoid_: Permanent session lock, blind same-account retry
