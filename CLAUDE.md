## What this is

`dockzy` (module `github.com/Gabriel-Valin/dockzy`) is an early-stage terminal UI for managing Docker, in the style of `lazydocker`. It's built with `tview`/`tcell` and is meant to eventually talk to the Docker daemon via `github.com/docker/go-sdk` / `github.com/moby/moby/client`.

The project is a single `main` package at the repo root — there is no internal package structure yet.

## Current state (important context)

- `main.go` is a **fully mocked UI**: `getMockData()` returns hardcoded `Service`/`Standalone`/`Image`/`Volume` data and static log/stats/config/top text. Nothing in `main.go` talks to Docker yet. Struct field comments (e.g. `// -> Container.State`) indicate which real Docker API call/field should eventually populate that field.
- No tests, no README, no linter config, no CI exist yet.

## Commands

```bash
go build ./...     # note: currently fails because of 1.go (see above)
go run .            # run the TUI (only works once 1.go's build error is resolved)
go vet ./...
```

There is no test suite (`go test ./...` has nothing to run).

## Architecture (main.go)

- **Data model**: `Service`, `Standalone`, `Image`, `Volume`, bundled in `mockData` (currently populated by `getMockData()`, intended to be replaced by real Docker API calls).
- **Left panel**: five stacked `tview.List`/`tview.TextView` boxes — Status, Services, Standalone Containers, Images, Volumes — built by `buildStatusBox`, `buildServicesBox`, `buildStandaloneBox`, `buildImagesBox`, `buildVolumesBox`. Row text is manually column-aligned via `formatRow`/`padRight`.
- **Right panel**: `tabbedPanel` (logs/stats/config/top) implemented with `tview.Pages` plus a manually-rendered tab header (`renderHeader`); `next()`/`prev()` switch pages and re-render the header.
- **Focus handling**: `main()` builds a `focusables` slice (the four left-panel lists + the tab panel) and cycles through it on Tab/Shift+Tab; when the tab panel is focused, Left/Right arrow keys call `tabs.prev()`/`tabs.next()` instead of navigating a list. Focused boxes get a green border via `SetFocusFunc`/`SetBlurFunc` (`applyFocusBorder`).
- `q` quits the app.

When wiring up real Docker data (per the comments in `main.go` and the prototype in `1.go`), the pattern to follow is: build a `docker/go-sdk` (or `moby/moby/client`) client once in `main()`, replace `getMockData()`'s static values with live API calls, and reuse the `tviewWriter`/goroutine + `context.CancelFunc` pattern from `1.go` for streaming logs into a `TextView` without blocking the UI goroutine (`app.QueueUpdateDraw`).

## Conventions

- UI-facing struct/type doc comments and code comments are written in **pt-BR**; keep that convention when extending mocked/UI code.
- Go standard formatting (`gofmt`) — no custom style config present.
