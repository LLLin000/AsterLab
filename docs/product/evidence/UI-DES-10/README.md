# UI-DES-10 — Plate geometry and budget sources

## Design consistency

The plate renderer extends the existing `internal/uikit/plate` widget and uses the shared `theme.GetTokens` palette and spacing/type scale; the Design budget bar remains the existing `internal/designer` bar and keeps Core and annotation values source-labeled and separate.

## 缺口

The Design workflow cannot yet author OperationSpec v2 (`workflow` currently exposes only `steps`, `edges`, and `business_order`), so Core time budgets are shown only when Core returns a populated projection; otherwise the bar explicitly retains the existing `设计标注` estimates without combining the two sources, and a follow-up Core/Docs card is needed to define the Design-side OperationSpec authoring entry point.
