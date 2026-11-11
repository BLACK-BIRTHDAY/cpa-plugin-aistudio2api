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

**Upstream Sync Cycle**:
A scheduled or manually dispatched reconciliation workflow that merges upstream `jianli9158/cpa-plugin-aistudio2api` changes, runs CGO build and unit test verification gates (with Agentic Conflict Resolution on failure), and chains into an In-Place Version Release.
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
