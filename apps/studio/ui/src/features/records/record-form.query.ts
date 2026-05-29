import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useMutation, useQuery } from '@tanstack/vue-query'

import { queryClient } from '@/app/query'
import { recordListBaseQueryKey } from './record-list.query'
import {
  createRecord,
  deleteRecord,
  getRecordByName,
  getSingleRecord,
  updateRecord,
  updateSingleRecord,
  type RecordData,
} from './records.api'

type QueryToggle = MaybeRefOrGetter<boolean>

type CreateRecordVariables = {
  entity: string
  data: RecordData
}

type UpdateRecordVariables = {
  entity: string
  recordName: string
  id: string | number
  data: RecordData
}

type UpdateSingleRecordVariables = {
  entity: string
  data: RecordData
}

type DeleteRecordVariables = {
  entity: string
  recordName: string
  id: string | number
}

export function recordByNameQueryKey(entity: string, recordName: string) {
  return ['records', 'detail', entity, recordName] as const
}

export function singleRecordQueryKey(entity: string) {
  return ['records', 'single', entity] as const
}

export function useRecordByNameQuery(
  entity: MaybeRefOrGetter<string>,
  recordName: MaybeRefOrGetter<string>,
  options: { enabled?: QueryToggle } = {},
) {
  const currentEntity = computed(() => toValue(entity).trim())
  const currentRecordName = computed(() => toValue(recordName).trim())

  return useQuery({
    queryKey: computed(() => recordByNameQueryKey(currentEntity.value, currentRecordName.value)),
    queryFn: ({ signal }) => getRecordByName(currentEntity.value, currentRecordName.value, { signal }),
    enabled: computed(() => (
      currentEntity.value !== ''
      && currentRecordName.value !== ''
      && toValue(options.enabled ?? true)
    )),
  })
}

export function useSingleRecordQuery(
  entity: MaybeRefOrGetter<string>,
  options: { enabled?: QueryToggle } = {},
) {
  const currentEntity = computed(() => toValue(entity).trim())

  return useQuery({
    queryKey: computed(() => singleRecordQueryKey(currentEntity.value)),
    queryFn: ({ signal }) => getSingleRecord(currentEntity.value, { signal }),
    enabled: computed(() => currentEntity.value !== '' && toValue(options.enabled ?? true)),
  })
}

export function useCreateRecordMutation() {
  return useMutation({
    mutationFn: ({ entity, data }: CreateRecordVariables) => createRecord(entity, data),
    onSuccess: (record, variables) => {
      cacheNamedRecord(variables.entity, record)
      invalidateRecordLists(variables.entity)
    },
  })
}

export function useUpdateRecordMutation() {
  return useMutation({
    mutationFn: ({ entity, id, data }: UpdateRecordVariables) => updateRecord(entity, id, data),
    onSuccess: (record, variables) => {
      queryClient.setQueryData(recordByNameQueryKey(variables.entity, variables.recordName), record)
      cacheNamedRecord(variables.entity, record)
      invalidateRecordLists(variables.entity)
    },
  })
}

export function useUpdateSingleRecordMutation() {
  return useMutation({
    mutationFn: ({ entity, data }: UpdateSingleRecordVariables) => updateSingleRecord(entity, data),
    onSuccess: (record, variables) => {
      queryClient.setQueryData(singleRecordQueryKey(variables.entity), record)
      cacheNamedRecord(variables.entity, record)
      invalidateRecordLists(variables.entity)
    },
  })
}

export function useDeleteRecordMutation() {
  return useMutation({
    mutationFn: ({ entity, id }: DeleteRecordVariables) => deleteRecord(entity, id),
    onSuccess: (_result, variables) => {
      queryClient.removeQueries({
        queryKey: recordByNameQueryKey(variables.entity, variables.recordName),
        exact: true,
      })
      invalidateRecordLists(variables.entity)
    },
  })
}

function cacheNamedRecord(entity: string, record: RecordData) {
  if (typeof record.name !== 'string' || record.name.length === 0) {
    return
  }

  queryClient.setQueryData(recordByNameQueryKey(entity, record.name), record)
}

function invalidateRecordLists(entity: string) {
  void queryClient.invalidateQueries({ queryKey: recordListBaseQueryKey(entity) })
}
