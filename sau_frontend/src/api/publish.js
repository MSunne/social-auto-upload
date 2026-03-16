import { http } from '@/utils/request'

// ═══════════════════════════════════════
// Publish API — SAU Backend
// ═══════════════════════════════════════

export const publishApi = {
  /** 单次发布视频 */
  postVideo(data) {
    return http.post('/postVideo', data)
  },

  /** 批量发布视频 */
  postVideoBatch(data) {
    return http.post('/postVideoBatch', data)
  },

  /** 获取发布任务列表 */
  getPublishTasks() {
    return http.get('/publishTasks')
  },

  /** 获取发布任务详情 */
  getPublishTaskDetail(uuid) {
    return http.get(`/publishTaskDetail?uuid=${uuid}`)
  },

  /** 通过 Skill API 创建发布任务 */
  createSkillPublish(data) {
    return http.post('/api/skill/publish', data)
  },

  /** 通过 Skill API 获取发布任务列表 */
  getSkillPublishTasks() {
    return http.get('/api/skill/publish/tasks')
  },

  /** 通过 Skill API 获取发布任务详情 */
  getSkillPublishTaskDetail(uuid) {
    return http.get(`/api/skill/publish/tasks/${uuid}`)
  },
}
