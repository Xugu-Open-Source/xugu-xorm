# Support Matrix

This matrix records evidence for this source tree. `partial` means the implementation and offline evidence exist but required real-server acceptance is incomplete.

## Environment

| Component | Version | Status | Evidence |
| --- | --- | --- | --- |
| xugu-xorm | v1.3.6-compatible candidate | partial | Root and verify-module offline tests and vet; exact-candidate black-box result is recorded externally. |
| xorm | `v1.3.6` | verified offline | Both Go modules pin `xorm.io/xorm v1.3.6`. |
| Go | `1.20.14` | verified offline | Root and verify-module tests and vet run with `GOWORK=off`. |
| Xugu Go driver | `v1.0.13` | untested on target server | Pinned by the verify module and black-box consumer. |
| Xugu server and mode | target environment | untested | Required database, schema, ordinary-role, and privileged-role inputs were unavailable for this candidate. |

## Functional Scope

| Capability | Offline evidence | Real Xugu | Current status |
| --- | --- | --- | --- |
| Driver/dialect registration and DSN parsing | Root and independent consumer tests | pending | partial |
| Type mapping, quoting, DDL generation | Root and verify-module tests | pending | partial |
| Metadata current database/schema isolation | SQL/result fixtures for all five metadata entry points | pending multi-database/multi-schema topology | partial |
| Composite catalog joins | `DB_ID + TABLE_ID` and `DB_ID + SCHEMA_ID` assertions | pending catalog observations | partial |
| Index `KEYS` parsing | Quoted identifiers, quoted commas, escaped quotes, malformed/expression rejection | pending raw `ALL_INDEXES.KEYS` observations | partial |
| Version parsing | Full-response parsing, build suffix, empty/malformed and query errors | pending raw `VERSION()` observations | partial |
| CRUD, pagination, commit, rollback | Independent consumer flow implemented | pending live execution | partial |
| Automatic existing-column type changes through `Sync2` | Explicit `ModifyColumnSQL` path only | n/a | unsupported: xorm v1.3.6 does not emit the ALTER automatically. |
| Expression/descending index introspection | Unsupported encodings return an error | pending server encoding evidence | unsupported pending a documented encoding contract |
| Savepoints, concurrency, locks, advanced SQL | none | none | untested |

## Validation Boundary

Real-database acceptance requires `XUGU_IT=1` plus `XUGU_TEST_DSN`, `XUGU_TEST_SECONDARY_DSN`, `XUGU_TEST_ORDINARY_DSN`, `XUGU_TEST_PRIMARY_SCHEMA`, and `XUGU_TEST_SECONDARY_SCHEMA`. Missing inputs are `BLOCKED/UNTESTED`, never a pass. Tests create and drop only uniquely prefixed tables and indexes; database creation and deletion are forbidden.

The exact candidate SHA, commands, outcomes, and remaining server-dependent catalog boundaries are recorded in the external Trellis validation report. Historical evidence from the public `v1.3.6-xugu` tag is not attributed to this candidate.
