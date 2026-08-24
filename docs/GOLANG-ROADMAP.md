# dockzy — Go learning roadmap

This document is a study roadmap, not an architecture reference (that's
what [`ARCHITECTURE.md`](ARCHITECTURE.md) is for). The idea is to take
every Go concept that actually shows up in dockzy's code and explain: what
it is, what it's for, when/why to use it, how it's being used here right
now, and how to recognize the next time you'll need it.

Prerequisite: you've already finished a Go course (basic syntax, types,
functions, structs). This document assumes that and focuses on the "why"
behind the decisions, not re-teaching syntax.

Format of each item:

- **What it is** — a direct definition.
- **What it's for** — the general problem it solves.
- **When to use it** — the signal that tells you "this calls for this
  concept".
- **Why use it** — the reason it exists over the alternatives.
- **How it applies here** — where and how it shows up in dockzy, with
  file:line references.
- **When you'll use it again** — the trigger to recognize it in future
  code.

Read in order — the numbering goes from what a basic course already
covers to what usually only shows up in practice, in real concurrent
projects.

---

## Part 0 — The project map in 30 seconds

```
main.go            → calls app.Run()
internal/docker/    → talks to the Docker daemon (no tview import)
internal/ui/         → tview widgets (no Docker client import)
internal/app/          → wires docker + ui, the only place with goroutines
```

Each part below points to one of these three packages. If you want the
"why" behind this layering, that's already in `ARCHITECTURE.md`, section
5. The focus here is: what *language concept* each piece of code
exercises.

---

## Part 1 — Code organization

### 1.1 Modules (`go.mod`) and import path

**What it is.** `go.mod` declares the *module path*
(`github.com/Gabriel-Valin/dockzy`) and the minimum Go version. Every
import inside the repo is `<module path>/<folder>`.

**What it's for.** It's what lets `go build`/`go get` resolve imports
without ambiguity, and it's the identifier other projects would use to
import your code (if it were a lib).

**When to use it.** Every Go project has a `go.mod` — it's not optional,
it's created once with `go mod init <path>`.

**Why use it.** Before modules (pre-Go 1.11) package resolution depended
on `$GOPATH`, a fixed folder on disk. Modules decouple "where the code
lives on your disk" from "what the import path is" — that's why `main.go`
imports `github.com/Gabriel-Valin/dockzy/internal/app` even when running
from inside the local clone.

**How it applies here.** `go.mod:1-3` declares the module and Go 1.26.3.
Every internal `import` in the project (`main.go:7`,
`internal/app/app.go:10-11`) uses that prefix.

**When you'll use it again.** Any time you create a new folder inside the
repo that's going to be imported from elsewhere — the import path is
always `module + path relative to the root`, never a relative path like
`../ui`.

---

### 1.2 `internal/` — visibility enforced by the compiler

**What it is.** Any package inside a folder called `internal/` can only
be imported by code that lives **inside the same tree above** that
`internal/`. It's a Go compiler rule, not a style convention.

**What it's for.** It prevents code outside the module (or from another
module in the same monorepo) from importing packages you consider
"implementation detail", without needing a comment or team discipline.

**When to use it.** Any package that isn't part of your module's public
API. If the project isn't a library (it's a binary, like dockzy),
normally **everything** goes into `internal/`.

**Why use it.** The alternative (root-level packages, like `/docker`,
`/ui`) is importable by any Go module on the planet as soon as your repo
goes public — even if that was never your intent. `internal/` closes that
door with no effort.

**How it applies here.** `internal/docker`, `internal/ui`, `internal/app`
— the project's three packages. `main.go` (outside `internal/`, but at
the root of the module itself, so it has access) is the only allowed
external consumer.

**When you'll use it again.** The moment you decide to extract some part
of dockzy to be reusable by another binary (for example, a CLI-only
version with no TUI) — that's when it's worth considering pulling
something out of `internal/` into a real public package.

---

### 1.3 Unidirectional dependency graph between packages

**What it is.** A project rule (not a language rule) *indirectly*
enforced by the language: Go refuses to compile a cyclic import (`A`
imports `B` which imports `A`), so a layered architecture can never
accidentally "escape" into a cycle without the build breaking right away.

**What it's for.** Each package can be read, understood and tested
without mentally loading the other two. `docker` doesn't know `ui`
exists; `ui` doesn't know a Docker client exists.

**When to use it.** Any time a project grows beyond "a single package" —
decide the direction of the dependency **before** writing the second
package, and protect that decision by reviewing imports.

**Why use it.** Without this discipline it's easy (especially coming from
other languages with DI containers) to end up with packages that import
each other "just a little", and Go blocks that: an import cycle is a
compile error, not a linter warning.

**How it applies here.** `docker` → nothing. `ui` → `docker` (only the
types `docker.Service`, `docker.Image` etc., see
`internal/ui/data.go:3`). `app` → `docker` + `ui`
(`internal/app/app.go:10-11`). Try importing `ui` inside
`internal/docker` and `go build` will close the cycle as soon as `ui`
imports `docker` back — the compiler is the enforcement.

**When you'll use it again.** Every time you add a new package, ask "who
imports whom" before writing the first line — see the checklist in
section 4.4 of `ARCHITECTURE.md`.

---

## Part 2 — Data types and composition

### 2.1 Structs as the domain model

**What it is.** A struct groups related fields under a named type.

**What it's for.** Representing a domain "thing" with all the data it
carries — instead of passing 4 loose variables around, you pass a
`Service`.

**When to use it.** Any time a set of values travels together (is born
together, is passed together, makes sense together).

**Why use it.** Without a struct you get "primitive obsession" —
functions with 5 `string` parameters, easy to swap the order by mistake
and with no name for the set. With a struct the compiler forces you to
name the fields (or respect declaration order) and you get a type with
its own identity.

**How it applies here.** `docker.Service` and `docker.Standalone`
(`internal/docker/docker.go:14-27`) — same shape (`State`, `Name`, `CPU`,
`ID`), but different purpose-specific types because a "compose" container
and a "standalone" one should never be confused in a function signature.
`docker.ContainerInfo` (`internal/docker/info.go:15-20`) groups the 4
tabs of the right panel. `ui.Data` (`internal/ui/data.go:10-20`) groups
everything `Dashboard` needs to draw the initial screen.

**When you'll use it again.** Every time you notice two or more functions
receiving/returning the same 3+ values together — that's a sign it should
be a struct.

---

### 2.2 Embedding — composition instead of inheritance

**What it is.** Placing a type (or pointer to a type) **without a field
name** inside a struct. The embedded type's methods "bubble up" to the
outer struct automatically.

**What it's for.** Extending/decorating a third-party type with your own
methods, without having to manually rewrite/delegate every method it
already has.

**When to use it.** When you want "is-a" with extension — your type
should behave like the embedded type, plus something extra.

**Why use it.** Go doesn't have class inheritance. Embedding is the
language's mechanism for behavior reuse through composition: you get all
the embedded type's methods "for free", but without the rigid coupling of
a class hierarchy — the embedded type is still just a field, replaceable.

**How it applies here.** `docker.Client` (`internal/docker/docker.go:31-33`):

```go
type Client struct {
    *client.Client // embedded, no field name
}
```

This is what lets `c.ContainerList(...)`, `c.ContainerStats(...)`,
`c.ContainerLogs(...)` (called in `docker.go`, `stats.go`, `info.go`) work
directly on `c *docker.Client` — they are methods of the `moby/moby/client`
lib's `*client.Client`, "inherited" through embedding. At the same time,
`docker.Client` can declare its own methods on top (`ListContainers`,
`StreamCPU`, `Info`) that **don't** exist in the original lib — that's the
"plus something extra" part of the composition.

**When you'll use it again.** When you want to wrap a third-party type to
add domain-specific convenience methods, without reimplementing its
entire API.

---

### 2.3 Methods and receivers — pointer vs. value

**What it is.** `func (c *Client) ListContainers(...)` — `c *Client` is
the *receiver*: `ListContainers` is a method of `*Client`, called as
`cli.ListContainers(...)`.

**What it's for.** Attaching behavior to a type. A pointer receiver
(`*Client`) lets the method see/change the same value the caller has,
without copying the whole struct on every call.

**When to use it.** Pointer (`*T`): the type is "heavy" (large structs) or
the method needs to mutate the receiver, or the type is already used by
pointer elsewhere (consistency). Value (`T`): small, immutable types
(structs with 2-3 primitive fields, types that represent a value, not an
identity).

**Why use it.** If `Client` had a value receiver, every method call would
copy the whole embedded `client.Client` (which carries HTTP connection
state) — semantically wrong and expensive. A pointer guarantees
`cli.Close()` closes the real connection, not a copy of a copy.

**How it applies here.** Every method of `docker.Client` uses a `*Client`
receiver: `ListContainers` (`docker.go:48`), `StreamCPU` (`stats.go:23`),
`Info` (`info.go:25`). In `app.go:20`, `cli, err := docker.New()` already
returns `*Client` — `cli` is always a pointer passed along.

**When you'll use it again.** When creating any type that represents a
"service" or "client" with state (connection, cache, counter) — use a
pointer by default. For a purely data type you don't intend to mutate
through the method (e.g. a `formatCPUPercent` that only reads `stats`), it
can be a standalone function, it doesn't even need to become a method.

---

### 2.4 Slices — `append` and `make` with capacity

**What it is.** `append(services, Service{...})` grows a slice; `make([]T,
0, n)` creates an empty slice with capacity already reserved for `n`
elements.

**What it's for.** Dynamically-sized collections. `make(..., 0, n)` avoids
repeated reallocations when you already know (or estimate) the final
size.

**When to use it.** `append` whenever the list grows in a loop.
`make(T, 0, cap)` when you know (or have a reasonable ceiling for) the
final size before the loop starts.

**Why use it.** Without pre-allocated capacity, every `append` that
exceeds current capacity forces the runtime to allocate a new (bigger)
array and copy everything — O(log n) reallocations over the growth, but
still avoidable work when the size is known.

**How it applies here.** `ListContainers` (`docker.go:48-66`) uses plain
`append` (`services = append(services, Service{...})`) because there's no
way to know beforehand how many containers will land in each group.
`Dashboard.RunningContainerIDs` (`internal/ui/dashboard.go:189`), on the
other hand, uses `make([]string, 0, len(d.rows))` — the maximum ceiling
(every running container) is known right there.

**When you'll use it again.** Every time you populate a list inside a
loop: if the final size is known or has a cheap-to-compute ceiling,
pre-allocate with `make`; otherwise, `append` starting from `nil` (a `nil`
slice is perfectly valid to `append` to).

---

### 2.5 Maps as an index

**What it is.** `rows map[string]*liveRow` — a map from `container ID` to
`*liveRow` (a struct that knows where that row is on screen).

**What it's for.** O(1) lookup by key, when you receive an identifier
(here, the container's ID) and need to quickly find "what does this
identifier correspond to".

**When to use it.** Any time the recurring operation is "given X, find Y"
and X isn't a sequential index (0, 1, 2...) — if it were, a slice would
be enough.

**Why use it.** The alternative would be walking the entire container
list on every CPU update to find the right row — O(n) on every update,
multiple times per second per container. The map pays the indexing cost
**once**, when the dashboard is built.

**How it applies here.** `internal/ui/dashboard.go:57-63` builds `rows`
once, inside `New`, walking `data.Services` and `data.Standalone` in the
same order the items were added to the visual lists — that's why the
`index` stored there always matches the real `tview.List` row. After
that, `UpdateCPU` (`dashboard.go:200-209`) just does
`row, ok := d.rows[id]` to find the row in O(1) every time a new CPU
sample arrives.

**When you'll use it again.** Any new per-container dynamic feature
(memory, network I/O) reuses this same `d.rows` — that's exactly what the
`liveRow` section of `ARCHITECTURE.md` explains in detail. A new index
(e.g. by *image* ID) needs a **separate** map, because container IDs and
image IDs aren't guaranteed to be disjoint.

---

### 2.6 Local/anonymous structs

**What it is.** A `type result struct {...}` declared **inside** a
function, visible only there.

**What it's for.** Grouping values that only make sense as "this
function's internal result", without cluttering the package with a type
no one else uses.

**When to use it.** When the struct is a single function's internal
implementation detail — it will never be a parameter or a public return
value.

**Why use it.** Declaring this as a package-level type (`type Result
struct{...}` at the top of the file) would export a concept that only
exists to orchestrate 4 goroutines inside a function — unnecessary noise
in the package's API.

**How it applies here.** `Info` (`internal/docker/info.go:26-30`):

```go
func (c *Client) Info(ctx context.Context, id string) (ContainerInfo, error) {
    type result struct {
        field string
        text  string
        err   error
    }
    ...
```

`result` only exists to carry "which field, which text, which error" back
through the channel — no one outside `Info` needs to know this type
exists.

**When you'll use it again.** Any time you need a "bundle of fields" that
only serves to pass data within a function (common in fan-out/fan-in
patterns, like item 5.4 below).

---

## Part 3 — Errors and multiple returns

### 3.1 `error` as a return value

**What it is.** Go doesn't have exceptions. A function that can fail
returns `(T, error)`, and the caller **decides** what to do by checking
`if err != nil`.

**What it's for.** Making the error path explicit in the function's type
— whoever reads the signature already knows it can fail, without having
to read the whole body or a separate doc looking for `throws`.

**When to use it.** Any operation that can fail in an expected way (I/O,
network, parsing, an external API call).

**Why use it.** It forces the caller to decide at the call site — there's
no such thing as an "unhandled error silently bubbling up" in Go like in
languages with unchecked exceptions; if you ignore `err`, it's because you
explicitly wrote `_`.

**How it applies here.** Throughout the `docker` package: `New()`
(`docker.go:37-43`), `ListContainers` (`docker.go:48`), `Info`
(`info.go:25`) and the 4 private functions it calls in parallel — each
returns `(string, error)` and the error becomes visible text on the tab
("erro: %s", `info.go:57`) instead of crashing the application.

**When you'll use it again.** Every new function in `internal/docker`
that talks to the daemon follows this pattern — it's the project's
external I/O boundary, so it's where errors are most likely and most need
to be handled explicitly (the general rule of "validating at system
boundaries", already in your global `CLAUDE.md`).

---

### 3.2 Named returns

**What it is.** `func (c *Client) ListContainers(ctx context.Context) (services
[]Service, standalone []Standalone, err error)` — the return names appear
in the signature, not just in the body.

**What it's for.** Documenting, right in the signature, what each return
value *means* — important when there's more than one return of the same
type (here, two slices) and the order alone wouldn't make clear which is
which to someone who only reads the signature.

**When to use it.** When two or more returns have the same/similar types
and the name avoids ambiguity (`services, standalone` instead of two
anonymous `[]Service`). Avoid overusing it in simple functions — a named
return isn't mandatory and doesn't always help.

**Why use it.** Without a name, whoever reads the signature from the
outside (editor autocomplete, `godoc`) only sees `([]Service, []Standalone,
error)` — the names show up in the IDE and in `go doc` as part of the
implicit documentation.

**How it applies here.** `internal/docker/docker.go:48`. Note that,
despite being named, the function body still uses `return nil, nil, err`
and `return services, standalone, nil` explicitly (it doesn't use a bare
`return`) — that's a valid stylistic choice: the names serve only as
documentation here, not as a return shortcut.

**When you'll use it again.** When two returns of the same plain type (two
`string`s, two `[]T`s) could be confused by someone just looking at the
signature.

---

### 3.3 `defer` for guaranteed cleanup

**What it is.** `defer expr` schedules `expr` to run when the **current**
function returns — no matter which `return` it exited through, or whether
it exited via `panic`.

**What it's for.** Guaranteeing that an open resource (connection, file,
HTTP response body) gets closed, **without duplicating** that call at
every `return` in the function.

**When to use it.** Right after opening/acquiring any resource that needs
cleanup — the Go convention is to write the `defer` on the **line right
after** the opening, never at the end of the function (where it would be
easy to forget if an early `return` is added later).

**Why use it.** Without `defer`, any function with multiple `return`s
would need to repeat `result.Body.Close()` before each one — easy to
forget one error path and leak the connection.

**How it applies here.** `app.Run` (`internal/app/app.go:18,24`): `defer
cancel()` right after creating the context, `defer cli.Close()` right
after opening the client. `StreamCPU` (`internal/docker/stats.go:28`):
`defer result.Body.Close()` — closes the HTTP stream no matter whether the
function exits because the `for` ends, because of a decode error, or
because of `ctx.Done()`.

**When you'll use it again.** Any `Open`, `New`, call that returns
something with a `.Close()`/`.Body`/file handle — `defer` the cleanup on
the next line, always.

---

## Part 4 — Functions as values: closures and callbacks

### 4.1 Closures — functions that capture variables from the surrounding scope

**What it is.** A function literal (`func() {...}`) that references
variables declared *outside* of it keeps seeing (and being able to
mutate) those variables even after the surrounding function has returned.

**What it's for.** Creating parameterized behavior without needing a
dedicated struct just to carry that state — the function itself "carries"
what it needs.

**When to use it.** When you need to pass "behavior with context" to
something that only accepts `func(...)` — UI callbacks, handlers.

**Why use it.** The alternative (a struct with a method, implementing a
single-method interface) is more verbose for small cases — closures give
the same result with less boilerplate when the "behavior" is used once,
in a single place.

**How it applies here.** `internal/ui/dashboard.go:95-110`:

```go
selectServices := func() {
    if d.onSelectContainer == nil || len(data.Services) == 0 {
        return
    }
    if idx := servicesBox.GetCurrentItem(); idx >= 0 && idx < len(data.Services) {
        d.onSelectContainer(data.Services[idx].ID)
    }
}
```

`selectServices` captures `d`, `data` and `servicesBox` — all local
variables of `New`. It's reused in two different places
(`SetChangedFunc` and `SetFocusFunc`, lines 112-123) without needing to
repeat the logic or create a new type just for it.

**When you'll use it again.** Any widget callback (`SetFocusFunc`,
`SetBlurFunc`, `SetChangedFunc`, `SetInputCapture`) that needs access to
data from `New`'s scope — it's the natural pattern in callback-driven UI
code.

---

### 4.2 Callbacks / behavior injection via a function field

**What it is.** A struct field of type `func(...)` (here,
`onSelectContainer func(id string)` on `Dashboard`,
`internal/ui/dashboard.go:30`), set from the outside via a public method
(`OnSelectContainer`, line 182-184).

**What it's for.** Letting `ui` "notify" `app` of an event (selection
changed) **without** `ui` knowing anything about what `app` will do with
that notification — inversion of control without an interface, without a
cross import.

**When to use it.** When the lower-level package (`ui`) needs to notify
the upper-level package (`app`) of something, but can't import that
upper-level package (that would break the rule in section 1.3).

**Why use it.** If `ui` imported `app` to call something directly, you'd
have an import cycle. A `func(...)` field solves this: `ui` only knows
the *shape* of the function (its signature), never the package that
implements it.

**How it applies here.** `Dashboard.OnSelectContainer` is called in
`app.go:89` with a closure (`onSelectContainer`, defined in
`app.go:72-88`) that knows how to cancel previous fetches and call
`cli.Info`. `ui` never imports `docker` nor knows `Info` exists — it only
knows that, when the user switches the selected row, it should call the
function that was registered.

**When you'll use it again.** Every UI-originated event that needs to
trigger work outside the UI (a new key, a menu action) follows this same
pattern: a `func(...)` field on `Dashboard` + a public setter + `app.go`
provides the real implementation.

---

### 4.3 Anonymous functions invoked immediately in a goroutine

**What it is.** `go func() { ... }()` — declares an anonymous function and
immediately starts it as a goroutine, without naming it or reusing it
anywhere else.

**What it's for.** Running a block of code in parallel when that block
doesn't need to exist as a named function anywhere — just there, once.

**When to use it.** Work that only happens at that point in the flow and
whose logic isn't reused.

**Why use it.** Naming and exporting a function just to call it once with
`go` would add indirection with no gain — the anonymous function keeps
the intent ("this runs in parallel, here, now") readable right at the
point of use.

**How it applies here.** `internal/app/app.go:53-62` (the CPU channel
consumer) and `app.go:81-88` (the `Info` fetch after selection) — both are
anonymous goroutines, each running exactly once per `go` call.

**When you'll use it again.** Any time you start concurrent work that
doesn't need its own name — that's the common case. Reserve a named
function (like `StreamCPU`) for when the same block of work is going to be
launched multiple times with different parameters (one goroutine per
container, for example).

---

## Part 5 — Concurrency (the part a basic course usually only skims)

### 5.1 Goroutines

**What it is.** `go f(...)` starts `f` in a goroutine — a thread managed
by the Go runtime, much cheaper than an OS thread (initial stack of
~2KB, grows on demand).

**What it's for.** Running concurrent work without blocking whoever
called `go` — the line right after `go` executes immediately, without
waiting for `f` to finish.

**When to use it.** When you have N independent units of work (one per
container, for example) that can progress without waiting on each other,
or when something needs to run "in the background" while the rest of the
program continues.

**Why use it.** Without goroutines, `StreamCPU` for 10 containers would
run sequentially — the second container would only start receiving
updates after the first (which runs forever, until `ctx` is cancelled)
finished. It would never finish. Goroutines give real parallelism (or
cooperative concurrency, depending on `GOMAXPROCS`) with no external lib
at all.

**How it applies here.** `internal/app/app.go:50`: `go
cli.StreamCPU(ctx, id, cpuUpdates)` — one goroutine per running container.
`app.go:53`: one goroutine consuming the channel. `info.go:35-50`: 4
parallel goroutines (logs/stats/config/top) inside a single call to
`Info`.

**When you'll use it again.** Any I/O call (network, disk) that doesn't
need to block the rest of the flow, or any situation of "N independent
units of work, I want all of them in parallel".

---

### 5.2 Channels — buffered, unbuffered and directional

**What it is.** A channel is a typed pipe through which goroutines
exchange values with built-in synchronization. `chan T` (regular
channel), `chan<- T` (write-only, from the perspective of whoever
receives the parameter), `<-chan T` (read-only). `make(chan T, n)`
creates a *buffered* channel with capacity `n`; `make(chan T)` is
*unbuffered*.

**What it's for.** Communicating data **and** synchronizing between
goroutines at the same time — "don't proceed until the other side is
ready" (unbuffered) or "accumulate up to N messages without blocking the
sender" (buffered).

**When to use it.** Any time one goroutine produces values that another
goroutine needs to consume — it's Go's idiomatic mechanism for this
(instead of a manually mutex-protected queue).

**Why use it.** The classic Go phrase: *"Don't communicate by sharing
memory; share memory by communicating"*. Channels avoid data races by
construction — there's no "forgot to lock this shared variable" because
there's no shared variable, there's a message passed.

**How it applies here.**

- `internal/app/app.go:48`: `cpuUpdates := make(chan docker.CPUUpdate,
  len(services)+len(standalone))` — buffered with capacity for the worst
  case (every container sending an update at the same time), so no
  `StreamCPU` goroutine gets stuck waiting for the consumer.
- `internal/docker/stats.go:23`: signature `updates chan<- CPUUpdate` —
  directional, write-only. `StreamCPU` **cannot** read from this channel
  by mistake, the compiler blocks it — it can only `<-` from outside, or
  send into it.
- `internal/docker/info.go:33`: `results := make(chan result,
  len(fields))` — buffered with capacity 4, one per fetch goroutine.

**When you'll use it again.** Any "several goroutines produce, one
consumes" pattern (section 5.4 below). Prefer a directional channel
(`chan<-`/`<-chan`) in the signature of any function that only sends or
only receives — it documents the intent and the compiler enforces it.

---

### 5.3 `select` — multiplexing channel operations

**What it is.** `select { case <-a: ...; case b <- v: ...; }` picks one
operation to execute among several ready channel operations (at random,
if more than one is ready at the same time).

**What it's for.** Waiting for "whatever happens first" among multiple
sources — in this project, always "send the data" vs. "I was cancelled".

**When to use it.** Any time a goroutine needs to compete between
sending/receiving on a channel **and** respecting cancellation — without
`select`, a blocking `updates <- v` would hang forever if no one else is
reading `updates` (for example, if the application already exited).

**Why use it.** It's the only idiomatic way to say "try to send this, but
give up if the context gets cancelled while doing so" — without
`select`, you'd need manual timeouts or a much more complicated design.

**How it applies here.** `internal/docker/stats.go:37-41`:

```go
select {
case updates <- CPUUpdate{ID: id, CPU: formatCPUPercent(stats)}:
case <-ctx.Done():
    return
}
```

If the consumer (`app.go:53-62`) has already exited (context cancelled),
the `case <-ctx.Done()` frees this goroutine instead of it hanging
forever on a send that would never have a receiver. The consumer, in
turn, also uses `select` (`app.go:54-61`) to choose between "an update
arrived" and "I was cancelled".

**When you'll use it again.** Any streaming goroutine that does `chan <-
value` inside a loop **always** needs this `select` alongside
`ctx.Done()` — sending directly (`updates <- v` without `select`) is the
classic goroutine-leak bug once no one reads the channel anymore.

---

### 5.4 Fan-out / fan-in pattern

**What it is.** *Fan-out*: launching N goroutines that do independent
work in parallel. *Fan-in*: converging the N results back into one place
through a shared channel.

**What it's for.** Parallelizing N independent operations (here, 4 Docker
API calls that don't depend on each other) while still "waiting for all
of them to finish" simply, without `sync.WaitGroup` plus a
mutex-protected variable.

**When to use it.** When you have a **fixed and known** number of
parallel tasks and need all of their results before continuing.

**Why use it.** Fetching logs, stats, config and top sequentially means
total latency is the sum of the 4 calls. In parallel, it's the
**maximum** of the 4 — the right panel appears as soon as the slowest one
finishes, not the sum of all of them.

**How it applies here.** `internal/docker/info.go:25-71`:

```go
results := make(chan result, len(fields))   // fan-in: 1 channel, N producers

go func() { ...; results <- result{"logs", text, err} }()   // fan-out
go func() { ...; results <- result{"stats", text, err} }()
go func() { ...; results <- result{"config", text, err} }()
go func() { ...; results <- result{"top", text, err} }()

var info ContainerInfo
for range fields {          // receives exactly 4 times — knows when to stop
    r := <-results
    ...
}
```

The `for range fields` (4 iterations, because `fields` has 4 elements) is
what replaces a `sync.WaitGroup` here — since the number of goroutines is
fixed and known, "I received 4 messages" is logically identical to "I
waited for all 4 to finish".

**When you'll use it again.** Any fetch of N independent values that need
to be combined into a single result before continuing — if N is fixed,
this pattern (buffered channel of capacity N + `for range` N times) is
simpler than `WaitGroup`. If N is dynamic (you don't know how many
goroutines ahead of time), then `sync.WaitGroup` is the right tool (see
Part 9).

---

### 5.5 `context.Context` — propagated cancellation

**What it is.** A `context.Context` carries a "this should stop" signal
(among other things, but cancellation is dockzy's use case).
`ctx.Done()` returns a channel that closes when the context is
cancelled; `ctx.Err()` says why.

**What it's for.** Propagating "stop whatever you're doing" through a
chain of calls and goroutines, without every layer having to invent its
own cancellation mechanism.

**When to use it.** Any long-running I/O operation or long-lived
goroutine that needs to be interruptible from outside — streams, network
calls, timeouts.

**Why use it.** Without context, cancelling 10 CPU streams running in
parallel would require 10 manual mechanisms (10 `done` channels, or 10
`atomic.Bool` flags). With context, **one** `cancel()` propagates to
everyone who received that `ctx` (or a derivative of it) at creation.

**How it applies here.**

- `internal/app/app.go:17`: `ctx, cancel := context.WithCancel(context.Background())`
  — the whole application's root context, created once.
- That same `ctx` is passed to `StreamCPU` (line 50) and to `cli.Info`
  via `selCtx` (line 77, derived from `ctx` — cancelling `ctx` cancels
  `selCtx` too, it's hierarchical).
- `cancel` becomes the `onQuit` passed to `ui.New` (line 44) — when the
  user presses `q`, `dashboard.go:163-169` calls `onQuit()`, which is that
  `cancel`, which closes `ctx.Done()`, which makes every `select` from
  section 5.3 return.
- `selectCancel` (line 70, 74-78): a **new** `context.WithCancel(ctx)` on
  every container selection — it specifically cancels the **previous**
  selection's fetch, without affecting the rest of the application.
  That's what prevents a slow `Info` call from overwriting the screen
  with stale data after the user has already moved to a different row
  (see section 3.3 of `ARCHITECTURE.md`).

**When you'll use it again.** Every new function that makes a network
call should receive `ctx context.Context` as its first parameter (a
strong Go convention) and respect it — either via `select` with
`ctx.Done()` (streams) or by checking `ctx.Err()` after a blocking call
(single fetches, like `info.go:36` implicitly does by checking
`selCtx.Err()` in `app.go:83`).

---

### 5.6 `sync.Mutex` — protecting shared state

**What it is.** A lock: `mu.Lock()` blocks until it gets exclusive
access; `mu.Unlock()` releases it. Only one goroutine gets past `Lock()`
at a time.

**What it's for.** Protecting a variable that **multiple goroutines**
might read/write at the same time, when a channel would be
over-engineering for the case.

**When to use it.** When you have genuinely shared mutable state (not a
message flow, a value that several goroutines touch) and the
channel-based alternative would be more complex than the problem.

**Why use it.** `selectCancel` (the function that cancels the previous
selection's fetch) is written and read from any call to
`onSelectContainer` — and that function runs on the main goroutine
(called from inside a `SetChangedFunc`), but the cancellation itself
happens in a way that **can race** with a following selection triggered
quickly (user navigating the lists fast). The mutex guarantees that
reading `selectCancel`, deciding to call it, and writing the new value is
an atomic operation.

**How it applies here.** `internal/app/app.go:68-79`:

```go
var (
    selectMu     sync.Mutex
    selectCancel context.CancelFunc
)
onSelectContainer := func(id string) {
    selectMu.Lock()
    if selectCancel != nil {
        selectCancel()
    }
    selCtx, selCancel := context.WithCancel(ctx)
    selectCancel = selCancel
    selectMu.Unlock()
    ...
```

**When you'll use it again.** When two or more goroutines (or callbacks
that can interleave) read **and** write the same variable. If the
pattern is just "producer(s) → single consumer", prefer a channel
(section 5.2) — a mutex is for when there's no natural message flow, just
a genuinely shared value.

---

## Part 6 — Implicit interfaces (structural duck typing)

**What it is.** In Go you don't write `implements Foo`. A type
automatically satisfies an interface if it has the methods it requires —
the check is structural, at compile time, with no explicit declaration
of intent.

**What it's for.** Decoupling whoever *defines* the contract (the
interface) from whoever *implements* it — the implementer doesn't even
need to know the interface exists.

**When to use it.** When writing a function/type that only needs *part*
of something's behavior, accept the smallest interface possible instead
of the concrete type — that's the "accept interfaces, return structs"
principle common in idiomatic Go.

**Why use it.** This is what lets completely unrelated libraries "fit
together": you never need `moby/moby/client` and `encoding/json` to have
been written with each other in mind.

**How it applies here.**

- `json.NewDecoder(r io.Reader)` (`internal/docker/stats.go:30`,
  `info.go:47`) — `result.Body` (the body of an HTTP response from the
  Docker client) satisfies `io.Reader` just by having a
  `Read([]byte) (int, error)` method. Neither `encoding/json` nor the
  Docker client needed to be written with the other in mind.
- `stdcopy.StdCopy(&out, &out, result)` (`info.go:86`) — `&out` is a
  `*bytes.Buffer`, which satisfies `io.Writer`; `result` satisfies
  `io.Reader`.
- `d.focusables []tview.Primitive` (`internal/ui/dashboard.go:27,78-80`)
  — `*tview.List`, `*tview.Flex` and `tabs.root` (also a `*tview.Flex`)
  go into the same slice because they all satisfy the lib's
  `tview.Primitive` interface.

**When you'll use it again.** When writing a new function that only needs
"to read bytes from somewhere" or "to write text somewhere", accept
`io.Reader`/`io.Writer` instead of the concrete type (`*os.File`,
`*bytes.Buffer`) — your function then works with any source/destination
already implemented by the stdlib or third parties.

---

## Part 7 — Streaming and text formatting

### 7.1 Streaming JSON decoding (a `Decode` loop)

**What it is.** `json.NewDecoder(r).Decode(&v)` called **repeatedly** on
the same `Decoder`, inside a loop, instead of a single `json.Unmarshal`
call.

**What it's for.** Reading a stream of multiple concatenated JSON objects
(one per line/chunk, like Docker sends in `ContainerStats` with
`Stream: true`) without needing to know where one object ends and the
next begins — the `Decoder` knows.

**When to use it.** Any HTTP response with `Transfer-Encoding: chunked`
that sends multiple JSON objects over time through the same open
connection (server-sent-events-like).

**Why use it.** `json.Unmarshal([]byte, &v)` requires the entire payload
in memory upfront and decodes exactly **one** value. A stream never
"ends" until the container stops — there's no way to wait for the whole
body.

**How it applies here.** `internal/docker/stats.go:30-42`:

```go
decoder := json.NewDecoder(result.Body)
for {
    var stats container.StatsResponse
    if err := decoder.Decode(&stats); err != nil {
        return
    }
    select { case updates <- CPUUpdate{...}: case <-ctx.Done(): return }
}
```

Each iteration of the `for` consumes exactly one JSON object from the
open HTTP stream (one stats sample per second) — the `Decoder` keeps
track of where it left off reading between calls.

**When you'll use it again.** Any API (Docker or otherwise) that exposes
a "streaming" endpoint of JSON events over the same connection — the
pattern is always `Decoder` + loop, never `Unmarshal` in a loop reading
bytes manually.

---

### 7.2 `bytes.Buffer` and `strings.Builder`

**What it is.** Two stdlib types for **building** text/bytes efficiently,
instead of concatenating strings with `+=` in a loop.

**What it's for.** `strings.Builder` accumulates text and becomes a
`string` only at the end (`.String()`); `bytes.Buffer` does the same but
also implements `io.Writer` and `io.Reader`, so I/O functions can write
into it directly.

**When to use it.** `strings.Builder`: when you only need to build a
string from several parts, within your own code (with
`fmt.Fprintf(&b, ...)` or `.WriteString(...)`). `bytes.Buffer`: when the
destination needs to be a real `io.Writer` because another I/O function
(that doesn't know about `strings.Builder`) is going to write into it.

**Why use it.** `s := s + parte` in a loop reallocates **a whole new
string** on every concatenation (strings in Go are immutable) — O(n²) in
the total size. `Builder`/`Buffer` grow an internal buffer amortized,
like a slice.

**How it applies here.**

- `containerConfig` (`internal/docker/info.go:147-171`) uses
  `strings.Builder` — it only needs to become a `string` at the end
  (`return b.String()`), no external function writes into it.
- `containerLogs` (`internal/docker/info.go:85-92`) uses `bytes.Buffer`
  because `stdcopy.StdCopy(&out, &out, result)` (a third-party function)
  needs a real `io.Writer` to split stdout/stderr — a `strings.Builder`
  wouldn't implement `io.Writer` the way `StdCopy` needs here (it
  actually does implement `io.Writer` too, but `bytes.Buffer` is the
  natural choice when the result will later be read as bytes, or when
  there's doubt about which of the two the destination requires).

**When you'll use it again.** Any function that builds a long string from
several formatted pieces in a loop — never `+=` in a loop, always
`Builder`/`Buffer` + `.String()` at the end.

---

### 7.3 `fmt.Sprintf` / `fmt.Fprintf`

**What it is.** `Sprintf` formats and returns a `string`. `Fprintf`
formats and **writes directly** into an `io.Writer` (avoids `Sprintf`'s
intermediate allocation plus `.WriteString`).

**What it's for.** Interpolating values into text with verbs (`%s`,
`%d`, `%.2f`, `%c`) and precise control over decimal places/width.

**When to use it.** `Sprintf` when you need the result as a value (a
function's return, another call's argument). `Fprintf` when you already
have an `io.Writer`/`Builder`/`Buffer` in hand and are just accumulating.

**Why use it.** Typed verbs (`%.2f` for 2 decimal places, `%d` for an
integer) let the compiler (via `go vet`) check whether the argument types
match the verbs — a common mistake (`%d` with a `string`) becomes a build
warning, not a bug in production.

**How it applies here.** `formatCPUPercent`
(`internal/docker/stats.go:73`): `fmt.Sprintf("%.2f%%", ...)`.
`formatBytes` (`info.go:195-202`): `fmt.Sprintf("%.1f%ciB", ...,
"KMGTPE"[exp])` — note the `%c` receiving a `byte` indexed from a string
literal, a common trick for picking the unit prefix (K, M, G...) without
a `switch`. `containerConfig` uses `Fprintf(&b, ...)` repeatedly because
`b` is already the destination `strings.Builder`.

**When you'll use it again.** Always. It's Go's standard formatting tool
— the only real decision is `Sprintf` vs `Fprintf` depending on whether
you already have a `Writer` in hand.

---

## Part 8 — Idiomatic patterns that show up in the project

### 8.1 Composition root

**What it is.** A single point in the program (here, `app.Run`) that
assembles **all** the concrete dependencies and connects them — no other
place in the code does `New()` for anything related to another layer.

**What it's for.** Having **one** place to answer "what exists in this
program and how do the pieces connect", instead of dependencies being
created scattered across the code.

**When to use it.** Every non-trivial program benefits from having an
explicit composition root — it's not exclusive to Go, but Go (with no DI
framework) makes it more visible: it's just a function.

**Why use it.** Without this, it would be easy for `internal/ui` to
create its own `docker.Client` internally "to be practical" — and then
`ui` would start depending on `docker`, breaking the rule from section
1.3. Centralizing the assembly in `app.Run` is what **maintains** the
layer separation for real, not just illustrated in `ARCHITECTURE.md`.

**How it applies here.** `internal/app/app.go`, the entire `Run` function
— it creates the `docker.Client`, builds `ui.Data`, calls `ui.New`,
starts every goroutine, registers the selection callback. It's the only
file in the project that imports `internal/docker` **and** `internal/ui`
at the same time.

**When you'll use it again.** Every time you add a new dependency (a
second API client, a cache, a configured logger) — it's born in
`app.Run`, never inside `ui` or `docker` trying to self-instantiate.

---

### 8.2 Placeholder / loading value instead of empty state

**What it is.** Populating a field with text like `"carregando..."` or
`"0.00%"` **before** the real data arrives, instead of leaving it
empty/zero-value with no context.

**What it's for.** The UI never shows "nothing" — it always shows a
readable state, even before the first fetch completes.

**When to use it.** Any value that gets filled in asynchronously after
the initial screen assembly.

**Why use it.** Leaving `Logs: ""` would render an empty tab
indistinguishable from "container with no logs" — the user wouldn't know
whether it's still loading or genuinely has nothing.

**How it applies here.** `internal/app/app.go:31-42`: `const loading =
"carregando...\n"`, used in the 4 text fields of `ui.Data` until the
first `Info` call completes. `docker.go:58,63`: initial `CPU: "0.00%"`,
until `StreamCPU` sends the first real sample.

**When you'll use it again.** Every field populated asynchronously needs
an initial value that communicates "loading", not an ambiguous
zero-value.

---

## Part 9 — What doesn't show up in the project yet (but you'll need)

These concepts have no example in dockzy today — it's up to you to
recognize when the project grows enough to need them.

- **Explicit interfaces to enable testing** — today `app.Run` calls
  `docker.New()` directly; to test `app.Run` without a real Docker
  daemon, you'd define a small interface (`type containerLister
  interface { ListContainers(ctx) (...) }`) that `*docker.Client` already
  implicitly satisfies (section 6), and a fake would implement it in
  tests.
- **`sync.WaitGroup`** — needed when the number of parallel goroutines is
  **dynamic** (not fixed like `Info`'s 4 fields). E.g.: if
  `RunningContainerIDs()` had zero containers, the fan-in pattern's `for
  range fields` (section 5.4) would still work because `fields` has a
  fixed size — but a fan-out over "N containers, N unknown until
  runtime" would need `wg.Add(1)` per goroutine and `wg.Wait()`.
- **Error wrapping (`%w`, `errors.Is`/`errors.As`)** — today errors are
  only returned or formatted as text (`fmt.Sprintf("erro: %s", r.err)`,
  `info.go:57`). If you ever need to programmatically distinguish
  "container not found" from "daemon unreachable" (not just display
  text), this is the tool.
- **Generics** — no generic type in the project today; it would show up
  if you created, for example, a `streamLoop[T any](ctx, fetch func()
  (T, error), updates chan<- T)` function to avoid duplicating
  `StreamCPU`'s structure in a future `StreamMemory`.
- **Table-driven tests** — the project still has no `_test.go` files
  (the project's `CLAUDE.md` already notes this). `formatCPUPercent`,
  `formatBytes`, `formatRow`, `padRight` are obvious candidates for the
  first tests — they're pure functions, no I/O, easy to test with a
  table of cases.

---

## Part 10 — How to evolve the project using what you just learned

When adding a new feature, section 4 of `ARCHITECTURE.md` ("Adding a new
feature") already gives you the step-by-step recipe — the value of this
document is giving you the vocabulary to understand *why* each step of
that recipe is what it is:

1. **Decide static vs. dynamic** (`ARCHITECTURE.md` §4.1) → static is "a
   function with `(T, error)`" (Part 3 here); dynamic is "a goroutine
   with `select` + `chan<-`" (Parts 5.1-5.3).
2. **`docker` layer** → a new method on `Client` (Part 2.3), returning an
   explicit error (Part 3.1), streaming via `select`+`ctx.Done()` if
   dynamic (Part 5.3).
3. **`ui` layer** → a setter on `Dashboard` that always wraps the
   mutation in `QueueUpdateDraw` (mentioned in `ARCHITECTURE.md`, it's
   the tview version of "only the UI goroutine touches the UI" — it's
   not `sync.Mutex`, it's a work queue for a single goroutine).
4. **`app` layer** → a new goroutine (Part 5.1) consuming a new channel
   (Part 5.2), with the same `ctx` (Part 5.5) that already exists.

Reread this document whenever a section of `ARCHITECTURE.md` mentions a
term that doesn't make sense yet — it's likely the concept is explained
somewhere above here.
