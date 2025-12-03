# kubectl-medic

A fast, intuitive TUI (terminal UI) for diagnosing Kubernetes pod issues. Get intelligent diagnostics, suggested kubectl commands, and pod insights—all from your terminal.

## Features

- ✅ **Smart Diagnostics** - 16 built-in diagnostic rules detecting CrashLoopBackOff, OOM kills, image pull errors, scheduling failures, probe issues, and more
- ✅ **3-Pane Layout** - Browse namespaces, pods, and details side-by-side with responsive layout (auto-adjusts to terminal width)
- ✅ **Copy-Friendly Commands** - Get suggested kubectl commands formatted for easy triple-click copying
- ✅ **Pod Details & Logs** - View pod specs, events, diagnostics, and streaming logs in one interface
- ✅ **Namespace Health** - Aggregate health summaries showing issue counts across entire namespaces
- ✅ **Filtering & Sorting** - Quickly find pods with live filtering and multiple sort modes
- ✅ **Read-Only by Design** - Never modifies your cluster, safe for production use
- ✅ **Fast & Lightweight** - No external dependencies, single binary, low memory footprint

## Installation

### Quick Install (Go)

```bash
go install github.com/NlCKDEV/kubectl-medic@latest
```

### Build from Source

```bash
git clone https://github.com/NlCKDEV/kubectl-medic.git
cd kubectl-medic
go build -o kubectl-medic .
```

### As kubectl Plugin

To use as `kubectl medic`:

```bash
# After installing, ensure binary is named 'kubectl-medic'
mv kubectl-medic /usr/local/bin/kubectl-medic

# Verify plugin is detected
kubectl plugin list

# Run
kubectl medic
```

## Usage

```bash
# Launch kubectl-medic (uses current kubectl context)
kubectl-medic

# Or as kubectl plugin
kubectl medic

# Specify kubeconfig
kubectl-medic --kubeconfig ~/.kube/config

# Specify context
kubectl-medic --context production
```

## Keybindings

### Global
- `q` or `Ctrl+C` - Quit
- `?` - Toggle help
- `Tab` - Next pane
- `Shift+Tab` - Previous pane

### Namespace Pane
- `↑/↓` or `k/j` - Navigate
- `Enter` - Select namespace (loads pods)
- `/` - Filter namespaces
- `s` - Cycle sort mode (name ↑/↓, status)
- `h` - View namespace health summary

### Pods Pane
- `↑/↓` or `k/j` - Navigate
- `Enter` or `d` - Show pod details
- `x` - Run diagnostics
- `l` - View logs
- `/` - Filter pods
- `s` - Cycle sort mode (name, status, restarts, age)

### Details Pane
- `↑/↓` or `k/j` - Scroll content
- `PgUp/PgDn` - Page up/down
- `c` - Switch container (logs view)
- `f` - Toggle follow mode (logs view)
- `Esc` - Return to previous view

## Diagnostic Rules

kubectl-medic includes 16 diagnostic rules that analyze pod status, events, and configuration:

| Category | Detection |
|----------|-----------|
| **Crashes** | CrashLoopBackOff, OOMKilled, high restart counts |
| **Images** | ImagePullBackOff, ErrImagePull, invalid image names |
| **Scheduling** | Unschedulable, insufficient CPU/memory, node selectors |
| **Probes** | Readiness probe failures, liveness probe failures |
| **Storage** | Volume mount failures, PVC not found/bound |
| **Config** | Missing ConfigMap/Secret references |
| **Init Containers** | Init container crashes and failures |
| **Resources** | Node pressure (disk, memory, PID) |

Each diagnostic includes:
- Severity level (Error, Warning, Info)
- Human-readable explanation
- Suggested kubectl commands to investigate further

## Layout Modes

kubectl-medic adapts to your terminal width:

**3-Pane Mode** (width ≥ 130 cols):
```
┌──────────┬────────────┬──────────────┐
│ Namespaces│   Pods     │   Details    │
│  (28%)   │   (36%)    │    (36%)     │
└──────────┴────────────┴──────────────┘
```

**2-Pane Mode** (85 ≤ width < 130):
```
┌──────────┬───────────────────────────┐
│Namespaces│ Pods or Details (active)  │
│  (35%)   │         (65%)             │
└──────────┴───────────────────────────┘
```

**1-Pane Mode** (width < 85):
```
┌──────────────────────────────────────┐
│    Active Pane (full width)          │
└──────────────────────────────────────┘
```

## Performance

- **Windowed Rendering** - Only renders visible rows, handles 1000+ pods smoothly
- **Efficient API Calls** - Idempotent key handlers prevent API flooding
- **Low Memory** - Single-binary, minimal allocations
- **Fast Startup** - Loads namespaces in <100ms (typical cluster)

## Requirements

- Go 1.21+ (if building from source)
- kubectl configured with access to a Kubernetes cluster
- Required RBAC permissions:
  - `get`, `list` on `namespaces`
  - `get`, `list` on `pods`
  - `get`, `list` on `events`
  - `get` on `pods/log`

## Troubleshooting

### "Cannot connect to cluster"
Check kubectl connectivity:
```bash
kubectl cluster-info
kubectl config view
```

### "Permission denied" errors
Verify RBAC permissions:
```bash
kubectl auth can-i list namespaces
kubectl auth can-i list pods --all-namespaces
```

### Binary not found (as kubectl plugin)
Ensure `kubectl-medic` is in your PATH:
```bash
echo $PATH
which kubectl-medic
```

## Architecture

```
kubectl-medic/
├── main.go                          # Entry point
├── cmd/medic/root.go                # Cobra CLI
├── internal/
│   ├── kube/client.go               # Read-only K8s API wrapper
│   ├── state/state.go               # Centralized app state
│   ├── analysis/engine.go           # 16 diagnostic rules
│   ├── tui/                         # Bubble Tea components
│   │   ├── model.go                 # Main TUI model (layout, commands)
│   │   ├── namespace/model.go       # Namespace list pane
│   │   ├── pods/model.go            # Pod list pane (filtering, sorting)
│   │   └── details/model.go         # Details/diagnostics/logs/health
│   ├── theme/theme.go               # Lip Gloss styles
│   └── util/util.go                 # String helpers
└── doc/                             # Design specs & implementation notes
```

## Documentation

- [Project Overview](doc/OVERVIEW.md) - Architecture and design decisions
- [Requirements](doc/REQUIREMENTS.md) - Functional and non-functional requirements
- [Scope](doc/SCOPE.md) - v1.0 feature boundaries
- [Use Cases](doc/USE_CASES.md) - User workflows and scenarios
- [UI Design](doc/UI_DESIGN.md) - Layout and keybindings
- [Roadmap](doc/ROADMAP.md) - Future versions

## Contributing

Contributions welcome! Please:
1. Read the [specification docs](doc/) to understand design principles
2. Follow the existing code style (see `internal/analysis/engine.go` for examples)
3. Add tests for new diagnostic rules
4. Update documentation

## Philosophy

> **"A doctor who diagnoses, not a surgeon who performs operations"**

kubectl-medic is designed to:
- ✅ Provide insights and intelligent diagnostics
- ✅ Suggest kubectl commands for users to run
- ✅ Help users understand pod issues
- ❌ **Never** modify cluster resources
- ❌ **Never** auto-remediate issues

This read-only philosophy makes it safe for production clusters.

## License

MIT

## Credits

Built with:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [client-go](https://github.com/kubernetes/client-go) - Kubernetes Go client

---

**Status:** v1.0-ready • **Test Coverage:** 99% (engine), 100% (TUI panes) • **Build:** [![Go](https://img.shields.io/badge/go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
