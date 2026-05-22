# Studio

Studio is dygo's main operational and builder UI.

It is where operators run the business, builders configure the system, and agents help implement the system.

Studio is a first-party dygo app, not a temporary admin panel. It should feel like one coherent product across records, lists, forms, saved views, jobs, audit logs, settings, and spaces.

The framework repo includes the Studio app manifest at `apps/studio/app.yml` and the first Vue/Vite frontend scaffold under `apps/studio/ui`. The current scaffold includes login, protected record routes, root-mounted page placeholders, and Dygo UI foundation components. Shell, stores, metadata renderers, and final page types are still being built.

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
- jobs UI
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
/:entity/:id
```

`/login` is public. The other routes require a valid Studio session.

The `:entity` segment is the Entity route slug, not the app name plus Entity name. It defaults to Entity `name` and can be set with `route.slug` when two apps would otherwise collide.

Route slug conflicts and reserved root slugs fail validation. dygo does not append numeric suffixes such as `contact-2`, because those URLs are unstable and unclear.

Entity navigation icons come from optional Entity metadata `icon` values. Studio resolves Lucide names such as `box` or `shield-check`; missing or unknown names fall back to `box`.

Record IDs are numeric in v1. Activity is shown inside the Record page instead of using a separate Studio URL.

## Design Rule

Business apps do not ship custom UI by default.

The default path is:

1. Define Entities, Fields, Permissions, Hooks, Fixtures, and Patches in an app.
2. Install the app.
3. Let Studio render the app globally.

Custom UI can come later, but the basic app shape should work through Studio metadata first.
