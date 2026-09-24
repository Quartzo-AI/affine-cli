# Doc Authoring Rules

These rules cover `AGENTS.md` and the extracted developer docs it points to.

## Editing `AGENTS.md`

The "Code & Comment Hygiene" rules apply to docs too:

- No dates, incidents, or ticket numbers in rules. Justification belongs in the PR introducing the rule, not embedded in it.
- Do not defend the doc's structure inside the doc. Write the rule and the pointer; skip narration about why the file is arranged that way.
- Make rules applicable at the moment they fire. Keep the inline `AGENTS.md` rule command-shaped: trigger, required action or prohibition, concrete values, then the pointer to the longer doc.
- Examples should be generic or anti-pattern-shaped, not lifted from the specific incident that prompted the rule.

## Pointer-rot rule

When an extracted doc's applicability changes, update the inline trigger sentence in `AGENTS.md` in the same PR. Applicability changes include a new fire condition, a removed fire condition, or a changed prohibition, enum, file path, test name, or required value.

## Developer-doc surface

The extracted developer docs are:

- `reference/GOLDEN.md` — golden harness rubric and fixture conventions
- `reference/GLOSSARY.md` — naming conventions, disambiguation defaults, and the implementation reference behind the concepts in `CONCEPTS.md`
- `reference/RELEASE.md` — release-please / goreleaser flow
- `reference/ATTRIBUTION.md` — creator + contributors attribution model
- `reference/ARTIFACTS.md` — local library, manuscripts, and public-library flow
- `reference/CODEX.md` — installing and using Printing Press skills in Codex
- `reference/CURSOR.md` — using printed CLIs and skills in Cursor
- `reference/DOCS.md` — this doc-authoring guidance
- `reference/PLUGIN-DEV.md` — persistent local plugin development setup
