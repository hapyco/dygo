import test from 'node:test'
import assert from 'node:assert/strict'

import type { MetadataEntity, MetadataEntityMeta } from '../features/metadata/metadata.api'
import { findEntityByRouteSlug, metadataCacheSlugs } from './metadata.identity.ts'

test('findEntityByRouteSlug uses slug only', () => {
  const customer = metadataEntity({ key: 'customer', slug: 'crm-customer' })
  const entities = [
    metadataEntity({ key: 'role', slug: 'role' }),
    customer,
  ]

  assert.equal(findEntityByRouteSlug(entities, 'crm-customer'), customer)
  assert.equal(findEntityByRouteSlug(entities, 'customer'), undefined)
})

test('metadataCacheSlugs stores requested slug and canonical slug once', () => {
  const meta = metadataEntity({ key: 'customer', slug: 'crm-customer' }) as MetadataEntityMeta

  assert.deepEqual(metadataCacheSlugs(meta, 'crm-customer'), ['crm-customer'])
  assert.deepEqual(metadataCacheSlugs(meta, 'old-customer'), ['old-customer', 'crm-customer'])
})

function metadataEntity(overrides: Partial<MetadataEntity>): MetadataEntity {
  return {
    name: `core.${overrides.key ?? 'entity'}`,
    key: overrides.key ?? 'entity',
    slug: overrides.slug ?? overrides.key ?? 'entity',
    label: overrides.label ?? 'Entity',
    description: '',
    icon: 'box',
    'is-single': false,
    'is-collection': false,
    app: { name: 'core', label: 'Core' },
  }
}
