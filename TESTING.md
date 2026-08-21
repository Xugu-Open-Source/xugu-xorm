# Testing

This repository maintains three test layers. A passing lower layer does not prove a higher layer.

| Layer | Purpose | Entry point |
| --- | --- | --- |
| Unit | Offline dialect, metadata SQL, parser, and error contracts | `GOWORK=off go test -count=1 ./...` |
| Integration | xorm against prepared Xugu database/schema isolation fixtures | Run the verify module with the real-database environment below |
| Black box | A separate consumer module installs an exact candidate commit and uses public APIs only | `scripts/test-blackbox.ps1 -ModuleVersion <full-sha> -CandidateRepository <git-repository>` |

## Real Database Tests

Default offline runs skip live tests unless `XUGU_IT=1`. Once enabled, all of these environment-only inputs are required:

- `XUGU_TEST_DSN`: privileged connection to the primary prepared test database.
- `XUGU_TEST_SECONDARY_DSN`: connection to a second prepared database.
- `XUGU_TEST_ORDINARY_DSN`: ordinary-user connection to the prepared fixtures.
- `XUGU_TEST_PRIMARY_SCHEMA` and `XUGU_TEST_SECONDARY_SCHEMA`: two prepared schemas in the primary database.

Missing input after explicit enablement fails the gate with `REAL_DB_BLOCKED`. A successful connection emits `REAL_DB_CONNECTED`. Tests own only uniquely prefixed tables and indexes and must never create or drop a database. Do not use shared or production objects.

Record sanitized server/driver/xorm/Go versions, roles, database/schema topology, catalog IDs, raw index `KEYS`/flags, and version responses. Never record DSNs or credentials.

## Black-Box Tests

The script copies the consumer template into a temporary directory, disables `go.work`, initializes a separate module, installs the requested module version, and runs public registration and consumer tests. It contains no `replace` directive and does not reference the candidate source tree as a Go module.

For a local candidate, `-CandidateRepository` installs the exact full commit SHA through a temporary process-local Git URL mapping. The mapping points Git at the candidate repository while `go get` remains versioned by SHA. Caller `GOPROXY` and `GOSUMDB` values are preserved unless candidate mode requires direct local Git resolution; otherwise defaults are supplied only when the caller did not configure them.

When `XUGU_IT=1`, missing `XUGU_TEST_DSN` is an error. Without real-database inputs, the black box proves exact-candidate installation and registration only; CRUD, pagination, commit, and rollback remain untested.
