# Accessibility release checks

Automated Vitest, Playwright, and axe checks are only the first gate. For each learner-facing feature, manually verify:

- Keyboard-only navigation, visible focus, and expected focus return after overlays.
- Screen-reader labels, headings, landmarks, errors, and dynamic status announcements.
- 320 px reflow and 200%/400% zoom without hidden controls or competing scroll regions.
- Reduced-motion settings and no surprise movement, autoplay, or time pressure.
- Predictable navigation, clear progress language, manageable visual density, and learner-controlled presentation.

The target is WCAG 2.2 AA plus the BTG cognitive-accessibility principles in the architecture baseline. No business screens exist in this skeleton yet.
