## What this is

`dockzy` (module `github.com/Gabriel-Valin/dockzy`) is a terminal UI for managing
Docker, in the style of `lazydocker`. Built with `tview`/`tcell`, talking to the
Docker daemon through `github.com/moby/moby/client`.

It is functional end-to-end against a real daemon — container/image/volume
listing, live CPU, docker-compose project scoping, and the Logs/Stats/Config/Top
detail panel. Nothing is mocked.

## Commands

```bash
go build ./...   # build
go run .         # run the TUI (scoped to the compose project in the cwd, if any)
go run . -all    # run against every container/image/volume on the host
go vet ./...
gofmt -l .       # should print nothing
```

There is no test suite yet (`go test ./...` has nothing to run).

`docker-compose.yml` at the root spins up a small Postgres/Redis/Nginx stack to
point dockzy at during development.

## Project structure

```
main.go                 entrypoint — parses -all, calls app.Run()
internal/
├── docker/             Docker daemon access. Never imports tview.
│   ├── docker.go         Client (embeds moby client), Service, Standalone, ListContainers
│   ├── compose.go        Project detection + project-scoped listing (compose labels)
│   ├── images.go         Image/Volume types, ListImages/ListVolumes, ImageInfo/VolumeInfo
│   ├── info.go           ContainerInfo (logs/stats/config/top), fetched in parallel
│   └── stats.go          CPUUpdate/StatsUpdate, StreamCPU/StreamStats, formatters
├── ui/                  tview widgets. Never imports the moby client.
│   ├── data.go           Data — the struct the dashboard is built from
│   ├── dashboard.go      Dashboard: layout, focus cycling, key handling, live updates
│   ├── panel.go          Left-panel box builders + row formatting (formatRow/padRight)
│   ├── detail.go         Right panel: container tabs vs. single-view resource page
│   ├── tabs.go           tabbedPanel — Logs/Stats/Config/Top pages + rendered header
│   └── theme.go          Focus border colors
└── app/app.go          Wiring: builds the client, loads data, connects UI callbacks to streams
docs/                   ARCHITECTURE.md (how to add a feature), ROADMAP.md, GOLANG-ROADMAP.md
docker-examples/        Config samples used by docker-compose.yml
```

**Dependency direction**: `app` → `ui` + `docker`. `ui` and `docker` never
import each other's concerns (`ui` imports `docker` only for its data types).
All wiring lives in `app.Run` — that is the only place that knows about both.

`docs/ARCHITECTURE.md` documents the exact shape a new feature is expected to
take, static and streamed. Read it before adding one.

## Conventions

- Go standard formatting (`gofmt`) — no custom style config.
- **Comments, log messages and user-facing strings are in pt-BR**; identifiers,
  file names and commit messages in English.
- The codebase is deliberately comment-light — prefer clear names over comments.
- `docker.Client` embeds `*client.Client`, so moby methods are called directly on
  it (`c.ContainerList(...)`). Wrapper methods return dockzy's own types, never
  moby types, so the `ui` package stays free of Docker API types.
- **Streamed values** follow the CPU pattern: a `docker.Stream*` method pushes
  typed updates onto a channel, `app.Run` reads that channel and calls a
  `Dashboard.Update*` method, which wraps the mutation in `App.QueueUpdateDraw`.
  Anything touching a tview widget from a goroutine must go through
  `QueueUpdateDraw`.
- **Selection cancels the previous selection.** `app.Run` keeps one
  `context.CancelFunc` for the active selection; every new one cancels the last
  so no stale logs/stats land after the user has moved on. Check `ctx.Err()`
  after any await before writing to the UI.
- Left-panel rows are manually column-aligned (`formatRow`, `padRight`), not
  table-driven — keep widths consistent when adding a column.
- Don't commit build artifacts. (A stray `verify` binary is currently tracked
  and there is no `.gitignore`.)
