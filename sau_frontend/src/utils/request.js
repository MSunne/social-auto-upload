import axios from 'axios'
import { ElMessage } from 'element-plus'

// ═══════════════════════════════════════
// Axios Instance — SAU Backend
// ═══════════════════════════════════════

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5409'

const request = axios.create({
  baseURL: API_BASE,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// ── Request interceptor ──
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    console.error('请求错误:', error)
    return Promise.reject(error)
  }
)

// ── Response interceptor ──
request.interceptors.response.use(
  (response) => {
    const { data } = response

    // Backend uses { code: 200, msg, data } convention
    if (data.code === 200 || data.success) {
      return data
    } else {
      const msg = data.msg || data.message || '请求失败'
      ElMessage.error(msg)
      return Promise.reject(new Error(msg))
    }
  },
  (error) => {
    console.error('响应错误:', error)

    if (error.response) {
      const { status } = error.response
      const messages = {
        401: '未授权，请重新登录',
        403: '拒绝访问',
        404: '请求地址不存在',
        500: '服务器内部错误',
      }
      ElMessage.error(messages[status] || `网络错误 (${status})`)
    } else if (error.code === 'ECONNABORTED') {
      ElMessage.error('请求超时，请检查网络')
    } else {
      ElMessage.error('网络连接失败，请确认后端已启动')
    }

    return Promise.reject(error)
  }
)

// ── Wrapped http helpers ──
export const http = {
  get(url, params) {
    return request.get(url, { params })
  },

  post(url, data, config = {}) {
    return request.post(url, data, config)
  },

  put(url, data, config = {}) {
    return request.put(url, data, config)
  },

  delete(url, params) {
    return request.delete(url, { params })
  },

  upload(url, formData, onUploadProgress) {
    return request.post(url, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000,
      onUploadProgress,
    })
  },
}

// ── SSE helper ──
export function createSSE(path, onMessage, onError) {
  const url = `${API_BASE}${path}`
  const source = new EventSource(url)

  source.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      onMessage(data)
    } catch {
      onMessage(event.data)
    }
  }

  source.onerror = (err) => {
    if (onError) onError(err)
    source.close()
  }

  return source
}

export { API_BASE }
export default request