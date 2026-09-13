# Module boundaries

The package owners follow section 32 of `BTG_LMS_Architecture_Decision_Baseline_v5.docx`. Packages under `internal/modules` are capability modules, not technical-layer buckets. `internal/platform` is the composition root for the executable; it may depend on modules. Modules must not depend on the composition root.

Short package names map to the baseline's capability names: `learning` is Learning Progress, `credentials` is Certificates & Badges, `community` is Community & Moderation, `search` is Search & Discovery, `audit` is Audit & Security, and `infrastructure` is Infrastructure Adapters. These are naming aliases, not additional modules.

`go run ./tools/archcheck` enforces these initial edges:

- No other module imports Administration.
- Courses imports neither Authoring nor Publishing.
- Business modules do not synchronously import Notifications, Search, or Audit. Their future integration is through domain events.
- Domain modules do not import Infrastructure adapters. Provider contracts will live with their consumers when their exact interfaces are designed.
- SQL query files under an owner's `db/query` folder may write only tables assigned to it in `docs/table-owners.json`. Add each table to that manifest when its first migration lands.

The checker is deliberately conservative about SQL writes and rejects writes to unregistered tables. New shared tables, ownership transfers, and cross-module transaction contracts require an explicit architecture decision before adjusting its rule. Identity owns the tables in the `identity` PostgreSQL schema; Goose applies its migration through the existing ordered migration directory.
