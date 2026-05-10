import { AuthApiError, getCurrentUser, type CurrentUser } from './auth.api'

let cachedUser: CurrentUser | null | undefined
let pendingUser: Promise<CurrentUser | null> | null = null

export function setCurrentUser(user: CurrentUser | null): void {
  cachedUser = user
  pendingUser = null
}

export async function loadCurrentUser(): Promise<CurrentUser | null> {
  if (cachedUser !== undefined) {
    return cachedUser
  }

  if (pendingUser) {
    return pendingUser
  }

  pendingUser = getCurrentUser()
    .then((user) => {
      cachedUser = user
      return user
    })
    .catch((error: unknown) => {
      if (error instanceof AuthApiError) {
        cachedUser = null
        return null
      }
      throw error
    })
    .finally(() => {
      pendingUser = null
    })

  return pendingUser
}
