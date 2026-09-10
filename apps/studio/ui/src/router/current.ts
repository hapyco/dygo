export function studioPathsEqual(left: string, right: string): boolean {
  return decodePath(left) === decodePath(right)
}

export function studioPathIsEntity(path: string, slug: string): boolean {
  return pathIsUnder(path, `/${slug}`)
}

export function studioPinIsCurrent(type: 'entity' | 'page' | 'record', pinPath: string | null, routePath: string): boolean {
  if (!pinPath) return false
  if (type === 'entity') return pathIsUnder(routePath, pinPath)
  return studioPathsEqual(pinPath, routePath)
}

function pathIsUnder(path: string, prefix: string): boolean {
  const current = decodePath(path)
  const base = decodePath(prefix)
  if (base === '/') return current === '/'
  return current === base || current.startsWith(`${base}/`)
}

function decodePath(path: string): string {
  try {
    return decodeURI(path)
  } catch {
    return path
  }
}
