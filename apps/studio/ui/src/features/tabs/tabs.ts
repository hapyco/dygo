import { RouteName, routeParam } from '../../router/routes.ts'
import { humanizeEntity } from '../../stores/metadata.identity.ts'

export type StudioTab = {
  path: string
  fullPath: string
  label: string
  cacheKey?: string
}

export type TabRouteInfo = {
  name?: string | symbol | null
  path: string
  fullPath: string
  params: Record<string, unknown>
}

let nextTabCacheKey = 0

export function normalizeStudioTabs(value: unknown): StudioTab[] {
  if (!Array.isArray(value)) return []

  const seen = new Set<string>()
  return value.flatMap((candidate): StudioTab[] => {
    if (!candidate || typeof candidate !== 'object') return []
    const item = candidate as Record<string, unknown>
    const path = normalizeTabPath(item.path)
    const fullPath = typeof item.fullPath === 'string' && item.fullPath.startsWith('/') ? item.fullPath : path
    const label = typeof item.label === 'string' ? item.label.trim() : ''
    if (!path || !label) return []
    if (seen.has(path)) return []
    seen.add(path)
    return [{ path, fullPath, label, cacheKey: validCacheKey(item.cacheKey) ?? createTabCacheKey(path) }]
  })
}

export function upsertStudioTab(tabs: StudioTab[], tab: StudioTab): StudioTab[] {
  const index = tabs.findIndex(item => item.path === tab.path)
  if (index === -1) return [...tabs, { ...tab, cacheKey: tab.cacheKey ?? createTabCacheKey(tab.path) }]
  return tabs.map((item, itemIndex) => itemIndex === index ? { ...item, fullPath: tab.fullPath, label: tab.label || item.label } : item)
}

export function morphStudioTab(tabs: StudioTab[], fromPath: string, tab: StudioTab): StudioTab[] {
  const fromIndex = tabs.findIndex(item => item.path === fromPath)
  const without = tabs.filter(item => item.path !== fromPath && item.path !== tab.path)
  const insertAt = fromIndex === -1 ? without.length : Math.min(fromIndex, without.length)
  return [...without.slice(0, insertAt), tab, ...without.slice(insertAt)]
}

export function closeStudioTab(tabs: StudioTab[], path: string, activePath: string): { tabs: StudioTab[], activate: StudioTab | null, closed: boolean } {
  if (tabs.length <= 1) return { tabs, activate: null, closed: false }
  const index = tabs.findIndex(item => item.path === path)
  if (index === -1) return { tabs, activate: null, closed: false }
  const next = tabs.filter(item => item.path !== path)
  if (path !== activePath) return { tabs: next, activate: null, closed: true }
  return { tabs: next, activate: next[index] ?? next[index - 1] ?? null, closed: true }
}

export function cycleStudioTab(tabs: StudioTab[], activePath: string, delta: number): StudioTab | null {
  if (tabs.length < 2) return null
  const index = tabs.findIndex(item => item.path === activePath)
  if (index === -1) return tabs[0] ?? null
  const nextIndex = ((index + delta) % tabs.length + tabs.length) % tabs.length
  return tabs[nextIndex] ?? null
}

export function shouldMorphNewRecordTab(fromPath: string, toPath: string): boolean {
  const from = fromPath.match(/^\/([^/]+)\/new$/)
  const to = toPath.match(/^\/([^/]+)\/([^/]+)$/)
  return Boolean(from && to && from[1] === to[1] && to[2] !== 'new')
}

export function tabLabelForRoute(route: TabRouteInfo, entityLabel: (slug: string) => string): string {
  const name = typeof route.name === 'string' ? route.name : ''
  if (name === RouteName.Home || route.path === '/') return 'Home'
  if (name === RouteName.NotFound) return 'Not found'
  const entity = paramValue(route.params.entity)
  if (name === RouteName.EntityRecords) return entityLabel(entity) || humanizeEntity(entity) || 'Records'
  if (name === RouteName.RecordNew) {
    const label = entityLabel(entity) || humanizeEntity(entity)
    return label ? `New ${label}` : 'New Record'
  }
  if (name === RouteName.RecordDetail) {
    const record = paramValue(route.params.recordName)
    const label = entityLabel(entity) || humanizeEntity(entity)
    return record ? `${label} / ${record}` : label || 'Record'
  }
  if (name.startsWith('page:')) {
    const key = name.slice(name.lastIndexOf(':') + 1)
    return humanizeEntity(key) || 'Page'
  }
  const leaf = route.path.replace(/\/+$/, '').split('/').at(-1) ?? ''
  return humanizeEntity(leaf) || 'Page'
}

export function tabCacheKey(tabs: StudioTab[], path: string, epoch: number): string {
  const tab = tabs.find(item => item.path === path)
  return `${tab?.cacheKey ?? path}::${epoch}`
}

function createTabCacheKey(path: string): string {
  nextTabCacheKey += 1
  return `${path}::tab-${nextTabCacheKey}`
}

function validCacheKey(value: unknown): string | null {
  return typeof value === 'string' && value.trim() !== '' ? value : null
}

function normalizeTabPath(value: unknown): string {
  if (typeof value !== 'string') return ''
  const path = value.trim()
  if (path === '' || !path.startsWith('/') || path.startsWith('//') || path.includes('?') || path.includes('#')) return ''
  return path.length > 1 ? path.replace(/\/+$/, '') : path
}

function paramValue(value: unknown): string {
  if (typeof value === 'string' || Array.isArray(value)) return routeParam(value)
  return ''
}
