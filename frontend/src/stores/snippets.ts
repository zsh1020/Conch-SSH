import { defineStore } from 'pinia'
import { ref } from 'vue'
import { GetSnippets, SaveSnippet, DeleteSnippet } from '../../wailsjs/go/main/App'
import type { store } from '../../wailsjs/go/models'

export type Snippet = store.Snippet

export const useSnippetsStore = defineStore('snippets', () => {
  const snippets = ref<Snippet[]>([])

  async function fetchAll() {
    try {
      snippets.value = await GetSnippets()
    } catch (e) {
      console.error('load snippets:', e)
    }
  }

  async function save(sn: Snippet) {
    await SaveSnippet(sn)
    await fetchAll()
  }

  async function remove(id: string) {
    await DeleteSnippet(id)
    await fetchAll()
  }

  /** Snippets grouped by group name */
  function byGroup(): Record<string, Snippet[]> {
    const groups: Record<string, Snippet[]> = {}
    for (const sn of snippets.value) {
      const g = sn.group || '默认'
      if (!groups[g]) groups[g] = []
      groups[g].push(sn)
    }
    return groups
  }

  return { snippets, fetchAll, save, remove, byGroup }
})
