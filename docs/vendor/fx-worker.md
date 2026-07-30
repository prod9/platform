<!-- derived from: fx.prodigy9.co@v0.8.6/worker/{worker,jobs}.go @ 2026-07-29 -->

# fx worker

The background-job machinery platform's jobs run on: `worker.New(cfg, jobs...)` +
`Start()`. Upstream is <https://fx.prodigy9.co>; the module source under
`$(go env GOMODCACHE)/fx.prodigy9.co@<ver>/worker/` is the truth for everything below.

## The job contract

A job is `worker.Interface` — `Name() string`, `Run(ctx) error` — and **its own struct is
the payload**. `ScheduleAt` marshals the job to JSON into `jobs.payload`; `processJob`
looks the name up in the registry, calls `Reset()` if the job implements
`worker.Resetter`, unmarshals the row's payload into that instance, then calls `Run`.

| Fact                              | Consequence for a job we write                                          |
|-----------------------------------|-------------------------------------------------------------------------|
| `Name()` is the registry key      | It is constant per job *type* — it can never carry an id or a parameter. |
| One instance is reused per name   | A field not covered by the payload keeps the previous run's value unless `Reset()` clears it. |
| Payload is `json.Marshal(job)`    | Every field a run needs is exported with a `json` tag.                  |
| `Run` returning an error is final | The row is marked `failed` and **nothing requeues it** — recovery is a scan job of our own. |

## Scheduling

`ScheduleNow` / `ScheduleIn` / `ScheduleAt` always insert a pending row, so **many pending
jobs coexist under one name** and are told apart only by their payloads.

The `*IfNotExists` variants check `findPendingJobByName` — the **name alone**, and only
rows still `pending` (a *running* job does not block a fresh schedule) — returning
`ErrJobExists` instead of scheduling. That makes them the primitive for a **singleton**
job, typically one that reschedules itself, and never a way to deduplicate per-entity work.

## One job at a time per process

`workOnce` holds the worker's mutex across the whole of `processJob`, and
`takeOnePendingJob` takes a single row (`ORDER BY RANDOM() LIMIT 1`, flipped to `running`
inside a transaction rather than `FOR UPDATE`, so in-flight work stays visible). A process
therefore runs **exactly one job at a time**, and parallelism means more processes.

🚨 **There is no way to point a process at a subset of jobs.** Every process draws from the
one queue, so a long job occupies a whole process and no process can be dedicated to a job
kind — scaling is OS process scheduling and nothing finer. Reliable parallelism across job
kinds needs a queue/partition capability fx does not have yet.

`Start()` owns its own world: it connects its own DB pool, creates the `jobs` table,
puts config + db in the context, registers a ctrl-c stop, then blocks until cancelled. The
loop drains every pending job, then idles for `WORKER_POLL` (default 1 minute).
