# ShopFlow (Go) — learn Go by building it, test-first

The Go twin of the .NET ShopFlow course: the same small order-processing backend, the same SQLite data, the same 11 steps — but every step is rebuilt the **Go way**: goroutines and channels instead of `async/await`, `context.Context` instead of `CancellationToken`, error values instead of exceptions, tiny implicit interfaces instead of big ones.

Each step has

- a **PRD** in `docs/steps/` that tells you *what* to build and *why* (with a "Concept corner" that teaches the Go idea behind it),
- **failing tests** in the same package that tell you when you're done,
- a **stub** with `todo.NotImplemented(...)` where your code goes,
- a **reference solution** in `_solutions/` for when you're truly stuck.

No frameworks, no mocking library, no testify: standard library only (plus the SQLite driver). Every line is something you can fully understand.

---

## 1. Setup (5–10 minutes)

You need

- **Go 1.22 or newer** (`go version`). The code uses `for i := range 10`, per-iteration loop variables, `min`/`max`, `slices`, `context.WithoutCancel`, generics.
- **A C compiler**, because the SQLite driver (`github.com/mattn/go-sqlite3`) uses cgo.
  Linux: `sudo apt install build-essential`. macOS: `xcode-select --install`. Windows: install MinGW-w64 (e.g. via MSYS2) and make sure `gcc` is on your PATH — or simply use WSL.
  *(The race detector, `go test -race`, also needs cgo — so this is the setup you want anyway.)*

```bash
cd shopflow-go
git init && git add -A && git commit -m "Starting point"     # strongly recommended: lets you see your own diffs later
go test ./internal/catalog -run Step01 -v
```

You should see **14 failing tests** with messages like `not implemented yet: Step 01: ...`. That is the "red" in red-green-refactor. 🎯

> The first run downloads the SQLite driver and compiles its C code (takes a minute, then it's cached).
> If Go complains about a missing `go.sum` entry, run `go mod tidy` once.

**No C compiler available?** The driver is used in exactly one file, `internal/db/db.go`. A pure-Go alternative is `modernc.org/sqlite`: change the blank import, use driver name `"sqlite"`, and write the DSN options as `?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate`. *(I could not test that swap in my environment — treat it as a starting point.)*

## 2. How to work through a step

1. Read the step's PRD (`docs/steps/step-NN-*.md`) — especially the **Concept corner**.
2. Open the tests (`internal/<package>/*_test.go`) and read them **before** coding. They are the spec.
3. Open the stub (the doc comment on each stub repeats the requirements).
4. Run only that step while you work:

   ```bash
   go test ./internal/pricing -run Step02 -v          # one step (-run matches the test name prefix)
   go test ./internal/jobs -run Step07 -race -v       # concurrency steps: ALWAYS add -race
   go test ./internal/validation -run Chain_Stops -v  # one single test
   go test ./...                                      # everything
   ```
5. When green, read **Reflect**, try a **Stretch** challenge, run `go vet ./...` and `gofmt -l .`, and *then* compare with `_solutions/stepNN/`. The differences are where the learning is.
6. `git commit`, next step.

**Stuck?** Hints in each PRD go from gentle to explicit. As a last resort:

```bash
bash scripts/apply-solution.sh 03            # macOS / Linux / Git-Bash
.\scripts\apply-solution.ps1 03              # Windows PowerShell
```

It copies the reference solution over your stub (use `git diff` / `git checkout` to compare or undo). Later steps build on earlier ones, so if you skip a step, apply its solution before moving on.

### Make the tests tell you more

- `-race` — the **race detector**. Any unsynchronised access to shared memory becomes a failing test. Use it on every concurrency step; it is Go's superpower.
- `-count=20` — repeat a test 20 times. Concurrency bugs love to hide in run #14.
- `-shuffle=on` — random test order, to catch tests (or code!) that secretly depend on order.
- `-timeout 30s` — a hung test becomes a goroutine dump instead of a frozen terminal.

## 3. The roadmap

| # | Package | Feature | Go ideas | Patterns / principles | Tests |
|---|---------|---------|----------|-----------------------|:----:|
| 01 | `catalog` | [SQLite repository](docs/steps/step-01-sqlite-repository.md) | `database/sql`, `context`, `rows.Close`, sentinel errors, `%w`, `errors.Is` | Repository, SRP, DIP | 14 |
| 02 | `pricing` | [Pricing rules](docs/steps/step-02-pricing-strategy.md) | interfaces, func adapters (`RuleFunc`), value vs pointer receivers, slice aliasing | Strategy, OCP | 16 |
| 03 | `validation` | [Validation middleware](docs/steps/step-03-validation-middleware.md) | function types, closures, higher-order functions, tiny consumer-side interfaces | Chain of Responsibility, ISP | 25 |
| 04 | `caching` | [Caching decorator](docs/steps/step-04-caching-decorator.md) | `sync.Mutex`, in-flight map, `close(done)`, `select`, `context.WithoutCancel` | Decorator, LSP, singleflight | 10 |
| 05 | `events`, `notify` | [Events & notifications](docs/steps/step-05-events-and-notifications.md) | goroutines, `WaitGroup`, `errors.Join`, `recover`, copy-on-write | Observer, Factory (registry) | 15 |
| 06 | `orders` | [Place an order](docs/steps/step-06-place-order.md) | `BeginTx`, `defer tx.Rollback()`, `errors.As`, `BEGIN IMMEDIATE` | Application service, transactions | 13 |
| 07 | `jobs` | [Job queue & workers](docs/steps/step-07-job-queue.md) | buffered channels, back-pressure, **close/send panic**, worker pool, graceful shutdown | Producer/Consumer | 17 |
| 08 | `resilience` | [Retry & circuit breaker](docs/steps/step-08-resilience.md) | context-aware sleep, `time.Timer`, mutex discipline, `defer` + panic | State, composition | 20 |
| 09 | `concurrency`, `sales` | [Bounded concurrency & streaming](docs/steps/step-09-concurrency-and-streaming.md) | generics, worker pool, `sync.Once`, cancel-then-wait, callback iteration, connection leaks | errgroup-from-scratch | 18 |
| 10 | `algo`, `analytics` | [Heaps & LRU](docs/steps/step-10-heaps-and-lru.md) | `container/heap`, `container/list`, generic data structures, `cmp.Ordered` | Top-K, LRU | 20 |
| 11 | `graph`, `ratelimit` | [Graphs & rate limiting](docs/steps/step-11-graphs-and-rate-limiting.md) | Kahn's algorithm, float tokens, injectable clock | Topological sort, token bucket | 21 |

**189 tests in total.** Steps 1–5 are mostly independent. Step 6 wires 1, 2, 3 and 5 together. Step 10's last test uses Step 9's reader. Everything else stands alone.

## 4. If you did the .NET course: the translation table

| .NET | Go |
|------|----|
| `async/await`, `Task<T>` | a goroutine + a channel / `sync.WaitGroup`; plain *blocking* style — no colouring of functions |
| `CancellationToken` | `context.Context` (first parameter, always) |
| exceptions, `try/catch` | `error` return values, `errors.Is` / `errors.As`, `fmt.Errorf("…: %w", err)` |
| `IDisposable` / `using` | `defer x.Close()` |
| `interface` + explicit `: IFoo` | implicit interfaces — *any* type with the methods qualifies, and the **consumer** defines the interface |
| `Lazy<Task<T>>` + `ConcurrentDictionary` | mutex + map + `chan struct{}` (a hand-made singleflight) |
| `Task.WhenAll` | `sync.WaitGroup` + a result slice (`errors.Join` for the errors) |
| `SemaphoreSlim` | a worker pool, or a buffered channel used as a semaphore |
| `Channel<T>`, `BackgroundService` | `chan T`, a goroutine that returns when `ctx` is cancelled |
| `lock` | `sync.Mutex` + `defer mu.Unlock()` |
| `PriorityQueue<,>` | `container/heap` (you implement 5 methods) |
| `IAsyncEnumerable<T>` | callback iteration (`ForEach(ctx, yield)`), channels, or `iter.Seq` (Go 1.23) |
| `FakeClock` / `IDelayer` | same idea: tiny `Clock` and `Sleeper` interfaces |

## 5. Project layout

```
shopflow-go/
├── README.md
├── go.mod / go.sum
├── db/
│   ├── schema.sql          # products, customers, orders, order_lines, pipeline_*
│   ├── seed.sql            # 20 products, 8 customers, 30 orders (64 live lines), a 10-task pipeline
│   └── shopflow.db         # the same data, ready to open in "DB Browser for SQLite" or `sqlite3`
├── docs/steps/             # ← the PRDs, one per step
├── internal/
│   ├── todo/ clock/ money/ domain/ db/ testutil/     # given (testutil = temp DB, fakes, fake clock)
│   ├── catalog/ pricing/ validation/ caching/ events/ notify/ orders/
│   ├── jobs/ resilience/ concurrency/ sales/ algo/ analytics/ graph/ ratelimit/
├── _solutions/stepNN/      # reference solutions (same paths as the repo root)
└── scripts/                # apply-solution.sh / .ps1
```

**Why `_solutions`?** The Go tool ignores directories whose names start with `_` (or `.`), so `go test ./...` and `go vet ./...` never try to compile the reference solutions as if they were packages.

**Given vs. yours:** anything that returns `todo.NotImplemented(...)` (or has a `// TODO Step NN` marker) is yours to write. Everything else — domain types, interfaces, the DB opener, the circuit-breaker context, the pipeline loader — is provided so you can concentrate on one idea per step. Reading the given code is part of the learning.

## 6. The database

Open `db/shopflow.db` in any SQLite browser — it is the same data the tests use. **Tests never touch that file**: every test builds its own throw-away database from `schema.sql` + `seed.sql` in `t.TempDir()`, so tests can't interfere with each other and can run in parallel.

```bash
rm -f db/shopflow.db && sqlite3 db/shopflow.db < db/schema.sql && sqlite3 db/shopflow.db < db/seed.sql   # rebuild it
```

Conventions worth noticing: **money is always integer cents** (`price_cents`, `int64` in Go); and the schema uses `CHECK` constraints so the database refuses nonsense (negative stock, unknown status).

## 7. Ground rules that make this effective

- **Don't edit the tests to make them pass.** If a test looks wrong, that is a great question — read the PRD again, then ask.
- **Resist peeking at `_solutions/`** until your tests are green.
- **Run `go vet ./...` and `gofmt -l .`** before every commit; make `-race` a habit.
- Keep the **Reflect** answers in a notes file. Explaining *why* in your own words is what makes it stick.

## 8. After the 11 steps

- Compare your Step 4 with `golang.org/x/sync/singleflight`, your Step 9 with `golang.org/x/sync/errgroup`, your Step 8 with a production library such as `sony/gobreaker`, and your Step 7 with how `net/http` servers shut down (`Server.Shutdown(ctx)`).
- Capstone idea: put an `net/http` handler in front of `orders.Service` (Go 1.22's new `ServeMux` patterns like `POST /orders` are enough), enqueue a "send invoice" job on every placed order, and run the `jobs.Worker` with `signal.NotifyContext` for graceful shutdown.
- Then go back to the .NET track and notice how much of the design carried over.
