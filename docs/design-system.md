# Academy frontend design system

This document is the practical contract for Academy presentation code. It defines how tokens, shared components, feature patterns, and pages relate. It does not prescribe a new visual design or change product behavior.

## Architecture

Academy uses five layers. Dependencies flow downward only.

```text
foundation values
  -> semantic theme tokens
    -> generic UI primitives
      -> layout and feature compositions
        -> routed pages
```

1. **Foundation values** are private scales: neutral palette steps, spacing, type metrics, radii, shadows, icon sizes, motion, and layer order. Feature code must not consume palette steps directly.
2. **Semantic tokens** express intent, such as page surface, muted text, primary action, or danger border. Themes replace this layer.
3. **UI primitives** own recurring appearance and interaction: buttons, fields, icons, dialogs, menus, messages, chips, and disclosures.
4. **Compositions** arrange primitives. Generic compositions include page headers, toolbars, panels, split workspaces, metadata strips, and ordered lists. Authoring compositions add Draft and Lesson semantics.
5. **Pages** own product data, workflows, routing, and genuinely unique layout. They should not recreate primitive appearance.

The existing `Btg` prefix remains the generic component namespace. Components tied to the authoring domain retain the `Authoring` prefix. A generic primitive must not import an Authoring component or feature service.

## Internationalization contract

Application interface text follows [the Academy i18n guide](./i18n.md). Application locale is independent from course/source language. Feature components translate complete product messages and pass translated visible and accessible labels into generic Btg primitives. Generic primitives must not hardcode English product text or import feature message namespaces.

Reusable controls own semantics and layout, while callers own domain wording. Dialog trigger/dismiss labels, menu labels, icon-button names, search labels/placeholders, drag-handle names, and previous/next navigation labels must be explicit. Application-level compositions may consume the small `common` namespace for genuinely shared interface wording.

Component tests use the shared i18n bootstrap rather than defining local message objects. Migrated reference surfaces require pseudo-locale coverage. CSS in new primitives should prefer logical inline properties so localization does not deepen left/right assumptions.

## Proposed source layout

Migration should converge on this shape without a big-bang file move:

```text
frontend/src/
  styles/
    foundation.css
    semantic.css
    themes/
      academy-default.css
    base.css
  components/
    ui/                 # BtgButton, BtgIcon, fields, dialog, menu, etc.
    layout/             # page, toolbar, surface, split workspace, metadata
    authoring/          # Authoring-specific compositions
  features/             # feature components and their local layout styles
  pages/                # routed composition and orchestration
```

During migration, compatibility aliases may remain in `tokens.css`; new components should use the target semantic contract. Academy should continue using plain CSS and Vue single-file components. Do not add a utility framework or a second component framework.

## Tokens

### Foundation scales

Foundation tokens describe available values, not their meaning. Keep them private to token and primitive implementation where possible.

| Scale | Proposed values |
| --- | --- |
| Neutral palette | 0, 50, 100, 200, 300, 500, 700, 900, 1000 |
| Spacing | 0, 0.25rem, 0.5rem, 0.75rem, 1rem, 1.5rem, 2rem, 3rem, 4.5rem |
| Type size | 0.75rem, 0.875rem, 1rem, 1.125rem, 1.25rem, 1.625rem, responsive display sizes |
| Type weight | 400, 500, 700 |
| Radius | 0, 0.25rem, 0.375rem, 999px |
| Border | 1px, 2px |
| Control height | 2.25rem compact, 2.75rem standard; icon-only touch targets remain at least 2.75rem |
| Icon size | 1rem, 1.125rem, 1.25rem, 1.5rem |
| Elevation | none, floating menu, modal |
| Motion | 120ms fast, 180ms standard; standard ease |
| Layer | base 0, sticky 10, dropdown 20, overlay 40, modal 50, toast 60 |

The existing spacing scale is sound and should be retained. Add a foundation value only when at least two reusable patterns need it. Reading widths, editor dimensions, illustration geometry, and split-pane ratios may remain documented composition values rather than tokens.

Breakpoints are not theme values. Use a small shared set for page gutters and global navigation, and prefer component/container queries for reusable compositions. Current repeated thresholds should converge on small reflow near 34rem, general composition reflow near 48rem, and feature-specific thresholds only when content requires them.

### Semantic tokens

Public component and feature CSS should consume semantic tokens. The proposed prefix avoids global ambiguity:

```css
/* Surfaces */
--academy-surface-page
--academy-surface-default
--academy-surface-elevated
--academy-surface-subtle
--academy-surface-selected
--academy-surface-danger-subtle

/* Text */
--academy-text-primary
--academy-text-secondary
--academy-text-muted
--academy-text-link
--academy-text-danger
--academy-text-on-primary

/* Borders */
--academy-border-default
--academy-border-subtle
--academy-border-strong
--academy-border-danger

/* Actions */
--academy-action-primary-bg
--academy-action-primary-bg-hover
--academy-action-primary-text
--academy-action-secondary-bg
--academy-action-secondary-border
--academy-action-danger-bg
--academy-action-danger-border
--academy-action-danger-text

/* State */
--academy-focus-ring
--academy-status-success
--academy-status-warning
--academy-status-danger
```

Existing `--btg-color-*` variables become compatibility aliases during migration. Component-specific custom properties are acceptable for structural values such as a split ratio, but not as private copies of colors, focus rings, button fills, or common spacing.

### Theme architecture

`foundation.css` defines stable scales. `semantic.css` declares the required semantic contract and safe fallbacks. `themes/academy-default.css` maps that contract to foundation values under `:root` or `[data-theme="academy-default"]`.

Future dark, high-contrast, or branded themes override semantic tokens only. Component selectors must not be duplicated inside theme files. Themes cannot change semantics, DOM order, accessible names, target sizes, focus visibility, or product behavior. User accessibility settings take precedence over brand choices.

The current homepage palette should become a named semantic theme or a documented editorial exception; it should not remain an unrelated set of page-specific color variables mixed into the foundation layer.

## Typography

Equivalent content uses semantic roles rather than page-owned font combinations.

| Role | Use |
| --- | --- |
| Display | Rare editorial hero text |
| Page title | One routed-page `h1` |
| Section title | Major `h2` within a page |
| Subsection title | Panel and inspector `h3` headings |
| Body | Default prose and controls |
| Body small | Secondary compact content |
| Label | Form and metadata labels |
| Helper | Field guidance and supporting copy |
| Metadata | Dates, positions, identifiers, context |
| Eyebrow | Short contextual label; uppercase is optional presentation |
| Code | Source code and technical identifiers |

Heading level remains a semantic HTML decision; a visual role does not select a heading level. Components should expose roles through shared classes or tokens rather than setting a unique `font-size` and `font-weight` pair per page.

## Spacing

Use the shared spacing scale by purpose:

| Purpose | Normal range |
| --- | --- |
| Icon-to-label and tight inline gap | space 1–2 |
| Related controls and field internals | space 2–3 |
| Field stack and row padding | space 3–4 |
| Panel padding and toolbar gap | space 4–5 |
| Section gap | space 5–6 |
| Page block spacing | space 6–8 |

A component may choose one value within a range, then owns it for every consumer. Pages must not override primitive padding to create locally compact or oversized variants.

## Core primitives

| Primitive | Responsibility and variants | Accessibility ownership | Feature code must not |
| --- | --- | --- | --- |
| `BtgButton` | `primary`, `secondary`, `tertiary`, `danger`, `danger-secondary`; small/medium; leading/trailing icon; loading/disabled | Native button semantics, disabled/loading state, focus, target size | Recreate fills, borders, radii, or icon spacing |
| `BtgIcon` | Semantic name to canonical library icon; small/medium/large | Decorative by default; hidden from AT unless explicitly meaningful | Import library icons or SVG paths directly |
| `BtgIconButton` | Compact icon-only action with shared tooltip pattern | Requires accessible label; target and focus behavior | Rely on tooltip or icon shape as the name |
| `BtgTextInput` | Text-like controls and invalid state | Attribute forwarding, focus, described-by support | Restyle control borders/focus locally |
| `BtgTextarea` | Multiline text control | Same field contract and resize behavior | Create page-specific textarea chrome |
| `BtgSelect` | Native select presentation | Label/error association and disabled state | Replace with a custom listbox without need |
| `BtgCheckbox` | Checkbox, label, helper, error | Enlarged hit target and associations | Separate the input from its visible label |
| `BtgRadioGroup` | Radios or a deliberate segmented variant | Group label, keyboard/native semantics | Use buttons without selection semantics |
| `BtgSearchField` | Search input with leading search icon and clear action where needed | Persistent or accessible label, native search semantics | Hand-position an icon over an input |
| `BtgBadge` | Read-only status/category label | Text conveys state, not color alone | Use it as an interactive control |
| `BtgChip` | Compact content/type value; optional removable variant | Removal has an explicit name | Create arbitrary badge dimensions |
| `BtgTabs` | In-panel tab interface only | Tab roles, roving focus, arrows, selected state | Use for route navigation |
| `BtgPageTabs` | Router-based section navigation | Landmark label and `aria-current` | Add ARIA tab roles to page links |
| `BtgSurface` | Border, background, radius, padding variants | Preserves semantic child structure | Become a generic nested-card default |
| `BtgSection` | Heading/action/body composition | Heading association and hierarchy hook | Restyle action buttons |
| `BtgToolbar` | Ordered controls, flexible slot, wrapping | Logical DOM/tab order | Reorder controls visually against DOM order |
| `BtgDialog` | Modal shell built on the existing Reka dependency | Focus trap, modal labeling, Escape, return focus | Reimplement modal focus behavior |
| `BtgMenu` | Trigger and action menu | Arrow/Escape navigation, focus return, labels | Build ad hoc `role="menu"` behavior |
| `BtgDisclosure` | Expand/collapse trigger and region | `aria-expanded`, controls relationship, keyboard activation | Make a decorative chevron the only target |
| `BtgEmptyState` | Consistent empty title, explanation, optional action | Heading and action semantics | Encode state only with an illustration |
| `BtgStatusMessage` | Info/success/warning/error/conflict variants | Appropriate live-region behavior without duplicate announcements | Add arbitrary alert roles to static text |

`quiet` may remain as a deprecated alias for `tertiary` while callers migrate. New variants require a distinct semantic purpose, not a page name.

## Button contract

Buttons express action hierarchy, not location:

```vue
<BtgButton variant="primary" leading-icon="add">
  Add module
</BtgButton>
<BtgButton variant="secondary" leading-icon="edit">
  Edit details
</BtgButton>
<BtgButton variant="danger-secondary" leading-icon="delete">
  Delete lesson
</BtgButton>
```

There should normally be one primary action per local action group. Links remain links when navigation is the outcome. Icon placement, loading indicator, height, padding, focus, and disabled presentation belong to `BtgButton`.

## Icon contract

Lucide via `@lucide/vue` is the canonical Academy icon family. All application UI accesses it through `BtgIcon`, using product-semantic names such as `search`, `module`, `lesson`, `edit`, and `delete`. The mapping layer owns library choice, aliases, default stroke width, sizes, alignment, and decorative semantics.

Feature components must not handcraft SVG path data, use Unicode characters or emoji as pseudo-icons, or import Lucide components directly. Bespoke brand marks, data visualizations, and editorial illustrations are allowed when they are not controls or substitutes for common UI icons; document those exceptions beside the component.

## Forms

The shared field composition is always:

```text
Label and required indicator
Control
Helper or error message
```

`BtgFormField` owns IDs and associations between those pieces. Shared controls own normal, hover, focus, invalid, and disabled appearance. Feature code owns field wording, value, validation rules, and when validation runs. Required state must be visible and announced. Submission errors focus the first invalid control when appropriate.

Specialized editors may wrap this contract but may not replace its labels, errors, or focus rules. Checkbox helper placement and repeatable-field row layout belong to shared form compositions.

## Layout and composition primitives

Extract these only where repeated use is established:

| Composition | Responsibility |
| --- | --- |
| `BtgPageContainer` | Canonical page max width, gutters, and shrink behavior |
| `BtgPageHeader` | Eyebrow, title, context, metadata, and route-navigation slots; pages provide wording and destinations |
| `BtgPageTabs` | Route-section navigation aligned to the page canvas |
| `BtgToolbar` | Flexible leading/content/action groups with logical wrapping; it does not assign action hierarchy |
| `BtgSurface` / `BtgSection` | Standard surface and section framing |
| `BtgMetadataStrip` | Label/value groups, optional icons, separators, and responsive wrapping |
| `BtgDialog` / `BtgMenu` | Reka-backed overlay/menu behavior, focus return, keyboard dismissal, and shared surfaces |
| `BtgDisclosure` | Chevron, expanded state, controlled region, and content spacing |
| `BtgSplitWorkspace` | Responsive two-pane grid, equal-height desktop regions, and content-driven stacking; `balanced` and `outline-inspector` are the only current variants |
| `BtgPreviousNextNavigation` | Compact adjacent-item navigation; pages supply selection behavior |
| `BtgOrderedList` / `BtgOrderedRow` | Ordered row rhythm, index/handle/content/action slots, and calm selected state |
| `BtgDragHandle` | Named pointer/keyboard handle surface; the feature owns gesture handling, persistence, and announcements |
| `AuthoringInspectorSection` | Authoring-specific icon/title/action/body section with shared separators |
| `AuthoringDirtyActionBar` | Authoring wording and discard/save grouping; persistence and dirty state remain feature-owned |

Split proportions and reflow thresholds are composition inputs. `BtgSplitWorkspace` must not know Course, Lesson, or Draft data. Ordered-list persistence remains a feature responsibility.

`BtgDragHandle` deliberately does not implement a reorder operation. The feature supplies pointer and keyboard listeners, stable IDs, movement constraints, persistence, rollback, and live announcements. The primitive owns the target size, icon, disabled state, focus treatment, and required accessible name.

## Authoring patterns

These patterns sit above generic primitives because they encode Academy Authoring semantics:

- `AuthoringDraftHeader` and `AuthoringLessonHeader`: route context and metadata.
- `AuthoringPageTitle` and `AuthoringTabs`: Authoring page identity and section navigation.
- `AuthoringSection`: numbered/editorial authoring section composition; may eventually delegate its surface to `BtgSection`.
- `AuthoringDirtyActionBar`: Authoring wording and save/discard workflow.
- `RepeatableObjectivesEditor`: objective identity, validation, and reordering.
- `CourseOutline`: Module/Lesson hierarchy, selection, disclosures, and feature actions.
- `InspectorSection`: repeated selected-item section composition when confirmed across Authoring inspectors.
- `AuthoringContentBlockList`: canonical content-block selection and reordering.

Feature patterns own domain labels, data, ordering rules, permissions, and persistence. They compose generic controls and surfaces rather than redefining them.

## Ownership rules

Feature components should own:

- product semantics, state, permissions, and API calls;
- stable identities and domain-specific ordering rules;
- route behavior and feature-specific error recovery;
- unique layout that has no credible second consumer.

Feature components should not normally own:

- button, field, chip, dialog, or menu appearance;
- standard borders, radii, shadows, focus rings, or surface colors;
- common typography roles or icon styling;
- duplicated empty, status, metadata, toolbar, or action-bar treatments.

Exceptions include content-driven colors, rich-text document output, diagrams, editorial illustrations, and genuinely unique geometry. Exceptions must use semantic surrounding surfaces, remain accessible, and include a short code comment when the reason is not obvious.

## CSS strategy and naming

- Global CSS contains token imports, reset/base rules, typography roles, and intentional cross-component utilities only.
- A primitive owns its styles in a colocated scoped block or a clearly named component stylesheet.
- Feature styles are scoped or namespaced to their feature and focus on composition.
- Responsive rules live with the component whose content causes the reflow. Page gutters and shell navigation remain global.
- Prefer component props and semantic modifiers to consumer overrides.
- Use semantic class names such as `.course-outline__row` and modifiers such as `.btg-button--danger`; avoid `.black-button`, `.gray-card`, or page-specific classes whose only job is restyling a primitive.
- Use ARIA attributes or `data-state` for state styling where they already express the state; do not maintain a duplicate visual-state class unnecessarily.
- Avoid descendant selectors that reach into another component's internal markup.

## Theme safety

Feature CSS must not use raw palette colors, hardcoded focus colors, replicated action backgrounds, direct icon SVG paths, or private theme selectors. It may use raw values for documented one-off geometry that is not a reusable design decision.

Every semantic foreground/background and border/background pairing must meet contrast requirements in each supported theme. High-contrast and reduced-motion preferences must not depend on a selected visual theme.

## Accessibility ownership

Recurring accessibility behavior belongs in shared primitives:

- buttons own disabled, loading, focus, and native activation semantics;
- icon-only buttons require an accessible label;
- fields own label/helper/error association;
- dialogs own focus trapping, modal labeling, Escape, and return focus;
- menus own composite keyboard interaction;
- interactive tabs own roving focus, while page tabs remain ordinary links;
- disclosures own expanded state and controlled-region association;
- ordered lists and drag handles own keyboard instructions and movement announcements;
- status messages own appropriate announcement behavior.

Pages remain responsible for meaningful heading order, landmark labels, operation-specific messages, post-navigation focus, and restoring focus when feature state removes a control. Information and selection must never rely on color or icons alone.

## Responsive ownership

Shared layers own page gutters, standard target sizes, field reflow, toolbar wrapping, metadata wrapping, and generic split-workspace stacking. Features own reflow driven by unique content, such as the Structure workspace threshold or Content block editor layout.

At 320 CSS pixels and at 200% text zoom, content must reflow without page-level horizontal scrolling, clipping, or loss of actions. Do not preserve columns by shrinking text and controls below the shared scale. Prefer natural page scrolling over independent pane scrolling.

## Testing

Primitive tests should cover:

- all semantic variants and slots/props;
- accessible names and field associations;
- disabled and loading behavior;
- keyboard interaction and focus restoration for composite widgets;
- focus-visible state through computed-style or focused screenshot coverage where useful;
- wrapping/reflow for toolbar, metadata, and split compositions.

Feature tests should focus on workflow and domain behavior rather than retesting primitive appearance. Keep a small Playwright visual-regression set for reference surfaces: Draft Structure desktop, an Authoring form at 320px and 200% text, Dashboard, and one dialog/menu state. Commit stable local screenshots only if the repository adopts snapshot review; until then, prefer geometry assertions plus a few targeted screenshots in CI artifacts.

## Enforcement

Start with small, explainable checks:

1. CI grep/check script rejects new raw hex/rgb/hsl colors outside token/theme and documented illustration files.
2. CI rejects new inline UI SVG paths and direct `@lucide/vue` imports outside `BtgIcon`.
3. ESLint prevents direct imports across the generic-to-feature dependency boundary.
4. A PR checklist asks whether an existing primitive fits, whether tokens are semantic, and whether 320px, 200% text, keyboard, and focus were checked.
5. Component tests and a lightweight in-app design-system examples route document supported variants during development; it need not ship publicly.

Adopt Stylelint only after the CSS is divided into ownership boundaries; otherwise its initial noise will obscure the rules that matter. Baseline existing violations, reject new ones, and burn down the baseline during migration.

## Migration plan

### M-UI.2 — tokens, icons, and core controls

- Split foundation, semantic, and default-theme layers with compatibility aliases.
- Complete the `BtgIcon` semantic map and remove direct UI-icon implementations as components migrate.
- Extend `BtgButton`; add icon button, search field, textarea/select/checkbox, badge/chip, tabs, panel, and generic section primitives.
- Consolidate focus, control height, and field-state behavior.

### M-UI.3 — remaining composition primitives

- Add toolbar, page header, metadata strip, dialog/menu, disclosure, split workspace, ordered list, and drag handle where repeated usage proves the contract.
- Move owned CSS out of the global stylesheet incrementally.

### M-UI.4 — Draft Structure reference migration

- Migrate Structure without changing selection, reordering, persistence, navigation, or responsive behavior.
- Use it to validate buttons, icons, search, panels, menus, ordered rows, metadata, inspector sections, destructive actions, and split layout.

### Reference migration: Draft Structure

Draft Structure composes `BtgToolbar`, `BtgSearchField`, `BtgSplitWorkspace`, `BtgPanel`, `BtgDisclosure`, `BtgOrderedRow`, `BtgDragHandle`, `BtgMetadataStrip`, `AuthoringInspectorSection`, and `BtgPreviousNextNavigation`. Buttons, icons, surfaces, selected rows, metadata, and responsive control treatment come from those primitives; feature code must not restyle them.

Structure retains only feature ownership: outline hierarchy, the module/lesson column arrangement, insertion indicators during reordering, inspector identity arrangement, empty-state placement, and destructive-action separation. It owns reorder persistence, selection, search filtering, and localized messages; it does not own visual identity. Future migrations should follow this composition-first pattern and add local CSS only for equivalent domain-specific layout or state.

### M-UI.5 — Lesson authoring

- Migrate Details, Content, and Prerequisites.
- Consolidate fields, status/conflict messages, dialogs, ordered lists, drag handles, and dirty bars.

### M-UI.6 — Draft authoring

- Migrate Overview, Members, Assessments, and Review.
- Consolidate headers, metadata, sections, tables/lists, empty states, and workflow messages.

### M-UI.7 — remaining Academy pages

- Migrate Dashboard, Courses, Home, authentication, Administration, Community, and Translation.
- Preserve intentional editorial differences through semantic themes/compositions rather than control forks.

### M-UI.8 — cleanup and enforcement

- Remove compatibility aliases, obsolete global selectors, custom UI SVGs, and baseline exceptions.
- Enable CI checks and publish the examples/gallery route for contributors.

Draft Structure is the recommended first reference migration. It exercises the widest useful cross-section of primitives and interaction states without requiring a new product workflow. Dashboard should be the first non-Authoring validation to ensure the system does not become Authoring-specific.

## Usage examples

Prefer semantic composition:

```vue
<BtgSection>
  <template #heading>Content</template>
  <template #action>
    <BtgButton variant="primary" leading-icon="edit">Edit content</BtgButton>
  </template>
  <BtgChip icon="image">Image</BtgChip>
</BtgSection>
```

Use compositions to arrange primitives without moving feature behavior into them:

```vue
<BtgToolbar>
  <template #leading><BtgSearchField v-model="query" label="Search lessons" /></template>
  <template #actions><BtgButton leading-icon="plus">Add lesson</BtgButton></template>
</BtgToolbar>

<BtgSplitWorkspace variant="outline-inspector" primary-label="Course structure" secondary-label="Lesson inspector">
  <template #primary><CourseOutline /></template>
  <template #secondary><LessonInspector /></template>
</BtgSplitWorkspace>
```

Avoid page-owned primitive appearance:

```vue
<!-- Do not add structure-edit-button CSS or inline SVG path data. -->
<button class="structure-edit-button">
  <svg><!-- custom pencil --></svg>
  Edit content
</button>
```

Before creating a new visual pattern, check the primitive catalog, state the missing semantic requirement, and extend the narrowest appropriate shared layer. Do not add a variant named after a page or feature.
