# UTC time contract operator

This command never starts business components. Configuration is consumed through
normal loaders; diagnostics deliberately omit raw SQL, DSNs and credentials.
All operations use a dedicated UTC connection. The persistent `state.json` lives
outside DB dumps, configuration restores and uploads; mount it read-only in the
server and read-write only in an explicitly authorized operator.

`-mode status` and `-mode snapshot` are read-only. Snapshot emits `target`,
`snapshot` and its canonical `snapshot_sha256`. Neither status nor reconcile clears
pending maintenance. `-mode plan -operation-id ID` reads the original DSN mapping
and verifies every existing TIMESTAMP cell has the audited +28800 interpretation;
unknown source mappings are rejected. It emits canonical JSON without a newline.
Redirect its exact bytes to a file and compute SHA256 without reformatting it.

All modes accept `-conf DIR -state-dir DIR` (default state directory is
`/app/data/time-contract`).

- `fresh-init -operation-id ID -writers-paused` requires a database with no tables
  before migration. UTC seeds from migration13 are included without shifting.
- `backup-receipt -manifest FILE -manifest-sha256 SHA -backup-file FILE.sql.gz
  -backup-manifest SIDECAR -writers-paused` verifies archive integrity and exact
  artifact hashes, source schema20, and current canonical before snapshot. It
  emits the typed conversion receipt.
- `apply` requires those manifest and archive flags plus `-backup-receipt RECEIPT
  -writers-paused`. Bare SHA strings cannot authorize conversion. Metadata DDL21
  is separate; the fixed TIMESTAMP updates and completion marker commit together.
  Repeating an already completed manifest performs no writes.
- `reconcile -manifest FILE -manifest-sha256 SHA` is read-only and reports
  `not_committed`, `committed_verified`, `already_complete` or `unknown`.
- `activate` with the manifest flags and `-writers-paused` explicitly completes a
  matching pending conversion after committed data and protected hashes verify.
  An unknown commit is never automatically replayed. `apply -resume` accepts only
  its original pending attempt, manifest and backup identity.
- `restore-begin -operation-id ID -backup-sha256 SHA -manifest-sha256 SHA` writes
  an independent pending intent with a fresh 32-byte random `attempt_nonce` before the restore executor stops writers.
- `restore-complete -verification-receipt FILE -writers-paused` requires the same
  attempt/target/backup/sidecar identity, successful import and redump verification,
  and independently verifies the canonical live DB snapshot. Legacy or incomplete
  imports stay pending; no automatic old-schema startup is performed.

Restore receipt JSON fields: `format_version` (1), `operation_id`, `attempt_nonce` (copied from restore-begin),
`target:{server_uuid,database}`, `backup_sha256`, `manifest_sha256`,
`import_succeeded`, `redump_verified`, `snapshot_sha256`. These are workflow
integrity evidence, never user authorization capabilities. The deployment executor
must verify all actual writers are stopped and the precise authorized target.

Only TIMESTAMP epochs in the fixed audited 13-table/26-column inventory move.
DATETIME(6), JSON, versions, relationships, account values and safety evidence
are protected. No WeChat, mail, model, indexing or user session calls occur.

Owned UTC fixtures use `internal/timecontract/testdata/bootstrap`, excluded from
normal `go build ./...`. The harness must verify Docker ownership first; the
helper additionally refuses non-loopback endpoints and non-fixture DB names.
There is no service environment flag to skip contract verification.

`TIMESTAMP(0)` is second precision. A Create response produced by Ent before a
read-back can still contain Go nanoseconds, and MySQL may round by one second.
The UTC invariant compares the persisted/retrieved API instant with SQL epoch;
it does not truncate the whole driver connection, since DATETIME(6) leases must
retain microseconds.

State directory precedence is explicit `-state-dir`, then the normal
`data.database.time_contract_state_dir` configuration, then the production
default. The server uses the same configuration field; local config defaults
to `./data/time-contract`. This routes persistent state and never bypasses it.

Conversion/snapshot inspection requires complete InnoDB foreign-key dictionary
visibility (`PROCESS` in MySQL 8.4). A permission error rejects the operation;
zero visible rows under a restricted table-permission view is not accepted as
proof. Timestamp columns participating in either direction of a foreign key
are rejected, including children or parents in other schemas. Audited database
names are limited to lowercase ASCII letters/digits/underscores, starting with
a letter, to avoid ambiguous dictionary identifier encodings. Normal readiness
checks do not run this global inspection.
