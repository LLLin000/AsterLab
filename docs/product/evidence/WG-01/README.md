# WG-01 evidence

- Base: `origin/main` at `8ed639b`; isolated with `ovnfs v0.5.2`.
- Kandev: Windows Gate workflow `2511d5d1-fce5-4521-9cbb-0b6033564993`; Windows Work WIP limit `2`; Windows Verify uses card-specific acceptance and evidence.
- Check: `git diff --check -- AGENTS.md docs/engineering/cross-platform-builds.md docs/engineering/kandev-card-template.md docs/product/ownership.md` — exit code 0, no output.
- Commit check: `git show --check --oneline HEAD` — `6d9fed7 docs(WG-01): define Windows Gate execution protocol`, exit code 0.
- PR: [#14](https://github.com/LLLin000/AsterLab/pull/14), open.
