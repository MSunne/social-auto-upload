import { http, API_BASE } from '@/utils/request'

// ═══════════════════════════════════════
// Account API — SAU Backend
// ═══════════════════════════════════════

export const accountApi = {
  /** 快速获取账号列表（不验证cookie） */
  getAccounts() {
    return http.get('/getAccounts')
  },

  /** 验证全部账号（较慢） */
  getValidAccounts() {
    return http.get('/getValidAccounts')
  },

  /** 验证单个账号 */
  validateAccount(id) {
    return http.get(`/validateAccount?id=${id}`)
  },

  /** 添加账号 */
  addAccount(data) {
    return http.post('/account', data)
  },

  /** 更新账号信息 */
  updateAccount(data) {
    return http.post('/updateUserinfo', data)
  },

  /** 删除账号 */
  deleteAccount(id) {
    return http.get(`/deleteAccount?id=${id}`)
  },

  /** 上传 Cookie 文件 */
  uploadCookie(formData) {
    return http.upload('/uploadCookie', formData)
  },

  /** 下载 Cookie — 返回直链 URL */
  getDownloadCookieUrl(id) {
    return `${API_BASE}/downloadCookie?id=${id}`
  },

  /**
   * 发起远端登录 SSE 流
   * @param {string} platform - 平台类型 1=小红书 2=视频号 3=抖音 4=快手
   * @param {string} accountName - 账号名称
   * @returns {string} SSE 事件流 URL
   */
  getLoginSSEUrl(platform, accountName) {
    return `${API_BASE}/login?platform=${platform}&account=${encodeURIComponent(accountName)}`
  },

  /** 远端登录请求 */
  remoteLogin(data) {
    return http.post('/remoteLogin', data)
  },
}
