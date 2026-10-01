import type { DropdownMenuItemModel } from '@/design/types'
import { RouteName } from '../../router/routes.ts'
import type { Router } from 'vue-router'

export type RecordRowContextMenuKey = 'open' | 'copy-name' | 'copy-link'

export function recordRowContextMenuItems(row: Record<string, unknown>): DropdownMenuItemModel[] {
  const name = row.name
  const hasName = typeof name === 'string' && name.length > 0
  return [
    { type: 'item', key: 'open', label: 'Open', disabled: !hasName },
    { type: 'separator', key: 'menu-separator' },
    { type: 'item', key: 'copy-name', label: 'Copy Record name', disabled: !hasName },
    { type: 'item', key: 'copy-link', label: 'Copy link', disabled: !hasName },
  ]
}

export function recordDetailHref(router: Router, entitySlug: string, recordName: string): string {
  const path = router.resolve({
    name: RouteName.RecordDetail,
    params: { entity: entitySlug, recordName },
  }).href
  if (typeof window === 'undefined') {
    return path
  }
  return new URL(path, window.location.origin).href
}

export async function copyText(value: string): Promise<boolean> {
  if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
    return false
  }
  try {
    await navigator.clipboard.writeText(value)
    return true
  } catch {
    return false
  }
}
