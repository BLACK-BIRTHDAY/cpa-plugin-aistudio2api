# Upstream-First Convergence and Zero-Fork Divergence

Status: accepted
Supersedes: ADR-0002 (custom patch conflict maintenance)

Maintaining custom protocol, scheduling, and recovery patches directly inside `BLACK-BIRTHDAY/cpa-plugin-aistudio2api` caused recurring three-way merge conflicts whenever Plugin Upstream (`jianli9158/cpa-plugin-aistudio2api`) synchronized with Core Upstream (`Mag1cFall/AIStudio2API`). We decided to adopt **Upstream-First Convergence**: all general protocol, runtime, and configuration features (`BUILD_STALL_TIMEOUT` defaulting to `120s`, `ASSISTANT_FALLBACK` defaulting to `false`, `PreferredAccountID` standby worker promotion and same-account hedging, and `*AbnormalFinishError` with `sentOutput` pre-error gating) are submitted in a single-commit PR to Core Upstream (`Mag1cFall/AIStudio2API`), while C-ABI host bridge adaptations are submitted to Plugin Upstream (`jianli9158/cpa-plugin-aistudio2api`), leaving `BLACK-BIRTHDAY/cpa-plugin-aistudio2api` to track upstream cleanly and publish dual-target (`linux/amd64` and `windows/amd64`) releases.
