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

Every form control needs a persistent visible label. `BtgFormField` supplies label, optional help, and error associations through its slot props. Required state includes both a visible marker and screen-reader text; errors use `role="alert"`, `aria-describedby`, and `aria-invalid`, never colour alone.

`BtgButton` has only `primary`, `secondary`, `quiet`, and `destructive` variants. New variants need a semantic reason. Bordered surfaces are preferred over cards and shadows; use elevation only where layer separation is necessary.

## Authentication forms

Authentication forms keep credentials and field errors in component-local state. Use `BtgFormField` for persistent labels and field-level validation; on failed client validation, focus the first invalid native control. Authentication outcomes use one visible, focusable form-level alert that is focused once after the result is available. The page owns its wording and focus behavior; the frontend auth service owns transport, session state, and memory-only CSRF state.

## Interaction and responsiveness

Focus uses one high-contrast 3px outline with an offset across native and custom controls. Do not remove it. Motion is short and nonessential, and the reduced-motion query suppresses it. Page gutters scale down at small widths; controls retain a 44px minimum target size and layouts must not introduce horizontal scrolling.

No dark mode, theme selector, Storybook, login UI, or course interface is part of this foundation. Future state-changing authenticated screens will continue to use the existing browser-security transport independently of these presentation tokens.
