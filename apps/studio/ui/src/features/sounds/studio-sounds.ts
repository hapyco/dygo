export type StudioSoundName = 'save' | 'error' | 'delete'

export const studioSoundPaths: Record<StudioSoundName, string> = {
  save: '/sounds/save.mp3',
  error: '/sounds/error.mp3',
  delete: '/sounds/delete.mp3',
}

const storageKey = 'studio:sounds-enabled'

export function readStudioSoundEnabled(storedValue: string | null, prefersReducedMotion: boolean): boolean {
  if (prefersReducedMotion) {
    return false
  }

  return storedValue !== 'false'
}

export function isStudioSoundEnabled(): boolean {
  if (typeof window === 'undefined') {
    return false
  }

  try {
    return readStudioSoundEnabled(
      window.localStorage.getItem(storageKey),
      window.matchMedia('(prefers-reduced-motion: reduce)').matches,
    )
  } catch {
    return true
  }
}

let preferenceChanged: ((value: boolean) => void) | null = null

export function bindStudioSoundPreference(listener: typeof preferenceChanged) {
  preferenceChanged = listener
}

export function setStudioSoundEnabled(enabled: boolean, notify = true): void {
  if (notify) preferenceChanged?.(enabled)
  if (typeof window === 'undefined') {
    return
  }

  try {
    window.localStorage.setItem(storageKey, enabled ? 'true' : 'false')
  } catch {
    // Sound preferences are best-effort when browser storage is unavailable.
  }
}

function playConfiguredSound(name: StudioSoundName): void {
  try {
    if (!isStudioSoundEnabled() || typeof window === 'undefined' || typeof window.Audio !== 'function') {
      return
    }

    const audio = new window.Audio(studioSoundPaths[name])
    audio.preload = 'auto'
    void audio.play().catch(() => {
      // Browsers may block audio until a user gesture; ignore silently.
    })
  } catch {
    // Sound feedback is best-effort and must never change action semantics.
  }
}

export const studioSounds = {
  save: () => playConfiguredSound('save'),
  error: () => playConfiguredSound('error'),
  delete: () => playConfiguredSound('delete'),
} as const
