<!--
PR title must be Conventional Commits format: type(scope): description
(e.g. feat(bookshelf): ..., fix(bookshelf-backend): ...) — squash-merge uses
this title as the release commit nx release reads to version/publish. If the
PR touches more than one nx project, scope all of them, comma-separated
(e.g. feat(bookshelf,bookshelf-backend): ...) — a project touched by the
diff but not named in the scope only gets an indirect patch bump, never the
real feat/fix-implied bump. See .github/workflows/pr-title.yml.
-->

## Summary

Briefly describe the changes and the reason behind them.

## Type of Change

- [ ] Bug fix
- [ ] Feature
- [ ] Documentation
- [ ] Other

## Approach

Describe your approach or solution for this change.

## How to Test

Provide instructions on how to test the changes.

## Additional Notes

Add any extra information or context here (if needed).
