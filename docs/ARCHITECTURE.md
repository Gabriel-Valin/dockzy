# dockzy — Architecture

This document explains how dockzy is put together, how data moves from the
Docker daemon onto the screen, and how to add a new feature — whether it's a
one-shot value (**static**) or a value that keeps changing while it's on
screen (**dynamic / streamed**), the same shape CPU% already uses.

## 1. Package layout

```
main.go                    entrypoint — 10 lines, calls app.Run()
internal/
├── docker/                "Services": talks to the Docker daemon. No tview import.
│   ├── docker.go            Client, Service, Standalone, ListContainers
│   ├── stats.go              CPUUpdate, StreamCPU, formatCPUPercent (dynamic example)
│   ├── info.go                Info, ContainerInfo (static example)
│   └── mock.go                 Image, Volume, MockImages, MockVolumes (still fake)
├── ui/                     "UI": tview widgets. No Docker API import.
│   ├── theme.go              colors
│   ├── data.go                 Data (everything Dashboard needs to draw)
│   ├── panel.go                  left panel: Status/Services/Standalone/Images/Volumes
│   ├── tabs.go                     right panel: the 4-tab Logs/Stats/Config/Top view
│   └── dashboard.go                 Dashboard: layout, focus, and the public update API
└── app/                    "Init app" (composition root): wires docker + ui together.
    └── app.go                Run() — the only place goroutines get started
```

Dependency direction is one-way and never cycles:

```
app  ──depends on──▶  ui  ──depends on──▶  docker
 │                                            ▲
 └────────────────────depends on──────────────┘
```

`docker` knows nothing about `ui` or `tview`. `ui` knows nothing about
`github.com/moby/moby/client` — it only sees the plain Go types `docker`
exports (`docker.Service`, `docker.CPUUpdate`, ...). `app` is the only
package allowed to import both and start goroutines. Keeping this boundary
is what makes each side of a feature (data vs. widget) easy to reason about
and to test in isolation.

## 2. The three layers

### `internal/docker` — Services

Everything that speaks the Docker API lives here, wrapped in `docker.Client`
(an embedded `*client.Client` from `github.com/moby/moby/client`, see
`docker.go:31`). Two shapes of function live in this package, matching the
two kinds of feature:

- **One-shot fetchers** — take a `context.Context`, do the API call(s), and
  return a value or an error. Example: `ListContainers` (`docker.go:48`),
  `Info` (`info.go:25`).
- **Streamers** — take a `context.Context`, an id, and a `chan<- T`; they
  loop until the context is cancelled or the stream ends, sending one value
  per sample. Example: `StreamCPU` (`stats.go:23`).

Nothing in this package touches a UI widget or calls `QueueUpdateDraw`. It
just produces data or errors.

### `internal/ui` — UI

`tview` widgets and nothing else. The two structs that matter:

- **`Data`** (`data.go`) — the snapshot `Dashboard` is built from at
  startup: `Services`, `Standalone`, `Images`, `Volumes`, and the initial
  `Logs`/`Stats`/`Config`/`Top` placeholder text.
- **`Dashboard`** (`dashboard.go`) — owns the `tview.Application`, the left
  panel boxes, the right panel tabs, focus-cycling, and a small **public
  update API** that is the *only* way anything outside `ui` is allowed to
  touch the screen after startup:

  | Method                                       | Used for                                   |
  | --------------------------------------------- | ------------------------------------------- |
  | `UpdateCPU(id, cpu string)`                   | dynamic: rewrite one row's CPU column        |
  | `SetContainerInfo(logs, stats, config, top)`  | static: rewrite all 4 right-panel tabs       |
  | `OnSelectContainer(fn func(id string))`       | register the selection callback (see below)  |
  | `RunningContainerIDs() []string`               | let `app` know which containers to stream    |

  Every one of these methods wraps its work in `d.App.QueueUpdateDraw(...)`
  (see `dashboard.go:205` and `:215`). That's not optional: tview widgets
  are **not** safe to mutate from any goroutine other than the one running
  `Application.Run()`. `QueueUpdateDraw` marshals the mutation onto that
  goroutine and redraws afterwards. Any new update method you add must do
  the same.

#### `liveRow`: how a per-row update finds its row

```go
// dashboard.go:10
type liveRow struct {
    list  *tview.List // servicesBox or standaloneBox
    index int          // its row inside that list
    state string        // cached — needed to redraw the row's color
    name  string          // cached — needed to redraw the row's name column
}
```

A `tview.List` only lets you rewrite a row by its **numeric index**
(`SetItemText(index, main, secondary)`, see `panel.go`'s `formatRow`) — it
has no notion of "the row for container `abc123`". `liveRow` is that
missing index, one per container, keyed by container ID:

```go
// dashboard.go:57-63 (inside New)
rows := map[string]*liveRow{}
for i, s := range data.Services {
    rows[s.ID] = &liveRow{list: servicesBox, index: i, state: s.State, name: s.Name}
}
for i, s := range data.Standalone {
    rows[s.ID] = &liveRow{list: standaloneBox, index: i, state: s.State, name: s.Name}
}
```

It's built exactly once, in `New`, by walking `data.Services`/
`data.Standalone` in the same order `buildServicesBox`/`buildStandaloneBox`
just added them to the list — so `index` always lines up with the row
`AddItem` created for that same container. It never gets rebuilt or
re-indexed after that; rows don't get reordered while the app is running.

`state` and `name` are cached here because rewriting a row means rewriting
its *entire* line — `formatRow(state, name, value, color)` needs all three
columns even though a CPU update only carries a new value for one of them.
Without the cache, `UpdateCPU` would have to go fetch the container's
current state/name from somewhere just to redraw a row whose CPU changed.

Both existing consumers just read the map, they don't mutate it:

```go
// RunningContainerIDs (dashboard.go:188) — decide who to stream at all
for id, row := range d.rows {
    if row.state == "running" { ids = append(ids, id) }
}

// UpdateCPU (dashboard.go:200) — the actual per-row rewrite
row, ok := d.rows[id]
...
text := formatRow(row.state, row.name, cpu, stateColor(row.state))
row.list.SetItemText(row.index, text, "")
```

**When you need it:** any time you add a *dynamic* feature that rewrites
one column of an existing Services/Standalone row per container (memory %,
network I/O, health...) — look the row up in `d.rows` exactly like
`UpdateCPU` does. Don't build a second id→row map; `d.rows` already is that
index for every container currently on the left panel.

**When you need to extend it:** only if the new column's value has to
survive *independently* of the others across renders. `UpdateCPU` gets
away with not caching CPU in `liveRow` because each `CPUUpdate` carries the
full string to print and nothing else reads or writes that column. If you
add a second live column (say memory) rendered on the *same* line as CPU,
a memory-only update would blank the CPU text unless something remembers
the last CPU value — at that point add fields to `liveRow` (e.g. `cpu`,
`mem`) and have each `Update*` method update its own field, then re-render
the whole row from all of them.

**When it doesn't apply:** `liveRow` only indexes Services/Standalone rows.
Images and Volumes aren't looked up by ID today (nothing rewrites them
after startup), and the right panel doesn't use it either —
`SetContainerInfo` always rewrites all four tabs for whichever one
container is currently selected, so there's nothing to index by ID there.
If you add live per-row updates to Images/Volumes, give them their own map
the same shape as `rows` rather than reusing this one — container IDs and
image/volume IDs aren't guaranteed disjoint.

### `internal/app` — the composition root ("init app")

`app.Run()` (`app.go`) is the only function that:

1. Creates the `docker.Client`.
2. Fetches the initial data and builds `ui.Data`.
3. Builds the `Dashboard` (`ui.New`).
4. Starts every background goroutine (CPU streams, selection fetches).
5. Calls `dashboard.Run()`, which blocks until the user presses `q`.

If you're adding a feature, the wiring — "start a goroutine, read a
channel/callback, call the `Dashboard` setter" — belongs in `app.go`. It's
the only file allowed to know that both `docker` and `ui` exist.

## 3. Data flow: Services → UI → Update screen

Two real features already exercise both shapes end-to-end. Point to these
when building a new one.

### 3.1 Startup (initial render, no goroutines yet)

```
app.Run()
  → cli.ListContainers(ctx)                 [docker]  one HTTP call to the daemon
  → ui.Data{ Services: ..., Standalone: ... }
  → ui.New(data, cancel)                    [ui]      builds every widget once, synchronously
  → dashboard.Run()                          blocks, draws the initial frame
```

Nothing here is live yet — this is just the first paint.

### 3.2 Dynamic example: CPU (**STREAM**)

```
docker.StreamCPU(ctx, id, cpuUpdates)   [goroutine, one per running container]
   loop: daemon pushes one sample/sec
     → formatCPUPercent(stats)                     → "3.14%"
     → cpuUpdates <- CPUUpdate{ID: id, CPU: "3.14%"}

app.Run()'s consumer goroutine                       [app.go:53-62]
   for u := range cpuUpdates:
     → dashboard.UpdateCPU(u.ID, u.CPU)

Dashboard.UpdateCPU(id, cpu)                          [ui, dashboard.go:200]
   → App.QueueUpdateDraw(func() {
         row.list.SetItemText(row.index, formatRow(...), "")
     })
```

One goroutine per container, all funneling into a single channel, drained
by a single consumer goroutine that is the only thing allowed to call
`QueueUpdateDraw` for CPU. `ctx` is cancelled on `q`
(`dashboard.go:163-169` calls the `onQuit` you passed into `ui.New`, which
is `app.go`'s `cancel`), which closes every open stats stream and lets
`streamContainerCPU`'s `decoder.Decode` return and the goroutine exit.

### 3.3 Static example: container selection → Logs/Stats/Config/Top (**NO STREAM**)

```
User moves the cursor in Services/Standalone (or the box gains focus)
  → tview fires SetChangedFunc / SetFocusFunc         [ui, dashboard.go:112-129]
  → selectServices()/selectStandalone() resolve the row → container ID
  → d.onSelectContainer(id)                            [set via OnSelectContainer]

app.go's onSelectContainer(id)                         [app.go:72-88]
  → cancel the PREVIOUS selection's in-flight fetch (if any)
  → go func() {
        info, err := cli.Info(selCtx, id)               [docker, info.go:25]
          → 4 goroutines in parallel: logs / stats / config / top
        if selCtx.Err() != nil { return }                 ← drop if superseded
        dashboard.SetContainerInfo(info.Logs, info.Stats, info.Config, info.Top)
    }()

Dashboard.SetContainerInfo(...)                         [ui, dashboard.go:214]
  → App.QueueUpdateDraw(func() { d.tabs.setContent(...) })
```

This one is "static" in the sense that matters here: it's a single
request/response per selection, not an open stream. The tricky part isn't
fetching the data, it's **not showing stale data**: if you flick through
three containers quickly, only the last selection's fetch should ever reach
`SetContainerInfo`. `app.go` handles that with one cancellable
`context.Context` per selection (`selectMu` + `selectCancel`,
`app.go:68-79`) — a new selection cancels whatever fetch was still running
for the previous one.

## 4. Adding a new feature

### 4.1 Decide: static or dynamic?

Ask: **does the value change while it's on screen, and do you want the
screen to follow it without the user doing anything?**

- No → it's **static**. Fetch it once, on some trigger (startup, selection
  change, a manual refresh key you add). Model it on §3.3 / `Info`.
- Yes → it's **dynamic**. The daemon (or your own ticker) needs to keep
  pushing values while the row/tab is visible. Model it on §3.2 /
  `StreamCPU`.

When in doubt, start static — it's less code and no goroutine lifetime to
manage. Only reach for a stream if a static value would look wrong or stale
within the time the user is looking at it (CPU is the textbook case;
"which image a container uses" is not).

### 4.2 Recipe: static feature (NO STREAM)

Worked example: showing real image data instead of `docker.MockImages()`.

1. **`internal/docker`** — add a one-shot fetcher. It takes `ctx`, returns
   `(T, error)`, and is otherwise Docker-only:

   ```go
   // internal/docker/images.go
   func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
       result, err := c.ImageList(ctx, client.ImageListOptions{})
       if err != nil {
           return nil, err
       }
       images := make([]Image, 0, len(result.Items))
       for _, img := range result.Items {
           images = append(images, Image{ /* map fields */ })
       }
       return images, nil
   }
   ```

   If the new value needs to appear alongside 3 other values fetched
   together (like `Info` does for logs/stats/config/top), fan the calls out
   into goroutines and fan the results back into one struct — see
   `info.go:25-71` for the pattern (a small `result` struct + a buffered
   channel + one `<-results` per field).

2. **`internal/ui`** — add a way to push the new value onto a widget.
   Either a field on `Data` (if it's needed at construction time, like
   `Images`) or a `Dashboard` setter (if it arrives later, like
   `SetContainerInfo`):

   ```go
   // internal/ui/dashboard.go
   func (d *Dashboard) SetImages(images []docker.Image) {
       d.App.QueueUpdateDraw(func() {
           d.imagesBox.Clear()
           for _, img := range images {
               d.imagesBox.AddItem(formatImageRow(img), "", 0, nil)
           }
       })
   }
   ```

   `imagesBox` will need to become a `Dashboard` field (it's currently a
   local var in `New`) if it wasn't already — anything a setter needs to
   reach after construction has to survive past `New`'s return.

3. **`internal/app`** — call the fetcher and push the result on whatever
   trigger makes sense. For a startup-only refresh:

   ```go
   // app.go, after ui.New(...)
   images, err := cli.ListImages(ctx)
   if err == nil {
       dashboard.SetImages(images)
   }
   ```

   For a per-selection value (like Config), fetch it inside
   `onSelectContainer` alongside `cli.Info` instead, respecting the same
   `selCtx` cancellation so a fast follow-up selection wins.

### 4.3 Recipe: dynamic feature (STREAM)

Worked example: a live memory-usage percentage per container, exactly
mirroring how CPU works today.

1. **`internal/docker`** — add a streamer with the same shape as
   `StreamCPU`: `(ctx, id, chan<- YourUpdate)`, runs until `ctx` is done or
   the source closes, one send per sample.

   ```go
   // internal/docker/stats.go
   type MemUpdate struct {
       ID  string
       Mem string
   }

   func (c *Client) StreamMemory(ctx context.Context, id string, updates chan<- MemUpdate) {
       result, err := c.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: true})
       if err != nil {
           return
       }
       defer result.Body.Close()

       decoder := json.NewDecoder(result.Body)
       for {
           var stats container.StatsResponse
           if err := decoder.Decode(&stats); err != nil {
               return
           }
           select {
           case updates <- MemUpdate{ID: id, Mem: formatMemPercent(stats)}:
           case <-ctx.Done():
               return
           }
       }
   }
   ```

   If the value comes from the *same* stats stream CPU already reads
   (memory does), don't open a second stream per container — extend
   `CPUUpdate`/`StreamCPU` to carry both fields instead of duplicating the
   HTTP stream. Only write a second streamer when the data genuinely comes
   from a different source (a second endpoint, a log line, a ticker you
   drive yourself).

2. **`internal/ui`** — add the row-rewrite method, same shape as
   `UpdateCPU`:

   ```go
   // internal/ui/dashboard.go
   func (d *Dashboard) UpdateMemory(id, mem string) {
       row, ok := d.rows[id]
       if !ok {
           return
       }
       d.App.QueueUpdateDraw(func() {
           text := formatRow(row.state, row.name, mem, stateColor(row.state))
           row.list.SetItemText(row.index, text, "")
       })
   }
   ```

   That's the same `d.rows` lookup `UpdateCPU` uses — see [`liveRow`](#liverow-how-a-per-row-update-finds-its-row)
   above. This naive version *overwrites* the CPU column with memory, since
   both would render into the same single value slot; showing CPU and
   memory side by side needs `liveRow` to cache both (also covered there).

3. **`internal/app`** — start one goroutine per running container and one
   consumer goroutine, same shape as the CPU wiring at `app.go:46-62`:

   ```go
   memUpdates := make(chan docker.MemUpdate, len(services)+len(standalone))
   for _, id := range dashboard.RunningContainerIDs() {
       go cli.StreamMemory(ctx, id, memUpdates)
   }
   go func() {
       for {
           select {
           case u := <-memUpdates:
               dashboard.UpdateMemory(u.ID, u.Mem)
           case <-ctx.Done():
               return
           }
       }
   }()
   ```

   `ctx` is already cancelled on quit (`cancel`, passed into `ui.New` as
   `onQuit`), so the new streams stop the same way the CPU ones do — no
   extra cleanup code needed.

### 4.4 Checklist

- [ ] Docker-only code (API calls, JSON decoding, formatting) lives in
      `internal/docker` and never imports `tview`.
- [ ] Widget-only code lives in `internal/ui` and never imports
      `github.com/moby/moby/*`.
- [ ] Every `Dashboard` method that mutates a widget after `New` returns
      wraps its body in `d.App.QueueUpdateDraw(...)`.
- [ ] Every goroutine you start in `app.go` respects `ctx` (streamers select
      on `ctx.Done()`; one-shot fetches check `ctx.Err()` before applying a
      result that might be stale).
- [ ] A stream's channel has exactly one consumer goroutine calling into
      `Dashboard` — don't call `Dashboard` setters directly from N producer
      goroutines.
- [ ] If the value is per-selection (like Config) rather than per-visible-row
      (like CPU), reuse the `selectMu`/`selectCancel` pattern in
      `onSelectContainer` instead of adding a second cancellation scheme.

## 5. Why it's shaped this way

- **`docker` has no `ui` import** so the Docker-facing code can be read,
  tested, or reused (e.g. from a future non-TUI entrypoint) without pulling
  in `tview`.
- **`ui` has no `docker/client` import** so widget code only deals in plain
  structs (`docker.Service`, `docker.CPUUpdate`, ...) — it can't
  accidentally make a network call from inside a draw path.
- **`app` is the only place with goroutines** so there's one file to read
  to answer "what background work is running and when does it stop" — not
  scattered across the codebase.
- **One `context.Context`, cancelled on quit** is threaded through every
  streamer and fetcher, so `q` reliably stops all outstanding Docker calls
  instead of leaking goroutines past the TUI's lifetime.
