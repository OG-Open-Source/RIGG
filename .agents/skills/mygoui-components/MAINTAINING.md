# Maintaining mygoui components

For work inside the mygoui repository. Paths are relative to its root.

## Before changing a component

- A new component or a public behavior or API change is an OpenSpec change: read `docs/agent-workflow.md` and `openspec/specs/native-library/spec.md` (plus `native-charts` or `overlay-flyouts` where relevant).
- Read the `mygo-maintenance` skill for MyGo lifetime, identity and testing rules, and `docs/design.md` for the package dependency direction: no package imports one that imports it.
- Visual changes go through the `impeccable` skill against `DESIGN.md`.

## Shared internals

- `internal/busy` is the only loading treatment: `Swap` for an action's content-to-spinner crossfade, `Show`/`Dim`/`Line` for a region's wait, dim and top line, `Spinner`/`PaintSpinner` for the indicator. A component with a `Loading` flag uses these instead of its own timing.
- `internal/motion` holds `Progress` and `Color`, the helpers for a state changing in place; durations and easings are theme tokens (`theme.Motion()`). See the `mygoui-motion` skill's MAINTAINING.md.
- The root `motion.go` holds the root package's shared transitions (`fadeTransition`, `stateIcon`).

## Keep the records aligned

- `docs/components.md`: the component's row, its subset and its gaps.
- `docs/design.md`: the packages table and ownership rules when they change.
- Run `make generate` after changing an exported struct; the suite fails while `internal/gallery/api_gen.go` is stale.
- Add or update the component's gallery page under `internal/gallery`.
- Test consumer-visible behavior with `ui.NewTester`; `make check` runs the suite.

The `docs/design.md` Packages table omits `content`; add it when that record next changes.

## Keep this skill current

`SKILL.md` states the public contract an application author relies on. When a change renames an API it names, alters a wiring rule (state ownership, `Loading`, `Revision`, identity, startup policy) or adds a family package, update `SKILL.md` in the same change and list it in that change's tasks. Check every name with `go doc` and compile changed examples against the checkout. Keep internal names out of `SKILL.md`; they belong here.
