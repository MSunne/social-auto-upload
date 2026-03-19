<template>
  <div class="task-center fade-in">
    <div class="page-head glass-card">
      <div>
        <div class="section-title" style="margin: 0"><el-icon><Tickets /></el-icon> 本地任务中心</div>
        <p class="page-subtitle">
          统一查看 OmniDrive 回流到 OmniBull 的 AI 任务，以及 SAU 本地实际执行的发布任务。
        </p>
      </div>
      <el-button type="primary" @click="fetchTasks" :loading="loading">
        <el-icon><Refresh /></el-icon> 刷新
      </el-button>
    </div>

    <el-row :gutter="16" class="summary-row">
      <el-col :span="12">
        <div class="glass-card summary-card">
          <div class="summary-label">AI 镜像任务</div>
          <div class="summary-value glow-violet">{{ aiTasks.length }}</div>
          <div class="summary-meta">正在做内容、待发布、已发布都在这里跟踪</div>
        </div>
      </el-col>
      <el-col :span="12">
        <div class="glass-card summary-card">
          <div class="summary-label">SAU 发布任务</div>
          <div class="summary-value glow-cyan">{{ publishTasks.length }}</div>
          <div class="summary-meta">本地账号真实执行、定时发布、发布结果都落这张表</div>
        </div>
      </el-col>
    </el-row>

    <el-tabs v-model="activeTab" class="task-tabs">
      <el-tab-pane label="AI 任务" name="ai">
        <div class="glass-card table-card">
          <el-table :data="aiTasks" stripe empty-text="暂无 AI 任务">
            <el-table-column prop="taskUuid" label="任务ID" min-width="180" />
            <el-table-column prop="jobType" label="类型" width="100" />
            <el-table-column prop="modelName" label="模型" min-width="160" />
            <el-table-column prop="status" label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="tagType(row.status)">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="说明" min-width="240" show-overflow-tooltip />
            <el-table-column prop="linkedPublishTaskUuid" label="关联发布任务" min-width="180" />
            <el-table-column prop="updatedAt" label="更新时间" min-width="180">
              <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane label="发布任务" name="publish">
        <div class="glass-card table-card">
          <el-table :data="publishTasks" stripe empty-text="暂无发布任务">
            <el-table-column prop="taskUuid" label="任务ID" min-width="180" />
            <el-table-column prop="platformName" label="平台" width="120" />
            <el-table-column prop="accountName" label="账号" min-width="160" />
            <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
            <el-table-column prop="status" label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="tagType(row.status)">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="说明" min-width="240" show-overflow-tooltip />
            <el-table-column prop="runAt" label="执行时间" min-width="180">
              <template #default="{ row }">{{ formatTime(row.runAt) }}</template>
            </el-table-column>
            <el-table-column prop="finishedAt" label="完成时间" min-width="180">
              <template #default="{ row }">{{ formatTime(row.finishedAt) }}</template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { publishApi, systemApi } from '@/api'

const loading = ref(false)
const activeTab = ref('ai')
const aiTasks = ref([])
const publishTasks = ref([])

const fetchTasks = async () => {
  loading.value = true
  try {
    const [aiRes, publishRes] = await Promise.all([
      systemApi.getAITasks({ limit: 200 }),
      publishApi.getPublishTasks(),
    ])
    aiTasks.value = aiRes?.data || []
    publishTasks.value = publishRes?.data || []
  } catch {
    ElMessage.error('获取任务列表失败')
  }
  loading.value = false
}

const tagType = (status) => {
  switch (status) {
    case 'scheduled':
      return 'info'
    case 'storyboarding':
      return 'warning'
    case 'success':
    case 'output_ready':
    case 'publish_pending':
      return 'success'
    case 'running':
    case 'generating':
    case 'publishing':
      return 'primary'
    case 'failed':
      return 'danger'
    case 'needs_verify':
      return 'warning'
    default:
      return 'info'
  }
}

const formatTime = (value) => {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
}

onMounted(fetchTasks)
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

.task-center {
  max-width: 1200px;
}

.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 22px;
}

.page-subtitle {
  margin-top: 8px;
  color: $text-secondary;
  font-size: 13px;
}

.summary-row {
  margin-top: 18px;
}

.summary-card {
  padding: 18px 20px;
}

.summary-label {
  color: $text-muted;
  font-size: 12px;
}

.summary-value {
  margin-top: 10px;
  font-size: 28px;
  font-weight: 700;
  color: $text-primary;
}

.summary-meta {
  margin-top: 8px;
  color: $text-secondary;
  font-size: 12px;
}

.glow-violet {
  color: $accent-color;
  text-shadow: 0 0 14px $accent-glow;
}

.glow-cyan {
  color: $info-color;
  text-shadow: 0 0 14px rgba(0, 212, 255, 0.28);
}

.task-tabs {
  margin-top: 20px;
}

.table-card {
  padding: 8px 12px 12px;
}

:deep(.el-table) {
  background: transparent;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-header-bg-color: rgba(255, 255, 255, 0.03);
  --el-table-border-color: #{$border-color};
  --el-table-row-hover-bg-color: rgba(255, 255, 255, 0.03);
  color: $text-primary;
}

:deep(.el-tabs__item) {
  color: $text-secondary;
}

:deep(.el-tabs__item.is-active) {
  color: $accent-color;
}
</style>
