import { http, API_BASE } from '@/utils/request'

// ═══════════════════════════════════════
// Material API — SAU Backend
// ═══════════════════════════════════════

export const materialApi = {
  /** 获取所有素材 */
  getAllMaterials() {
    return http.get('/getFiles')
  },

  /** 上传素材 */
  uploadMaterial(formData, onUploadProgress) {
    return http.upload('/uploadSave', formData, onUploadProgress)
  },

  /** 删除素材 */
  deleteMaterial(id) {
    return http.get(`/deleteFile?id=${id}`)
  },

  /** 获取素材预览 URL */
  getPreviewUrl(filename) {
    return `${API_BASE}/getFile?filename=${filename}`
  },

  /** 获取上传接口地址 */
  getUploadUrl() {
    return `${API_BASE}/upload`
  },
}