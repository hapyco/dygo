import type { MetadataEntity, MetadataEntityMeta } from '../features/metadata/metadata.api'

export function findEntityByRouteSlug(entities: MetadataEntity[], slug: string): MetadataEntity | undefined {
  return entities.find((entity) => entity.slug === slug)
}

export function metadataCacheSlugs(meta: MetadataEntityMeta, requestedSlug?: string): string[] {
  return Array.from(new Set([requestedSlug, meta.slug].filter(Boolean) as string[]))
}
