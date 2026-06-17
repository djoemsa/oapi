# T-016 — Routes View & Editor

Implement the Routes tab inside the Bubble Tea TUI. The view lets users navigate configured proxy routes, inspect their primary chain and fallback pool slots, reorder slots via keyboard, add new routes/slots via `huh` forms, and delete routes/slots with a confirmation prompt.

**Save strategy:** Discrete mutations (add/delete) persist immediately via `saveNow()`. Reorder operations are buffered in memory (`isDirty=true`) and flushed via `FlushIfDirty()` on Esc or on application quit.

> [!NOTE]
> **Amendment history:** v1 → Tab conflict fix + dirty flag; v2 → isDirty clearing via `saveNow()`, exported `FlushIfDirty()`, app.go quit handler, static placeholder nit; v3 → `max(0,...)` clamp for routeIdx/slotIdx; v4 → formSlotProvider pre-init in initAddSlotForm().

---

## Proposed Changes

### Component: `internal/tui/views/`

#### [MODIFY] [routes.go](file:///d:/Codes/oapi/internal/tui/views/routes.go)

Full rewrite of the current 41-line stub (~330 lines). Follows structural patterns of [providers.go](file:///d:/Codes/oapi/internal/tui/views/providers.go).

**Struct definition:**

```go
type routesViewMode int

const (
    routesModeRouteList   routesViewMode = iota
    routesModeSlotList
    routesModeAddRoute
    routesModeAddSlot
    routesModeDeleteRoute
    routesModeDeleteSlot
)

type RoutesModel struct {
    cfg        *config.Config
    configPath string
    Width, Height int

    mode        routesViewMode
    routeIdx    int  // selected route (left panel)
    slotSection int  // 0 = chain, 1 = fallback
    slotIdx     int  // index within current section

    isDirty   bool   // pending reorder changes not yet saved
    statusMsg string

    addRouteForm *huh.Form
    addSlotForm  *huh.Form

    formRouteName    string
    formRouteAlias   string
    formSlotProvider string
    formSlotModel    string
}
```

**Constructor:**

```go
func NewRoutesModel(cfg *config.Config, configPath string) RoutesModel
```

---

**Save mechanics — two internal helpers:**

```go
// saveNow: persist cfg to disk AND clear isDirty.
// Called by all discrete mutations (add, delete).
func (m *RoutesModel) saveNow() {
    _ = config.SaveConfig(m.configPath, m.cfg)
    m.isDirty = false
}

// FlushIfDirty: persist only when dirty. Exported so AppModel
// can call it on quit. No-op when isDirty=false.
func (m *RoutesModel) FlushIfDirty() {
    if m.isDirty {
        _ = config.SaveConfig(m.configPath, m.cfg)
        m.isDirty = false
    }
}
```

> [!IMPORTANT]
> Every function that writes `cfg` to disk **must** go through `saveNow()` or `FlushIfDirty()` — never call `config.SaveConfig` directly. This guarantees `isDirty` stays in sync with the on-disk state and eliminates redundant double-saves.

---

**Mutation helpers** (pointer receivers):

| Helper               | Save call                                                  | `isDirty` after |
| -------------------- | ---------------------------------------------------------- | --------------- |
| `initAddRouteForm()` | none                                                       | unchanged       |
| `initAddSlotForm()`  | none                                                       | unchanged       |
| `saveRoute()`        | `saveNow()`                                                | `false`         |
| `deleteRoute(idx)`   | `saveNow()` + clamp `routeIdx = max(0, len(cfg.Routes)-1)` | `false`         |
| `saveSlot()`         | `saveNow()`                                                | `false`         |
| `deleteSlot()`       | `saveNow()` + clamp `slotIdx = max(0, len(section)-1)`     | `false`         |
| `reorderSlotUp()`    | **none**                                                   | `true`          |
| `reorderSlotDown()`  | **none**                                                   | `true`          |

After each of `saveRoute`, `deleteRoute`, `saveSlot`, `deleteSlot`: the full in-memory `cfg` (including any already-reordered slots) is written to disk **once** and `isDirty` is cleared. No duplicate save on subsequent Esc.

---

**Key bindings:**

| Mode            | Key                | Action                                                   |
| --------------- | ------------------ | -------------------------------------------------------- |
| Route list      | `↑`/`↓` or `k`/`j` | Navigate routes                                          |
| Route list      | `Enter` / `→`      | Enter slot editor                                        |
| Route list      | `a`                | Open add-route form                                      |
| Route list      | `d` / `x`          | Open delete-route confirm                                |
| Slot list       | `k` / `j`          | Navigate slots (cursor only)                             |
| Slot list       | `↑`                | **Reorder**: swap with neighbor above → `isDirty=true`   |
| Slot list       | `↓`                | **Reorder**: swap with neighbor below → `isDirty=true`   |
| Slot list       | `Space`            | Toggle section: Chain ↔ Fallback (clamps `slotIdx`)      |
| Slot list       | `a`                | Open add-slot form (section inferred from `slotSection`) |
| Slot list       | `d` / `x`          | Open delete-slot confirm                                 |
| Slot list       | `Esc`              | Call `FlushIfDirty()`, return to route list              |
| Add-route form  | huh-managed        | `Esc` aborts, `Enter` completes → `saveNow()`            |
| Add-slot form   | huh-managed        | `Esc` aborts, `Enter` completes → `saveNow()`            |
| Delete confirms | `y` / `Y`          | Confirm → `saveNow()`                                    |
| Delete confirms | `n` / `N` / `Esc`  | Cancel                                                   |

> [!IMPORTANT]
> **Why not Tab for section toggle**: `AppModel.Update()` in [app.go](file:///d:/Codes/oapi/internal/tui/app.go) intercepts `"tab"` and `"shift+tab"` with an early `return m, nil` before dispatching to any sub-model. `RoutesModel.Update()` never sees Tab events. `Space` is unintercepted at the root level.

---

**Provider options in `initAddSlotForm()`:**

```go
// Source: registry.Providers — not cfg.Keys.
// Allows adding a slot for a provider before any keys exist for it.
providerIDs := make([]string, 0, len(registry.Providers))
for id := range registry.Providers {
    providerIDs = append(providerIDs, id)
}
sort.Strings(providerIDs) // deterministic dropdown order

// Pre-initialize formSlotProvider to the first sorted provider so that
// the RecommendedModels placeholder lookup below never hits an empty key.
// On a fresh form open, formSlotProvider is ""; without this guard,
// registry.RecommendedModels[""] returns nil and [0] would panic or
// produce a silent empty placeholder.
if m.formSlotProvider == "" {
    m.formSlotProvider = providerIDs[0]
}

var opts []huh.Option[string]
for _, id := range providerIDs {
    opts = append(opts, huh.NewOption(id, id))
}
```

The Model text input placeholder is taken from `registry.RecommendedModels[m.formSlotProvider][0]` (guarded with `len(...) > 0`) at **form-open time**.

> [!NOTE]
> **Static placeholder — accepted Phase 1 limitation.** Because `initAddSlotForm()` builds the form once, the placeholder reflects the provider selected at open time. If the user changes the provider dropdown mid-form, the model hint won't update. Do not add an `onChange` handler or dynamic rebuild — this is intentional scope control for Phase 1.

---

**UI layout:**

```
┌──────────────────────────────────────────────────────────────┐
│  Routes                                                      │
├────────────┬─────────────────────────────────────────────────┤
│ Routes     │  Alias: gpt-4       Name: GPT-4 Route          │
│ > gpt-4    │                                                  │
│   claude   │  PRIMARY CHAIN                                   │
│            │  > [1] groq / llama-3.3-70b-versatile           │
│            │    [2] google / gemini-2.0-flash                 │
│            │    + Add Slot  [a]                               │
│            │                                                  │
│            │  FALLBACK POOL                                   │
│            │    [1] openrouter / llama-3-70b                  │
│            │    + Add Fallback  [a]                           │
├────────────┴─────────────────────────────────────────────────┤
│ [a] add  [d] delete  [Enter] edit slots                      │
│ slots: [j/k] nav  [↑/↓] reorder  [Space] toggle  [x] delete │
└──────────────────────────────────────────────────────────────┘
```

---

### Component: `internal/tui/`

#### [MODIFY] [app.go](file:///d:/Codes/oapi/internal/tui/app.go)

**Two changes** in two separate locations:

**Change 1 — `New()` constructor** (line ~40):

```diff
-routes: views.NewRoutesModel(),
+routes: views.NewRoutesModel(tuiCfg.Cfg, tuiCfg.ConfigPath),
```

**Change 2 — quit handler in `Update()`** (line ~55–59):

```diff
 case "ctrl+c", "q":
     if m.tuiCfg.Cancel != nil {
         m.tuiCfg.Cancel()
     }
+    m.routes.FlushIfDirty()
     return m, tea.Quit
```

`FlushIfDirty()` is a no-op when `isDirty=false`, so this adds zero overhead in the common case. It is called unconditionally (not gated on `activeTab == 2`) because the user may have reordered slots and then switched to another tab before quitting — the dirty flag persists in the struct regardless of active tab.

> [!NOTE]
> `AppModel.Update()` uses a value receiver, so `m` is a local copy. `m.routes.FlushIfDirty()` takes `&m.routes` automatically (Go addressable local). The `config.SaveConfig` disk write executes before `tea.Quit` is returned — this is the only side-effect we need.

---

## Dependencies

| Package                              | Version         | Status                                                                |
| ------------------------------------ | --------------- | --------------------------------------------------------------------- |
| `github.com/charmbracelet/bubbletea` | v1.3.6 (v1 API) | Already in `go.mod`                                                   |
| `github.com/charmbracelet/lipgloss`  | existing        | Already in `go.mod`                                                   |
| `github.com/charmbracelet/huh`       | existing        | Already in `go.mod`                                                   |
| `oapi/internal/config`               | —               | Already in scope                                                      |
| `oapi/internal/registry`             | —               | Already in scope — `registry.Providers`, `registry.RecommendedModels` |
| `oapi/internal/tui/styles`           | —               | Already in scope                                                      |
| `sort` (stdlib)                      | —               | For stable provider dropdown order                                    |

**No new third-party dependencies.**

---

## Risks & Tradeoffs

| Risk                                     | Mitigation                                                                                                                                                                                                                                         |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Bubble Tea value-receiver contract       | Mutation helpers use pointer receivers; `Update()` uses value receiver — mirrors `providers.go`                                                                                                                                                    |
| `huh` form lifecycle                     | Call `form.Init()` immediately after `huh.NewForm(...)` — same as `providers.go` line 175                                                                                                                                                          |
| `slotIdx` out-of-bounds on Space toggle  | Clamp: `if m.slotIdx >= len(section) { m.slotIdx = max(0, len(section)-1) }`                                                                                                                                                                       |
| `routeIdx` negative after route delete   | `m.routeIdx = max(0, len(m.cfg.Routes) - 1)` — bare `len - 1` evaluates to `-1` on an empty slice and corrupts state for the next add. `max(0, ...)` keeps it at `0`; the empty-state guard in `View()` prevents any index access when `len == 0`. |
| `slotIdx` negative after slot delete     | `m.slotIdx = max(0, len(section) - 1)` — same reason. Section render must also guard on `len(section) > 0` before indexing.                                                                                                                        |
| Empty routes list                        | Right panel shows empty-state message; all index accesses on `cfg.Routes` gated on `len(m.cfg.Routes) > 0`.                                                                                                                                        |
| `registry.Providers` iteration order     | `sort.Strings` ensures alphabetical, stable dropdown order.                                                                                                                                                                                        |
| Double-save on Esc after discrete action | Eliminated: `saveNow()` clears `isDirty`; `FlushIfDirty()` is then a no-op                                                                                                                                                                         |
| Dirty reorders lost on `q` quit          | Eliminated: `AppModel.Update()` calls `m.routes.FlushIfDirty()` in the quit handler before returning `tea.Quit`                                                                                                                                    |
| Static model placeholder in huh form     | Accepted Phase 1 limitation — documented, no fix needed                                                                                                                                                                                            |

---

## Verification Plan

### Build check

```powershell
cmd /c "cd d:\Codes\oapi && go build ./..."
```

Must compile with zero errors.

### Manual smoke test checklist

1. Navigate to Routes tab — confirm empty-state when `cfg.Routes` is empty.
2. `[a]` → fill Name + ModelAlias → confirm route in left panel + `oapi.yaml` updated.
3. `Enter` → confirm slot editor mode activates.
4. `[a]` in slot list → Provider dropdown shows registry providers (alphabetical, not from `cfg.Keys`) → fill Model → confirm slot added to Chain.
5. Hold `↓` arrow rapidly — confirm fast reorder; verify `oapi.yaml` is **not** hammered (no writes until Esc).
6. `Esc` → confirm single `SaveConfig` write flushes reorder + clears `isDirty`.
7. Re-enter slot editor, reorder, then press `[x]` + `[y]` to delete → confirm `saveNow()` writes full reordered+deleted state once.
8. `Space` → confirm section toggles Chain↔Fallback, `slotIdx` clamped.
9. Reorder slots, then press `q` (without Esc) → reopen `oapi.yaml` and confirm reorder was saved.
10. Confirm `Tab` key still switches TUI tabs (not consumed by `RoutesModel`).
