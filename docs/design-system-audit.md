# Academy frontend design-system audit

Date: 2026-10-05

This report records the current frontend presentation architecture and recommends the migration order. Counts are approximate static-search results intended to locate hotspots, not quality scores. No product behavior or broad visual refactor is part of this milestone.

## Executive summary

Academy has the beginning of a design system, but not yet a consistently enforced one. It has a useful token file, a canonical page container, shared Button/FormField/TextInput components, several Authoring compositions, and Reka UI for complex interaction. Most presentation code, however, lives in one 5,460-line global stylesheet, while feature components frequently own complete visual patterns.

The highest-value next step is not another page-level polish pass. It is to separate foundation values from semantic theme tokens, standardize the existing Lucide-backed icon abstraction, complete the core control set, and then extract repeated compositions. Draft Structure is the best reference migration because it exercises most of the required system; Dashboard should validate that the result is not Authoring-specific.

## Audit scope and method

The audit inspected:

- global tokens, base rules, and the main stylesheet;
- `AppShell`, page containers, routed pages, and shared components;
- Draft Structure and Overview;
- Lesson Details, Content, and Prerequisites;
- Dashboard, Home, Courses, dialogs, menus, list/table patterns, statuses, empty states, and drag handles;
- package dependencies and icon sources;
- responsive queries, raw visual values, and repeated class-name families.

Snapshot indicators:

- about 172 Vue/TypeScript/CSS files under `frontend/src`;
- 5,460 lines in `style.css`, 261 in `base.css`, and 96 in `tokens.css`;
- only eight Vue files contain scoped style blocks;
- approximately 1,212 top-level selector, media-query, and container-query lines in `style.css`.

The repository had unrelated in-progress frontend changes during the audit. This report describes the inspected working tree and does not claim every count is a clean-branch baseline.

## Current architecture

| Layer | Current implementation | Finding |
| --- | --- | --- |
| Entry | `main.ts` imports `style.css`; it imports tokens and base | Clear entry point, but all features then share one global cascade |
| Tokens | `styles/tokens.css` | Useful start; foundation, semantic, homepage, layout, and Authoring values are mixed |
| Base | `styles/base.css` | Shared element defaults and focus behavior exist |
| Global feature CSS | `style.css` | Dominant styling layer; shell, public pages, Authoring, learner pages, and overrides coexist |
| Generic components | `BtgButton`, `BtgFormField`, `BtgTextInput`, `BtgPageContainer`, `BtgDialog`, `BtgIcon` | Good seeds, but incomplete contracts and uneven adoption |
| Authoring components | headers, tabs, sections, dirty bar, checkbox field, objective editor | Genuine reuse exists; several generic visual concerns remain embedded in feature CSS |
| Feature/page CSS | global page-prefixed selectors plus eight scoped blocks | Ownership is inconsistent and cascade order can become part of behavior |

Approximate component adoption by importing files:

| Component | Files |
| --- | ---: |
| `BtgButton` | 46 |
| `BtgPageContainer` | 21 |
| `BtgFormField` | 15 |
| `BtgTextInput` | 9 |
| `AuthoringDirtyActionBar` | 4 |
| `AuthoringSection` | 2 |
| `AuthoringTabs` | 2 |
| `BtgIcon` | 2 |
| `BtgDialog` | 0 |

Native controls remain common: roughly 29 native buttons, 35 inputs, 21 textareas, 13 selects, and six dialogs appear in Vue templates. Native HTML is not itself a problem; the issue is that equivalent presentation and composite behavior are repeatedly assembled around it.

## UI inventory

| Category | Existing reuse | Local duplication or inconsistency | Candidate |
| --- | --- | --- | --- |
| Buttons | `BtgButton` with primary, secondary, quiet, destructive, destructive-secondary | Native buttons and feature classes reproduce icon spacing, compact actions, menu triggers, and button-like links | Extend `BtgButton`; add `BtgIconButton` |
| Text inputs | `BtgTextInput`, global input rules | Search, translation, configuration, and editor fields wrap controls differently | Complete text-field contract |
| Textareas | Global native styling | Pages assemble labels, errors, sizing, and resize behavior locally | `BtgTextarea` |
| Selects | Global native styling | Repeated field wrappers and status handling | `BtgSelect` |
| Checkboxes | `AuthoringCheckboxField` | Non-Authoring pages use other markup/helper placement | `BtgCheckbox`; Authoring wrapper only if semantics differ |
| Radio/segmented | Native controls and local groups | Layout and selected appearance vary | `BtgRadioGroup` with optional segmented presentation |
| Search fields | Structure and Prerequisites have local icon/input wrappers | Icon positioning, clear behavior, labels, and filtered-state help differ | `BtgSearchField` |
| Route tabs | `AuthoringTabs` | Similar underline navigation remains feature-specific | `BtgPageTabs`; keep links, not ARIA tabs |
| Interactive tabs | No clear generic primitive | Any future in-panel tabs risk ad hoc keyboard behavior | `BtgTabs` only when actually needed |
| Links | Base rules and router links | Some links are styled as buttons through feature classes | Shared link treatment; buttons for actions |
| Badges/status labels | Page-local chips, labels, and pills | Size, radius, color, and semantic wording vary | `BtgBadge` |
| Content chips | Structure and metadata/type displays | Local padding/icon alignment | `BtgChip` |
| Panels/cards | Page-prefixed panel/card classes | Border, surface, radius, padding, and shadows repeat | `BtgSurface` with restrained variants |
| Sections | `AuthoringSection` | Inspector, Dashboard, Review, and public sections implement heading/action layout separately | Generic `BtgSection`, composed by `AuthoringSection` |
| Toolbars | Many `__toolbar` selectors | Wrapping, flexible search, action gaps, and DOM order are repeatedly solved | `BtgToolbar` |
| Action/dirty bars | `AuthoringDirtyActionBar` | Content and other workflows have local save/status bars | Keep Authoring pattern; extract generic base only with non-Authoring reuse |
| Dialogs | `BtgDialog` on Reka exists | It has no consumers while about six feature dialogs remain native/local; contracts differ | Make `BtgDialog` flexible and migrate deliberately |
| Menus | Local overflow/action menus | Trigger naming, positioning, keyboard behavior, and outside-click logic risk divergence | `BtgMenu` using existing Reka dependency |
| Disclosures | Native/local module and section toggles | Chevron, target area, and expanded semantics are repeated | `BtgDisclosure` |
| Lists | Semantic lists plus many feature row systems | Row padding, separators, actions, and metadata alignment vary | Shared row composition only after repeated contract is clear |
| Ordered/reorderable lists | Objectives, Structure, Content, Prerequisites | Drag handles, insertion cues, keyboard moves, announcements, and disabled states repeat | `BtgOrderedList` and `BtgDragHandle` behavior/presentation shells |
| Tables | Global table rules and feature tables | Compactness, status cells, actions, and mobile handling vary | Shared table styles/composition; not a universal data-grid |
| Empty states | Page-local `__empty` families | Similar title/help/action treatment repeated | `BtgEmptyState` |
| Status/error/conflict | Page-local status, error, conflict classes | Surface, icon, role, focus, and live announcements vary | `BtgStatusMessage` |
| Metadata | Draft headers, course pages, inspector, review, dashboard | Label/value grids and separators are repeatedly coded | `BtgMetadataStrip` and definition-list pattern |
| Page headers | Draft/Lesson shared headers; many page-local headers | Eyebrow/title/helper/action composition differs | `BtgPageHeader`; retain domain wrappers |
| Page shell/container | `AppShell`, `BtgPageContainer` | This is the strongest shared layout area | Keep and enforce canonical canvas |
| Split panes | Structure and Prerequisites local grids | Ratios, stretch, stacking, and min-width handling repeat | `BtgSplitWorkspace` |
| Inspectors | Structure detail and other selected-item surfaces | Header, metadata, sections, and footer actions are feature-local | `BtgInspector` plus optional Authoring section pattern |
| Icons | `BtgIcon` backed by Lucide is newly present | Inline SVG, Unicode pseudo-icons, and inconsistent sizing still coexist | Make `BtgIcon` the only UI icon gateway |

## Duplication hotspots

A selector-name scan found approximately 296 references across recurring visual families:

| Pattern | Approximate references | Consequence |
| --- | ---: | --- |
| Metadata | 63 | Many label/value grids and typography variants |
| Drag/reorder | 46 | Repeated handles, cues, active state, and accessibility hooks |
| Panel | 30 | Repeated surface/border/padding decisions |
| Toolbar | 28 | Repeated gaps, wrapping, flexible fields, and alignment |
| Empty | 24 | Page-local empty-state composition |
| Conflict | 21 | Repeated warning surfaces and actions |
| Save/dirty | 21 | Multiple action-bar/status arrangements |
| Status | 17 | Inconsistent message semantics and appearance |
| Error | 16 | Field and operation errors overlap but are styled locally |
| Actions | 16 | Repeated action-group rules |
| Chip | 10 | Similar compact labels with different dimensions |
| Menu | 4 | Low count but high interaction/accessibility risk |

Large page-prefix families also demonstrate visual ownership at the feature level: Authoring Review, Home, Dashboard, Prerequisites, Create, Content, Members, the Lesson page, Structure, and Administration each carry substantial selector sets. The issue is not BEM naming; it is that many of those selectors implement the same surface, control, metadata, and status concepts.

Repeated token usage shows that adoption is real: spacing tokens 2–5 and the small type/radius tokens appear dozens of times. At the same time, raw `1rem`, `1.125rem`, `1.25rem`, `2rem`, `2.25rem`, `2.5rem`, `2.75rem`, and `3rem` values recur often enough to indicate missing type, icon, row, and control roles. Breakpoints proliferate around 20, 24, 28, 30, 32, 34, 38, 42, 44, 48, and 52rem, plus 480px.

## Hardcoded-value classification

### A. Legitimate one-off geometry

- rich-text/editor and media preview dimensions;
- illustration and diagram geometry;
- a feature split ratio chosen for actual content;
- code block, canvas, or embedded-runtime dimensions;
- content-driven values received from safe data.

These values should remain local and documented when their purpose is not obvious.

### B. Values that should become tokens

- repeated control heights around 2.25–2.75rem;
- repeated UI icon sizes around 1–1.375rem;
- typography roles currently expressed as recurring raw size/weight pairs;
- menu/modal elevation and backdrop values;
- z-index layers;
- standard row heights/padding and toolbar/section gaps;
- common reflow thresholds where content requirements align.

### C. Suspicious duplication

- raw fallback colors such as `#b8c0c5`, `#b8b8b8`, `#9b1c1c`, and `#536878` outside the token source;
- several repeated `rgb(0 0 0 / ...)` shadows and backdrops in `style.css`;
- homepage presentation values embedded beside global foundation tokens;
- repeated neutral status surfaces, borders, selected rows, and helper text rules.

### D. Values expected to disappear after extraction

- feature-specific button fills, borders, padding, and icon gaps;
- local input/search chrome and focus rules;
- dialog/menu shells and overlays;
- metadata-grid separators and label styles;
- empty/status/conflict surface rules;
- repeated chip, drag-handle, and action-group dimensions.

Raw colors are not widespread in absolute terms, which makes prevention practical. The larger risk is semantically duplicated token combinations and arbitrary dimensions inside the global stylesheet.

## Icon audit

`@lucide/vue` is installed and `BtgIcon.vue` already maps semantic names to Lucide icons. This is the correct canonical direction, but adoption is currently limited to roughly two importing files.

At least six Vue files still contain inline SVG. They include rich-text/content editing, asset attachment, prerequisite/objective reordering, and the public Home page. Unicode symbols are also used as pseudo-icons, including plus and overflow glyphs. Some of the Home SVG is a legitimate illustration; common actions and controls are not.

Recommendation:

1. Keep Lucide as the single icon family.
2. Require UI code to render it through `BtgIcon`, never direct imports.
3. Map product-semantic names in one place and normalize size/stroke/alignment there.
4. Add `BtgIconButton` for accessible icon-only controls.
5. Permit bespoke SVG only for logos, diagrams, data visualization, and editorial illustration, with a documented exception.
6. Reject new inline UI paths, Unicode pseudo-icons, and emoji in CI.

## Representative page findings

### Draft Structure

Structure already uses `BtgButton`, `BtgIcon`, the canonical page container, and shared Authoring headers. It still owns search composition, toolbar, two panels, hierarchical rows, drag state, menu/dialog presentation, metadata strip, chips, inspector sections, and destructive grouping. Its CSS also reflects several rounds of local refinement and override.

The proposed system replaces visual mechanics while leaving module disclosure, search, selection, reordering, revision persistence, rollback, previous/next navigation, and edit flows in the feature. Because it exercises nearly every proposed layer, it is the best reference migration.

### Draft Overview

Overview benefits from `AuthoringSection`, `BtgFormField`, `BtgTextInput`, `RepeatableObjectivesEditor`, and `AuthoringDirtyActionBar`. Local panels, two-column layout, statuses, and action arrangements remain. Shared sections, fields, status messages, and responsive composition would substantially reduce its CSS without changing its manual-save behavior.

### Lesson Details

Details repeats much of Overview's field, checkbox, objective, section, and dirty-state language under a Lesson header. It is a strong second consumer for the form and Authoring-section contracts. Product-specific revision/conflict handling stays local.

### Lesson Content

Content has an ordered block list, toolbar/actions, selection, local menus, dialogs/pickers, status/conflict/empty states, dirty actions, and custom SVG or Unicode control icons. It would consume shared buttons/icons/menu/dialog/status, plus ordered-list and drag-handle compositions. Canonical content schema and persistence remain untouched.

### Lesson Prerequisites

Prerequisites uses a responsive split workspace, search, disclosure, selected ordered rows, drag/keyboard reordering, status/conflict handling, and a dirty bar. It is a natural second consumer for SplitWorkspace, SearchField, Disclosure, OrderedList, and DragHandle. Prerequisite semantics and save behavior remain Authoring-owned.

### Dashboard

Dashboard uses the canonical container and button but implements its own page header, eyebrow, loading/empty states, widget cards, management panels, badges/status labels, and action groups in scoped CSS. It also contains a Unicode plus. Migrating it immediately after the Authoring reference set verifies that generic primitives work across the product rather than encoding an Authoring look.

### Additional coverage

- Courses repeat list/card, metadata, pagination, empty, and status treatments.
- Home has a deliberate editorial visual identity, but shared controls should still use common primitives under a theme; its hero artwork can remain bespoke.
- Administration, Community, Translation, authentication, and course reading contain further form, table/list, dialog, status, and action variants to migrate later.

## Risks

1. **Cascade risk:** moving rules out of the large global stylesheet can reveal undocumented selector-order dependencies. Migrate one component family at a time with screenshots and behavior tests.
2. **Over-abstraction:** generic row or inspector components can become slot-heavy feature proxies. Extract appearance/interaction contracts only after two real consumers agree.
3. **Behavior regression:** drag/drop, dialogs, menus, route tabs, and dirty bars combine visuals with focus and persistence. Preserve feature state and move only reusable responsibilities.
4. **Theme leakage:** renaming current variables without separating palette from semantics would preserve the present problem. Establish the layer boundary before mass replacement.
5. **Authoring bias:** Structure covers many primitives but not public editorial or Dashboard patterns. Use Dashboard as the first non-Authoring validation.
6. **Dirty-tree overlap:** current Structure work is in progress. Begin its reference migration only from an agreed functional baseline.

## Recommended order

1. Define foundation/semantic/default-theme files with compatibility aliases.
2. Finish `BtgIcon`, Button, form controls, SearchField, Chip/Badge, and StatusMessage.
3. Add interaction primitives for icon button, dialog, menu, and disclosure using existing Vue/Reka infrastructure.
4. Extract proven compositions: Toolbar, Surface/Section, MetadataStrip, SplitWorkspace, OrderedList, and DragHandle.
5. Migrate Draft Structure as the reference page, preserving all behavior.
6. Migrate Lesson Details, Content, and Prerequisites; then the remaining Draft pages.
7. Validate genericity through Dashboard, then Courses/Home and remaining areas.
8. Remove compatibility CSS and enable new-code enforcement.

Detailed contracts and milestone phases are defined in [design-system.md](./design-system.md).

## Enforcement recommendation

Begin with two small CI checks: no new raw colors outside theme/exception files, and no new inline UI SVG/direct Lucide import outside `BtgIcon`. Add a PR checklist and primitive component tests. Defer broad Stylelint adoption until CSS ownership is separated enough for actionable output.

For visual regression, use the existing Playwright stack. Maintain a small reference matrix rather than screenshotting every page: Draft Structure desktop, one Authoring form at 320px and 200% text, Dashboard, and representative dialog/menu states. Geometry and semantic assertions should remain the primary stable checks; screenshots should catch high-level composition drift.

## Conclusion

Academy does not need a new CSS framework. It needs to make its existing shared direction explicit: semantic tokens, Lucide behind one icon abstraction, complete generic primitives, a small set of proven compositions, and clear feature ownership. Incremental migration can then reduce the global cascade without changing domain behavior or forcing a product-wide redesign.
