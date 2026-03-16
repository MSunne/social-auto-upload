import { http } from '@/utils/request'

// ═══════════════════════════════════════
// System API — SAU Backend
// ═══════════════════════════════════════

export const systemApi = {
  /** 获取系统全局状态（设备、账号统计、任务统计等） */
  getSkillStatus() {
    return http.get('/api/skill/status')
  },

  /** 获取 CloudAgent 连接状态 */
  getCloudAgentStatus() {
    return http.get('/cloudAgentStatus')
  },

  /** 获取 OmniDrive Agent 连接状态 */
  getOmniDriveAgentStatus() {
    return http.get('/omnidriveAgentStatus')
  },

  /** 获取 AI 任务列表 */
  getAITasks(params) {
    return http.get('/aiTasks', params)
  },

  /** 获取 AI 任务详情 */
  getAITaskDetail(uuid) {
    return http.get(`/aiTaskDetail?uuid=${uuid}`)
  },

  /** 获取素材根目录列表 */
  getMaterialRoots() {
    return http.get('/api/skill/materials/roots')
  },
}
