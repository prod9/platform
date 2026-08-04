<!-- derived from: https://neon.com/docs/connect/connection-pooling +
     https://neon.com/docs/connect/choose-connection @ 2026-08-04 -->

# Neon connection pooling

- Endpoints come in two forms: **direct** and **pooled** (`-pooler` in the hostname).
  Neon recommends pooled **by default**; direct is for schema migrations,
  `CREATE INDEX CONCURRENTLY`, `LISTEN`/`NOTIFY`, temporary tables, and other
  session-state work.
- The pooler is **PgBouncer in transaction mode** (`pool_mode=transaction`), up to
  10,000 client connections multiplexed over few real sessions.
- **Protocol-level prepared statements ARE supported** on the pooled endpoint
  (PgBouncer ≥ 1.22, `max_prepared_statements=1000` per connection). Only SQL-level
  `PREPARE`/`EXECUTE` are unsupported.

Gotcha that sent us the wrong way once: pgx v5's default `QueryExecModeCacheStatement`
uses protocol-level named statements, so it is *supposed* to work against Neon's pooler —
an `08P01 prepared statement name is already in use` there is a real interaction bug to
chase, not a "poolers can't do prepared statements" incompatibility, and
`default_query_exec_mode=simple_protocol` is a mask, not the fix.
