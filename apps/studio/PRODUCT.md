# Product

## Register

product

## Users

Studio is used by three overlapping groups:

- Operators who run business workflows, inspect Records, follow Activity, and act on daily work.
- Builders who configure Entities, Fields, Permissions, Spaces, Pages, Views, Reports, Dashboards, Fixtures, and Patches.
- Coding agents that help implement or maintain business systems by following explicit metadata and UI conventions.

Users are usually in a task-heavy work context. They need to scan, compare, edit, recover, validate, and understand business data without losing orientation.

## Product Purpose

Studio is dygo's first-party operational and builder UI. It turns installed App metadata into usable product surfaces while keeping a consistent Shell, navigation model, permission behavior, and design language.

Studio should make the standard path feel complete before a business app writes custom UI. A builder should be able to define an App, sync metadata, apply fixtures, and see the result in Studio through delivered Page Types such as Space, List, Form, Report, Dashboard, Settings, and Activity.

Custom Pages are supported as an escape hatch, but they should still run inside the Studio Shell and reuse Studio design components.

## Brand Personality

Calm, exact, capable.

Studio should feel like a serious product for business systems, not an admin template, marketing dashboard, or prototype surface. It should be direct, durable, and quiet enough for repeated daily use.

## Anti-references

- Marketing-style SaaS dashboards with oversized hero metrics and decorative card grids.
- CRUD scaffolds where every screen feels like a generated table pasted into a template.
- Dark-blue enterprise dashboards used as a category reflex.
- Glassy, blurred, gradient-heavy interfaces.
- Custom controls that ignore expected keyboard, focus, and accessibility behavior.
- Dense ERP screens that expose every internal concept at once.

## Design Principles

1. Studio is the product surface.
   It is not a temporary admin panel. It should feel coherent across login, Spaces, Pages, Views, Records, Activity, permissions, jobs, and settings.

2. Metadata should become designed UI.
   Generated Pages and Views should look intentional. Metadata-driven does not mean visually raw.

3. The Shell stays stable.
   Sidebar, top bar, command menu, user menu, and the Page outlet provide orientation while Pages change inside them.

4. Custom UI follows the same contract.
   Apps may define custom Pages, but those Pages should use Studio design components and keep the Studio Shell.

5. Permissions and state are visible.
   Empty, loading, denied, invalid, stale, failed, and read-only states should be explicit. Hidden failure creates mistrust.

6. Agents should be able to extend it safely.
   File structure, component names, tokens, Page Types, and route conventions should be predictable enough for coding agents to follow without inventing local patterns.

## Accessibility & Inclusion

Studio should target WCAG 2.2 AA as the baseline.

Core interaction rules:

- Every interactive element has visible focus.
- Keyboard navigation must work for Shell navigation, menus, dialogs, forms, and tables.
- Reka UI should be used for behavior-heavy primitives such as Dialog, Dropdown, Select, Tabs, Tooltip, Popover, and focus-managed menus.
- Motion must respect reduced-motion preferences.
- Color cannot be the only indicator for status, validation, selection, or permission state.
- Form errors must be attached to the relevant field and summarized when useful.
