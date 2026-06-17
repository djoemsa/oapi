# oapi 🚀

`oapi` is an OpenAI-compatible reverse proxy designed to run locally. It manages API key rotation, handles rate limits, monitors token usage, and provides an interactive terminal user interface (TUI) built using Bubble Tea v2 (Lip Gloss & Huh).

---

## Features

- **🔄 Key Pool Rotation & Auto-Routing**:
  - Rotates API keys within a pool based on rate limit availability and health status.
  - Supports an automatic routing optimization engine (`[o] auto-optimize`) that partitions keys into capability tiers (large vs fast), ranks them by RPM limits, and generates optimized fallback chains.
- **🛡️ Rate Limiting**: Tracks RPM (Requests Per Minute), RPD (Requests Per Day), and TPM (Tokens Per Minute) limit metrics.
- **🔌 Multi-Provider Support**: Supports OpenAI, Anthropic, Google Gemini (OpenAI compatibility endpoint), Groq, Cerebras, GitHub Models, OpenRouter, Mistral, NVIDIA NIM, OpenCode Zen, and Cohere.
- **🖥️ Bubble Tea v2 TUI**:
  - **Dashboard**: Real-time request telemetry, active keys count, rate limit status, and overall throughput.
  - **Providers View**: Monitor individual LLM providers and trigger connection tests.
  - **Add/Edit Keys**: Interactive forms using `charmbracelet/huh` to add or edit API keys.
  - **Routes Editor**: View and modify configurations and routing mappings live. Includes a step-by-step Route Creation Wizard and an Auto-Routing Optimization mode.
  - **Logs Panel**: Interactive, non-blocking logs view with viewport scrolling.
- **🧪 Comprehensive Test Suite**: Includes integration tests verifying endpoints, streaming completions, rotation logic, graceful shutdown, and memory leak regression protection.

---

## Tech Stack

- **Language**: Go 1.26+
- **CLI Framework**: [Cobra](https://github.com/spf13/cobra)
- **TUI & Forms**: [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), [Huh](https://github.com/charmbracelet/huh), [Bubbles](https://github.com/charmbracelet/bubbles)
- **Configuration**: YAML/JSON state management

---

## Installation & Setup

### Prerequisites
Make sure you have [Go](https://go.dev/doc/install) (version 1.26 or higher) installed on your machine.

### Run the Setup & Verify
Clone or navigate to the repository directory and run the test suite to verify the setup:
```bash
go test ./...
```

---

## Running the Application

### 1. Interactive TUI Mode (Default)
To launch the proxy server and the interactive Bubble Tea terminal dashboard, run:
```bash
go run main.go
```
*Note: If no configuration file exists yet, the application will prompt you to configure or add a key.*

### 2. Headless Mode (Daemon)
To start the proxy server in headless mode (without the interactive TUI UI) routing requests in the background:
```bash
go run main.go serve
```
You can override the default server port using the `--port` flag:
```bash
go run main.go serve --port 9090
```

---

## CLI Commands Reference

`oapi` supports several subcommands under the main CLI:

| Command | Description |
|---------|-------------|
| `go run main.go` | Starts the TUI + proxy server. |
| `go run main.go serve` | Starts the proxy server in headless mode. |
| `go run main.go add-key` | Interactively adds a new LLM provider API key via terminal forms. |
| `go run main.go delete-key` | Deletes a configured API key (pass key ID or select interactively). |
| `go run main.go status` | Displays the current status of the proxy server and key pool. |
| `go run main.go test-keys` | Validates all API keys currently configured in the pool. |
| `go run main.go config` | Views or edits the proxy configurations. |
| `go run main.go logs` | Streams the proxy runtime log output. |
| `go run main.go version` | Shows the application version info. |

### Global Flags
- `--config <path>`: Path to a custom config file (defaults to `oapi.yaml` in the user's config directory).
- `--port <number>`: Override the default port specified in the config file.

---

## Configuration

The application stores its configuration in `oapi.yaml`. Below is a sample configuration structure:

```yaml
server:
  port: 8080
  host: "127.0.0.1"
keys:
  - id: "openai_1"
    provider: "openai"
    model: "gpt-4o"
    api_key: "sk-proj-..."
    rpm_limit: 500
    rpd_limit: 10000
    tpm_limit: 90000
    status: "active"
```

---

## Development & Testing

Run all unit and integration tests using:
```bash
go test -v ./...
```
The integration tests (`tests/proxy_test.go`) cover key components:
- Upstream completion routing & response rewriting.
- Streaming responses with Server-Sent Events (SSE).
- Dynamic error handling (handling 500 errors gracefully).
- Memory growth regression testing under high-throughput loops.
