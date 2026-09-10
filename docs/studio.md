# Studio

Studio is dygo's main operational and builder UI.

It is where operators run the business, builders configure the system, and agents help implement the system.

Studio is a first-party dygo app, not a temporary admin panel. It should feel like one coherent product across records, lists, forms, saved views, audit logs, settings, and spaces. Job Execution cancel and retry use the shared Record list and form Entity action controls. Specialized Job and Schedule operation screens are still coming soon.

The framework repo includes the Studio app manifest at `apps/studio/app.yml` and the Vue/Vite frontend under `apps/studio/ui`.

Release builds bundle the built Studio UI into the dygo binary. Generated projects cache those assets under `.dygo/apps/studio/ui/dist` so the project-local `cmd/dygo` runner can serve Studio without requiring the framework source checkout. `dygo upgrade` refreshes that cache when the running dygo binary includes newer bundled Studio assets.

## Mental Model

The Studio contains Spaces.

A Space organizes work around a business function.

The Studio globally renders Entities and Records.

Business apps provide metadata and behavior. Studio turns that metadata into usable product surfaces.

## Responsibilities

Studio owns:

- shell
- sidebar and navigation
- header
- command menu
- Spaces UI
- global list renderer
- global record renderer
- global form renderer
- field renderers
- collection renderer
- saved views UI
- Job Execution cancel and retry on Record list and form actions
- audit log UI
- settings UI
- metadata API client
- frontend stores

## Route Model

Studio is root-mounted by default. Global Studio pages live at root paths; metadata-backed Records also live at root paths through globally unique route slugs.

Current routes:

```txt
/login
/
/:entity
/:entity/new
/:entity/:name
```

`/login` is public. The other routes require a valid Studio session. `/` resolves through the boot default `home` route when configured.

The `:entity` segment is the Entity slug, not the app name plus Entity key. It defaults to the Entity key and can be set with `route.slug` when two apps would otherwise collide.

The `:name` segment is the Record system `name`.

Route slug conflicts and reserved root slugs fail validation. dygo does not append numeric suffixes such as `contact-2`, because those URLs are unstable and unclear.

Entity navigation icons come from optional Entity metadata `icon` values. Studio resolves Lucide names such as `box` or `shield-check`; missing or unknown names fall back to `box`.

Activity is shown inside the Record page instead of using a separate Studio URL.

## Design Rule

Business apps do not ship custom UI by default.

The default path is:

1. Define Entities, Fields, Permissions, Hooks, Fixtures, and Patches in an app.
2. Install the app.
3. Let Studio render the app globally.

Custom UI is coming soon, but the basic app shape should work through Studio metadata first.

## Appearance

Studio uses Light, Dark, or System appearance.

Open the user menu in the header and choose Theme.

Light and Dark stay fixed. System follows the operating system color scheme.

Studio saves signed-in preferences in the Studio Preference Entity. Theme, sounds, sidebar state, recent pages, page size, and hidden columns follow the user across sessions. The browser retains the theme for the login screen. Existing browser settings are imported only when the server has no value for that key.

Pin an Entity, Page, or saved Record from its header to add it to the personal Pinned section above the main navigation. Pinned items follow the signed-in user. The section shows five items until See more is selected, and supports drag or keyboard reordering.

## Page Tabs

Studio keeps open Page Sheets in a tab strip above the current sheet.

Each tab is one route path. Entity lists, Records, New Record forms, reports, custom Pages, and other route-backed Page Types each get their own tab. Opening a path that is already open activates that tab. Query state such as list filters stays on that tab.

The browser URL always matches the active tab. Back and Forward activate an open tab when that path is still open, or open a new tab.

Switching tabs keeps Record drafts on the hidden sheet. Closing a dirty tab asks for confirmation. The last tab stays open. Creating a Record replaces the New Record tab with the saved Record tab.

Open tabs belong to the current browser tab. Reload restores them in that browser tab. They do not follow the signed-in user across other browser tabs or devices.

Tabs are horizontally scrollable when they exceed the available width. Tabs are not pinned: Pinned navigation is a separate persistent sidebar shortcut. Dirty state currently applies to Record forms; other Page Types need their own draft contract before Studio can mark them dirty.

Close tab, Next tab, and Previous tab are available from the command palette. They do not bind Mod+W, so the browser can still close its own tab.

## Record List Filters

Use Add filter to search available Field labels and names. Value controls follow Field metadata: options, booleans, dates, datetimes, numbers, and Links. Link choices respect Record permissions and dependent filters. Clear all removes filters and ID search while keeping sort and display choices.

Saved filters are private and belong to one Entity. Save the applied filters, rename them, replace them with the current filters, or delete them. Applying a saved filter replaces the current filters. Sort and display choices are not part of a saved filter. If metadata makes a saved filter invalid, replace or delete it; Studio does not silently remove its conditions.

Filter and sort state stays in the URL. A fresh Entity visit starts unfiltered. Preference keys use App namespaces and canonical App/Entity identity for per-Entity display choices.

## Record Views

The Record view selector shows List for normal Entities and List/Tree for Tree Entities. List is the default. Studio saves the selected view per user and Entity. Switching views keeps URL filters and sort, but resets selection and pagination. Grid, calendar, and Gantt views are not implemented.

Tree view loads roots first and children when expanded. Use the chevron or Left/Right arrow keys to collapse or expand. Use Up/Down to move focus. Click a Record label or press Enter to open the existing Record page. Create and move Records through the normal form and parent Link field. Siblings default to Record-name order; an explicit sort applies within each sibling group.

Filtered trees show matching Records with readable ancestors marked as context. Context nodes do not have to match the filter. An inaccessible path is marked unavailable; hidden ancestor details are not exposed. Each sibling group and matching result set has its own Load more control.

## Command Menu

Use Command K on macOS or Control K elsewhere. The menu shows current-page actions and recent pages before typing. It also searches navigation and app actions.

Choose Search records, select an Entity, and type part of a Record ID. On an Entity page, that Entity is selected first. Results respect Record permissions and are limited to 20. Use the arrow keys and Enter to open a result. Current list actions include New Record, Clear filters, and Apply saved filter.

## Keyboard Shortcuts

`Mod` means Command on macOS and Control on Windows or Linux.

| Action | Shortcut | Scope |
| --- | --- | --- |
| Open or close command palette | Mod+K | Signed-in Studio; close from the palette |
| Keyboard shortcuts | Mod+/ | Signed-in Studio |
| Save or create Record | Mod+S | Current Record form, including input fields |
| New Record | Mod+Enter | Entity list, outside input fields |
| Toggle sidebar | Mod+\\ | When sidebar collapse is available |

Open Keyboard shortcuts from the user menu or command palette to search current commands and disabled reasons. Record search, ID-search focus, Add filter, Reset changes, Home, Entity-list navigation, Close tab, Next tab, and Previous tab are available from the palette without extra key bindings.

Studio matches exact modifiers and character keys. It ignores repeat, composition, AltGraph, and handled events. Dialogs, menus, and popovers keep keyboard control. Save is reserved on Record forms even when disabled, so it does not open the browser Save Page dialog. Browser Back, Forward, Find, Reload, Location, and New Window shortcuts are unchanged.

Reset requires confirmation. Closing a dirty tab requires confirmation. Switching tabs keeps the draft on the hidden Page Sheet. Failed saves and background refreshes preserve the draft. Navigation does not save automatically.

### Internal registration

Use `features/commands/context.ts` to register reactive current-page commands with `usePageCommands`. Supply a stable ID, label, optional group, disabled reason, and handler. The active Page Sheet owns the current-page commands. Hidden sheets unregister while they are in the background; a later activation restores them. Disposal removes that page's commands.

Keep default keys and input policy in `features/commands/shortcuts.ts`. All bindings require Mod. The shell installs one listener. `runStudioCommand` and `executeCommand` share eligibility and in-flight checks for buttons, palette selection, and shortcuts. Focus actions execute after the palette closes and restores focus. Help and key badges use the same bindings.

Duplicate active bindings fail in development. Production reports the conflict and does not execute an ambiguous binding. This registry is internal Studio code, not a Business App or Go SDK contract.
