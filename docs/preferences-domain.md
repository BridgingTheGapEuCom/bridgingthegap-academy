# Application preferences

`preferences.user_preferences` stores user-owned application chrome only: an optional supported application locale and an appearance choice (`SYSTEM`, `LIGHT`, or `DARK`). It is separate from Identity account/profile data and from authored Course data.

The authenticated session determines the only readable or writable row. `GET /api/me/preferences` returns an unpersisted default (`applicationLocale: null`, `appearance: SYSTEM`, revision `0`) until the first `PUT`; that first complete replacement must use revision `0`. Persisted updates use the normal revision compare-and-swap contract.

During authenticated bootstrap, an explicit persisted locale wins; a null or unavailable preference falls back to the existing local/browser/default resolver. Appearance resolves centrally at the document root: `SYSTEM` follows live `prefers-color-scheme` changes, while `LIGHT` and `DARK` override it. Settings UI and preference editing remain future milestones.
