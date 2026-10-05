## Response efficiency

Keep task updates and final responses concise.

During implementation:
- Do not narrate routine actions.
- Do not explain obvious code changes while making them.
- Do not repeat the task description.
- Do not summarize unchanged architecture.
- Do not include full command output unless a command fails.
- For successful tests/builds, report only the command and final result.
- Do not list every modified file unless specifically requested.
- Avoid restating implementation details already visible in the diff.

Final response should normally contain only:
1. What was implemented.
2. Any important design decision or deviation.
3. Validation result.
4. Any unresolved issue.

Target roughly 5–10 lines for normal implementation tasks.
