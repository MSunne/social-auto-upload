<template>
  <div class="material-management fade-in">
    <!-- ═══ Stats & Toolbar ═══ -->
    <div class="toolbar glass-card">
      <div class="stats-row">
        <span class="stat-chip">全部 <b>{{ appStore.materialStats.total }}</b></span>
        <span class="stat-chip video">视频 <b>{{ appStore.materialStats.videos }}</b></span>
        <span class="stat-chip image">图片 <b>{{ appStore.materialStats.images }}</b></span>
        <span class="stat-chip other">其他 <b>{{ appStore.materialStats.others }}</b></span>
      </div>
      <div class="toolbar-actions">
        <el-input v-model="searchQuery" prefix-icon="Search" placeholder="搜索素材…" clearable class="search-input" />
        <el-button-group>
          <el-button :type="viewMode === 'grid' ? 'primary' : ''" @click="viewMode = 'grid'">
            <el-icon><Grid /></el-icon>
          </el-button>
          <el-button :type="viewMode === 'table' ? 'primary' : ''" @click="viewMode = 'table'">
            <el-icon><List /></el-icon>
          </el-button>
        </el-button-group>
        <el-button type="primary" @click="showUpload = true">
          <el-icon><Upload /></el-icon> 上传
        </el-button>
        <el-button @click="fetchMaterials">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- ═══ Grid View ═══ -->
    <div v-if="viewMode === 'grid'" class="material-grid">
      <div v-for="item in filteredMaterials" :key="item.id" class="material-card glass-card" @click="previewFile(item)">
        <div class="card-thumb">
          <el-icon v-if="isVideo(item.filename)" :size="40" class="thumb-icon"><VideoCamera /></el-icon>
          <el-icon v-else-if="isImage(item.filename)" :size="40" class="thumb-icon"><Picture /></el-icon>
          <el-icon v-else :size="40" class="thumb-icon"><Document /></el-icon>
        </div>
        <div class="card-info">
          <div class="card-name" :title="item.filename">{{ item.filename }}</div>
          <div class="card-meta">
            <span>{{ item.filesize }} MB</span>
            <el-tag :type="getTypeColor(item.filename)" size="small" effect="dark">{{ getTypeLabel(item.filename) }}</el-tag>
          </div>
        </div>
        <el-button class="card-delete" type="danger" size="small" circle @click.stop="deleteFile(item.id)">
          <el-icon><Delete /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- ═══ Table View ═══ -->
    <div v-else class="glass-card table-wrapper">
      <el-table :data="filteredMaterials" v-loading="loading" style="width: 100%">
        <el-table-column prop="filename" label="文件名" min-width="260" />
        <el-table-column prop="filesize" label="大小" width="100">
          <template #default="{ row }">{{ row.filesize }} MB</template>
        </el-table-column>
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="getTypeColor(row.filename)" size="small" effect="dark">{{ getTypeLabel(row.filename) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="upload_time" label="上传时间" width="180" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" plain @click="previewFile(row)">预览</el-button>
            <el-button size="small" type="danger" plain @click="deleteFile(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-empty v-if="!loading && filteredMaterials.length === 0" description="暂无素材" />

    <!-- ═══ Upload Dialog ═══ -->
    <el-dialog v-model="showUpload" title="上传素材" width="550px" destroy-on-close>
      <el-upload
        drag
        :action="uploadUrl"
        :headers="authHeaders"
        :on-success="onUploadSuccess"
        :on-error="() => ElMessage.error('上传失败')"
        multiple
        accept="video/*,image/*"
      >
        <el-icon class="el-icon--upload" :size="48"><Upload /></el-icon>
        <div class="el-upload__text">拖拽文件到此处，或<em>点击上传</em></div>
        <template #tip>
          <div class="el-upload__tip">支持视频和图片文件</div>
        </template>
      </el-upload>
    </el-dialog>

    <!-- ═══ Preview Dialog ═══ -->
    <el-dialog v-model="showPreview" :title="previewItem?.filename" width="700px" destroy-on-close>
      <div class="preview-content">
        <video v-if="previewItem && isVideo(previewItem.filename)" controls :src="previewSrc" class="preview-media" />
        <img v-else-if="previewItem && isImage(previewItem.filename)" :src="previewSrc" class="preview-media" />
        <div v-else class="preview-fallback">
          <el-icon :size="64"><Document /></el-icon>
          <p>该文件类型暂不支持预览</p>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { materialApi } from '@/api/material'
import { ElMessage, ElMessageBox } from 'element-plus'

const appStore = useAppStore()
const loading = ref(false)
const searchQuery = ref('')
const viewMode = ref('grid')
const showUpload = ref(false)
const showPreview = ref(false)
const previewItem = ref(null)
const previewSrc = ref('')

const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5409'
const uploadUrl = `${apiBase}/uploadSave`
const authHeaders = computed(() => ({ Authorization: `Bearer ${localStorage.getItem('token') || ''}` }))

const VIDEO_EXTS = ['.mp4', '.avi', '.mov', '.wmv', '.flv', '.mkv', '.webm']
const IMAGE_EXTS = ['.jpg', '.jpeg', '.png', '.gif', '.bmp', '.webp']
const isVideo = (f) => VIDEO_EXTS.some(e => f?.toLowerCase().endsWith(e))
const isImage = (f) => IMAGE_EXTS.some(e => f?.toLowerCase().endsWith(e))
const getTypeLabel = (f) => isVideo(f) ? '视频' : isImage(f) ? '图片' : '其他'
const getTypeColor = (f) => isVideo(f) ? 'success' : isImage(f) ? 'warning' : 'info'

const filteredMaterials = computed(() => {
  if (!searchQuery.value) return appStore.materials
  const q = searchQuery.value.toLowerCase()
  return appStore.materials.filter(m => m.filename?.toLowerCase().includes(q))
})

const fetchMaterials = async () => {
  loading.value = true
  try {
    const res = await materialApi.getAllMaterials()
    if (res?.data) appStore.setMaterials(res.data)
  } catch { ElMessage.error('获取素材失败') }
  loading.value = false
}

const deleteFile = async (id) => {
  await ElMessageBox.confirm('确定删除该素材？', '确认', { type: 'warning' })
  try {
    await materialApi.deleteFile(id)
    ElMessage.success('已删除')
    fetchMaterials()
  } catch { ElMessage.error('删除失败') }
}

const previewFile = (item) => {
  previewItem.value = item
  previewSrc.value = materialApi.getPreviewUrl(item.filename)
  showPreview.value = true
}

const onUploadSuccess = (res) => {
  if (res.code === 200) {
    ElMessage.success('上传成功')
    fetchMaterials()
  } else {
    ElMessage.error(res.msg || '上传失败')
  }
}

onMounted(fetchMaterials)
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

.material-management {
  max-width: 1400px;
}

// ── Toolbar ──
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 20px;
  margin-bottom: 20px;
  gap: 16px;
  flex-wrap: wrap;

  .stats-row {
    display: flex;
    gap: 12px;
  }

  .stat-chip {
    font-size: 13px;
    color: $text-secondary;
    b { color: $text-primary; font-weight: 600; margin-left: 4px; }

    &.video b { color: $success-color; }
    &.image b { color: $warning-color; }
    &.other b { color: $info-color; }
  }

  .toolbar-actions {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .search-input { max-width: 200px; }
}

// ── Grid ──
.material-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.material-card {
  position: relative;
  cursor: pointer;
  padding: 0;
  overflow: hidden;
  transition: all $transition-base;

  &:hover {
    transform: translateY(-4px);

    .card-delete { opacity: 1; }
    .card-thumb .thumb-icon { transform: scale(1.15); }
  }

  .card-thumb {
    height: 120px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(177, 73, 255, 0.05);
    border-bottom: 1px solid $border-color;

    .thumb-icon {
      color: $text-muted;
      transition: transform $transition-base, color $transition-base;
    }
  }

  &:hover .card-thumb .thumb-icon { color: $accent-color; }

  .card-info {
    padding: 12px 14px;
  }

  .card-name {
    font-size: 13px;
    font-weight: 500;
    color: $text-primary;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-bottom: 6px;
  }

  .card-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
    color: $text-muted;
  }

  .card-delete {
    position: absolute;
    top: 8px;
    right: 8px;
    opacity: 0;
    transition: opacity $transition-fast;
  }
}

// ── Table ──
.table-wrapper {
  padding: 4px;
  overflow: hidden;
}

// ── Preview ──
.preview-content {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 300px;
}

.preview-media {
  max-width: 100%;
  max-height: 500px;
  border-radius: 12px;
}

.preview-fallback {
  text-align: center;
  color: $text-muted;

  .el-icon { margin-bottom: 12px; }
}
</style>
