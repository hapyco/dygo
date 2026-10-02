import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import assert from 'node:assert/strict'
import { test } from 'node:test'

const sourceRoot = join(import.meta.dirname, '..')
const designRoot = join(sourceRoot, 'design')

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry)
    if (statSync(path).isDirectory()) return sourceFiles(path)
    return /\.(ts|vue)$/.test(entry) ? [path] : []
  })
}

test('only the design system imports reka-ui', () => {
  const offenders = sourceFiles(sourceRoot)
    .filter((path) => !path.startsWith(designRoot))
    .filter((path) => /from\s+['"]reka-ui['"]/.test(readFileSync(path, 'utf8')))
    .map((path) => relative(sourceRoot, path))

  assert.deepEqual(offenders, [], `feature code must import design components instead of reka-ui: ${offenders.join(', ')}`)
})

test('design system exports the shared primitives', () => {
  const index = readFileSync(join(designRoot, 'index.ts'), 'utf8')
  for (const name of ['Popover', 'Dialog', 'ContextMenu', 'Combobox', 'Tree', 'DropdownMenu', 'DropdownMenuItem', 'Select', 'Switch', 'RadioGroup']) {
    assert.match(index, new RegExp(`export \\{ default as ${name} \\}`), `design/index.ts must export ${name}`)
  }
})
