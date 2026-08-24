# dockzy — Product roadmap

Unlike [`GOLANG-ROADMAP.md`](GOLANG-ROADMAP.md) (a Go study roadmap) and
[`ARCHITECTURE.md`](ARCHITECTURE.md) (how the current code is organized),
this document records **features that are planned but not yet
implemented** — decisions already made about "what" and "how", for when
it's time to build them.

---

## 1. Distribution as a CLI binary (`dockzy` on the PATH)

**Status:** deferred — no packaging/installation work for now.

**Goal:** the user installs dockzy once and runs `dockzy` from inside any
project folder, as a real binary (`go install`, a GitHub release, a brew
tap, etc.), instead of `go run .` from inside the repo itself.

**Why it's mentioned now:** it's the motivation for the feature below —
"detecting which folder the user is in" only makes sense once dockzy runs
as an installed CLI, invoked from inside the user's project (today it
only runs from inside dockzy's own repo).

---

## 2. Docker Compose integration (per-project scoping)

**Status:** planned, spec defined, implementation not started.

**Goal:** when running `dockzy` inside a folder that has an associated
docker-compose project, show only that project's resources — containers,
images and volumes — instead of everything on the host. Same behavior as
lazydocker when run inside a project.

### 2.1 Project detection

Containers started by `docker compose` automatically carry two useful
labels:

- `com.docker.compose.project` — the project name (default: the folder
  name, or whatever is in `name:` in the compose file /
  `COMPOSE_PROJECT_NAME`).
- `com.docker.compose.project.working_dir` — the absolute path of the
  folder `docker compose up` was run from.

**Decision:** detect the project by comparing existing containers'
`com.docker.compose.project.working_dir` against `os.Getwd()` at dockzy's
startup — no need to parse the compose YAML or call the `docker
compose`/`docker-compose` CLI, just read labels the Docker API already
returns in `ContainerList`. `ListContainers` (`internal/docker`) already
reads `item.Labels`; it just needs to also capture
`com.docker.compose.project.working_dir` and compare it.

**Consequence:** a project can only be detected if at least one of its
containers is (or has been) running — there's no way to detect "this
directory has a compose file but nothing has been started yet" from the
Docker API alone. This is acceptable for the current scope (it's the
same limit `ListContainers` already has: it only sees what the daemon
knows about).

### 2.2 What gets filtered when a project is detected

**Decision:** total filtering, not just on Services.

- **Services** — only the containers whose
  `com.docker.compose.project.working_dir` matches the current cwd.
- **Standalone** — stays empty. Inside a specific compose project,
  "standalone" (a loose container, with no `com.docker.compose.service`)
  stops being a relevant concept for this screen.
- **Images** — only the images used by the project's containers (via the
  `Image`/`ImageID` of each listed container, or via the
  `com.docker.compose.project` label when present on the image).
- **Volumes** — only the volumes associated with the project, via the
  `com.docker.compose.project` label (named volumes created by compose
  carry this label too).

### 2.3 Status

**Decision:** instead of the static title `"lazydocker"` (currently
hardcoded in `internal/app/app.go`), the Status panel shows the detected
project's name and the names of its related services — same as
lazydocker.

### 2.4 No project detected

Current dockzy behavior, no change: loads all containers, images and
volumes on the host (that's what `app.Run` already does today).

### 2.5 `--all` flag

`dockzy --all` forces the "load everything on the host" behavior, even
when running from inside a folder with a detectable compose project.
Useful for anyone who wants the overall view while still being inside a
specific project.

### 2.6 Implementation notes (draft, not committed)

- `main.go` needs to start reading `os.Args` (today it just calls
  `app.Run()` with no argument) — probably `app.Run(all bool)` or an
  options struct if more flags show up later.
- `internal/docker` gains something like `DetectProject(ctx) (project
  string, workingDir string, ok bool)`, and
  `ListContainers`/`ListImages`/`ListVolumes` gain a way to filter by
  that `workingDir`/label — without breaking the current signature for
  the `--all`/no-project case.
- `internal/app/app.go` (composition root) decides, at the start of
  `Run`, whether to call the filtered version or the "everything"
  version of the three listings — the decision rule (`--all` OR no
  project detected → everything; otherwise → filtered) lives there, not
  inside `internal/docker`.
- `ui.Data.StatusTitle` (today a fixed `string`) will probably become
  something a bit more structured if we want to show "project name" +
  "services" as two distinct pieces of information, not just one loose
  string.
