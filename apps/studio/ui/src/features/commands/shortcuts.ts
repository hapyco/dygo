export type StudioCommand = {
  id: string
  label: string
  group?: string
  detail?: string
  keywords?: string
  shortcut?: string
  input?: 'allow' | 'ignore'
  consumeDisabled?: boolean
  disabled?: boolean
  disabledReason?: string
  run: () => void | Promise<void>
}

export const bindings: Record<string, Pick<StudioCommand, 'shortcut' | 'input' | 'consumeDisabled'>> = {
  'app:palette': { shortcut: 'k', input: 'allow' },
  'app:shortcuts': { shortcut: '/', input: 'allow' },
  'record:save': { shortcut: 's', input: 'allow', consumeDisabled: true },
  'records-new': { shortcut: 'Enter' },
  'app:sidebar': { shortcut: '\\' },
}
export const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)
export function commandBinding(command: StudioCommand) { return { ...bindings[command.id], ...command } }
export function shortcutLabel(key?: string, mac = isMac) { return key ? `${mac ? '⌘' : 'Ctrl+'}${key.length === 1 ? key.toUpperCase() : key}` : '' }
export function ariaShortcut(key?: string, mac = isMac) { return key ? `${mac ? 'Meta' : 'Control'}+${key}` : undefined }
export function unavailable(command: StudioCommand) { return !!(command.disabled || command.disabledReason) }

export function nextAvailableCommandId(
  items: StudioCommand[],
  currentId: string,
  direction: 1 | -1,
): string {
  const available = items.filter((item) => !unavailable(item))
  if (available.length === 0) return ''
  const currentIndex = available.findIndex((item) => item.id === currentId)
  if (currentIndex === -1) {
    return available[direction === 1 ? 0 : available.length - 1]!.id
  }
  return available[(currentIndex + direction + available.length) % available.length]!.id
}

// One execution gate covers palette, shortcut and button calls, including the
// interval before reactive mutation state has updated.
const pending = new Set<string>()
export async function executeCommand(command: StudioCommand) {
  if (unavailable(command) || pending.has(command.id)) return
  pending.add(command.id)
  try { await command.run() } finally { pending.delete(command.id) }
}

export function bindingConflicts(commands: StudioCommand[]) {
  const byKey = new Map<string, string[]>()
  for (const command of commands.map(commandBinding)) {
    if (!command.shortcut) continue
    const key = command.shortcut.toLowerCase()
    byKey.set(key, [...(byKey.get(key) ?? []), command.id])
  }
  return [...byKey.entries()].filter(([, ids]) => ids.length > 1)
}

export function matchShortcut(event: Pick<KeyboardEvent, 'key' | 'metaKey' | 'ctrlKey' | 'shiftKey' | 'altKey' | 'repeat' | 'isComposing' | 'defaultPrevented' | 'getModifierState'>, commands: StudioCommand[], editable: boolean, overlay: boolean, mac = isMac) {
  if (event.defaultPrevented || event.repeat || event.isComposing || event.getModifierState('AltGraph') || overlay) return
  if (event.altKey || event.shiftKey || event.metaKey !== mac || event.ctrlKey === mac) return
  const matches = commands.map(commandBinding).filter(command => command.shortcut?.toLowerCase() === event.key.toLowerCase())
  if (matches.length !== 1) return
  const command = matches[0]!
  if (editable && command.input !== 'allow') return
  return command
}
