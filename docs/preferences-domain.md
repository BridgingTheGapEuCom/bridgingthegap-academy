# Application preferences

M-SET.1 through M-SET.4 are complete: persisted preferences, bootstrap resolution, the Preferences route, and the intentionally read-only Account route all use the shared `SettingsShell` pattern.

`preferences.user_preferences` stores user-owned application chrome only: an optional supported application locale and an appearance choice (`SYSTEM`, `LIGHT`, or `DARK`). It is separate from Identity account/profile data and from authored Course data.

The authenticated session determines the only readable or writable row. `GET /api/me/preferences` returns an unpersisted default (`applicationLocale: null`, `appearance: SYSTEM`, revision `0`) until the first `PUT`; that first complete replacement must use revision `0`. Persisted updates use the normal revision compare-and-swap contract.

During authenticated bootstrap, an explicit persisted locale wins; a null or unavailable preference falls back to the existing local/browser/default resolver. Appearance resolves centrally at the document root: `SYSTEM` follows live `prefers-color-scheme` changes, while `LIGHT` and `DARK` override it.

`/settings/preferences` provides the Preferences UI. It renders null locale as “Use browser/default language”, keeps changes local until explicit Save, and uses the current revision for compare-and-swap updates. Discard restores the last stored record; conflicts require reloading before another save.

`/settings/account` is deliberately read-only. The authenticated-session contract currently exposes only an expiry time appropriate for authors, so Academy does not imply support for profile editing, email or password changes, identity-provider management, roles, device information, or multi-session controls. Those capabilities remain separate future work until their server contracts exist.

Settings integration coverage includes a PostgreSQL-backed authenticated HTTP test and an opt-in real-stack browser test (`pnpm test:e2e:settings-integration`). The browser test requires a running migrated backend/frontend plus credentials for a fresh test user; mocked Settings tests remain responsible for deterministic conflict and failure states.
