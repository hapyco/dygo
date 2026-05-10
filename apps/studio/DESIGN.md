# Design

## Overview

Studio is a product UI for operating and building business systems. The visual system should be restrained, durable, and precise. The goal is not to impress on first glance; it is to stay trustworthy after thousands of repeated actions.

The design system should be owned by dygo. Reka UI may provide unstyled accessible primitives, but feature code should import Studio design components rather than importing Reka directly.

## Theme

Light-first.

Scene: a builder or operator is working on a laptop or desktop during the workday, moving between metadata, Records, permissions, and Activity while making careful business changes.

Dark mode can come later as a full token mode, not as the default visual identity.

## Color

Use OKLCH tokens. Avoid raw `#000` and `#fff`; neutrals should be lightly tinted.

Color strategy: restrained.

Accent color is reserved for:

- primary actions
- current navigation state
- selected table rows or active tabs
- focus rings
- important state markers

Semantic colors must exist for success, warning, danger, info, and neutral states. Semantic states should use text, icon, or shape in addition to color.

Suggested token roles:

```css
--studio-bg
--studio-surface
--studio-surface-raised
--studio-sidebar
--studio-border
--studio-border-strong
--studio-text
--studio-text-muted
--studio-text-subtle
--studio-accent
--studio-accent-strong
--studio-focus
--studio-danger
--studio-warning
--studio-success
--studio-info
```

## Typography

Use a product UI sans stack:

```css
font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
```

Use fixed rem sizes, not viewport-scaled type.

Suggested scale:

```txt
12px  metadata, captions, table hints
13px  dense labels, sidebar items, table cells
14px  default body and controls
16px  section headings
20px  Page titles
24px  rare high-level titles
```

Avoid display fonts in controls, tables, labels, and metadata surfaces.

## Layout Model

Studio uses a persistent Shell:

```txt
Studio
  Shell
    Sidebar
    Top Bar
    Page Outlet
      Page
```

The Shell owns:

- sidebar
- top bar
- command menu
- user menu
- global search entry
- route/page outlet
- authenticated/forbidden/loading shell states

Pages render inside the Page outlet.

## Product Vocabulary

Use these UI nouns consistently:

```txt
Shell
Page
Page Type
Space
View
Layout
Renderer
```

Definitions:

- Shell: persistent Studio frame.
- Page: route-level surface inside Studio.
- Page Type: rendering style and behavior used by a Page.
- Space: a Page Type for grouped navigation and work.
- View: data presentation mode for Entity and Record data.
- Layout: arrangement of sections and fields inside a Page or View.
- Renderer: internal component that turns metadata into UI.

## Standard Page Types

Studio should deliver these Page Types before business apps need custom UI:

```txt
space
list
form
report
dashboard
activity
settings
custom
```

Standard Page Types should be implemented as generic renderers, not one-off files for every Entity.

## Component Architecture

Use atomic design with dygo-owned component APIs:

```txt
design/
  tokens/
  primitives/
  atoms/
  molecules/
  organisms/
```

Primitives wrap behavior libraries:

```txt
Dialog
DropdownMenu
Select
Tabs
Tooltip
Popover
CommandMenu
```

Atoms:

```txt
Button
IconButton
Input
Label
Textarea
Checkbox
Badge
Spinner
```

Molecules:

```txt
Field
FormSection
Toolbar
EmptyState
ErrorState
SearchBox
DataCell
```

Organisms:

```txt
Shell
Sidebar
TopBar
EntityTable
RecordForm
ActivityTimeline
```

Feature code should use dygo design components. It should not import Reka directly unless a new design primitive is being built.

## Interaction States

Every reusable interactive component should define:

- default
- hover
- focus
- active
- selected
- disabled
- loading
- error
- readonly where applicable

Pages should define:

- loading
- empty
- forbidden
- not found
- invalid metadata
- schema not ready
- network/server error

## Motion

Use motion only for state change, reveal, feedback, and focus orientation.

Default duration: 150ms to 220ms.

Use ease-out curves. Avoid decorative page-load choreography.

Respect `prefers-reduced-motion`.

## Bans

Do not use:

- gradient text
- decorative gradient blobs
- glassmorphism as a default surface
- nested cards
- hero-style dashboard metrics
- side-stripe accent borders
- custom controls where standard accessible primitives exist
- page layouts that hide the Shell or break route orientation without a deliberate full-screen mode

## First Implementation Target

The first Studio UI task should build:

- Vue + Vite app scaffold under `apps/studio/ui`
- Studio Shell skeleton
- design tokens
- first atoms and molecules
- login page
- authenticated start page
- clear forbidden state

The visual result should be minimal but structurally correct.
