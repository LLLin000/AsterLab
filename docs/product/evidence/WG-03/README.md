# WG-03 evidence — Windows Work parallel cap 2 → WIP=4

## Baseline

- Commit `ad8ed02ba67f430c6ee0177fb0a4aef89c02ec79` (PR #152 merge).
- Work done in isolated checkout `O:/wg03-followup`, created with
  `ovnfs new --name wg03-followup --rev ad8ed02ba67f430c6ee0177fb0a4aef89c02ec79`.
- `O:/wg03` (older draft) was read as source only and never edited.

## Change summary

- `docs/engineering/cross-platform-builds.md` §10: `WIP=2` → `WIP=4`
  ("At most two" → "At most four"). Everything else in §10 is unchanged:
  pairwise-disjoint machine-readable `scope.write`, one separate `ovnfs`
  checkout per card, real ordering represented by Kandev `blocked_by`,
  and Windows GUI/build/E2E serialized on the shared host.
- `docs/engineering/kandev-card-template.md` Definition of Ready: "at most
  two independent work cards" → "at most four". Everything else unchanged,
  including `blocked_by`-only dependency recording (DoR items, §2,
  hard rules).
- This README. Docs-only change: no workflow settings, no code.

## Draft review (why the old `O:/wg03` draft was not copied verbatim)

- The old draft replaced §10 wholesale with a shorter "Windows parallel work"
  section. On this baseline §10 already contains the entry/checkout procedure
  and the `Windows Work` → `Windows Verify` → `Done` handoff; replacing it
  would have deleted procedure, so the cap was bumped with a two-line edit
  that keeps every guarantee.
- The old draft created `kandev-card-template.md` as a new thin file with a
  per-card `depends_on` front-matter field. On this baseline that file already
  exists as the comprehensive card template (DoR, `kandev-scope` /
  `kandev-delivery` contracts), so overwriting it would have destroyed content.
  Only the parallel-card limit line was touched.
- Dependency naming follows the actual Kandev relation `blocked_by`, not a
  per-card `depends_on` field. Task dependencies must be recorded in Kandev
  `blocked_by` before parallel work starts (anchors below).
- The old evidence claim "no prior WIP cap" is wrong on this baseline
  (see correction below); the old baseline/commit references were also stale.

## Prior-cap correction

On baseline `ad8ed02`, Windows Work WIP=2 was stated in all three active
policy files, plus one historical record:

- `AGENTS.md:56` — LB-18 updates this rule to WIP=4 in its coordinated change.
- `docs/engineering/cross-platform-builds.md:196` — WIP=2 before this card,
  now WIP=4.
- `docs/engineering/kandev-card-template.md:220` — "at most two" before this
  card, now "at most four".
- `docs/product/evidence/WG-01/README.md:4` — historical WIP-2 record;
  unchanged.

## Grep results and anchors (run in `O:/wg03-followup` @ `ad8ed02`)

Prior cap (pre-edit values; no "no prior cap" — the cap was 2):

```
$ grep -n -i 'WIP' AGENTS.md
56:- 默认串行；Kandev `Windows Work` WIP=2 是上限。只有 `scope.write` 不重叠且数据隔离时才并行；GUI/native build/截图/E2E/最终集成始终串行。

$ grep -rn --include='*.md' -i 'WIP' docs/   # Windows-Work-relevant hits
docs/engineering/cross-platform-builds.md:196:- Default to serial execution. `Windows Work` has WIP=2 as a hard ceiling, not
docs/engineering/kandev-card-template.md:220:- [ ] Windows Gate defaults to serial and allows at most two independent work cards; ...
docs/product/evidence/WG-01/README.md:4:- Kandev: ... Windows Work WIP limit `2`; ...
```

(Other `WIP` hits are other lanes — `Agent`/`Review` WIP=4, VPS Dev WIP=4 —
or the words "wipe"/"wiped" in research notes; none govern Windows Work.)

Dependency relation is `blocked_by` (no per-card `depends_on` field exists
anywhere in `docs/`, `scripts/`, or `prototypes/` scope output — zero hits
for a `depends_on:` field):

```
$ grep -rn 'blocked_by' docs/engineering/cross-platform-builds.md docs/engineering/kandev-card-template.md AGENTS.md
docs/engineering/cross-platform-builds.md:185:   and baseline. Do not start while any `blocked_by` dependency is incomplete.
docs/engineering/cross-platform-builds.md:200:  Kandev `blocked_by`, and each uses its own ovnfs checkout and isolated data.
docs/engineering/kandev-card-template.md:23:- 一张卡只能有一个分类。...拆成两张卡，用 `blocked_by` 连接。
docs/engineering/kandev-card-template.md:75:- upstream: none（或 `DES-00, RDO-01`，必须与 Kandev 的 blocked_by 完全一致）
docs/engineering/kandev-card-template.md:97:- ...这些用 Kandev 的 label、priority 和 `blocked_by`。
docs/engineering/kandev-card-template.md:215:- [ ] 依赖已写入 Kandev `blocked_by`，正文不重复完整列表。
docs/engineering/kandev-card-template.md:218:- [ ] Parallel cards have disjoint ... `scope.write` sets; ... real ordering is recorded with `blocked_by`.
AGENTS.md:54:- Windows Gate 是人工泳道，...开工前检查 Kandev 卡片的 `blocked_by`、范围和基线，...
```

Serialization anchors (pre-existing, unchanged by this card):

```
$ grep -n -i 'at the same time\|one build at a time\|one.*checkout per' docs/product/environment-gate.md docs/engineering/mygo-v1-workstreams.md
docs/product/environment-gate.md:94:1. No two agents build at the same time on D:.
docs/product/environment-gate.md:113:3. Accept 13 GB and keep one build at a time; cargo no longer grows D:.
docs/engineering/mygo-v1-workstreams.md:6:- One `ovnfs` checkout per workstream; never let two writers touch one file.
```

Plus `cross-platform-builds.md` §§6–8: `mygo-build.ps1` (app + `Runs.dll`
copy), `mygo native check`, `go test ./...` against the host DLL, and
`scripts/gui-e2e-v11.ps1` with the §8 Windows merge gate.

Template / evidence-dir pre-existence on this baseline:

```
$ ls docs/engineering/kandev-card-template.md
docs/engineering/kandev-card-template.md     # already existed; edited, not created
$ ls docs/product/evidence/WG-03/
cannot access 'docs/product/evidence/WG-03/': No such file or directory   # created by this card
```

## No-conflict statement

At baseline `ad8ed02`, all three active policy files read WIP=2. This card
updates the two owned engineering docs to WIP=4; LB-18 PR #155 updates
`AGENTS.md:56` to WIP=4. Historical WG-01 evidence still records the old limit.
The parallel-work guards remain pairwise-disjoint `scope.write`, separate
`ovnfs` checkouts, Kandev `blocked_by` before parallel work, and serial
GUI/build/E2E.

## Coordinator verification

- The active-parallelism scan found the WIP=4 cap in both owned engineering
  docs; the stale `WIP=2` / `at most two` scan returned no matches.
- The trailing-whitespace scan of all three changed documents returned no
  matches. `git diff --check` was not run; the coordinator used a scoped
  `awk` scan instead.
- PR #154 (`agent/wg03-wip4-docs-20261116`), commit `23ab0fc2b7`; all three
  files are within this card's `scope.write`.

