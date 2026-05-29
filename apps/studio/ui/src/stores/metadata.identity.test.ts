import test from 'node:test'
import assert from 'node:assert/strict'

import type { MetadataEntity } from '../features/metadata/metadata.api'
import { findEntityByRouteSlug } from './metadata.identity.ts'

test('findEntityByRouteSlug uses slug only', () => {
  const customer = metadataEntity({ key: 'customer', slug: 'crm-customer' })
  const entities = [
    metadataEntity({ key: 'role', slug: 'role' }),
    customer,
  ]

  assert.equal(findEntityByRouteSlug(entities, 'crm-customer'), customer)
  assert.equal(findEntityByRouteSlug(entities, 'customer'), undefined)
})

test('findEntityByRouteSlug ignores collection entities without slugs', () => {
  const collection = metadataEntity({ key: 'invoice-item', slug: null, 'is-collection': true })

  assert.equal(findEntityByRouteSlug([collection], 'invoice-item'), undefined)
})

function metadataEntity(overrides: Partial<MetadataEntity>): MetadataEntity {
  return {
    name: `core.${overrides.key ?? 'entity'}`,
    key: overrides.key ?? 'entity',
    slug: overrides.slug === undefined ? (overrides.key ?? 'entity') : overrides.slug,
    label: overrides.label ?? 'Entity',
    description: '',
    icon: 'box',
    'is-single': false,
    'is-system': false,
    'is-collection': false,
    app: { name: 'core', label: 'Core' },
  }
}
