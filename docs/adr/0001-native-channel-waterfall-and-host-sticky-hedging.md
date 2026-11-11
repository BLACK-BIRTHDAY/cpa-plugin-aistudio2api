# Native Playground-to-Assistant Waterfall, Host Affinity Routing, and Sticky Hedging

Status: accepted

Because upstream `ProxyStreamedCall` is discontinued and loopback HTTP self-proxying introduces double serialization and obscures per-account cooldown state, `cpa-plugin-aistudio2api` natively cascades requests from the **Playground Channel** (`GenerateContent`) to the **Assistant Applet Channel** (`-build` / `CodeAssistantOffline`) within a single plugin execution hop when all candidate accounts are in quota cooldown on Playground.

To coordinate with the CLIProxyAPI host gateway:
1. **Prefixed Model Host Affinity Routing**: Prefixed model requests (`AIStudio/<model>`, `aistudio/<model>`, `aistudio2api/<model>`) route via `ModelRouteTargetProvider` (`Target: "aistudio2api"`) whenever host credentials exist in `req.AvailableProviders`, allowing host `SessionAffinitySelector` to bind multi-turn conversations to a `PreferredAccountID`.
2. **Host-Propagated Cooldown Failover**: When a request carries a `PreferredAccountID` that is in quota cooldown on all eligible non-assistant channels while at least one sibling account in the pool is ready and not cooling, the plugin returns HTTP `429 Too Many Requests` (with `Retry-After`) to the host so `AuthManager` and `SessionAffinitySelector` rebind the session to a healthy account. Only when all eligible Playground accounts are simultaneously cooling does the plugin waterfall into `-build` on the preferred account.
3. **Standby Preferred-Account Promotion & Sticky Hedging**: `acquireWarmLease` promotes a `StandbyReady` preferred account rather than short-circuiting to warm sibling workers. `First-Event Hedge` races across alternate warm accounts for unbound requests, but restricts hedging strictly to an alternate concurrency slot on the same preferred account (`[same]`) for session-bound requests.
4. **Runtime ApplyConfig Synchronization**: `PluginRuntime.ApplyConfig` synchronizes `activeManagement`, live capacity limits (`ApplyCapacity`), and hedging parameters (`SetFirstEventHedge`) across `trackedService`, worker manager, and account pool under lock.
