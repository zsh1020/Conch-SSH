import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { GetSessions, SaveSession, DeleteSession, GetGroups, SaveGroupOrder, RenameGroup, DeleteGroup, ReorderSessions } from '../../wailsjs/go/main/App'
import type { store } from '../../wailsjs/go/models'

export type Session = store.Session

export const useSessionsStore = defineStore('sessions', () => {
  const sessions = ref<Session[]>([])
  const groups = ref<string[]>([])

  const sessionsByGroup = computed(() => {
    const map: Record<string, Session[]> = {}
    for (const s of sessions.value) {
      const g = s.group || '默认'
      if (!map[g]) map[g] = []
      map[g].push(s)
    }
    return map
  })

  // 最近使用的5条（按 last_used 降序，排除从未使用）
  const recentSessions = computed(() => {
    return [...sessions.value]
      .filter(s => s.last_used && s.last_used !== '0001-01-01T00:00:00Z')
      .sort((a, b) => {
        const ta = new Date(a.last_used || 0).getTime()
        const tb = new Date(b.last_used || 0).getTime()
        return tb - ta
      })
      .slice(0, 5)
  })

  // 收藏的会话
  const starredSessions = computed(() => sessions.value.filter(s => s.starred))

  // 所有已使用的 tags
  const allTags = computed(() => {
    const set = new Set<string>()
    for (const s of sessions.value) {
      if (s.tags) s.tags.forEach(t => t && set.add(t))
    }
    return [...set].sort()
  })

  async function fetchAll() {
    try {
      sessions.value = (await GetSessions()) ?? []
      groups.value = (await GetGroups()) ?? []
    } catch (e) {
      console.error('fetchAll:', e)
    }
  }

  async function save(sess: Session) {
    await SaveSession(sess)
    await fetchAll()
  }

  async function remove(id: string) {
    await DeleteSession(id)
    sessions.value = sessions.value.filter(s => s.id !== id)
    groups.value = [...new Set(sessions.value.map(s => s.group || '默认'))]
  }

  function getById(id: string) {
    return sessions.value.find(s => s.id === id)
  }

  async function toggleStar(id: string) {
    const sess = sessions.value.find(s => s.id === id)
    if (!sess) return
    await save({ ...sess, starred: !sess.starred })
  }

  async function saveGroupOrder(order: string[]) {
    await SaveGroupOrder(order)
    groups.value = [...order]
  }

  async function renameGroup(oldName: string, newName: string) {
    await RenameGroup(oldName, newName)
    await fetchAll()
  }

  async function deleteGroup(name: string) {
    await DeleteGroup(name)
    await fetchAll()
  }

  async function reorderSessions(ids: string[]) {
    await ReorderSessions(ids)
    // 本地立即按 ids 重排，避免等待 fetchAll 的闪烁
    const idx = new Map(sessions.value.map(s => [s.id, s]))
    const ordered: Session[] = []
    const seen = new Set<string>()
    for (const id of ids) {
      const s = idx.get(id)
      if (s) { ordered.push(s); seen.add(id) }
    }
    for (const s of sessions.value) {
      if (!seen.has(s.id)) ordered.push(s)
    }
    sessions.value = ordered
  }

  return {
    sessions, groups,
    sessionsByGroup, recentSessions, starredSessions, allTags,
    fetchAll, save, remove, getById, toggleStar,
    saveGroupOrder, renameGroup, deleteGroup, reorderSessions,
  }
})
