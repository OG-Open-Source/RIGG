---
name: mygoui-components
description: "Building MyGo application UI with mygoui components. Use when choosing a component, composing several, or wiring one to application state."
---

# mygoui components

For work on the mygoui repository itself, also read [MAINTAINING.md](MAINTAINING.md).

mygoui builds native MyGo controls from typed Go options. Two lookups answer most questions:

- **API**: `go doc github.com/Cyenoch/mygoui/form SwitchOptions`. Option comments state defaults, required fields and what the caller owns.
- **Records**, shipped with the module version your application uses: `go list -m -f '{{.Dir}}' github.com/Cyenoch/mygoui` (when vendored, the repository at that version), then `docs/components.md` (every component, its Go API, supported subset and gaps) and `docs/design.md` (packages and the ownership contract).

## Choose

Find the need in `docs/components.md`; its row names the constructor and states what the native version leaves out. Prefer a listed component over a hand-drawn one: the component carries native focus, keyboard, selection and accessibility. Where the row records a gap, build on MyGo's own widgets, not on pointer-only drawing.

## Wire

Constructors take the context first and a typed options struct, `pkg.Name(c, pkg.NameOptions{...})`; editors also take the value pointer, containers a `body func()` for independent content, and collections a typed slice (`list.ListBoxItem`, `mygoui.GroupItem`, `navigation.CommandItem`) for coordinated items.

- **Theme first.** Call `theme.Apply(c)` at each view root before building children.
- **Caller-owned state.** Values are pointers to model fields (`form.Input(c, &m.name, ...)`, `overlay.Dialog(c, &m.open, ...)`); state structs (`list.ListBoxState`, `form.ComboBoxState`, `data.TableState`) live in the window's model, one per component, never copied. A nil value pointer renders the control disabled where its doc says so.
- **Build-scoped values.** `*ui.Context`, `ui.Element` and custom parts last one build. Keep model values, state structs and `ui.Handle` instead.
- **Identity.** Give each repeated or conditionally built component `c.Key(id)` before constructing it; give collection items their `Key` field. A stable key keeps focus, animation and local state with the right item.
- **Actions.** Put side effects in handled callbacks (`OnClick`, `OnChange`, `OnSubmit`); a view rebuilds several times per event.
- **Background work.** Run I/O in a goroutine and publish results with `Window.Update`. Capture `c.Services()` for clipboard and URL callbacks.
- **`Loading` flags** are caller-owned and start no work: set the flag when your work starts, clear it in `Window.Update`. Composed content without a flag goes inside `mygoui.LoadingOverlay`.
- **`Revision`.** `ComboBox`, `Autocomplete`, `Command`, `Tree` and `Table` keep filtered results until the query, the item slices or `State.Revision` change; bump `Revision` after editing items in place.
- **Startup policy.** Importing mygoui registers nothing. Register fonts (`fonts.Register`) and library messages (`locale.Set`) at application startup.
- **Toasts.** Install `ToastState.View(c)` once at the window root, after `theme.Apply`.
- **Errors.** Constructors returning `(ui.Element, error)` (charts, `content.JSONViewer`, `content.QRCode`) report bad input; handle the error.

```go
type inbox struct {
	win     *mygo.Window
	threads list.ListBoxState
	items   []list.ListBoxItem
	syncing bool
	notify  bool
}

func (m *inbox) View(c *ui.Context) {
	theme.Apply(c)
	ui.Column(c).Gap(theme.Space12).Padding(theme.Space16).Children(func() {
		form.Switch(c, form.SwitchOptions{Label: "Notifications", On: &m.notify})
		mygoui.Button(c, mygoui.ButtonOptions{
			Label: "Sync", Loading: m.syncing, OnClick: m.sync,
		})
		list.ListBox(c, list.ListBoxOptions{
			Label: "Threads", Items: m.items, State: &m.threads,
			Selection: list.SelectionSingle, Loading: m.syncing, Height: 240,
		})
	})
}

func (m *inbox) sync() {
	m.syncing = true
	go func() {
		items := fetchThreads() // slow I/O stays off the UI thread
		m.win.Update(func() { m.items, m.syncing = items, false })
	}()
}
```

## Compose

Components compose through Go callbacks: pass each one what it needs in its options or body. `mygoui.Group` hands each `GroupItem.Build` its segment corners, which the builder applies to the control it returns. Card, Frame and overlay sections own their padding; overlay footers take actions built directly, with `ButtonOptions.FullWidth` chosen for narrow layouts. Layout between components uses MyGo's `ui.Row`/`ui.Column` with `theme.Space*` gaps; `docs/mygo-vs-css.md` maps CSS habits to MyGo layout.

An icon-only button still takes `ButtonOptions.Label`: with `IconOnly` it becomes the accessible name.

## Window shell (`layout`)

A window that draws its own title bar (`mygo.WindowOptions.TitleBarStyle`) starts from `layout.Scaffold`: a `TitleBar` with `MinHeight` set to the window's title bar height, a `Start` sidebar with its `StartBar`, an optional `End` inspector and an optional `StatusBar`. Reach for `layout.SplitView` alone for panes inside a page and `layout.TitleBar` alone for a bar without panes.

Each pane's `Size`, `Collapsed` and `Open` live in window state, passed as pointers. Give every collapsible pane a `layout.PaneToggle` in the title bar with the same pointers and `Compact` mode: it is the built-in way to reopen a pane that has left the row in a compact window. Read `layout.Compact(c, mode)` when content adapts with the shell.

```go
layout.Scaffold(c, layout.ScaffoldOptions{
	TitleBar: layout.TitleBarOptions{
		MinHeight: titleBarHeight, Title: "Inbox",
		Leading: func() {
			layout.PaneToggle(c, layout.PaneToggleOptions{Collapsed: &m.collapsed, Open: &m.open})
		},
	},
	Start: &layout.PaneOptions{
		Label: "Sidebar", Size: &m.sidebar, Collapsed: &m.collapsed, Open: &m.open,
		Body: func() { /* navigation */ },
	},
	StartBar: layout.TitleBarOptions{Title: "Mail"},
	Content:  func() { /* page */ },
})
```

Related skills: `mygoui-theme` for colors and sizes, `mygoui-design` for layout, feedback and testing, `mygoui-motion` for animation, `mygoui-effects` for shader effects.
