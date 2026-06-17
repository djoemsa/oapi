# PRD: `oapi` — OpenAI-Compatible API Key Rotation Proxy

> **Delegation target:** AI coding agent  
> **Language:** Go 1.23+  
> **Purpose:** Personal-use local proxy between LunaTranslator and multiple free-tier LLM providers, with key rotation, rate limit tracking, and a Bubble Tea TUI.

---

## 1. Project Identity

| Field | Value |
|---|---|
| Binary name | `oapi` |
| Invocation | Typing `oapi` in terminal opens full interactive TUI |
| Role | OpenAI-compatible reverse proxy with key pool management |
| Target client | LunaTranslator (any OpenAI-compatible client works) |
| Deployment | Single binary, runs locally on user's machine |

---

## 2. Problem Statement

The user translates Visual Novel dialog lines using LunaTranslator, which fires rapid sequential translation requests. Multiple free-tier LLM API providers each have different RPM and RPD caps. Managing keys manually across providers is error-prone and causes dropped translations when a single key hits its limit.

The solution is a local proxy that:
- Absorbs all requests from LunaTranslator on one unified endpoint
- Tracks per-key RPM and RPD usage in real time
- Rotates to the next available key when limits are approached
- Falls back to secondary providers when the primary chain is exhausted

---

## 3. Tech Stack

| Component | Library / Tool | Version |
|---|---|---|
| Language | Go | 1.23+ |
| TUI framework | `github.com/charmbracelet/bubbletea` | v2.0.7+ |
| TUI styling | `github.com/charmbracelet/lipgloss` | v2.0.4+ |
| TUI components | `github.com/charmbracelet/bubbles` | v2.1.0+ |
| TUI forms | `github.com/charmbracelet/huh` | latest v2 |
| CLI subcommands | `github.com/spf13/cobra` | latest |
| HTTP proxy | `net/http` | stdlib |
| Config format | YAML via `github.com/go-yaml/yaml` | v3 |
| Runtime state | Single `state.json` file alongside config | — |
| HTTP client | `net/http` with per-provider timeouts | stdlib |

**Critical — Charmbracelet v2**: Bubble Tea, Lip Gloss, and Bubbles all have v2 major releases as of 2026. Do **not** use v0/v1 import paths. v2 introduces layers/compositing, renderer improvements, and breaking API changes from v1. Refer to the official Charm v2 upgrade guide before writing any TUI code.

**huh**: Use `charmbracelet/huh` for all add/edit forms (key entry, route editor). It handles input validation, dropdowns, and confirmation dialogs natively and eliminates manual form state management in Bubble Tea.

**Cobra**: Use for subcommand dispatch (`serve`, `add-key`, `test-keys`, `status`, `version`, `logs`). The root command with no subcommand invokes the TUI.

No database. No external services. No Docker required.

---

## 4. Architecture Overview

```
LunaTranslator
      │
      │  POST /v1/chat/completions
      │  Authorization: Bearer <dummy-key>
      ▼
┌─────────────────────┐
│   oapi proxy server  │  :8080 (configurable)
│   (net/http)         │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   Rotation Engine    │
│   - Route matching   │
│   - Key selection    │
│   - Limit checking   │
│   - 429 handling     │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────────────────────────────┐
│              Provider Slots                  │
│  Groq → Google → GitHub → Cerebras → ...    │
│  (each slot has N keys, round-robin)         │
└─────────────────────────────────────────────┘
```

The TUI and the proxy server run as **concurrent goroutines** within the same process. The TUI renders live stats by reading shared state under a mutex.

---

## 5. Provider Registry (Hardcoded Defaults)

These are baked in as Go constants and used as defaults when a user adds a key for a known provider. Users can override RPM/RPD per-provider in config.

| Provider ID | Base URL | Default RPM | Default RPD | Reset Behavior | Notes |
|---|---|---|---|---|---|
| `groq` | `https://api.groq.com/openai/v1` | 30 | 14400 | Rolling 24h | Per-model RPD varies — see model table below |
| `google` | `https://generativelanguage.googleapis.com/v1beta/openai` | 15 | 1500 | Midnight PT | Use `gemini-2.5-flash` or `gemini-2.5-flash-001`; do not use legacy 1.5 endpoints |
| `cerebras` | `https://api.cerebras.ai/v1` | 5 | 1000 | Continuous bucket | Suppress reasoning via `reasoning_effort`; see Implementation Note 4 |
| `github` | `https://models.inference.ai.azure.com` | 15 | 150 | Rolling 24h | |
| `openrouter` | `https://openrouter.ai/api/v1` | 20 | 50 | Midnight PT | Per free model |
| `mistral` | `https://api.mistral.ai/v1` | 300 | — | None (TPM-based) | No RPD, only RPM/TPM |

### Per-Model RPD Overrides (Groq)

When a user selects a Groq model, the following RPD defaults apply automatically:

| Model ID | RPM | RPD | TPM | TPD |
|---|---|---|---|---|
| `llama-3.1-8b-instant` | 30 | 14400 | 6000 | 500000 |
| `llama-3.3-70b-versatile` | 30 | 1000 | 12000 | 100000 |
| `openai/gpt-oss-120b` | 30 | 1000 | 8000 | 200000 |
| `openai/gpt-oss-20b` | 30 | 1000 | 8000 | 200000 |
| `meta-llama/llama-4-scout-17b-16e-instruct` | 30 | 1000 | 30000 | 500000 |

### Recommended Models per Provider

These are suggestions shown in the TUI when a user adds a key:

| Provider | Recommended Model(s) |
|---|---|
| `groq` | `llama-3.1-8b-instant`, `llama-3.3-70b-versatile` |
| `google` | `gemini-2.5-flash`, `gemini-2.5-flash-001` |
| `cerebras` | `gpt-oss-120b`, `zai-glm-4.7` |
| `openrouter` | `openai/gpt-oss-120b:free`, `meta-llama/llama-3.3-70b-instruct:free`, `qwen/qwen3-next-80b-a3b-instruct:free` |
| `mistral` | `mistral-small-2506`, `ministral-3b-2512` |
| `github` | `gpt-4o-mini`, `Meta-Llama-3.1-70B-Instruct` |

---

## 6. Core Concepts & Terminology

### Key
A single API credential belonging to one provider. Has:
- Unique ID (auto-generated)
- Raw API key string (stored in config)
- Assigned provider
- Assigned model
- Status: `untested` | `active` | `cooling_rpm` | `cooling_rpd` | `disabled` | `error`
- Runtime counters: requests this minute, requests today

### Slot
A reference to a provider within a routing chain. A slot maps to **all active keys belonging to that provider**. Keys within a slot rotate round-robin.

### Route
A named routing policy with:
- A model alias (what LunaTranslator sends as the `model` field)
- A **primary chain** (ordered list of slots)
- A **fallback pool** (unordered set of slots, activated when chain is exhausted)

### Policy
The complete set of named routes. A wildcard route `*` catches all unmatched model names.

---

## 7. Rotation Engine — Detailed Semantics

This is the core logic of the application.

### 7.1 Request Lifecycle

```
1. Request arrives at proxy
2. Extract model name from request body
3. Find matching Route (exact model alias match, then wildcard)
4. Walk Primary Chain left-to-right:
   a. For each Slot:
      i.  Collect keys for this provider with status == active
      ii. Filter: rpm_ok AND rpd_ok (see 7.2)
      iii. If filtered list is non-empty:
           - Select key via round-robin (see 7.3)
           - Forward request to provider (see 7.4)
           - On success: increment counters, return response to client
           - On 429: mark key cooling (see 7.5), retry with next key in slot
           - On other 4xx/5xx: log error, try next key in same slot
           - If all keys in slot exhausted during retry: move to next Slot
      iv. If filtered list is empty: move to next Slot
5. If end of Primary Chain reached with no successful response:
   a. Activate Fallback Pool
   b. Collect all active, non-rate-limited keys across all fallback slots
   c. Round-robin across them (ignoring slot boundaries)
   d. Same retry logic as above
6. If Fallback Pool also exhausted:
   a. Return HTTP 429 to client
   b. Include Retry-After header = earliest cooling expiry across all keys
   c. Log event
```

### 7.2 Rate Limit Checking

**RPM check (sliding window):**
```
maintain a ring buffer of N timestamps where N = RPM limit
key is rpm_ok if: len(timestamps_in_last_60s) < RPM_limit
```

**RPD check (daily counter):**
```
key is rpd_ok if: requests_today < RPD_limit
```

**TPM check (optional, if configured):**
```
maintain a rolling token sum over last 60s
key is tpm_ok if: tokens_in_last_60s + estimated_tokens < TPM_limit
estimated_tokens = prompt_tokens + min(max_tokens_in_request, max_completion_tokens_default)
```

For Cerebras specifically: token estimation must use `max_completion_tokens` from the request body, defaulting to 60 if absent or exceeding 60.

### 7.3 Key Selection Within a Slot

Default strategy: **round-robin** using a per-slot atomic integer index.

```
selected_key = slot.keys[slot.index % len(slot.keys)]
slot.index++
skip if key is not active/available, advance index until one found or all exhausted
```

### 7.4 Request Forwarding

- Strip the incoming `Authorization` header
- Inject provider's real API key as `Authorization: Bearer <key>`
- Rewrite `Host` header to provider's base URL host
- Rewrite request URL: replace proxy prefix with provider base URL
- For Google: translate OpenAI format to Gemini REST format (or use Gemini's OpenAI-compatible endpoint)
- Stream-through: if client requests `stream: true`, proxy streams the response
- Forward all other headers unchanged
- Inject `max_completion_tokens: 60` for Cerebras if not already set or if set higher than 60

### 7.5 Key Cooling on 429

When a 429 is received from a provider:

1. Read `Retry-After` header (in seconds) from provider response
2. If `Retry-After` present: mark key as `cooling_rpm` until `now + retry_after`
3. If absent: mark key as `cooling_rpm` until start of next 60s window
4. If RPD header indicates daily exhaustion (`x-ratelimit-remaining-requests-day == 0` on Groq/Cerebras): mark as `cooling_rpd` until next midnight

### 7.6 Cooling Recovery

A background goroutine runs every 5 seconds:
- Check all keys with status `cooling_rpm`
- If cooling expiry has passed: set status back to `active`
- Check all keys with status `cooling_rpd`
- If past midnight (local time): reset `requests_today` counter, set status to `active`
- For Groq specifically: RPD resets on a rolling 24h window — track `first_request_of_day_timestamp` and reset when `now - first_request > 86400s`

### 7.7 Route Configuration Example

The user defines this in the TUI. Internally stored as:

```yaml
routes:
  - name: default
    model_alias: "*"
    chain:
      - provider: groq
        model: llama-3.1-8b-instant
      - provider: google
        model: gemini-2.5-flash
      - provider: groq
        model: llama-3.3-70b-versatile
      - provider: github
        model: Meta-Llama-3.1-70B-Instruct
      - provider: cerebras
        model: gpt-oss-120b
      - provider: cerebras
        model: zai-glm-4.7
      - provider: openrouter
        model: openai/gpt-oss-120b:free
    fallback:
      - provider: mistral
        model: mistral-small-2506
```

Note: the same provider can appear multiple times in the chain (e.g. Groq with two different models = two separate slots).

---

## 8. Config Schema

Config file location (in order of precedence):
1. `--config` flag path
2. `./oapi.yaml` (current directory)
3. `~/.config/oapi/config.yaml`

```yaml
server:
  port: 8080
  host: "127.0.0.1"
  dummy_api_key: "oapi-key"   # LunaTranslator uses this value as its API key

keys:
  - id: "groq_1"               # auto-generated UUID on creation
    provider: "groq"
    model: "llama-3.1-8b-instant"
    api_key: "gsk_xxxxxxxxxxxx"
    rpm_limit: 30              # overrides provider default if set
    rpd_limit: 14400
    tpm_limit: 6000            # optional
    status: "active"           # active | disabled

  - id: "cerebras_1"
    provider: "cerebras"
    model: "gpt-oss-120b"
    api_key: "csk_xxxxxxxxxxxx"
    rpm_limit: 5
    rpd_limit: 1000
    tpm_limit: 30000
    max_completion_tokens: 60  # cerebras-specific cap
    status: "active"

routes:
  - name: "default"
    model_alias: "*"
    chain:
      - provider: "groq"
        model: "llama-3.1-8b-instant"
      - provider: "google"
        model: "gemini-2.5-flash"
      - provider: "cerebras"
        model: "gpt-oss-120b"
    fallback:
      - provider: "mistral"
        model: "mistral-small-2506"

providers:
  # Optional overrides of built-in defaults
  groq:
    reset_behavior: "rolling_24h"
  google:
    reset_behavior: "midnight_pt"
  cerebras:
    reset_behavior: "continuous"
  mistral:
    reset_behavior: "none"
```

### Runtime State File (`state.json`)

Written alongside the config. Not hand-edited by user. Tracks:

```json
{
  "keys": {
    "groq_1": {
      "requests_this_minute": 12,
      "requests_today": 340,
      "tokens_this_minute": 4800,
      "cooling_until": null,
      "last_used": "2025-01-01T12:00:00Z",
      "first_request_today": "2025-01-01T08:00:00Z"
    }
  },
  "total_requests_today": 352,
  "uptime_start": "2025-01-01T08:00:00Z"
}
```

---

## 9. TUI Design

Invoked by running `oapi` with no arguments. Takes over the terminal (alternate screen buffer). Quit with `q` or `Ctrl+C`.

### 9.1 Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  oapi  [D]ashboard  [P]roviders  [R]outes  [L]ogs              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   <active view content>                                          │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│  Server: ● running  127.0.0.1:8080   Req/min: 4   Today: 1,240 │
└─────────────────────────────────────────────────────────────────┘
```

- Top bar: app name + tab navigation (keyboard shortcuts in brackets)
- Center: active view, changes per tab
- Bottom bar: always-visible server status strip (updates every second)

### 9.2 Dashboard View (`d`)

Displayed on startup. Shows:
- Server status (running/stopped) with toggle button (`s` to start/stop)
- Listening address
- Total requests today / this minute
- Key health table: one row per key, columns: `Provider | Model | Status | RPM used | RPD used | Last used`
- Color coding: green = active, yellow = cooling, red = disabled/error

### 9.3 Providers View (`p`)

Two-panel layout:
- Left panel: list of configured keys (navigable with arrow keys)
- Right panel: detail for selected key

**Key list columns:** `# | Provider | Model | Status | API Key (masked)`

**Actions (bottom of view):**
- `a` — Add new key (opens huh form)
- `d` — Delete selected key (confirm prompt)
- `e` — Edit selected key
- `t` — Test connection for selected key
- `T` — Test all keys
- `i` — Bulk import keys from JSON or CSV file (path prompt, see import format below)

**Bulk Import Format** (JSON):
```json
[
  {"provider": "groq", "model": "llama-3.1-8b-instant", "api_key": "gsk_xxx"},
  {"provider": "google", "model": "gemini-2.5-flash", "api_key": "AIza_xxx"}
]
```
Each imported key gets an auto-generated ID and status `untested`. User is prompted to run connection test immediately after import.

**Add/Edit Key Form (inline, replaces right panel):**
```
Provider:    [ groq ▼ ]       (dropdown of known providers + "custom")
Model:       [ llama-3.1-8b-instant ▼ ]   (populated from registry)
API Key:     [ ________________________________ ]
RPM Limit:   [ 30  ]  (pre-filled from provider defaults)
RPD Limit:   [ 14400 ]
TPM Limit:   [ 6000  ]  (optional)

[ Save ]  [ Cancel ]  [ Test & Save ]
```

**Connection Test Flow:**
1. Show spinner: "Testing..."
2. Fire minimal request: `{"model": "<model>", "messages": [{"role":"user","content":"Hi"}], "max_tokens": 5}`
3. On 200: mark status `active`, show "✓ Connection successful"
4. On 401/403: mark status `error`, show "✗ Invalid API key"
5. On 429: mark status `cooling_rpm`, show "⚠ Key valid but rate limited"
6. On other error: show raw error message, mark `error`

### 9.4 Routes View (`r`)

Shows all configured routes in a list. Select a route to edit.

**Route Editor (takes over full view):**

```
Route Name:   [ default              ]
Model Alias:  [ *                    ]  (* = catch all)

PRIMARY CHAIN  (ordered, drag to reorder)        FALLBACK POOL
──────────────────────────────────────────────   ─────────────────
  1  groq       llama-3.1-8b-instant              mistral  small-2506
  2  google     gemini-2.5-flash
  3  groq       llama-3.3-70b-versatile
  4  cerebras   gpt-oss-120b

  [+ Add Slot]                                    [+ Add Fallback]

[ Save Route ]  [ Cancel ]  [ Delete Route ]
```

Navigation: arrow keys to select a chain slot, `↑`/`↓` to reorder, `x` to remove, Enter to edit slot's provider/model.

### 9.5 Logs View (`l`)

Live scrolling log of proxy activity. Updates in real time.

```
[12:04:01] → groq/llama-3.1-8b-instant (key: groq_1)  200  143ms  120 tok
[12:04:02] → groq/llama-3.1-8b-instant (key: groq_1)  200  98ms   88 tok
[12:04:03] → groq/llama-3.1-8b-instant (key: groq_2)  429  12ms   — [cooling]
[12:04:03]   ↳ retry → google/gemini-2.5-flash (key: google_1)  200  201ms  95 tok
[12:04:04] → groq/llama-3.1-8b-instant (key: groq_1)  200  110ms  102 tok
```

Format: `[time] → provider/model (key: id)  status  latency  tokens [note]`

Max 500 lines in memory, auto-scroll to bottom, `p` to pause scroll, `/` to filter by provider or status code, `e` to export visible log buffer to `oapi-log-<timestamp>.txt` in the current directory.

---

## 10. Proxy Server

### Endpoints

```
POST   /v1/chat/completions   ← primary translation endpoint
GET    /v1/models             ← returns a static model list for client compatibility
GET    /health                ← returns JSON {status, uptime, keys_active, requests_today}
```

`/v1/models` returns a hardcoded OpenAI-compatible model list containing all model aliases defined in active routes. This is needed because some clients (including some LunaTranslator configurations) query `/v1/models` on startup to populate dropdowns. Return a static JSON response; do not forward to any provider.

`/health` is unauthenticated and intended for local diagnostics. Does not require the dummy API key.

Any value in `Authorization` header is accepted if `dummy_api_key` in config is empty. If set, it must match exactly (simple bearer check, no JWT). The `/health` endpoint bypasses this check.

### Behavior

- Accept only `POST /v1/chat/completions` initially (other endpoints return 404)
- Parse request body as JSON, extract `model` and `messages` fields
- Do **not** validate the full schema — pass the body through verbatim, only modifying:
  - `Authorization` header (replaced with real provider key, or stripped for Google)
  - Cerebras-specific reasoning suppression fields (see Implementation Note 4)
  - Request URL (rewritten to provider's base URL)
- Support both streaming (`"stream": true`) and non-streaming
- For streaming: pipe SSE chunks from provider directly to client
- Set timeouts: connect 10s, response header 30s, total 120s

### Error Responses

All errors returned as OpenAI-compatible JSON:
```json
{
  "error": {
    "message": "All providers exhausted. Retry after 47 seconds.",
    "type": "rate_limit_error",
    "code": "all_providers_exhausted"
  }
}
```

---

## 11. CLI Flags (Non-TUI Mode)

When called with subcommands, `oapi` operates as a traditional CLI (no TUI):

```bash
oapi                          # opens TUI (default)
oapi serve                    # start proxy server without TUI (daemon mode)
oapi add-key                  # non-interactive: prompts for fields inline
oapi test-keys                # runs connection test on all configured keys, prints results
oapi status                   # prints server status + key health as plain text
oapi config                   # prints current config path and contents
oapi version                  # prints binary version and build info
oapi logs                     # tails the runtime log output (plain text, no TUI)
oapi --config ./custom.yaml   # use custom config file
oapi --port 9090              # override port (also works with TUI)
```

---

## 12. File Structure

```
oapi/
├── main.go                         # entry point, cobra root dispatch
├── go.mod
├── go.sum
│
├── cmd/
│   ├── root.go                     # cobra root: no args → launch TUI
│   ├── serve.go                    # `oapi serve` headless daemon
│   ├── addkey.go                   # `oapi add-key` inline CLI via huh
│   ├── testkeys.go                 # `oapi test-keys`
│   ├── status.go                   # `oapi status`
│   ├── logs.go                     # `oapi logs` tail mode
│   └── version.go                  # `oapi version`
│
├── internal/
│   ├── tui/
│   │   ├── app.go                  # root Bubble Tea v2 model, tab switching
│   │   ├── styles/
│   │   │   └── styles.go           # lipgloss v2 style definitions
│   │   └── views/
│   │       ├── dashboard.go        # Dashboard view
│   │       ├── providers.go        # Providers view + huh key form
│   │       ├── routes.go           # Routes view + route editor
│   │       └── logs.go             # Live logs view
│   │
│   ├── proxy/
│   │   ├── server.go               # net/http server, graceful shutdown via http.Server.Shutdown()
│   │   └── handler.go              # request handler, body parsing, SSE streaming
│   │
│   ├── rotation/
│   │   ├── engine.go               # chain walk, fallback activation
│   │   ├── pool.go                 # key pool, round-robin selector
│   │   └── limiter.go              # sliding window RPM, RPD counter, cooling state
│   │
│   ├── registry/
│   │   └── providers.go            # hardcoded provider defaults, model lists, base URLs
│   │
│   ├── config/
│   │   ├── config.go               # YAML load/save, struct definitions
│   │   └── state.go                # state.json atomic load/save
│   │
│   └── testutil/
│       └── probe.go                # connection test logic (used by TUI and CLI)
│
└── tests/
    ├── rotation_test.go            # unit tests: chain walk, round-robin, cooling logic
    ├── limiter_test.go             # unit tests: sliding window, RPD reset, memory growth
    ├── proxy_test.go               # integration tests: mock provider server, 429 handling
    └── config_test.go              # unit tests: YAML parse, atomic write, state round-trip
```

---

## 13. Non-Goals

These are explicitly out of scope for v1:

- Web UI (browser-based dashboard)
- Multi-user or team access
- Authentication beyond a simple dummy key check
- Persistent request history or analytics database
- Automatic API key registration or account creation
- Model response quality evaluation or routing by quality
- Token counting via tiktoken (use provider-returned token counts instead)
- Docker packaging (single binary is the deployment target)
- Windows service / systemd unit file (out of scope, user runs manually)

---

## 14. Security Notes

- **API keys in config file**: `oapi.yaml` contains plaintext API keys. The agent must generate a `.gitignore` file in the config directory excluding `oapi.yaml` and `state.json` if the project is initialized inside a git repo. Display a one-time warning in the TUI if the config file is detected inside a `.git` tree.
- **Key masking**: Never log or display full API key strings anywhere in the TUI or CLI output. Mask to `sk-...xxxx` (last 4 characters visible) in all views.
- **Local-only binding**: Default to `127.0.0.1`, never `0.0.0.0`, unless user explicitly sets `host: 0.0.0.0` in config with a confirmation prompt warning about network exposure.
- **Input sanitization**: Minimally validate JSON body from LunaTranslator before forwarding — reject non-JSON bodies with a 400 to avoid forwarding garbage to providers.

---

## 15. Testing Scope

The `tests/` directory must cover the following before the implementation is considered complete:

| Test File | What It Covers |
|---|---|
| `rotation_test.go` | Chain walk order, fallback activation, round-robin key selection, slot exhaustion |
| `limiter_test.go` | Sliding window RPM accuracy, RPD counter reset, memory growth check (run 10k iterations, assert no unbounded growth), cooling expiry timing |
| `proxy_test.go` | Mock HTTP provider server returning 200 / 429 / 500; verify correct key selection, header rewriting (especially Google ?key= param), streaming passthrough |
| `config_test.go` | YAML parse/write round-trip, atomic rename behavior, state.json persistence across restart |

Minimum: all rotation engine and limiter logic must be unit-testable without a network connection. Use interface abstraction on the HTTP client so tests can inject a mock.

---

## 16. Implementation Notes for Agent

1. **Concurrency**: proxy handler goroutines share key state with the TUI renderer. Use a `sync.RWMutex` on the key pool, but **never hold the lock during network I/O**. The pattern must be: acquire read lock → copy the key selection result → release lock → perform the HTTP forward request → acquire write lock → update counters → release lock. For hot-path counters (requests this minute, requests today), prefer `sync/atomic` integer operations over mutex-guarded struct fields entirely. This prevents TUI frame refreshes (1-second tick) from blocking proxy throughput or causing visible terminal stutter under request floods.

2. **TUI + server in same process**: Run the HTTP server in a goroutine before starting the Bubble Tea program. Pass a channel for log events from the proxy handler to the TUI logs view. Use `tea.Tick` at 1-second intervals to refresh live counters on the dashboard.

3. **Sliding window RPM**: Use a `[]time.Time` ring buffer of length `RPM_limit`. On each request, append current time. Filter to only times within last 60s. If `len(filtered) >= RPM_limit`, key is cooling. **Critical:** the background 5-second goroutine must explicitly reslice this buffer to drop all entries older than 60 seconds on every tick — do not let the slice grow unbounded. Correct pattern: `buf = buf[i:]` where `i` is the index of the first timestamp within the 60s window. Alternative for simpler RPM-only enforcement: `golang.org/x/time/rate` (token bucket, stdlib-adjacent) — acceptable for personal use but less precise for true sliding window semantics under bursty VN dialog loads.

4. **Cerebras reasoning overhead**: Both `gpt-oss-120b` and `zai-glm-4.7` emit internal chain-of-thought reasoning tokens inside their response generation. A hard token clamp will cut off the model mid-reasoning before the actual translation text is produced. Instead, suppress reasoning at the request level using the following **priority order** when forwarding to Cerebras:
   - **First**: inject `"reasoning_effort": "none"` (standard OpenAI-compatible parameter, confirmed working on both Cerebras models; use `"low"` if user wants minimal reasoning)
   - **Second** (for `zai-glm-4.7` specifically if `reasoning_effort` has no effect): inject `"disable_reasoning": true` or `"clear_thinking": true` — these are non-standard params, pass via `extra_body` if the client wrapper requires it
   - **Last resort**: set `max_completion_tokens` to a user-configurable ceiling (default 300 in config, not 60) to give the model room to finish output
   The config field `max_completion_tokens` on a Cerebras key entry is a user-configurable ceiling, never a hardcoded global constant.

5. **Config and state writes must be atomic**: All disk writes — both `oapi.yaml` (on key add/edit/delete) and `state.json` (every 30 seconds) — must use a write-then-rename pattern to prevent file corruption if the TUI triggers a config save at the same moment the background state saver fires. Pattern: write to a temp file (e.g. `oapi.yaml.tmp`) using `os.WriteFile`, then call `os.Rename("oapi.yaml.tmp", "oapi.yaml")`. On POSIX systems `os.Rename` is atomic at the filesystem level. Do not use `os.OpenFile` with truncation directly on the target file, as a crash mid-write produces a zero-byte or partial config.

6. **Graceful shutdown**: On `Ctrl+C` or `q` in TUI: call `http.Server.Shutdown(ctx)` with a 5-second context deadline to stop accepting new requests and wait for in-flight handlers to complete. Use a `sync.WaitGroup` in the handler to track in-flight requests independently — increment on request entry, decrement on exit, wait before writing final state. Write `state.json` atomically after WaitGroup drains, then exit.

7. **Context propagation**: Pass a root `context.Context` (cancelled on shutdown signal) to the HTTP server, all background goroutines (cooling recovery, state flush), and the TUI's quit handler. This ensures clean teardown without goroutine leaks. Use `context.WithCancel` at startup, call cancel in the shutdown path.

8. **Google provider**: Google's OpenAI-compatible endpoint (`/v1beta/openai`) accepts standard OpenAI request format. However, it does **not** authenticate via `Authorization: Bearer` headers. Instead, append the API key as a query parameter: rewrite the forwarded URL to `https://generativelanguage.googleapis.com/v1beta/openai/chat/completions?key=<api_key>`. Do not send an `Authorization` header to Google at all. Strip it from the forwarded request.

9. **OpenRouter**: Requires `HTTP-Referer` header (any value works, use `http://localhost`) and optionally `X-Title`. Inject these when forwarding to OpenRouter.

10. **Mistral**: No RPD limit — only TPM-based. Treat `cooling_rpd` as never triggering for Mistral. Only RPM/TPM limits apply.

11. **First run experience**: If config file does not exist on startup, open TUI directly to the Providers view with a welcome banner: "No keys configured yet. Press [a] to add your first key."
