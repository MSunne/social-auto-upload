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
          <el-table :data="aiTasks" class="premium-table" empty-text="暂无 AI 任务">
            <el-table-column prop="taskUuid" label="任务ID" min-width="180">
              <template #default="{ row }">
                <span class="uuid-text">{{ row.taskUuid }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="jobType" label="类型" width="100" />
            <el-table-column prop="modelName" label="模型" min-width="160" />
            <el-table-column prop="status" label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="tagType(row.status)" effect="dark" class="status-tag">
                  {{ statusLabel(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="说明" min-width="240" show-overflow-tooltip />
            <el-table-column prop="linkedPublishTaskUuid" label="关联发布任务" min-width="180" />
            <el-table-column prop="updatedAt" label="更新时间" min-width="180">
              <template #default="{ row }">{{ formatTime(row.updatedAt, 'utc') }}</template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane label="发布任务" name="publish">
        <div class="glass-card table-card">
          <el-table :data="publishTasks" class="premium-table" empty-text="暂无发布任务">
            <el-table-column prop="taskUuid" label="任务ID" min-width="180">
              <template #default="{ row }">
                <span class="uuid-text">{{ row.taskUuid }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="platformName" label="平台" width="120" />
            <el-table-column prop="accountName" label="账号" min-width="160" />
            <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
            <el-table-column prop="status" label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="tagType(row.status)" effect="dark" class="status-tag">
                  {{ statusLabel(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="说明" min-width="240" show-overflow-tooltip />
            <el-table-column prop="runAt" label="执行时间" min-width="180">
              <template #default="{ row }">{{ formatTime(row.runAt, 'local') }}</template>
            </el-table-column>
            <el-table-column prop="finishedAt" label="完成时间" min-width="180">
              <template #default="{ row }">{{ formatTime(row.finishedAt, 'utc') }}</template>
            </el-table-column>
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button 
                  v-if="['failed', 'needs_verify', 'cancelled'].includes(row.status)" 
                  size="small" 
                  type="primary" 
                  plain 
                  @click="retryTask(row)"
                  :loading="row._retrying"
                >重试</el-button>
              </template>
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

const retryTask = async (row) => {
  row._retrying = true
  try {
    const res = await publishApi.retryPublishTask(row.taskUuid)
    if (res?.code === 200) {
      ElMessage.success('任务已加入重试队列')
      fetchTasks()
    } else {
      ElMessage.error(res?.msg || '重试请求失败')
    }
  } catch (e) {
    ElMessage.error('网络请求失败')
  } finally {
    row._retrying = false
  }
}

const tagType = (status) => {
  switch (status) {
    case 'scheduled':
      return 'info'
    case 'storyboarding':
    case 'waiting_recharge':
    case 'needs_verify':
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
    default:
      return 'info'
  }
}

const statusLabel = (status) => {
  switch (status) {
    case 'queued_cloud':
      return '等待云端'
    case 'scheduled':
      return '未开始'
    case 'storyboarding':
      return '优化分镜中'
    case 'running':
      return '执行中'
    case 'generating':
      return '生成中'
    case 'output_ready':
      return '制作完成'
    case 'publish_pending':
      return '待发布'
    case 'publishing':
      return '发布中'
    case 'waiting_recharge':
      return '欠费'
    case 'needs_verify':
      return '待人工确认'
    case 'success':
      return '已完成'
    case 'failed':
      return '失败'
    case 'cancelled':
      return '已取消'
    case 'pending':
      return '等待中'
    default:
      return status || '未知'
  }
}

const TIME_FORMATTER = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})

const LOCAL_DATETIME_RE = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?(?:\.\d+)?$/

const formatTime = (value, source = 'local') => {
  if (!value) return '-'

  const parsed =
    source === 'utc'
      ? parseUtcDateTime(value)
      : parseLocalDateTime(value)

  if (!parsed) return String(value)
  return TIME_FORMATTER.format(parsed).replace(/\//g, '-')
}

const parseLocalDateTime = (value) => {
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value
  }

  const raw = String(value).trim()
  const match = raw.match(LOCAL_DATETIME_RE)
  if (match) {
    const [, year, month, day, hour, minute, second = '00'] = match
    return new Date(
      Number(year),
      Number(month) - 1,
      Number(day),
      Number(hour),
      Number(minute),
      Number(second),
    )
  }

  const parsed = new Date(raw)
  return Number.isNaN(parsed.getTime()) ? null : parsed
}

const parseUtcDateTime = (value) => {
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value
  }

  const raw = String(value).trim()
  const match = raw.match(LOCAL_DATETIME_RE)
  if (match) {
    const [, year, month, day, hour, minute, second = '00'] = match
    return new Date(Date.UTC(
      Number(year),
      Number(month) - 1,
      Number(day),
      Number(hour),
      Number(minute),
      Number(second),
    ))
  }

  const parsed = new Date(raw)
  return Number.isNaN(parsed.getTime()) ? null : parsed
}

onMounted(fetchTasks)
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

.task-center {
  width: 100%;
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
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-header-bg-color: rgba(255, 255, 255, 0.05);
  --el-table-header-text-color: rgba(255, 255, 255, 0.9);
  --el-table-border-color: rgba(255, 255, 255, 0.08);
  --el-table-row-hover-bg-color: rgba(255, 255, 255, 0.08);
  --el-table-text-color: rgba(255, 255, 255, 0.75);
  background: transparent;
  color: var(--el-table-text-color);
  border-radius: 8px;
  overflow: hidden;
}

:deep(.el-table th.el-table__cell) {
  border-bottom: 1px solid var(--el-table-border-color);
  font-weight: 600;
  letter-spacing: 0.5px;
  padding: 12px 0;
}

:deep(.el-table td.el-table__cell) {
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
  padding: 14px 0;
  transition: all 0.3s ease;
}

:deep(.el-table tr:hover td.el-table__cell) {
  background-color: var(--el-table-row-hover-bg-color);
}

:deep(.el-table::before) {
  display: none;
}

.uuid-text {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.6);
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
}

.status-tag {
  border: none;
  font-weight: 600;
  padding: 0 10px;
  border-radius: 4px;
}

:deep(.el-tabs__item) {
  color: $text-secondary;
  font-size: 15px;
  font-weight: 500;
  transition: all 0.3s ease;
}

:deep(.el-tabs__item:hover) {
  color: $text-primary;
}

:deep(.el-tabs__item.is-active) {
  color: $accent-color;
  font-weight: 600;
}

:deep(.el-tabs__active-bar) {
  background-color: $accent-color;
  height: 3px;
  border-radius: 3px;
}
</style>
