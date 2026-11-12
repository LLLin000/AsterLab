# Fixture: REL-X (Build) 越界 PR

## 结果
Fixture used by `scripts/ci/test_scope_check.py`; the card scope only covers
`crates/`, keeps `README.md` read-only and forbids `docs/contracts/**`.

## 范围
<!-- kandev-scope
write:
  - crates/
read:
  - README.md
forbid:
  - docs/contracts/**
-->
