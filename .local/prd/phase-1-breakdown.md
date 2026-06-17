# Phase 1 — Task Breakdown
**Parent PRD:** `oapi-prd (1).md`
**Informed by:** None (Initial Phase)
**Total Tasks:** 18
**Summary reports go in:** `docs/reports/summary/`
**Summary filename format:** `{YYYYMMDD_HHMMSS}_{task-id}_{short-description}.md`

**How to Read This Document**
> Each task has a fixed **Goal** and a flexible **Approach**. Goals are non-negotiable. Each **Group** ends with a **Validation Checkpoint** that must be fully checked before the next group starts. Where previous handoff artifacts conflict with the parent PRD, treat handoff documents as the authoritative baseline.

**Summary Report Template**
```markdown
# Summary: {Task ID} — {Task Title}
**Timestamp:** {YYYYMMDD_HHMMSS}
**Status:** Completed | Blocked | Partially Complete
## What was done
## Approach taken
## Outputs produced
## Deviations from PRD
## Blockers or open questions
## Notes for downstream tasks
```

**Master Checklist**
```
Group 0 — Orientation
[x] T-001  Project setup and initial Git configuration
Group 1 — Foundation Services
[x] T-002  Hardcoded Provider Registry & Model List
[x] T-003  YAML Config Parsing and Atomic Save
[x] T-004  Runtime State JSON Manager
[x] T-005  Mockable HTTP Client and Connection Test Utility
Group 2 — Core Rotation Engine & Limiter
[x] T-006  Key Pool & Rate Limiter
[x] T-007  Rotation Engine Routing
[x] T-008  Request Rewriter & Forwarding Handler
Group 3 — Proxy Server & Life-cycle
[x] T-009  Proxy HTTP Server & Endpoint Handling
[x] T-010  Graceful Shutdown & Context Management
Group 4 — CLI Interfaces
[x] T-011  Cobra Subcommands
Group 5 — Bubble Tea v2 TUI
[x] T-012  Lipgloss Styles & Main TUI App Model
[x] T-013  Dashboard View
[x] T-014  Providers View & Connection Test Trigger
[x] T-015  Add/Edit Key Form (Huh)
[ ] T-016  Routes View & Editor
[ ] T-017  Logs View
Group 6 — Integration & Verification
[ ] T-018  Integration and Verification tests
```

### Task Index Table
| ID | Title | Group | Depends On | Unblocks |
|---|---|---|---|---|
| T-001 | Project setup and initial Git configuration | Group 0 | Nothing | T-002, T-003, T-004, T-005 |
| T-002 | Hardcoded Provider Registry & Model List | Group 1 | T-001 | T-003, T-006 |
| T-003 | YAML Config Parsing and Atomic Save | Group 1 | T-001, T-002 | T-004, T-006, T-011 |
| T-004 | Runtime State JSON Manager | Group 1 | T-001, T-003 | T-006, T-011 |
| T-005 | Mockable HTTP Client and Connection Test Utility | Group 1 | T-001 | T-011, T-014, T-018 |
| T-006 | Key Pool & Rate Limiter | Group 2 | T-002, T-003, T-004 | T-007 |
| T-007 | Rotation Engine Routing | Group 2 | T-006 | T-008, T-018 |
| T-008 | Request Rewriter & Forwarding Handler | Group 2 | T-007 | T-009 |
| T-009 | Proxy HTTP Server & Endpoint Handling | Group 3 | T-008 | T-010 |
| T-010 | Graceful Shutdown & Context Management | Group 3 | T-009 | T-011 |
| T-011 | Cobra Subcommands | Group 4 | T-003, T-004, T-005, T-010 | T-012 |
| T-012 | Lipgloss Styles & Main TUI App Model | Group 5 | T-011 | T-013, T-014, T-015, T-016, T-017 |
| T-013 | Dashboard View | Group 5 | T-012 | T-018 |
| T-014 | Providers View & Connection Test Trigger | Group 5 | T-005, T-012 | T-015, T-018 |
| T-015 | Add/Edit Key Form (Huh) | Group 5 | T-014 | T-018 |
| T-016 | Routes View & Editor | Group 5 | T-012 | T-018 |
| T-017 | Logs View | Group 5 | T-012 | T-018 |
| T-018 | Integration and Verification tests | Group 6 | T-005, T-007, T-013, T-014, T-015, T-016, T-017 | Phase end |

---

### Detailed Execution Groups

### [x] T-001 — Project setup and initial Git configuration
**Goal:** Initialize the Go module and configure `.gitignore` to prevent API key exposure.
**Depends on:** Nothing
**Unblocks:** T-002, T-003, T-004, T-005
**Outputs:**
- `go.mod`
- `.gitignore`
**What to do / Specifications:**
1. Initialize Go module with `go mod init oapi`.
2. Create `.gitignore` to ignore plain-text credentials (`oapi.yaml`, `state.json`), temporary build files, and local logs.
3. Validate that the workspace git configuration is clean and set up for atomic commits.
**Done when:**
* `go.mod` is initialized with Go 1.23+.
* `.gitignore` ignores `oapi.yaml` and `state.json`.
**Summary report:** `docs/reports/summary/20260617_132829_T-001_project_setup.md`

---

### [x] T-002 — Hardcoded Provider Registry & Model List
**Goal:** Define default parameters, models, and URLs for all supported LLM providers.
**Depends on:** T-001
**Unblocks:** T-003, T-006
**Outputs:**
- `internal/registry/providers.go`
**What to do / Specifications:**
1. Setup static defaults (RPM/RPD limits, base URLs, reset behaviors) for Groq, Google, Cerebras, GitHub, OpenRouter, Mistral, and custom.
2. Define per-model overrides for Groq as specified in the PRD (e.g., `llama-3.1-8b-instant`).
3. Define the recommended model lists shown in TUI options.
**Done when:**
* Provider constants are defined and testable.
* Groq per-model RPD override maps are correctly defined.
**Summary report:** `docs/reports/summary/20260617_133330_T-002_provider_registry.md`

---

### [x] T-003 — YAML Config Parsing and Atomic Save
**Goal:** Read and write `oapi.yaml` configurations using `gopkg.in/yaml.v3` with atomic write-then-rename protection.
**Depends on:** T-001, T-002
**Unblocks:** T-004, T-006, T-011
**Outputs:**
- `internal/config/config.go`
**What to do / Specifications:**
1. Define YAML config structs for server configuration, key pool entries, and routes.
2. Implement load routing (checking flags, local `./oapi.yaml`, and fallback user home config).
3. Implement atomic writes to prevent corruption (write to `oapi.yaml.tmp` then rename to `oapi.yaml`).
**Done when:**
* Configuration file can be loaded, parsed, and updated.
* Atomic writing is verified by testing that a partial write/crash doesn't destroy the existing config.
**Summary report:** `docs/reports/summary/20260617_133400_T-003_config_parsing.md`

---

### [x] T-004 — Runtime State JSON Manager
**Goal:** Persist key counters and cooling state variables in `state.json` with atomic writes.
**Depends on:** T-001, T-003
**Unblocks:** T-006, T-011
**Outputs:**
- `internal/config/state.go`
**What to do / Specifications:**
1. Define `state.json` runtime struct mapping keys to their metrics (e.g., `requests_this_minute`, `requests_today`, `cooling_until`, `first_request_today`).
2. Write atomic save/load functions for `state.json`.
3. Provide a periodic flush mechanism (e.g., every 30s) and a flush-on-shutdown mechanism.
**Done when:**
* State file can be written and loaded back.
* Concurrency checks pass for multi-goroutine state persistence.
**Summary report:** `docs/reports/summary/20260617_134000_T-004_state_json.md`

---

### [x] T-005 — Mockable HTTP Client and Connection Test Utility
**Goal:** Create a testing utility that probes provider keys for connection testing.
**Depends on:** T-001
**Unblocks:** T-011, T-014, T-018
**Outputs:**
- `internal/testutil/probe.go`
**What to do / Specifications:**
1. Create a `ProbeKey` method that fires a minimal Chat Completion payload (`max_tokens: 5`) to the key's target provider.
2. Map status codes: 200 -> Success (active), 401/403 -> Invalid (error), 429 -> Rate limited (cooling), other -> raw error.
3. Ensure client timeout is configured (e.g. 10s).
4. Abstain from real HTTP calls in unit tests (mock via standard interface or `http.RoundTripper`).
**Done when:**
* Key probing logic is functional and handles standard HTTP status codes.
* Unit tests verify behavior without firing real network requests.
**Summary report:** `docs/reports/summary/20260617_134100_T-005_connection_probe.md`

---

### [x] T-006 — Key Pool & Rate Limiter
**Goal:** Implement the thread-safe key state store, sliding window RPM tracking, and daily RPD limit verification.
**Depends on:** T-002, T-003, T-004
**Unblocks:** T-007
**Outputs:**
- `internal/rotation/limiter.go`
- `internal/rotation/pool.go`
**What to do / Specifications:**
1. Implement thread-safe key registry using `sync.RWMutex`.
2. Implement sliding window RPM checks using a ring buffer (slice) of timestamps.
3. Implement RPD checks.
4. Set up a background tick routine running every 5 seconds to clean expired sliding window entries (preventing memory growth) and recover keys from `cooling_rpm` and `cooling_rpd` status.
5. Account for Groq rolling 24h vs Google midnight PT reset rules.
**Done when:**
* Limiter unit tests pass for rate limits (sliding window and daily).
* Background thread successfully recovers cooling keys.
**Summary report:** [20260617_135500_T-006_limiter.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_135500_T-006_limiter.md)

---

### [x] T-007 — Rotation Engine Routing
**Goal:** Route incoming client requests based on primary routing chains and fallback pools.
**Depends on:** T-006
**Unblocks:** T-008, T-018
**Outputs:**
- `internal/rotation/engine.go`
**What to do / Specifications:**
1. Parse the route rules (exact match of client-requested model, then wildcard match).
2. Walk the Primary Chain slots from left to right.
3. Retrieve key lists for the active slot, select using round-robin index.
4. If a slot is exhausted, move to the next slot in the chain.
5. If the chain is completely exhausted, activate the Fallback Pool.
6. Return a custom error if all keys are rate-limited, indicating the earliest cooldown timestamp.
**Done when:**
* Unit tests verify chain traversal, slot fallback, and round-robin index advancement.
* Boundary conditions (empty pools, unknown models) are handled safely.
**Summary report:** [20260617_140500_T-007_routing_engine.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_140500_T-007_routing_engine.md)

---

### [x] T-008 — Request Rewriter & Forwarding Handler
**Goal:** Rewrite incoming client payloads, authorize requests, and handle specific provider parameters.
**Depends on:** T-007
**Unblocks:** T-009
**Outputs:**
- `internal/proxy/handler.go`
**What to do / Specifications:**
1. Intercept `POST /v1/chat/completions`, decode request, identify model.
2. Rewrite headers: strip dummy auth, inject provider-specific authorization token.
3. Implement Google provider rewriting (strip auth header, append API key to query URL as `?key=api_key`).
4. Implement OpenRouter header injections (`HTTP-Referer`).
5. Implement Cerebras parameter adjustments: Inject `"reasoning_effort": "none"` and enforce/override `max_completion_tokens: 60` (or configured value).
**Done when:**
* Forwarded requests match downstream provider specifications exactly.
* Unit tests confirm payload updates for Google and Cerebras.
**Summary report:** [20260617_141700_T-008_forwarding_handler.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_141700_T-008_forwarding_handler.md)

---

### [x] T-009 — Proxy HTTP Server & Endpoint Handling
**Goal:** Spin up the local HTTP proxy listener to serve chat completions, model lists, and health checks.
**Depends on:** T-008
**Unblocks:** T-010
**Outputs:**
- `internal/proxy/server.go`
**What to do / Specifications:**
1. Start the HTTP server on configured address (default `127.0.0.1:8080`).
2. Implement `GET /v1/models` returning a list of model aliases.
3. Implement `GET /health` with runtime health stats.
4. Stream completions when `stream: true` is configured in incoming JSON using Server-Sent Events (SSE).
**Done when:**
* HTTP proxy responds correctly to completions, models, and health routes.
* Streaming request tests verify that data packets flow immediately without buffering.
**Summary report:** [20260617_142100_T-009_proxy_server.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_142100_T-009_proxy_server.md)

---

### [x] T-010 — Graceful Shutdown & Context Management
**Goal:** Coordinate proxy server shutdown to complete in-flight transactions and flush final metrics to state.json.
**Depends on:** T-009
**Unblocks:** T-011
**Outputs:**
- Lifecycle hooks in `internal/proxy/server.go`
**What to do / Specifications:**
1. Implement `sync.WaitGroup` to monitor active request handlers.
2. Intercept context cancellation or terminate signals (OS interrupts).
3. Shut down the HTTP listener using `Shutdown` with a 5s deadline.
4. Wait for in-flight handlers, write final `state.json` atomically, and exit.
**Done when:**
* Server shuts down cleanly under SIGINT.
* Dynamic state is saved successfully before exit.
**Summary report:** [20260617_142655_T-010_graceful_shutdown.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_142655_T-010_graceful_shutdown.md)

---

### [x] T-011 — Cobra Subcommands
**Goal:** Implement the Cobra-based command line interface for non-interactive controls.
**Depends on:** T-003, T-004, T-005, T-010
**Unblocks:** T-012
**Outputs:**
- `main.go`
- `cmd/root.go`
- `cmd/serve.go`
- `cmd/addkey.go`
- `cmd/testkeys.go`
- `cmd/status.go`
- `cmd/logs.go`
- `cmd/version.go`
**What to do / Specifications:**
1. Build Cobra root command: if no arguments are provided, launch the TUI.
2. Implement subcommands: `serve` (runs proxy daemon without TUI), `add-key` (interactive inline input), `test-keys` (run probe test on all keys), `status` (print plaintext summary), `version`, and `logs` (plain-text tailing of proxy log output).
**Done when:**
* Standard CLI commands compile and respond with appropriate usage info.
* Configuration paths are properly wired into flags.
**Summary report:** [20260617_144629_T-011_cobra-subcommands.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_144629_T-011_cobra-subcommands.md)

---

### T-012 — Lipgloss Styles & Main TUI App Model
**Goal:** Initialize the Charm Bubble Tea v2 structure, styles, tab navigation, and root layouts.
**Depends on:** T-011
**Unblocks:** T-013, T-014, T-015, T-016, T-017
**Outputs:**
- `internal/tui/app.go`
- `internal/tui/styles/styles.go`
**What to do / Specifications:**
1. Import Bubble Tea v2 and Lip Gloss v2.
2. Define a base tab-switching model matching tabs: Dashboard, Providers, Routes, Logs.
3. Configure common Lipgloss layouts, colors (green for active, yellow for cooling, red for error), and spacing.
4. Enforce alternate screen buffer use.
**Done when:**
* Main TUI application compiles and starts in alternate screen buffer.
* Keyboard inputs (`q`, `ctrl+c`, `tab` / brackets shortcuts) navigate successfully.
**Summary report:** [20260617_151800_T-012_tui-app-root.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_151800_T-012_tui-app-root.md)

---

### T-013 — Dashboard View
**Goal:** Implement the primary dashboard showing server status, metrics, and key pool status.
**Depends on:** T-012
**Unblocks:** T-018
**Outputs:**
- `internal/tui/views/dashboard.go`
**What to do / Specifications:**
1. Display server running/stopped toggle with `s` shortcut.
2. List server address, total requests today, requests/minute.
3. Render key health status table (`Provider | Model | Status | RPM used | RPD used | Last used`).
4. Apply 1-second ticks (`tea.Tick`) to refresh active counters dynamically.
**Done when:**
* Dashboard renders live stats correctly.
* Keyboard controls toggle server state successfully.
**Summary report:** [20260617_151800_T-013_dashboard-view.md](file:///d:/Codes/oapi/docs/reports/summary/20260617_151800_T-013_dashboard-view.md)

---

### T-014 — Providers View & Connection Test Trigger
**Goal:** Create the two-panel provider view showing key configurations, connection testing, and deletion.
**Depends on:** T-005, T-012
**Unblocks:** T-015, T-018
**Outputs:**
- `internal/tui/views/providers.go`
**What to do / Specifications:**
1. Build two-panel layout: Left lists keys, Right details selected key.
2. Implement key deletion with confirmation prompt.
3. Implement bulk import from JSON (prompt user for path, parse keys, trigger validation).
4. Bind test connection triggers (`t` for selected key, `T` for all keys).
**Done when:**
* Provider panel supports list navigation and deletion.
* Bulk import successfully parses JSON keys and refreshes list.
**Summary report:** `docs/reports/summary/{timestamp}_T-014_providers_view.md`

---

### T-015 — Add/Edit Key Form (Huh)
**Goal:** Embed the `charmbracelet/huh` form in the TUI to edit or add new keys.
**Depends on:** T-014
**Unblocks:** T-018
**Outputs:**
- Inline form handling in `internal/tui/views/providers.go` (or related file)
**What to do / Specifications:**
1. Build Huh form: Provider dropdown, Model text field/dropdown, API key password input, numeric inputs for RPM, RPD, TPM.
2. Save inputs to configuration (utilizing T-003 atomic writing).
3. Mask API keys in display using `sk-...xxxx` formats.
**Done when:**
* Forms validate input and successfully save keys to `oapi.yaml`.
* Masking prevents printing sensitive credentials in plaintext.
**Summary report:** `docs/reports/summary/{timestamp}_T-015_key_form.md`

---

### T-016 — Routes View & Editor
**Goal:** Display proxy route policies and implement a drag-and-drop/reorder route slot editor.
**Depends on:** T-012
**Unblocks:** T-018
**Outputs:**
- `internal/tui/views/routes.go`
**What to do / Specifications:**
1. Render route settings (Model Alias, Name, primary chain list, fallback pool).
2. Implement slot reordering (`↑`/`↓` keys), adding (`+ Add Slot`/`+ Add Fallback`), and deletion (`x` key).
3. Save modified route arrays atomically.
**Done when:**
* User can navigate route configurations.
* Reordering slots correctly adjusts primary chain list order and persists config.
**Summary report:** `docs/reports/summary/{timestamp}_T-016_routes_view.md`

---

### T-017 — Logs View
**Goal:** Implement the scrolling real-time logs panel with search filtering and text buffer export.
**Depends on:** T-012
**Unblocks:** T-018
**Outputs:**
- `internal/tui/views/logs.go`
**What to do / Specifications:**
1. Connect proxy logger channel to TUI log view list.
2. Cap memory buffer at 500 lines.
3. Build pause scrolling toggle (`p`), search query input (`/`), and export to text file (`e`).
**Done when:**
* Logs scroll automatically as requests arrive.
* Filters isolate requests and export creates plain-text logs accurately.
**Summary report:** `docs/reports/summary/{timestamp}_T-017_logs_view.md`

---

### T-018 — Integration and Verification tests
**Goal:** Run end-to-end integration tests and verify full key rotation under mock rate limits.
**Depends on:** T-005, T-007, T-013, T-014, T-015, T-016, T-017
**Unblocks:** Phase end
**Outputs:**
- `tests/rotation_test.go`
- `tests/limiter_test.go`
- `tests/proxy_test.go`
- `tests/config_test.go`
**What to do / Specifications:**
1. Write `rotation_test.go` to verify chain traversal and fallback under pool exhaustion.
2. Write `limiter_test.go` to assert sliding window behavior, RPD resets, and check for memory leaks (10k iterations).
3. Write `proxy_test.go` using mock HTTP server to test response codes (200, 429, 500) and headers.
4. Write `config_test.go` to verify atomic writes and JSON/YAML marshalling.
5. Run full test suites via `go test ./...`.
**Done when:**
* All unit and integration tests compile and pass.
* Memory growth verification checks are clean.
**Summary report:** `docs/reports/summary/{timestamp}_T-018_integration_tests.md`

---

### ✅ Group 0 Validation Checkpoint
[x] T-001 summary written
[x] Workspace initialized, `go.mod` exists, `.gitignore` excludes secrets.

### ✅ Group 1 Validation Checkpoint
[x] T-002, T-003, T-004, T-005 summaries written
[x] Unit tests for YAML/JSON atomic loading and saving pass.
[x] Connection test utility handles mocked 200/401/429/500 scenarios cleanly.

### ✅ Group 2 Validation Checkpoint
[x] T-006, T-007, T-008 summaries written
[x] Limiter sliding window keeps memory footprint bounded.
[x] Routing engine round-robin indices work correctly across multiple active slots.
[x] Google query param auth works and Cerebras reasoning effort fields are injected.

### ✅ Group 3 Validation Checkpoint
[x] T-009, T-010 summaries written
[x] Chat completion forwarding and SSE streaming proxy works.
[x] SIGINT server shutdown terminates gracefully under 5 seconds.

### ✅ Group 4 Validation Checkpoint
[x] T-011 summaries written
[x] Headless serve command and status check execute as expected from CLI interface.

### ✅ Group 5 Validation Checkpoint
[ ] T-012 (Done), T-013 (Done), T-014 (Done), T-015 (Done), T-016, T-017 summaries written
[x] Live dashboard refreshes stats every second.
[x] Key deletion and import from JSON are fully verified.
[ ] Routes editor reorders slots using key bindings.
[ ] Logs search filter isolates matching log lines and exports correctly.

### ✅ Group 6 Validation Checkpoint
[ ] T-018 summary written
[ ] All automated tests pass successfully (`go test ./...` returns PASS).

---

## Appendix: Dependency Order for Sequential Execution

```
T-001
  ├─ T-002
  │    └─ T-003 ───┐
  │          ├─────┼─── T-006
  │          │     │      └─ T-007
  │          │     │           └─ T-008
  │          │     │                └─ T-009
  │          │     │                     └─ T-010
  │          │     │                          └─ T-011
  │          │     │                               └─ T-012
  │          │     │                                    ├─ T-013 ──┐
  │          │     │                                    ├─ T-014 ──┼─ T-018
  │          │     │                                    ├─ T-015 ──┤
  │          │     │                                    ├─ T-016 ──┤
  │          │     │                                    └─ T-017 ──┘
  ├─ T-004 ──┘     │
  └─ T-005 ────────┴───────────────────────────────────────────────┘
```

**Recommended sequential order for a single agent:**
T-001 → T-002 → T-003 → T-004 → T-005 → T-006 → T-007 → T-008 → T-009 → T-010 → T-011 → T-012 → T-013 → T-014 → T-015 → T-016 → T-017 → T-018
