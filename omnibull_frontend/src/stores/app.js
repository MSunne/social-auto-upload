import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// ═══════════════════════════════════════
// App Store — global state
// ═══════════════════════════════════════

const VIDEO_EXTS = ['.mp4', '.avi', '.mov', '.wmv', '.flv', '.mkv', '.webm']
const IMAGE_EXTS = ['.jpg', '.jpeg', '.png', '.gif', '.bmp', '.webp']

// Keep file-kind classification in one place so all material views count items consistently.
function classifyFile(filename) {
  const lower = filename.toLowerCase()
  if (VIDEO_EXTS.some((ext) => lower.endsWith(ext))) return 'video'
  if (IMAGE_EXTS.some((ext) => lower.endsWith(ext))) return 'image'
  return 'other'
}

export const useAppStore = defineStore('app', () => {
  const materials = ref([])
  const sidebarCollapsed = ref(false)

  // Replace the material cache after a full backend refresh.
  const setMaterials = (list) => {
    materials.value = list
  }

  // Append a single material after an optimistic create/upload flow.
  const addMaterial = (m) => {
    materials.value.push(m)
  }

  // Remove a material from the local cache once deletion succeeds server-side.
  const removeMaterial = (id) => {
    const idx = materials.value.findIndex((m) => m.id === id)
    if (idx > -1) materials.value.splice(idx, 1)
  }

  // Toggle the left navigation width without coupling views to layout internals.
  const toggleSidebar = () => {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  // ── Computed material stats ──
  const materialStats = computed(() => {
    const all = materials.value
    const videos = all.filter((m) => classifyFile(m.filename) === 'video').length
    const images = all.filter((m) => classifyFile(m.filename) === 'image').length
    return {
      total: all.length,
      videos,
      images,
      others: all.length - videos - images,
    }
  })

  const recentMaterials = computed(() => {
    return [...materials.value]
      .sort((a, b) => new Date(b.upload_time) - new Date(a.upload_time))
      .slice(0, 5)
  })

  return {
    materials,
    sidebarCollapsed,
    materialStats,
    recentMaterials,
    setMaterials,
    addMaterial,
    removeMaterial,
    toggleSidebar,
    classifyFile,
  }
})
