# UI Design Specification

## Layout

┌─────────────────────┬───────────────────────────┬───────────────────────────┐
│ Namespace List       │ Pod List                  │ Detail / Diagnostics Pane │
└─────────────────────┴───────────────────────────┴───────────────────────────┘


Each pane is navigable. User switches panes with `Tab`.

---

## Pane Responsibilities

### Namespace Pane
- List namespaces
- Filter `/`
- Health summary (`h`)

### Pod Pane
- List pods in selected namespace
- Filter `/`
- Sort `s`
- Open detail (`Enter`)
- Diagnostics (`x`)
- Logs (`l`)

### Detail Pane
- Pod details
- Diagnostics results
- Suggested commands
- Scrollable

---

## Keybindings

### Global
- `q` – quit
- `?` – help
- `Tab` – switch pane

### Namespace
- `/` – filter
- `Enter` – load pods
- `h` – namespace health

### Pods
- `/` – filter
- `s` – sort
- `d` or `Enter` – detail
- `x` – diagnostics
- `l` – logs

### Logs
- `↑ ↓` – scroll
- `c` – switch container
- `f` – follow
- `Esc` – back

---

## Styling

Define a shared `theme.go`:
- `ColorOK`
- `ColorWarn`
- `ColorError`
- Border style
- Padding rules

