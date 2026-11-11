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
