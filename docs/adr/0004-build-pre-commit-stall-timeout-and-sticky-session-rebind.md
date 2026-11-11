# Build Pre-Commit Stall Timeout and Sticky Session Rebind on Failover

Status: accepted

Because Assistant Applet (`-build` / `CodeAssistantOffline`) and Build channel generations are excluded from `First-Event Hedge` and occasionally hang after returning an initial in-progress step frame or partial thinking snapshot without ever emitting visible content or tool calls, bound requests could previously stall until the 30-minute global `REQUEST_TIMEOUT` and remain pinned to the same stalled `preferredAccountID` on retry.

We decided to:
1. **Enforce a 2-Minute Hard Pre-Commit Stall Timeout on Build and `-build` Channels**: Apply a 2-minute inactivity deadline (`2 * time.Minute`) to both `firstGenerateEvent` (`上游首事件等待`) and pre-commit stream forwarding (while `contentEvents == 0 && toolCalls == 0` and no media has been committed). When the deadline expires, immediately cancel the stalled attempt—triggering background `CancelCodeAssistantOfflineGeneration` cleanup and releasing the account concurrency slot—and fail over to the next eligible account within the same downstream stream.
2. **Unbind Sticky Affinity on Retry and Re-Anchor Session Tip**: Once a `preferredAccountID` (from `assistantSessions` or host session affinity) is recorded in `retry.attempted`, clear it for subsequent retry attempts so `acquireWarmLease` selects a healthy sibling account, and record the new winning account as the conversation's `assistantSessions` tip upon completion.
3. **Scope Exclusion to the Active Request Only**: Exclude the timed-out account via `retry.attempted` for the current request without marking a pool-wide account cooldown, since single-applet hangs in `CodeAssistantOffline` do not imply account-wide quota exhaustion.
