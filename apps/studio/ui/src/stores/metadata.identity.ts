import type { MetadataEntity } from '../features/metadata/metadata.api'

export function findEntityByRouteSlug(entities: MetadataEntity[], slug: string): MetadataEntity | undefined {
  return entities.find((entity) => entity.slug !== null && entity.slug === slug)
}
