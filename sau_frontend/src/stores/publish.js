import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// ═══════════════════════════════════════
// Publish Store — publish tasks state
// ═══════════════════════════════════════

export const usePublishStore = defineStore('publish', () => {
  const tasks = ref([])

  const setTasks = (list) => {
    tasks.value = list
  }

  const recentTasks = computed(() => {
    return [...tasks.value].slice(0, 5)
  })

  const taskStats = computed(() => {
    const all = tasks.value
    const byStatus = {}
    all.forEach((t) => {
      const s = t.status || 'unknown'
      byStatus[s] = (byStatus[s] || 0) + 1
    })
    return { total: all.length, byStatus }
  })

  return {
    tasks,
    recentTasks,
    taskStats,
    setTasks,
  }
})
