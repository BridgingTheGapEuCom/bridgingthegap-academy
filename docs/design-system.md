# Academy visual foundation

The Academy is the structured-learning counterpart to BridgingTheGap.eu.com. It keeps the public site's typography-led hierarchy, near-neutral surfaces, thin dividers, compact corners, and restrained decoration. It does not copy the editorial layout or turn learning into a dashboard or game.

## Tokens and layout

`frontend/src/styles/tokens.css` is the single presentation-token layer. It defines semantic colour, type, spacing, layout, shape, and motion values. Components consume token names such as `--btg-color-text` and `--btg-color-border`; they must not introduce arbitrary hard-coded component colours. Future declarative instance themes may override presentation tokens, but never component semantics or accessibility behaviour.

The type and layout tokens preserve user scaling and provide reading, application, and form widths. Content uses natural text alignment and readable line height. Do not use justified paragraphs, text-only uppercase blocks, tiny essential text, or fixed layouts that prevent future learner font-size and reading-width preferences.

Accessibility precedence is:

1. architectural accessibility requirement
2. learner accessibility preference
3. theme/design token
4. instance branding

The built-in theme must remain usable if a future branding override has poor contrast.

## Component principles

The Academy owns `BtgButton`, `BtgFormField`, `BtgTextInput`, and `BtgPageContainer`. Prefer native semantic HTML for simple controls: native buttons, inputs, and labels carry their normal browser and assistive-technology behaviour. Use Reka UI only for behaviour that native HTML cannot provide well, such as the existing dialog primitive.

Every form control needs a persistent visible label. `BtgFormField` supplies label, optional help, and error associations through its slot props. Required state includes both a visible marker and screen-reader text; errors use `aria-describedby` and `aria-invalid`, never colour alone. On submission errors, focus the first invalid control so its associated error is announced once.

`BtgButton` has only `primary`, `secondary`, `quiet`, and `destructive` variants. New variants need a semantic reason. Bordered surfaces are preferred over cards and shadows; use elevation only where layer separation is necessary.

## Authentication forms

Authentication forms keep credentials and field errors in component-local state. Use `BtgFormField` for persistent labels and field-level validation; on failed client validation, focus the first invalid native control. Authentication outcomes use one visible, focusable form-level error that is focused once after the result is available, avoiding duplicate live-region announcements. The page owns its wording and focus behavior; the frontend auth service owns transport, session state, and memory-only CSRF state.

Authenticated-route UI uses backend-authoritative checks for authorization. Protected API calls go through `useAuth().request` so session invalidation and memory-only CSRF attachment remain consistent; the standalone API client currently serves liveness only. A `401` moves the frontend to its unauthenticated flow; a `403` leaves the authenticated session intact and presents access denied. Never cache roles or capabilities as frontend truth: route states may be local and short-lived, but every protected capability check remains a backend request.

Authoring routes are Draft-scoped below `/authoring/drafts/:draftId`. The shell reads only Draft metadata through the authenticated request boundary, then provides temporary route-local context to Overview, Structure, and Members sections. The Overview editor saves changed metadata manually with the current Draft revision; a revision conflict preserves local form values until the user explicitly reloads the server version. The backend’s opaque `404` remains one neutral Draft-unavailable state; the frontend does not infer Authoring roles or persist private Draft data. Draft section navigation uses text links in a labelled navigation landmark and preserves standard router `aria-current` semantics.

## Interaction and responsiveness

Focus uses one high-contrast 3px outline with an offset across native and custom controls. Do not remove it. Motion is short and nonessential, and the reduced-motion query suppresses it. Page gutters scale down at small widths; controls retain a 44px minimum target size and layouts must not introduce horizontal scrolling.

## Learner course pages

Course discovery is an ordered editorial list, not a dashboard. Course overviews use one page H1, then metadata, learning objectives as a real list, the fully visible ordered outline, and quiet attribution/license details. Modules and lessons use nested headings and actual ordered lists; lesson titles are links, while recommended prerequisites stay plain advisory text and never become locks, completion indicators, or progress controls. Course license labels must say they apply to course content. Render all API metadata as text and never use `v-html` for course data.

Lessons use one semantic block-renderer boundary for both Continuous and Focus reading modes. Continuous mode is ordinary document flow; Focus mode exposes one block, an explicit `Block N of M` position, and native previous/next controls. It is view-local navigation, never learner progress. Content block data is rendered as escaped semantic DOM only: no `v-html`, dynamic content components, editor state, or executable payloads. Asset keys remain unresolved until a published-asset delivery boundary exists, and knowledge checks stay visible placeholders until Assessments exists.

No dark mode, theme selector, Storybook, or learner-progress UI is part of this foundation. Future state-changing authenticated screens will continue to use the existing browser-security transport independently of these presentation tokens.
