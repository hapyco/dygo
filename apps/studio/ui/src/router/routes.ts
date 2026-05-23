export const RouteName = {
  Login: 'login',
  Home: 'home',
  EntityRecords: 'entity-records',
  RecordNew: 'record-new',
  RecordDetail: 'record-detail',
  NotFound: 'not-found',
} as const

export type RouteNameValue = (typeof RouteName)[keyof typeof RouteName]

export const entityChildReservedSlugs = new Set(['new'])

export function isEntityChildReservedSlug(value: string): boolean {
  return entityChildReservedSlugs.has(normalizeSlug(value))
}

export function normalizeSlug(value: string): string {
  return value.trim().toLowerCase()
}

export function routeParam(value: string | string[]): string {
  return Array.isArray(value) ? (value[0] ?? '') : value
}
