import { readFile } from "node:fs/promises";
import path from "node:path";

const DEFAULT_BASE_URL = "http://127.0.0.1:8410";
const DEFAULT_TIMEOUT_MS = 45000;
const DEFAULT_LOCAL_OMNIBULL_BASE_URL = "http://127.0.0.1:5409";
const DEFAULT_LOCAL_OMNIBULL_TIMEOUT_MS = 10000;
const DEFAULT_CHAT_MODEL = "glm-5";
const DEFAULT_IMAGE_MODEL = "gemini-3-pro-image-preview";
const DEFAULT_VIDEO_MODEL = "veo-3.1-fast-fl";
const DEFAULT_VIDEO_DURATION_SECONDS = 8;
const OPENCLAW_SKILL_CHAT_SOURCE = "openclaw_skill";
const OPENCLAW_MAIN_CHAT_SOURCE = "openclaw_main_chat";
const FINAL_AI_JOB_STATUSES = new Set(["success", "completed", "failed", "cancelled", "needs_verify"]);
const LONG_JOB_OBSERVATION_WINDOW_MS = 5 * 60 * 1000;
const LONG_JOB_OBSERVATION_POLL_INTERVAL_MS = 15 * 1000;
const TOOL_MEDIA_FETCH_TIMEOUT_MS = 15000;
const TOOL_MAX_EMBEDDED_IMAGE_BYTES = 8 * 1024 * 1024;
const TOOL_MAX_EMBEDDED_IMAGE_COUNT = 1;

let cachedSession = null;
const OPENCLAW_AVAILABLE_SKILLS = [
  "omnidrive_auth",
  "omnidrive_models",
  "omnidrive_chat",
  "omnidrive_image",
  "omnidrive_video",
  "omnidrive_mix_video",
  "omnidrive_jobs",
  "omnidrive_job_detail",
  "omnibull_status",
  "omnibull_accounts",
  "omnibull_materials",
  "omnibull_publish",
];

function resolveConfig(api) {
  const pluginConfig = api.pluginConfig || {};
  return {
    baseUrl: String(pluginConfig.baseUrl || process.env.OMNIDRIVE_BASE_URL || DEFAULT_BASE_URL).replace(/\/+$/, ""),
    accessToken: String(pluginConfig.accessToken || process.env.OMNIDRIVE_ACCESS_TOKEN || "").trim(),
    email: String(pluginConfig.email || process.env.OMNIDRIVE_EMAIL || "").trim(),
    password: String(pluginConfig.password || process.env.OMNIDRIVE_PASSWORD || "").trim(),
    timeoutMs: Number(pluginConfig.timeoutMs || process.env.OMNIDRIVE_TIMEOUT_MS || DEFAULT_TIMEOUT_MS),
    localOmniBullBaseUrl: String(
      pluginConfig.localOmniBullBaseUrl || process.env.OMNIBULL_BASE_URL || DEFAULT_LOCAL_OMNIBULL_BASE_URL,
    ).replace(/\/+$/, ""),
    localOmniBullApiKey: String(pluginConfig.localOmniBullApiKey || process.env.OMNIBULL_API_KEY || "").trim(),
    localOmniBullTimeoutMs: Number(
      pluginConfig.localOmniBullTimeoutMs || process.env.OMNIBULL_TIMEOUT_MS || DEFAULT_LOCAL_OMNIBULL_TIMEOUT_MS,
    ),
    localDeviceCode: String(pluginConfig.localDeviceCode || process.env.OMNIBULL_DEVICE_CODE || "").trim(),
    defaultChatModel: String(
      pluginConfig.defaultChatModel || process.env.OMNIDRIVE_DEFAULT_CHAT_MODEL || DEFAULT_CHAT_MODEL,
    ).trim(),
    defaultImageModel: String(
      pluginConfig.defaultImageModel || process.env.OMNIDRIVE_DEFAULT_IMAGE_MODEL || DEFAULT_IMAGE_MODEL,
    ).trim(),
    defaultVideoModel: String(
      pluginConfig.defaultVideoModel || process.env.OMNIDRIVE_DEFAULT_VIDEO_MODEL || DEFAULT_VIDEO_MODEL,
    ).trim(),
    defaultVideoDurationSeconds: Number(
      pluginConfig.defaultVideoDurationSeconds ||
        process.env.OMNIDRIVE_DEFAULT_VIDEO_DURATION_SECONDS ||
        DEFAULT_VIDEO_DURATION_SECONDS,
    ),
  };
}

function ensure(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function textBlock(text) {
  return {
    type: "text",
    text: String(text || ""),
  };
}

function isFormDataBody(body) {
  return typeof FormData !== "undefined" && body instanceof FormData;
}

function toolResult(data, content = null) {
  if (Array.isArray(content) && content.length > 0) {
    return { content };
  }
  return {
    content: [
      textBlock(JSON.stringify(data, null, 2)),
    ],
  };
}

function summarizeBoundDevice(boundDevice) {
  if (!boundDevice) {
    return null;
  }
  return {
    id: boundDevice.id,
    deviceCode: boundDevice.deviceCode,
    name: boundDevice.name,
    isEnabled: boundDevice.isEnabled,
    defaultReasoningModel: boundDevice.defaultReasoningModel || null,
    defaultChatModel: boundDevice.defaultChatModel || null,
    defaultImageModel: boundDevice.defaultImageModel || null,
    defaultVideoModel: boundDevice.defaultVideoModel || null,
  };
}

function summarizeSessionUser(user) {
  if (!user) {
    return null;
  }
  return {
    id: user.id || null,
    email: user.email || null,
    name: user.name || null,
  };
}

function summarizeCachedSession(session) {
  if (!session) {
    return null;
  }
  return {
    email: session.email || null,
    source: session.source || null,
    loggedInAt: session.loggedInAt || null,
    apiBaseUrl: session.apiBaseUrl || null,
    authState: session.authState || null,
  };
}

function extractGatewayParams(request) {
  if (!request || typeof request !== "object") {
    return {};
  }
  const candidate =
    request.params ||
    request.payload ||
    request.input ||
    request.arguments ||
    {};
  if (!candidate || typeof candidate !== "object" || Array.isArray(candidate)) {
    return {};
  }
  return candidate;
}

function buildGatewayError(methodName, error) {
  const failure = {
    ok: false,
    gatewayMethod: methodName,
    error: String(error?.message || error || "unknown error"),
  };
  if (typeof error?.code === "string" && error.code.trim()) {
    failure.errorCode = error.code.trim();
  }
  if (error?.fallbackRecommended === true) {
    failure.fallbackRecommended = true;
  }
  if (typeof error?.blockedReason === "string" && error.blockedReason.trim()) {
    failure.blockedReason = error.blockedReason.trim();
  }
  return failure;
}

function normalizeChatSource(source, fallbackSource) {
  const value = String(source || "").trim();
  return value || fallbackSource;
}

function createFallbackGatewayError(message, code, extras = {}) {
  const error = new Error(String(message || "OmniDrive 主聊天不可用"));
  error.code = String(code || "omnidrive_chat_unavailable").trim() || "omnidrive_chat_unavailable";
  error.fallbackRecommended = true;
  if (typeof extras.blockedReason === "string" && extras.blockedReason.trim()) {
    error.blockedReason = extras.blockedReason.trim();
  }
  if (extras.status !== undefined) {
    error.status = extras.status;
  }
  return error;
}

function classifyMainChatFallbackError(error) {
  if (error?.fallbackRecommended === true && typeof error?.code === "string" && error.code.trim()) {
    return error;
  }

  const message = String(error?.message || error || "OmniDrive 主聊天不可用").trim() || "OmniDrive 主聊天不可用";
  const normalized = message.toLowerCase();

  if (
    normalized.includes("token expired") ||
    normalized.includes("invalid access token") ||
    message.includes("OmniDrive 会话已失效") ||
    message.includes("无法从本地 OmniBull 刷新会话")
  ) {
    return createFallbackGatewayError(message, "omnidrive_session_unavailable");
  }
  if (
    message.includes("尚未绑定") ||
    message.includes("未绑定") ||
    message.includes("无法从本地 OmniBull 读取 deviceCode")
  ) {
    return createFallbackGatewayError(message, "omnidrive_device_unbound", { blockedReason: message });
  }
  if (message.includes("已被停用或解绑")) {
    return createFallbackGatewayError(message, "omnidrive_device_unavailable", { blockedReason: message });
  }
  if (error?.name === "AbortError" || normalized.includes("aborted") || normalized.includes("timeout")) {
    return createFallbackGatewayError(message, "omnidrive_cloud_unavailable");
  }
  return createFallbackGatewayError(message, "omnidrive_cloud_unavailable");
}

function isLongRunningJobType(jobType) {
  const value = String(jobType || "").trim().toLowerCase();
  return value === "video" || value === "digital_human";
}

function parseTimestampMs(value) {
  if (typeof value !== "string" || !value.trim()) {
    return null;
  }
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function getLongJobObservationRemainingMs(job, nowMs = Date.now()) {
  if (!isLongRunningJobType(job?.jobType)) {
    return Number.POSITIVE_INFINITY;
  }
  const createdAtMs = parseTimestampMs(job?.createdAt);
  if (!createdAtMs) {
    return LONG_JOB_OBSERVATION_WINDOW_MS;
  }
  const elapsedMs = Math.max(0, nowMs - createdAtMs);
  return Math.max(0, LONG_JOB_OBSERVATION_WINDOW_MS - elapsedMs);
}

function buildPublishMetadataInputPayload(params = {}) {
  const accountId = String(params.accountId || "").trim();
  const platform = String(params.platform || "").trim();
  const accountName = String(params.accountName || "").trim();
  const publishAt = String(params.publishAt || "").trim();
  const hasAnyPublishField =
    accountId !== "" ||
    platform !== "" ||
    accountName !== "" ||
    publishAt !== "";

  if (!hasAnyPublishField) {
    return {};
  }

  ensure(accountId && platform && accountName && publishAt, "自动发布需要同时提供 accountId、platform、accountName、publishAt");

  return {
    accountId,
    platform,
    accountName,
    publishAt,
    publishPayload: {
      runAt: publishAt,
      requestedRun: publishAt,
      targets: [
        {
          accountId,
          platform,
          accountName,
        },
      ],
    },
  };
}

function normalizeMixVideoAssetInput(item, label) {
  ensure(item && typeof item === "object", `${label} 必须是对象`);
  const absolutePath = String(item.absolutePath || "").trim();
  const url = String(item.url || "").trim();
  ensure(absolutePath || url, `${label} 需要提供 absolutePath 或 url`);
  return {
    absolutePath: absolutePath || undefined,
    url: url || undefined,
    fileName: String(item.fileName || (absolutePath ? path.basename(absolutePath) : "")).trim() || undefined,
    mimeType: String(item.mimeType || "").trim() || undefined,
  };
}

function buildMixVideoCreateInput(params = {}) {
  const scriptText = String(params.scriptText || "").trim();
  ensure(scriptText, "缺少 scriptText");

  const sourceVideos = Array.isArray(params.sourceVideos)
    ? params.sourceVideos.map((item, index) => normalizeMixVideoAssetInput(item, `sourceVideos[${index}]`))
    : [];
  ensure(sourceVideos.length > 0, "至少需要一个 sourceVideos");

  const refAudio = normalizeMixVideoAssetInput(params.refAudio || params.referenceAudio || {}, "refAudio");
  const publish = buildPublishMetadataInputPayload(params || {});
  return {
    scriptText,
    sourceVideos,
    refAudio,
    publish: Object.keys(publish).length > 0 ? publish : null,
  };
}

function buildQuery(params) {
  const search = new URLSearchParams();
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value === undefined || value === null || value === "") {
      return;
    }
    search.set(key, String(value));
  });
  const query = search.toString();
  return query ? `?${query}` : "";
}

function clearCachedSession() {
  cachedSession = null;
}

async function requestLocalOmniBull(api, path, options = {}) {
  const cfg = resolveConfig(api);
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), cfg.localOmniBullTimeoutMs);
  const headers = {
    Accept: "application/json",
    ...(options.headers || {}),
  };
  if (cfg.localOmniBullApiKey) {
    headers["X-Omnibull-Key"] = cfg.localOmniBullApiKey;
  }
  if (options.body !== undefined && !headers["Content-Type"] && !isFormDataBody(options.body)) {
    headers["Content-Type"] = "application/json";
  }

  try {
    const response = await fetch(`${cfg.localOmniBullBaseUrl}${path}`, {
      ...options,
      headers,
      signal: controller.signal,
    });
    const text = await response.text();
    let payload = null;
    try {
      payload = text ? JSON.parse(text) : null;
    } catch {
      payload = text;
    }
    if (!response.ok) {
      throw new Error(extractErrorMessage(payload, response.status));
    }
    return payload;
  } finally {
    clearTimeout(timeout);
  }
}

async function fetchLocalOmniDriveSession(api) {
  const payload = await requestLocalOmniBull(api, "/api/skill/omnidrive/session", { method: "GET" });
  const data = payload?.data || payload || {};
  const accessToken = String(data.accessToken || "").trim();
  ensure(accessToken, "本地 OmniBull 未返回可用的 OmniDrive accessToken");
  const apiBaseUrl = String(data.apiBaseUrl || data.cloudUrl || "").trim();
  cachedSession = {
    accessToken,
    user: data.user || null,
    device: data.device || null,
    email: data?.user?.email || null,
    source: "local_agent_session",
    loggedInAt: nowISO(),
    apiBaseUrl: apiBaseUrl || null,
    cloudUrl: apiBaseUrl || null,
    authState: String(data.authState || "").trim() || "authorized",
    reason: String(data.reason || "").trim(),
    availableSkills: Array.isArray(data.availableSkills) ? data.availableSkills : [],
  };
  return cachedSession;
}

function nowISO() {
  return new Date().toISOString();
}

function extractErrorMessage(payload, status) {
  if (!payload) {
    return `请求失败 (${status})`;
  }
  if (typeof payload === "string") {
    return payload;
  }
  if (typeof payload.error === "string" && payload.error.trim()) {
    return payload.error.trim();
  }
  if (typeof payload.msg === "string" && payload.msg.trim()) {
    return payload.msg.trim();
  }
  if (typeof payload.message === "string" && payload.message.trim()) {
    return payload.message.trim();
  }
  return `请求失败 (${status})`;
}

async function rawRequest(api, path, options = {}, accessToken = "") {
  const cfg = resolveConfig(api);
  const runtimeBaseUrl = String(cachedSession?.apiBaseUrl || cachedSession?.cloudUrl || "").trim() || cfg.baseUrl;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), cfg.timeoutMs);
  const headers = {
    Accept: "application/json",
    ...(options.headers || {}),
  };
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  if (options.body !== undefined && !headers["Content-Type"] && !isFormDataBody(options.body)) {
    headers["Content-Type"] = "application/json";
  }

  try {
    const response = await fetch(`${runtimeBaseUrl.replace(/\/+$/, "")}${path}`, {
      ...options,
      headers,
      signal: controller.signal,
    });
    const text = await response.text();
    let payload = null;
    try {
      payload = text ? JSON.parse(text) : null;
    } catch {
      payload = text;
    }
    return {
      ok: response.ok,
      status: response.status,
      payload,
    };
  } finally {
    clearTimeout(timeout);
  }
}

async function performLogin(api, credentials = {}) {
  const cfg = resolveConfig(api);
  const email = String(credentials.email || cfg.email || "").trim();
  const password = String(credentials.password || cfg.password || "").trim();
  ensure(email, "缺少 OmniDrive 邮箱，请在工具参数或插件配置里提供 email");
  ensure(password, "缺少 OmniDrive 密码，请在工具参数或插件配置里提供 password");

  const response = await rawRequest(api, "/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    throw new Error(extractErrorMessage(response.payload, response.status));
  }

  const accessToken = String(response.payload?.accessToken || "").trim();
  ensure(accessToken, "OmniDrive 登录成功但未返回 accessToken");

  cachedSession = {
    accessToken,
    user: response.payload?.user || null,
    email,
    source: credentials.email || credentials.password ? "runtime" : "config",
    loggedInAt: nowISO(),
  };
  return cachedSession;
}

async function ensureAccessToken(api, overrides = {}) {
  const explicitToken = String(overrides.accessToken || "").trim();
  if (explicitToken) {
    return explicitToken;
  }
  if (cachedSession?.accessToken) {
    return cachedSession.accessToken;
  }

  try {
    const session = await fetchLocalOmniDriveSession(api);
    return session.accessToken;
  } catch {
    // Fallback to configured credentials or access token when local OmniBull bridge is unavailable.
  }

  const cfg = resolveConfig(api);
  if (cfg.accessToken) {
    cachedSession = {
      accessToken: cfg.accessToken,
      user: null,
      email: cfg.email || null,
      source: "config_access_token",
      loggedInAt: null,
      apiBaseUrl: cfg.baseUrl,
      cloudUrl: cfg.baseUrl,
    };
    return cfg.accessToken;
  }

  const session = await performLogin(api, overrides);
  return session.accessToken;
}

async function refreshAccessTokenAfterUnauthorized(api, overrides = {}) {
  clearCachedSession();
  try {
    const session = await fetchLocalOmniDriveSession(api);
    return session.accessToken;
  } catch {
    // Fall through to explicit development fallbacks when the local bridge is unavailable.
  }

  const explicitToken = String(overrides.accessToken || "").trim();
  if (explicitToken) {
    return explicitToken;
  }

  const cfg = resolveConfig(api);
  if (cfg.accessToken) {
    cachedSession = {
      accessToken: cfg.accessToken,
      user: null,
      email: cfg.email || null,
      source: "config_access_token",
      loggedInAt: null,
      apiBaseUrl: cfg.baseUrl,
      cloudUrl: cfg.baseUrl,
      authState: "authorized",
      reason: "",
      availableSkills: [],
    };
    return cfg.accessToken;
  }

  const hasCredentials =
    Boolean(String(overrides.email || "").trim() && String(overrides.password || "").trim()) ||
    Boolean(cfg.email && cfg.password);
  if (hasCredentials) {
    const session = await performLogin(api, overrides);
    return session.accessToken;
  }

  throw new Error("OmniDrive 会话已失效，且无法从本地 OmniBull 刷新会话");
}

async function requestJson(api, path, options = {}, authOptions = {}) {
  const authEnabled = authOptions.auth !== false;
  let accessToken = "";
  if (authEnabled) {
    accessToken = await ensureAccessToken(api, authOptions);
  }

  let response = await rawRequest(api, path, options, accessToken);
  if (
    authEnabled &&
    response.status === 401 &&
    authOptions.retryOnAuth !== false
  ) {
    accessToken = await refreshAccessTokenAfterUnauthorized(api, authOptions);
    response = await rawRequest(api, path, options, accessToken);
  }

  if (!response.ok) {
    throw new Error(extractErrorMessage(response.payload, response.status));
  }
  return response.payload;
}

async function getCurrentUser(api, overrides = {}) {
  return requestJson(api, "/api/v1/auth/me", { method: "GET" }, overrides);
}

function summarizeArtifacts(artifacts) {
  return (artifacts || []).map((item) => ({
    id: item.id,
    artifactKey: item.artifactKey,
    artifactType: item.artifactType,
    title: item.title || null,
    fileName: item.fileName || null,
    mimeType: item.mimeType || null,
    publicUrl: item.publicUrl || null,
    textContent: item.textContent || null,
    sizeBytes: item.sizeBytes ?? null,
  }));
}

function extractOutputText(workspace) {
  const output = workspace?.job?.outputPayload || null;
  if (output && typeof output.text === "string" && output.text.trim()) {
    return output.text.trim();
  }
  const textArtifact = (workspace?.artifacts || []).find(
    (item) => item.artifactType === "text" && typeof item.textContent === "string" && item.textContent.trim(),
  );
  return textArtifact?.textContent || null;
}

function extractPublicUrls(workspace) {
  return (workspace?.artifacts || [])
    .map((item) => item.publicUrl)
    .filter((value) => typeof value === "string" && value.trim());
}

function findDeviceByCode(items, deviceCode) {
  return (items || []).find((item) => String(item?.deviceCode || "").trim() === String(deviceCode || "").trim()) || null;
}

function summarizeWorkspace(workspace) {
  const job = workspace?.job || {};
  return {
    jobId: job.id || null,
    jobType: job.jobType || null,
    modelName: job.modelName || null,
    status: job.status || null,
    message: job.message || null,
    createdAt: job.createdAt || null,
    finishedAt: job.finishedAt || null,
    text: extractOutputText(workspace),
    publicUrls: extractPublicUrls(workspace),
    artifacts: summarizeArtifacts(workspace?.artifacts),
    billingUsageEvents: workspace?.billingUsageEvents || [],
    bridge: workspace?.bridge || null,
    actions: workspace?.actions || null,
  };
}

function normalizeArtifactDetails(item) {
  if (!item || typeof item !== "object") {
    return null;
  }
  const artifactType = String(item.artifactType || "").trim();
  const mimeType = String(item.mimeType || "").trim();
  const fileName = String(item.fileName || item.title || "").trim();
  const publicUrl = String(item.publicUrl || item.url || "").trim();
  const textContent = String(item.textContent || "").trim();
  return {
    artifactType,
    mimeType,
    fileName,
    publicUrl,
    textContent,
  };
}

function collectMediaResultDetails(result = {}) {
  const job = result?.job && typeof result.job === "object" ? result.job : {};
  const workspace = result?.workspace && typeof result.workspace === "object" ? result.workspace : {};
  const rawArtifacts = [];
  for (const source of [workspace?.artifacts, result?.artifacts]) {
    if (!Array.isArray(source)) {
      continue;
    }
    for (const item of source) {
      const normalized = normalizeArtifactDetails(item);
      if (normalized) {
        rawArtifacts.push(normalized);
      }
    }
  }

  const publicUrls = [];
  const seenUrls = new Set();
  for (const source of [workspace?.publicUrls, result?.publicUrls]) {
    if (!Array.isArray(source)) {
      continue;
    }
    for (const item of source) {
      const url = String(item || "").trim();
      if (url && !seenUrls.has(url)) {
        seenUrls.add(url);
        publicUrls.push(url);
      }
    }
  }
  for (const artifact of rawArtifacts) {
    if (artifact.publicUrl && !seenUrls.has(artifact.publicUrl)) {
      seenUrls.add(artifact.publicUrl);
      publicUrls.push(artifact.publicUrl);
    }
  }

  return {
    jobId: String(job?.id || workspace?.jobId || "").trim(),
    jobType: String(job?.jobType || workspace?.jobType || "").trim().toLowerCase(),
    modelName: String(job?.modelName || workspace?.modelName || "").trim(),
    status: String(workspace?.status || job?.status || "").trim(),
    message: String(workspace?.message || result?.message || "").trim(),
    text: String(workspace?.text || result?.text || "").trim(),
    nextStep: String(result?.nextStep || "").trim(),
    publicUrls,
    artifacts: rawArtifacts,
  };
}

function inferMediaJobType(preferredJobType, details) {
  const explicit = String(preferredJobType || details?.jobType || "").trim().toLowerCase();
  if (explicit === "image" || explicit === "video") {
    return explicit;
  }
  for (const artifact of details?.artifacts || []) {
    if (artifact.mimeType.startsWith("image/")) {
      return "image";
    }
    if (artifact.mimeType.startsWith("video/")) {
      return "video";
    }
  }
  return "";
}

function summarizeMediaToolPayload(preferredJobType, result = {}) {
  const details = collectMediaResultDetails(result);
  const jobType = inferMediaJobType(preferredJobType, details);
  const noun = jobType === "video" ? "视频" : "图片";
  const lines = [];

  if (details.publicUrls.length > 0) {
    lines.push(`${noun}已生成完成。`);
  } else if (details.jobId) {
    lines.push(`${noun}任务已提交。`);
  } else {
    lines.push(`${noun}请求已处理。`);
  }

  if (details.jobId) {
    lines.push(`任务 ID：${details.jobId}`);
  }
  if (details.modelName) {
    lines.push(`模型：${details.modelName}`);
  }
  if (details.status) {
    lines.push(`状态：${details.status}`);
  }
  if (details.publicUrls.length > 0) {
    lines.push("结果地址：");
    lines.push(...details.publicUrls.slice(0, 3));
  } else if (details.nextStep) {
    lines.push(details.nextStep);
  }
  if (details.artifacts.length > 0) {
    lines.push("结果明细：");
    for (const artifact of details.artifacts.slice(0, 5)) {
      const typeLabel = artifact.artifactType || "artifact";
      const fileLabel = artifact.fileName || "unnamed";
      let line = `- ${typeLabel} ${fileLabel}`;
      if (artifact.mimeType) {
        line += ` (${artifact.mimeType})`;
      }
      if (artifact.publicUrl) {
        line += `: ${artifact.publicUrl}`;
      } else if (artifact.textContent) {
        line += `: ${artifact.textContent.slice(0, 120)}`;
      }
      lines.push(line);
    }
  }
  if (details.message) {
    lines.push(`消息：${details.message}`);
  }
  if (details.text) {
    lines.push(details.text);
  }

  return {
    jobType,
    details,
    summaryText: lines.join("\n"),
  };
}

function inferImageMimeType(url, fallbackMimeType = "") {
  const normalizedFallback = String(fallbackMimeType || "").trim().split(";")[0].trim().toLowerCase();
  if (normalizedFallback.startsWith("image/")) {
    return normalizedFallback;
  }
  const lowerUrl = String(url || "").trim().toLowerCase();
  if (lowerUrl.endsWith(".png")) {
    return "image/png";
  }
  if (lowerUrl.endsWith(".jpg") || lowerUrl.endsWith(".jpeg")) {
    return "image/jpeg";
  }
  if (lowerUrl.endsWith(".webp")) {
    return "image/webp";
  }
  if (lowerUrl.endsWith(".gif")) {
    return "image/gif";
  }
  return "";
}

function collectEmbeddedImageCandidates(details) {
  const candidates = [];
  const seen = new Set();
  for (const artifact of details?.artifacts || []) {
    if (!artifact.publicUrl) {
      continue;
    }
    const mimeType = inferImageMimeType(artifact.publicUrl, artifact.mimeType);
    if (!mimeType.startsWith("image/")) {
      continue;
    }
    if (seen.has(artifact.publicUrl)) {
      continue;
    }
    seen.add(artifact.publicUrl);
    candidates.push({
      url: artifact.publicUrl,
      mimeType,
    });
  }
  for (const url of details?.publicUrls || []) {
    if (seen.has(url)) {
      continue;
    }
    const mimeType = inferImageMimeType(url);
    if (!mimeType.startsWith("image/")) {
      continue;
    }
    seen.add(url);
    candidates.push({ url, mimeType });
  }
  return candidates;
}

async function fetchImageContentBlock(url, fallbackMimeType = "", options = {}) {
  const fetchImpl = typeof options.fetchImpl === "function" ? options.fetchImpl : fetch;
  const timeoutMs = Number(options.timeoutMs || TOOL_MEDIA_FETCH_TIMEOUT_MS);
  const maxBytes = Number(options.maxBytes || TOOL_MAX_EMBEDDED_IMAGE_BYTES);
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetchImpl(url, { signal: controller.signal });
    ensure(response?.ok, `加载图片失败: ${url}`);

    const contentLength = Number(response.headers?.get?.("content-length") || 0);
    if (Number.isFinite(contentLength) && contentLength > maxBytes) {
      throw new Error(`图片过大，无法内嵌预览: ${url}`);
    }

    const buffer = Buffer.from(await response.arrayBuffer());
    if (buffer.length > maxBytes) {
      throw new Error(`图片过大，无法内嵌预览: ${url}`);
    }

    const mimeType = inferImageMimeType(url, response.headers?.get?.("content-type") || fallbackMimeType);
    ensure(mimeType.startsWith("image/"), `结果不是图片资源: ${url}`);
    return {
      type: "image",
      mimeType,
      data: buffer.toString("base64"),
    };
  } finally {
    clearTimeout(timeout);
  }
}

async function buildMediaToolResultContent(preferredJobType, result = {}, options = {}) {
  const summary = summarizeMediaToolPayload(preferredJobType, result);
  const content = [textBlock(summary.summaryText || JSON.stringify(result, null, 2))];
  if (summary.jobType !== "image") {
    return content;
  }

  let embeddedCount = 0;
  for (const candidate of collectEmbeddedImageCandidates(summary.details)) {
    if (embeddedCount >= TOOL_MAX_EMBEDDED_IMAGE_COUNT) {
      break;
    }
    try {
      content.push(await fetchImageContentBlock(candidate.url, candidate.mimeType, options));
      embeddedCount += 1;
    } catch {
      // Keep the tool result usable even if the preview fetch fails.
    }
  }

  return content;
}

async function buildMediaToolResult(preferredJobType, result = {}, options = {}) {
  return toolResult(result, await buildMediaToolResultContent(preferredJobType, result, options));
}

function normalizeReferenceImages(items) {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
    .map((item) => {
      if (typeof item === "string") {
        return { url: item };
      }
      if (item && typeof item === "object") {
        return item;
      }
      return null;
    })
    .filter(Boolean);
}

async function resolveLocalDeviceCode(api) {
  const cfg = resolveConfig(api);
  if (cfg.localDeviceCode) {
    return cfg.localDeviceCode;
  }
  const payload = await requestLocalOmniBull(api, "/api/skill/status", { method: "GET" });
  const deviceCode = String(payload?.data?.deviceCode || payload?.deviceCode || "").trim();
  ensure(deviceCode, "无法从本地 OmniBull 读取 deviceCode，请检查本地插件配置");
  return deviceCode;
}

async function resolveBoundOmniBullDevice(api, overrides = {}) {
  const localDeviceCode = await resolveLocalDeviceCode(api);
  const devices = await requestJson(api, "/api/v1/devices", { method: "GET" }, overrides);
  const device = findDeviceByCode(devices, localDeviceCode);
  ensure(device, "当前 OpenClaw 所在 OmniBull 尚未绑定到当前 OmniDrive 账户，无法使用云端 AI");
  ensure(device.isEnabled !== false, "当前 OmniBull 设备已被停用或解绑，无法使用云端 AI");
  return device;
}

async function createAIJob(api, payload, overrides = {}) {
  return requestJson(
    api,
    "/api/v1/ai/jobs",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    overrides,
  );
}

function buildDirectMainChatMessages(params = {}) {
  const prompt = typeof params.prompt === "string" ? params.prompt.trim() : "";
  const messages = Array.isArray(params.messages)
    ? params.messages
        .filter((item) => item && typeof item === "object" && !Array.isArray(item))
        .map((item) => ({
          ...item,
          role: String(item.role || "").trim() || undefined,
          content: item.content,
        }))
        .filter((item) => item.role && item.content !== undefined)
    : [];
  const systemPrompt = typeof params.systemPrompt === "string" ? params.systemPrompt.trim() : "";

  if (messages.length > 0) {
    return systemPrompt
      ? [{ role: "system", content: systemPrompt }, ...messages]
      : messages;
  }

  ensure(prompt, "chat 需要 prompt 或 messages");
  if (systemPrompt) {
    return [
      { role: "system", content: systemPrompt },
      { role: "user", content: prompt },
    ];
  }
  return [{ role: "user", content: prompt }];
}

function buildDirectMainChatPayload(params = {}, modelName) {
  const payload = {
    model: String(modelName || "").trim(),
    messages: buildDirectMainChatMessages(params),
  };
  if (params.temperature !== undefined) {
    payload.temperature = params.temperature;
  }
  if (params.maxTokens !== undefined) {
    payload.max_tokens = params.maxTokens;
  }
  return payload;
}

function flattenOpenAIMessageContent(content) {
  if (typeof content === "string") {
    return content.trim();
  }
  if (!Array.isArray(content)) {
    return "";
  }
  return content
    .map((item) => {
      if (typeof item === "string") {
        return item;
      }
      if (item && typeof item === "object") {
        if (typeof item.text === "string") {
          return item.text;
        }
        if (item.type === "text" && typeof item.content === "string") {
          return item.content;
        }
      }
      return "";
    })
    .join("")
    .trim();
}

function extractDirectMainChatText(payload) {
  const choice = Array.isArray(payload?.choices) ? payload.choices[0] : null;
  const messageContent = flattenOpenAIMessageContent(choice?.message?.content);
  if (messageContent) {
    return messageContent;
  }
  const textContent = flattenOpenAIMessageContent(choice?.text);
  if (textContent) {
    return textContent;
  }
  return "";
}

async function requestDirectMainChatCompletion(api, payload, overrides = {}) {
  let accessToken = await ensureAccessToken(api, overrides);
  let response = await rawRequest(
    api,
    "/openai/v1/chat/completions",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    accessToken,
  );

  if (response.status === 401 && overrides.retryOnAuth !== false) {
    accessToken = await refreshAccessTokenAfterUnauthorized(api, overrides);
    response = await rawRequest(
      api,
      "/openai/v1/chat/completions",
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
      accessToken,
    );
  }

  if (response.ok) {
    return response.payload;
  }

  const message = extractErrorMessage(response.payload, response.status);
  if (response.status === 401 || response.status === 403) {
    throw createFallbackGatewayError(message, "omnidrive_session_unavailable", { status: response.status });
  }
  if (response.status === 404) {
    throw createFallbackGatewayError(message, "omnidrive_model_unavailable", { status: response.status });
  }
  if (response.status >= 500) {
    throw createFallbackGatewayError(message, "omnidrive_cloud_unavailable", { status: response.status });
  }
  throw createFallbackGatewayError(message, "omnidrive_cloud_unavailable", { status: response.status });
}

async function fetchJobWorkspace(api, jobId, overrides = {}) {
  ensure(jobId, "缺少 jobId");
  return requestJson(api, `/api/v1/ai/jobs/${encodeURIComponent(jobId)}/workspace`, { method: "GET" }, overrides);
}

async function updateBoundDevice(api, payload, overrides = {}) {
  const boundDevice = await resolveBoundOmniBullDevice(api, overrides);
  const body = {};
  if (payload.name !== undefined) {
    body.name = payload.name;
  }
  if (payload.defaultReasoningModel !== undefined) {
    body.defaultReasoningModel = payload.defaultReasoningModel;
  }
  if (payload.defaultChatModel !== undefined) {
    body.defaultChatModel = payload.defaultChatModel;
  }
  if (payload.defaultImageModel !== undefined) {
    body.defaultImageModel = payload.defaultImageModel;
  }
  if (payload.defaultVideoModel !== undefined) {
    body.defaultVideoModel = payload.defaultVideoModel;
  }
  if (payload.isEnabled !== undefined) {
    body.isEnabled = payload.isEnabled;
  }
  return requestJson(
    api,
    `/api/v1/devices/${encodeURIComponent(boundDevice.id)}`,
    {
      method: "PATCH",
      body: JSON.stringify(body),
    },
    overrides,
  );
}

async function pollWorkspaceUntilFinal(api, jobId, options = {}, fetchWorkspace = fetchJobWorkspace) {
  const timeoutMs = Number(options.timeoutMs || 0) > 0 ? Number(options.timeoutMs) : 60000;
  const pollIntervalMs = Number(options.pollIntervalMs || 0) > 0 ? Number(options.pollIntervalMs) : 2500;
  const observationPollIntervalMs =
    Number(options.observationPollIntervalMs || 0) > 0
      ? Number(options.observationPollIntervalMs)
      : LONG_JOB_OBSERVATION_POLL_INTERVAL_MS;
  const startedAt = Date.now();

  while (true) {
    const workspace = await fetchWorkspace(api, jobId, options);
    const status = String(workspace?.job?.status || "").trim();
    if (FINAL_AI_JOB_STATUSES.has(status)) {
      return {
        workspace,
        terminal: true,
        waitExpired: false,
        observationExpired: false,
      };
    }

    const observationRemainingMs = getLongJobObservationRemainingMs(workspace?.job, Date.now());
    if (observationRemainingMs === 0) {
      return {
        workspace,
        terminal: false,
        waitExpired: true,
        observationExpired: true,
      };
    }
    if (Date.now() - startedAt >= timeoutMs) {
      return {
        workspace,
        terminal: false,
        waitExpired: true,
        observationExpired: false,
      };
    }

    const elapsedMs = Date.now() - startedAt;
    const remainingTimeoutMs = Math.max(1, timeoutMs - elapsedMs);
    let effectivePollIntervalMs = pollIntervalMs;
    if (Number.isFinite(observationRemainingMs)) {
      effectivePollIntervalMs = Math.min(observationPollIntervalMs, observationRemainingMs);
    }
    effectivePollIntervalMs = Math.min(effectivePollIntervalMs, remainingTimeoutMs);
    await new Promise((resolve) => setTimeout(resolve, Math.max(1, effectivePollIntervalMs)));
  }
}

async function buildAuthStatusPayload(api, params = {}) {
  const cfg = resolveConfig(api);
  let authenticated = false;
  let authSource = cachedSession?.source || null;
  let boundDevice = null;
  let sessionUser = cachedSession?.user || null;
  let availableSkills = Array.isArray(cachedSession?.availableSkills) ? cachedSession.availableSkills : [];
  let authState = String(cachedSession?.authState || "").trim() || "blocked";
  let authReason = String(cachedSession?.reason || "").trim();

  try {
    await ensureAccessToken(api, params || {});
    authenticated = Boolean(cachedSession?.accessToken || cfg.accessToken);
    authSource = cachedSession?.source || (cfg.accessToken ? "config_access_token" : null);
    sessionUser = cachedSession?.user || null;
    availableSkills = Array.isArray(cachedSession?.availableSkills) ? cachedSession.availableSkills : availableSkills;
    authState = String(cachedSession?.authState || "").trim() || "authorized";
    authReason = String(cachedSession?.reason || "").trim();
    boundDevice = await resolveBoundOmniBullDevice(api, params || {});
  } catch {
    boundDevice = null;
  }

  const headlessAgentSessionActive = authSource === "local_agent_session";
  let gatewayBlockedReason = null;
  if (!authenticated) {
    gatewayBlockedReason = "OmniDrive 会话不可用";
  } else if (!boundDevice) {
    gatewayBlockedReason = "当前 OpenClaw 所在 OmniBull 未绑定或未启用";
  }
  if (!authenticated) {
    authState = "blocked";
  } else if (!authState) {
    authState = "authorized";
  }
  if (!authReason) {
    authReason = gatewayBlockedReason || "";
  }
  if (authenticated && boundDevice && availableSkills.length === 0) {
    availableSkills = [...OPENCLAW_AVAILABLE_SKILLS];
  }

  return {
    authenticated,
    authState,
    reason: authReason,
    authSource,
    supportsHeadlessAgentSession: true,
    headlessAgentSessionActive,
    manualCredentialsConfigured: Boolean(cfg.accessToken || (cfg.email && cfg.password)),
    manualCredentialsRequired: !authenticated && !headlessAgentSessionActive,
    hasLocalDeviceCode: Boolean(cfg.localDeviceCode),
    cachedSession: summarizeCachedSession(cachedSession),
    sessionUser: summarizeSessionUser(sessionUser),
    baseUrl: cfg.baseUrl,
    effectiveBaseUrl: cachedSession?.apiBaseUrl || cachedSession?.cloudUrl || cfg.baseUrl,
    localOmniBullBaseUrl: cfg.localOmniBullBaseUrl,
    boundDevice: summarizeBoundDevice(boundDevice),
    availableSkills,
    recommendedMainChatRoute: {
      mode: "gateway",
      gatewayMethod: "omnidrive.chat",
      statusMethod: "omnidrive.status",
      ready: Boolean(authenticated && boundDevice),
      blockedReason: gatewayBlockedReason,
      requestSource: OPENCLAW_MAIN_CHAT_SOURCE,
      modelSource: boundDevice?.defaultChatModel ? "bound_device.defaultChatModel" : "plugin_default_fallback",
    },
  };
}

async function executeAuth(api, params) {
  const action = String(params.action || "status").trim();
  if (action === "status") {
    return toolResult(await buildAuthStatusPayload(api, params || {}));
  }
  if (action === "logout") {
    clearCachedSession();
    return toolResult({ success: true, authenticated: false });
  }
  if (action === "register") {
    const email = String(params.email || "").trim();
    const name = String(params.name || "").trim();
    const password = String(params.password || "").trim();
    ensure(email, "register 需要 email");
    ensure(name, "register 需要 name");
    ensure(password, "register 需要 password");
    const user = await requestJson(
      api,
      "/api/v1/auth/register",
      {
        method: "POST",
        body: JSON.stringify({ email, name, password }),
      },
      { auth: false },
    );
    return toolResult({ success: true, user });
  }
  if (action === "login") {
    const session = await performLogin(api, params || {});
    const user = await getCurrentUser(api, { accessToken: session.accessToken, retryOnAuth: false });
    cachedSession.user = user;
    let boundDevice = null;
    try {
      boundDevice = await resolveBoundOmniBullDevice(api, { accessToken: session.accessToken, retryOnAuth: false });
    } catch {
      boundDevice = null;
    }
    return toolResult({
      success: true,
      accessTokenPreview: `${session.accessToken.slice(0, 10)}...`,
      user,
      source: session.source,
      boundDevice: summarizeBoundDevice(boundDevice),
    });
  }
  if (action === "me") {
    const user = await getCurrentUser(api, params || {});
    if (cachedSession) {
      cachedSession.user = user;
    }
    let boundDevice = null;
    try {
      boundDevice = await resolveBoundOmniBullDevice(api, params || {});
    } catch {
      boundDevice = null;
    }
    return toolResult({
      user,
      boundDevice: summarizeBoundDevice(boundDevice),
    });
  }
  throw new Error(`不支持的 auth action: ${action}`);
}

async function executeModels(api, params) {
  const category = String(params.category || "").trim();
  const items = await requestJson(
    api,
    `/api/v1/ai/models${buildQuery({ category })}`,
    { method: "GET" },
    params || {},
  );
  return toolResult(items);
}

async function executeDeviceConfig(api, params) {
  const action = String(params.action || "status").trim();
  if (action === "status") {
    const boundDevice = await resolveBoundOmniBullDevice(api, params || {});
    return toolResult({
      device: {
        id: boundDevice.id,
        deviceCode: boundDevice.deviceCode,
        name: boundDevice.name,
        isEnabled: boundDevice.isEnabled,
        defaultReasoningModel: boundDevice.defaultReasoningModel || null,
        defaultChatModel: boundDevice.defaultChatModel || null,
        defaultImageModel: boundDevice.defaultImageModel || null,
        defaultVideoModel: boundDevice.defaultVideoModel || null,
      },
    });
  }

  if (action === "set_defaults") {
    const hasAnyUpdate =
      params.defaultReasoningModel !== undefined ||
      params.defaultChatModel !== undefined ||
      params.defaultImageModel !== undefined ||
      params.defaultVideoModel !== undefined;
    ensure(hasAnyUpdate, "set_defaults 至少需要一个默认模型字段");
    const updated = await updateBoundDevice(
      api,
      {
        defaultReasoningModel:
          params.defaultReasoningModel !== undefined ? String(params.defaultReasoningModel || "").trim() : undefined,
        defaultChatModel:
          params.defaultChatModel !== undefined ? String(params.defaultChatModel || "").trim() : undefined,
        defaultImageModel:
          params.defaultImageModel !== undefined ? String(params.defaultImageModel || "").trim() : undefined,
        defaultVideoModel:
          params.defaultVideoModel !== undefined ? String(params.defaultVideoModel || "").trim() : undefined,
      },
      params || {},
    );
    return toolResult({
      success: true,
      device: {
        id: updated.id,
        deviceCode: updated.deviceCode,
        name: updated.name,
        isEnabled: updated.isEnabled,
        defaultReasoningModel: updated.defaultReasoningModel || null,
        defaultChatModel: updated.defaultChatModel || null,
        defaultImageModel: updated.defaultImageModel || null,
        defaultVideoModel: updated.defaultVideoModel || null,
      },
    });
  }

  throw new Error(`不支持的 device config action: ${action}`);
}

async function runChat(api, params, options = {}) {
  const cfg = resolveConfig(api);
  const prompt = typeof params.prompt === "string" ? params.prompt.trim() : "";
  const messages = Array.isArray(params.messages) ? params.messages : null;
  ensure(prompt || (messages && messages.length > 0), "chat 需要 prompt 或 messages");
  const boundDevice = await resolveBoundOmniBullDevice(api, params || {});
  const requestSource = normalizeChatSource(options.source || params.source, OPENCLAW_SKILL_CHAT_SOURCE);
  if (params.deviceId && String(params.deviceId).trim() !== String(boundDevice.id || "").trim()) {
    throw new Error("OpenClaw OmniDrive chat 只能使用当前本机已绑定的 OmniBull 设备");
  }

  const payload = {
    source: requestSource,
    jobType: "chat",
    modelName: String(params.modelName || boundDevice.defaultChatModel || cfg.defaultChatModel || DEFAULT_CHAT_MODEL).trim(),
    prompt: prompt || undefined,
    deviceId: boundDevice.id,
    skillId: params.skillId || undefined,
    inputPayload: {
      ...(messages ? { messages } : {}),
      ...(params.systemPrompt ? { systemPrompt: String(params.systemPrompt) } : {}),
      ...(params.temperature !== undefined ? { temperature: params.temperature } : {}),
      ...(params.maxTokens !== undefined ? { maxTokens: params.maxTokens } : {}),
    },
  };

  const job = await createAIJob(api, payload, params || {});
  const wait = params.wait !== false;
  if (!wait) {
    return {
      source: "omnidrive",
      gatewayMethod: "omnidrive.chat",
      requestSource,
      waited: false,
      device: summarizeBoundDevice(boundDevice),
      effectiveModelName: job?.modelName || payload.modelName,
      job,
      nextStep: "使用 omnidrive_job_detail 查询结果",
    };
  }

  const workspace = await pollWorkspaceUntilFinal(api, job.id, params || {});
  return {
    source: "omnidrive",
    gatewayMethod: "omnidrive.chat",
    requestSource,
    waited: true,
    device: summarizeBoundDevice(boundDevice),
    effectiveModelName: workspace?.job?.modelName || job?.modelName || payload.modelName,
    job,
    workspace: summarizeWorkspace(workspace),
    text: extractOutputText(workspace),
  };
}

async function runGatewayChat(api, params, options = {}) {
  try {
    const cfg = resolveConfig(api);
    const boundDevice = await resolveBoundOmniBullDevice(api, params || {});
    const requestSource = normalizeChatSource(options.source || params.source, OPENCLAW_MAIN_CHAT_SOURCE);
    if (params.deviceId && String(params.deviceId).trim() !== String(boundDevice.id || "").trim()) {
      throw createFallbackGatewayError(
        "OpenClaw OmniDrive chat 只能使用当前本机已绑定的 OmniBull 设备",
        "omnidrive_device_unbound",
      );
    }

    const modelName = String(
      params.modelName || boundDevice.defaultChatModel || cfg.defaultChatModel || DEFAULT_CHAT_MODEL,
    ).trim();
    const requestPayload = buildDirectMainChatPayload(params, modelName);
    const completion = await requestDirectMainChatCompletion(api, requestPayload, params || {});
    const text = extractDirectMainChatText(completion);
    ensure(text, "OmniDrive 主聊天未返回有效文本结果");

    return {
      source: "omnidrive",
      gatewayMethod: "omnidrive.chat",
      requestSource,
      waited: true,
      device: summarizeBoundDevice(boundDevice),
      effectiveModelName: String(completion?.model || modelName).trim() || modelName,
      text,
    };
  } catch (error) {
    throw classifyMainChatFallbackError(error);
  }
}

async function executeChat(api, params) {
  return toolResult(await runChat(api, params, { source: OPENCLAW_SKILL_CHAT_SOURCE }));
}

async function executeImage(api, params) {
  const cfg = resolveConfig(api);
  const prompt = typeof params.prompt === "string" ? params.prompt.trim() : "";
  ensure(prompt, "image 需要 prompt");
  const boundDevice = await resolveBoundOmniBullDevice(api, params || {});
  if (params.deviceId && String(params.deviceId).trim() !== String(boundDevice.id || "").trim()) {
    throw new Error("OpenClaw OmniDrive image 只能使用当前本机已绑定的 OmniBull 设备");
  }

  const payload = {
    source: "openclaw_skill",
    jobType: "image",
    modelName: String(params.modelName || boundDevice.defaultImageModel || cfg.defaultImageModel || DEFAULT_IMAGE_MODEL).trim(),
    prompt,
    deviceId: boundDevice.id,
    skillId: params.skillId || undefined,
    inputPayload: {
      ...(params.aspectRatio ? { aspectRatio: String(params.aspectRatio) } : {}),
      ...(params.resolution ? { resolution: String(params.resolution) } : {}),
      ...(normalizeReferenceImages(params.referenceImages).length > 0
        ? { referenceImages: normalizeReferenceImages(params.referenceImages) }
        : {}),
    },
  };

  const job = await createAIJob(api, payload, params || {});
  const wait = params.wait !== false;
  if (!wait) {
    return buildMediaToolResult("image", { job, nextStep: "使用 omnidrive_job_detail 查询结果" });
  }

  const pollResult = await pollWorkspaceUntilFinal(api, job.id, {
    ...params,
    timeoutMs: params.timeoutMs || 120000,
    pollIntervalMs: params.pollIntervalMs || 3000,
  });
  return buildMediaToolResult("image", {
    job,
    workspace: summarizeWorkspace(pollResult.workspace),
    ...(pollResult.waitExpired && !pollResult.terminal
      ? { waitExpired: true, nextStep: "图片仍在生成中，请稍后使用 omnidrive_job_detail 查询结果" }
      : {}),
  });
}

async function executeVideo(api, params) {
  const cfg = resolveConfig(api);
  const prompt = typeof params.prompt === "string" ? params.prompt.trim() : "";
  ensure(prompt, "video 需要 prompt");
  const boundDevice = await resolveBoundOmniBullDevice(api, params || {});
  if (params.deviceId && String(params.deviceId).trim() !== String(boundDevice.id || "").trim()) {
    throw new Error("OpenClaw OmniDrive video 只能使用当前本机已绑定的 OmniBull 设备");
  }

  const payload = {
    source: "openclaw_skill",
    jobType: "video",
    modelName: String(params.modelName || boundDevice.defaultVideoModel || cfg.defaultVideoModel || DEFAULT_VIDEO_MODEL).trim(),
    prompt,
    deviceId: boundDevice.id,
    skillId: params.skillId || undefined,
    inputPayload: {
      ...(params.aspectRatio ? { aspectRatio: String(params.aspectRatio) } : {}),
      ...(params.resolution ? { resolution: String(params.resolution) } : {}),
      durationSeconds:
        params.durationSeconds !== undefined
          ? Number(params.durationSeconds)
          : Number(cfg.defaultVideoDurationSeconds || DEFAULT_VIDEO_DURATION_SECONDS),
      ...(normalizeReferenceImages(params.referenceImages).length > 0
        ? { referenceImages: normalizeReferenceImages(params.referenceImages) }
        : {}),
      ...buildPublishMetadataInputPayload(params || {}),
    },
  };

  const job = await createAIJob(api, payload, params || {});
  const wait = params.wait === true;
  if (!wait) {
    return buildMediaToolResult("video", { job, nextStep: "视频默认异步生成，请使用 omnidrive_job_detail 轮询结果" });
  }

  const pollResult = await pollWorkspaceUntilFinal(api, job.id, {
    ...params,
    timeoutMs: params.timeoutMs || LONG_JOB_OBSERVATION_WINDOW_MS,
    pollIntervalMs: params.pollIntervalMs || LONG_JOB_OBSERVATION_POLL_INTERVAL_MS,
  });
  const result = {
    job,
    workspace: summarizeWorkspace(pollResult.workspace),
  };
  if (pollResult.waitExpired && !pollResult.terminal) {
    result.waitExpired = true;
    result.nextStep =
      "任务仍在 OmniDrive 云端执行，请稍后使用 omnidrive_job_detail 查询。若已配置发布目标，生成完成后会继续自动创建 OmniBull 发布任务。";
  }
  return buildMediaToolResult("video", result);
}

async function readMixVideoAssetData(asset) {
  if (asset.absolutePath) {
    return {
      data: await readFile(asset.absolutePath),
      fileName: asset.fileName || path.basename(asset.absolutePath),
      mimeType: asset.mimeType || "",
    };
  }
  const response = await fetch(asset.url);
  ensure(response?.ok, `下载素材失败: ${asset.url}`);
  return {
    data: Buffer.from(await response.arrayBuffer()),
    fileName:
      asset.fileName ||
      path.basename(new URL(asset.url).pathname || "asset.bin") ||
      "asset.bin",
    mimeType: String(response.headers?.get?.("content-type") || asset.mimeType || "").trim(),
  };
}

async function buildMixVideoCreateFormData(payload) {
  const formData = new FormData();
  formData.set("scriptText", payload.scriptText);
  for (const asset of payload.sourceVideos) {
    const file = await readMixVideoAssetData(asset);
    formData.append("assets[]", new Blob([file.data], { type: file.mimeType || undefined }), file.fileName || "asset.bin");
  }
  const refAudioFile = await readMixVideoAssetData(payload.refAudio);
  formData.set("refAudio", new Blob([refAudioFile.data], { type: refAudioFile.mimeType || undefined }), refAudioFile.fileName || "audio.bin");
  if (payload.publish) {
    formData.set("accountId", payload.publish.accountId);
    formData.set("platform", payload.publish.platform);
    formData.set("accountName", payload.publish.accountName);
    formData.set("publishAt", payload.publish.publishAt);
  }
  return formData;
}

function summarizeMixVideoToolPayload(action, payload = {}) {
  const lines = [];
  switch (String(action || "").trim()) {
    case "billing_preview":
      lines.push("混剪计费预估：");
      if (payload.estimatedDurationSeconds !== undefined) {
        lines.push(`预计时长：${payload.estimatedDurationSeconds}s`);
      }
      if (payload.estimatedCredits !== undefined) {
        lines.push(`预计积分：${payload.estimatedCredits}`);
      }
      if (payload.creditBalance !== undefined) {
        lines.push(`当前余额：${payload.creditBalance}`);
      }
      if (payload.shortfallCredits !== undefined && payload.shortfallCredits > 0) {
        lines.push(`还差积分：${payload.shortfallCredits}`);
      }
      break;
    case "tasks":
      lines.push("混剪任务列表：");
      for (const item of Array.isArray(payload) ? payload.slice(0, 10) : []) {
        lines.push(`- ${item.id || "unknown"} | ${item.status || "unknown"} | ${item.accountName || item.source || ""}`.trim());
      }
      break;
    case "task_detail":
      lines.push("混剪任务详情：");
      lines.push(`任务 ID：${payload.id || ""}`);
      lines.push(`状态：${payload.status || ""}`);
      if (payload.platform || payload.accountName) {
        lines.push(`发布目标：${payload.platform || ""} ${payload.accountName || ""}`.trim());
      }
      if (payload.resultAsset?.publicUrl) {
        lines.push(`成片地址：${payload.resultAsset.publicUrl}`);
      }
      if (payload.message) {
        lines.push(`消息：${payload.message}`);
      }
      break;
    default:
      lines.push("混剪任务已创建。");
      lines.push(`任务 ID：${payload.id || ""}`);
      lines.push(`状态：${payload.status || ""}`);
      if (payload.resultAsset?.publicUrl) {
        lines.push(`成片地址：${payload.resultAsset.publicUrl}`);
      }
      if (payload.accountName || payload.platform) {
        lines.push(`发布目标：${payload.platform || ""} ${payload.accountName || ""}`.trim());
      }
      break;
  }
  return {
    summaryText: lines.filter(Boolean).join("\n"),
    details: payload,
  };
}

async function executeMixVideo(api, params) {
  const action = String(params.action || "create").trim();
  switch (action) {
    case "billing_preview": {
      const scriptText = String(params.scriptText || "").trim();
      ensure(scriptText, "缺少 scriptText");
      const preview = await requestJson(
        api,
        `/api/v1/mix-video/tasks/billing-preview${buildQuery({ scriptText })}`,
        { method: "GET" },
        params || {},
      );
      return toolResult(preview, [textBlock(summarizeMixVideoToolPayload(action, preview).summaryText)]);
    }
    case "tasks": {
      const items = await requestJson(
        api,
        `/api/v1/mix-video/tasks${buildQuery({ status: params.status, limit: params.limit })}`,
        { method: "GET" },
        params || {},
      );
      return toolResult(items, [textBlock(summarizeMixVideoToolPayload(action, items).summaryText)]);
    }
    case "task_detail": {
      const taskId = String(params.taskId || "").trim();
      ensure(taskId, "缺少 taskId");
      const item = await requestJson(api, `/api/v1/mix-video/tasks/${encodeURIComponent(taskId)}`, { method: "GET" }, params || {});
      return toolResult(item, [textBlock(summarizeMixVideoToolPayload(action, item).summaryText)]);
    }
    case "create":
    default: {
      const payload = buildMixVideoCreateInput(params || {});
      const formData = await buildMixVideoCreateFormData(payload);
      const item = await requestJson(api, "/api/v1/mix-video/tasks", { method: "POST", body: formData }, params || {});
      return toolResult(item, [textBlock(summarizeMixVideoToolPayload(action, item).summaryText)]);
    }
  }
}

async function executeJobs(api, params) {
  const items = await requestJson(
    api,
    `/api/v1/ai/jobs${buildQuery({
      jobType: params.jobType,
      status: params.status,
      skillId: params.skillId,
      deviceId: params.deviceId,
      source: params.source,
      limit: params.limit,
    })}`,
    { method: "GET" },
    params || {},
  );
  return toolResult(items);
}

async function executeJobDetail(api, params) {
  const jobId = String(params.jobId || "").trim();
  ensure(jobId, "缺少 jobId");

  const wait = params.wait === true;
  const includeArtifacts = params.includeArtifacts === true;
  let workspace = null;
  let pollResult = null;

  if (wait) {
    pollResult = await pollWorkspaceUntilFinal(api, jobId, {
      ...params,
      timeoutMs: params.timeoutMs || LONG_JOB_OBSERVATION_WINDOW_MS,
      pollIntervalMs: params.pollIntervalMs || LONG_JOB_OBSERVATION_POLL_INTERVAL_MS,
    });
    workspace = pollResult.workspace;
  } else if (params.includeWorkspace !== false) {
    workspace = await fetchJobWorkspace(api, jobId, params || {});
  }

  const result = {};
  if (workspace) {
    result.workspace = summarizeWorkspace(workspace);
  } else {
    result.job = await requestJson(api, `/api/v1/ai/jobs/${encodeURIComponent(jobId)}`, { method: "GET" }, params || {});
  }

  if (includeArtifacts) {
    result.artifacts = await requestJson(
      api,
      `/api/v1/ai/jobs/${encodeURIComponent(jobId)}/artifacts`,
      { method: "GET" },
      params || {},
    );
  }
  if (pollResult?.waitExpired && !pollResult?.terminal) {
    result.waitExpired = true;
    result.nextStep =
      "任务仍在 OmniDrive 云端执行，请稍后继续使用 omnidrive_job_detail 查询。若已配置发布目标，生成完成后会继续自动创建 OmniBull 发布任务。";
  }
  const summary = summarizeMediaToolPayload("", result);
  if (summary.jobType === "image" || summary.jobType === "video") {
    return buildMediaToolResult(summary.jobType, result);
  }
  return toolResult(result);
}

const plugin = {
  id: "omnidrive",
  name: "OmniDrive",
  description: "OmniDrive cloud auth and AI tools for OpenClaw",
  register(api) {
    api.registerGatewayMethod("omnidrive.status", async (request = {}) => {
      const params = extractGatewayParams(request);
      try {
        const payload = await buildAuthStatusPayload(api, params);
        if (typeof request.respond === "function") {
          request.respond(true, payload);
          return;
        }
        return payload;
      } catch (error) {
        const failure = buildGatewayError("omnidrive.status", error);
        if (typeof request.respond === "function") {
          request.respond(false, failure);
          return;
        }
        return failure;
      }
    });

    api.registerGatewayMethod("omnidrive.chat", async (request = {}) => {
      const params = extractGatewayParams(request);
      try {
        const payload = await runGatewayChat(api, params, { source: OPENCLAW_MAIN_CHAT_SOURCE });
        if (typeof request.respond === "function") {
          request.respond(true, payload);
          return;
        }
        return payload;
      } catch (error) {
        const failure = buildGatewayError("omnidrive.chat", error);
        if (typeof request.respond === "function") {
          request.respond(false, failure);
          return;
        }
        return failure;
      }
    });

    api.registerTool({
      name: "omnidrive_auth",
      description: "登录 OmniDrive 云端账户，查询当前登录状态，或读取当前用户信息。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          action: { type: "string", enum: ["status", "login", "register", "me", "logout"] },
          email: { type: "string" },
          name: { type: "string" },
          password: { type: "string" },
          accessToken: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeAuth(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_models",
      description: "列出 OmniDrive 可用 AI 模型，可按 chat、image、video 分类筛选。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          category: { type: "string", enum: ["chat", "image", "video"] },
          accessToken: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeModels(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_device_config",
      description: "查看或更新当前绑定 OmniBull 设备的默认聊天、作图、视频模型配置。用户在 OmniDrive/OmniBull AI 上下文里说“切换模型”“当前是什么模型”时优先使用这个工具，而不是 OpenClaw 主模型设置。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          action: { type: "string", enum: ["status", "set_defaults"] },
          defaultReasoningModel: { type: "string" },
          defaultChatModel: { type: "string" },
          defaultImageModel: { type: "string" },
          defaultVideoModel: { type: "string" },
          accessToken: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeDeviceConfig(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_chat",
      description: "使用 OmniDrive 云端聊天模型执行文案生成、问答和内容整理。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          prompt: { type: "string" },
          messages: { type: "array", items: { type: "object" } },
          systemPrompt: { type: "string" },
          modelName: { type: "string" },
          deviceId: { type: "string" },
          skillId: { type: "string" },
          temperature: { type: "number" },
          maxTokens: { type: "integer", minimum: 1 },
          wait: { type: "boolean" },
          pollIntervalMs: { type: "integer", minimum: 500, maximum: 60000 },
          timeoutMs: { type: "integer", minimum: 1000, maximum: 600000 },
          accessToken: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeChat(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_image",
      description: "使用 OmniDrive 云端图片模型执行文生图或图生图。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          prompt: { type: "string" },
          modelName: { type: "string" },
          deviceId: { type: "string" },
          skillId: { type: "string" },
          aspectRatio: { type: "string" },
          resolution: { type: "string" },
          referenceImages: { type: "array", items: { type: ["string", "object"] } },
          wait: { type: "boolean" },
          pollIntervalMs: { type: "integer", minimum: 500, maximum: 60000 },
          timeoutMs: { type: "integer", minimum: 1000, maximum: 600000 },
          accessToken: { type: "string" },
        },
        required: ["prompt"],
      },
      async execute(_id, params) {
        return executeImage(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_video",
      description: "使用 OmniDrive 云端视频模型执行文生视频或图生视频。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          prompt: { type: "string" },
          modelName: { type: "string" },
          deviceId: { type: "string" },
          skillId: { type: "string" },
          aspectRatio: { type: "string" },
          resolution: { type: "string" },
          durationSeconds: { type: "integer", minimum: 1, maximum: 120 },
          referenceImages: { type: "array", items: { type: ["string", "object"] } },
          accountId: { type: "string" },
          platform: { type: "string" },
          accountName: { type: "string" },
          publishAt: { type: "string" },
          wait: { type: "boolean" },
          pollIntervalMs: { type: "integer", minimum: 500, maximum: 60000 },
          timeoutMs: { type: "integer", minimum: 1000, maximum: 1800000 },
          accessToken: { type: "string" },
        },
        required: ["prompt"],
      },
      async execute(_id, params) {
        return executeVideo(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_mix_video",
      description: "使用 OmniDrive 云端混剪能力创建混剪任务，查询计费、任务列表和任务详情。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          action: { type: "string", enum: ["create", "billing_preview", "tasks", "task_detail"] },
          scriptText: { type: "string" },
          sourceVideos: { type: "array", items: { type: "object" } },
          refAudio: { type: "object" },
          taskId: { type: "string" },
          status: { type: "string" },
          limit: { type: "integer", minimum: 1, maximum: 200 },
          accountId: { type: "string" },
          platform: { type: "string" },
          accountName: { type: "string" },
          publishAt: { type: "string" },
          accessToken: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeMixVideo(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_jobs",
      description: "查询 OmniDrive AI 任务列表，可按类型、状态、设备、技能过滤。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          jobType: { type: "string", enum: ["chat", "image", "video"] },
          status: { type: "string" },
          skillId: { type: "string" },
          deviceId: { type: "string" },
          source: { type: "string" },
          limit: { type: "integer", minimum: 1, maximum: 200 },
          accessToken: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeJobs(api, params || {});
      },
    });

    api.registerTool({
      name: "omnidrive_job_detail",
      description: "查询单个 OmniDrive AI 任务详情、workspace、artifacts，或等待任务完成。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          jobId: { type: "string" },
          includeWorkspace: { type: "boolean" },
          includeArtifacts: { type: "boolean" },
          wait: { type: "boolean" },
          pollIntervalMs: { type: "integer", minimum: 500, maximum: 60000 },
          timeoutMs: { type: "integer", minimum: 1000, maximum: 1800000 },
          accessToken: { type: "string" },
        },
        required: ["jobId"],
      },
      async execute(_id, params) {
        return executeJobDetail(api, params || {});
      },
    });
  },
};

export default plugin;
export {
  buildAuthStatusPayload,
  buildMediaToolResultContent,
  buildMixVideoCreateInput,
  buildPublishMetadataInputPayload,
  clearCachedSession,
  collectMediaResultDetails,
  fetchImageContentBlock,
  getLongJobObservationRemainingMs,
  inferMediaJobType,
  isLongRunningJobType,
  pollWorkspaceUntilFinal,
  requestJson,
  summarizeMixVideoToolPayload,
  summarizeMediaToolPayload,
};
