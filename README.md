<p align="center">
  <img src="docs/assets/logo.svg" width="200" alt="dockzy logo" />
</p>

<h1 align="center">dockzy</h1>

<p align="center">
  A terminal UI for managing Docker — in the style of <a href="https://github.com/jesseduffield/lazydocker">lazydocker</a>.
</p>

<p align="center">
  <img alt="Go version" src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white">
  <img alt="status" src="https://img.shields.io/badge/status-early--stage-orange">
  <img alt="TUI" src="https://img.shields.io/badge/UI-tview%20%2F%20tcell-3fb950">
</p>

---

## What is dockzy?

If you work with Docker day to day, you know the drill: `docker ps` to see
what's running, `docker logs -f <id>` to check on it, `docker stats` in
another tab to see if it's eating your CPU, `docker inspect` when something
looks off — a handful of commands and container IDs juggled across terminal
tabs just to answer "what's going on with my containers right now?"

`dockzy` puts all of that in one screen. It's a terminal dashboard that shows
your containers, images and volumes side by side, with live CPU and resource
usage updating in place, and a detail panel that gives you logs, stats,
config and running processes for whatever you have selected — no more typing
out IDs, no more switching windows. Everything is keyboard-driven: arrow
keys to move around, `Tab` to jump between panels, and it just works whether
you're looking at a single container or a whole docker-compose stack.

Point it at a docker-compose project and it automatically scopes the whole
dashboard to just that stack's containers, images and volumes — or pass
`-all` to see everything running on the machine.

It's built for people who'd rather glance at a dashboard than remember which
`docker` subcommand does what.

## Features

- **Live container list** — services (grouped by `com.docker.compose.service`)
  and standalone containers, split into their own panels.
- **Docker Compose project scoping** — run dockzy inside a compose project's
  working directory and it auto-detects the project, scoping the whole
  dashboard (containers, images, volumes) to just that stack. Pass `-all` to
  see everything on the host instead.
- **Live CPU%** — one stats stream per running container, updated in place,
  no polling/refresh needed.
- **Live per-container stats** — CPU, memory, network and block I/O update in
  place, once per second, while a container is selected.
- **Images and volumes panels** — browse local images (grouped by repo/tag)
  and volumes alongside containers, with inspect-style detail on selection.
- **Per-container detail tabs** — Logs, Stats, Config and `top`, fetched the
  moment you select a row and cancelled cleanly if you move on before they land.
- **Full keyboard navigation** — cycle panels with `Tab` / `Shift+Tab`, move
  through tabs with the arrow keys, quit with `q`.
- **Zero external dependencies at runtime** — talks straight to the Docker
  daemon over its socket via [`moby/moby/client`](https://github.com/moby/moby).

## Status

Early-stage but functional end-to-end against a real Docker daemon: container
listing, live CPU, images, volumes, docker-compose project scoping, and the
Logs/Stats/Config/Top panel are all wired up — nothing is mocked.

## Planned features

- **Container actions** — Restart/Stop/Down when a container is selected.
- **Compose stack control** — Up/Down a docker-compose stack from inside the project view.
- **Real-time charts** — CPU/Memory usage visualization as time-series charts.

## Install & run

Requires Go 1.26+ and a reachable Docker daemon (`DOCKER_HOST` or the default
socket).

```bash
git clone https://github.com/Gabriel-Valin/dockzy.git
cd dockzy
go run .
```

Inside a docker-compose project directory, dockzy automatically scopes to
that project's containers, images and volumes. Use `-all` to see everything
on the host instead:

```bash
go run . -all
```

No Docker environment handy? `docker-compose.yml` spins up a small
Postgres/Redis/Nginx stack you can point dockzy at (see also
[`docker-examples/`](docker-examples/) for standalone config samples like
`nginx.conf`):

```bash
docker compose up -d
go run .
```

## Using dockzy

The left panel stacks five boxes — Status, Services, Standalone Containers,
Images, Volumes. The right panel shows detail for whatever row is currently
selected on the left, as four tabs: Logs, Stats, Container Config and `Top`.

- On launch, the first service (or the first standalone container, if there
  are no services) is auto-selected and its detail starts loading.
- Moving the selection in Services/Standalone/Images/Volumes immediately
  refreshes the right panel — logs/config/top are fetched once, stats and CPU
  stream continuously while a container stays selected.
- `Tab` / `Shift+Tab` cycles focus across the five panels; the focused box
  gets a green border.
- With the right panel focused, `←`/`→` switch between the Logs/Stats/Config/Top
  tabs, and `↑`/`↓`/`Page Up`/`Page Down`/`Home`/`End` scroll that tab's
  content instead of moving a selection.
- Switching selection while a fetch is still in flight cancels it — no
  stale logs/stats land after you've already moved on.

## Keybindings

| Key                                 | Action                                                                     |
| ------------------------------------ | --------------------------------------------------------------------------- |
| `Tab` / `Shift+Tab`                  | Cycle focus: Services → Standalone → Images → Volumes → right panel        |
| `↑` / `↓`                             | Move selection within the focused list (Services/Standalone/Images/Volumes) |
| `↑` / `↓` (right panel focused)      | Scroll the active tab's content up/down                                    |
| `←` / `→`                             | Switch tab (Logs/Stats/Config/Top) when the right panel is focused         |
| `Page Up` / `Page Down`              | Scroll the active tab's content a page at a time (right panel focused)     |
| `Home` / `End`                       | Jump to the top/bottom of the active tab's content (right panel focused)   |
| Mouse click                          | Focus a panel and select the clicked row                                   |
| Mouse wheel                          | Scroll the list or tab content under the cursor                            |
| `q`                                   | Quit (cancels every in-flight stream first)                                |

## Contributing

This is a personal, early-stage project — issues and PRs are welcome, but
expect the internals to shift. Read [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
first; it documents the exact shape a new feature (static or streamed) is
expected to take. New to Go concurrency patterns? [`docs/GOLANG-ROADMAP.md`](docs/GOLANG-ROADMAP.md)
walks through every Go concept used in this codebase (goroutines,
channels, context, embedding...) with file:line references.
