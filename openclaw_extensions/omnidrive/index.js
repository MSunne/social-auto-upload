const DEFAULT_BASE_URL = "http://127.0.0.1:8410";
const DEFAULT_TIMEOUT_MS = 45000;
const DEFAULT_LOCAL_OMNIBULL_BASE_URL = "http://127.0.0.1:5409";
const DEFAULT_LOCAL_OMNIBULL_TIMEOUT_MS = 10000;
const DEFAULT_CHAT_MODEL = "gemini-3.1-pro-preview";
const DEFAULT_IMAGE_MODEL = "gemini-3-pro-image-preview";
const DEFAULT_VIDEO_MODEL = "veo-3.1-fast-fl";
const DEFAULT_VIDEO_DURATION_SECONDS = 8;
const FINAL_AI_JOB_STATUSES = new Set(["success", "completed", "failed", "cancelled", "needs_verify"]);

let cachedSession = null;

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

function toolResult(data) {
  return {
    content: [
      {
        type: "text",
        text: JSON.stringify(data, null, 2),
      },
    ],
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
  if (options.body !== undefined && !headers["Content-Type"]) {
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
  cachedSession = {
    accessToken,
    user: data.user || null,
    email: data?.user?.email || null,
    source: "local_agent_session",
    loggedInAt: nowISO(),
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
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), cfg.timeoutMs);
  const headers = {
    Accept: "application/json",
    ...(options.headers || {}),
  };
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  if (options.body !== undefined && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }

  try {
    const response = await fetch(`${cfg.baseUrl}${path}`, {
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
    };
    return cfg.accessToken;
  }

  const session = await performLogin(api, overrides);
  return session.accessToken;
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
    const cfg = resolveConfig(api);
    if (cfg.email && cfg.password) {
      clearCachedSession();
      accessToken = await ensureAccessToken(api, authOptions);
      response = await rawRequest(api, path, options, accessToken);
    }
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

async function pollWorkspaceUntilFinal(api, jobId, options = {}) {
  const timeoutMs = Number(options.timeoutMs || 0) > 0 ? Number(options.timeoutMs) : 60000;
  const pollIntervalMs = Number(options.pollIntervalMs || 0) > 0 ? Number(options.pollIntervalMs) : 2500;
  const startedAt = Date.now();

  while (true) {
    const workspace = await fetchJobWorkspace(api, jobId, options);
    const status = String(workspace?.job?.status || "").trim();
    if (FINAL_AI_JOB_STATUSES.has(status)) {
      return workspace;
    }
    if (Date.now() - startedAt >= timeoutMs) {
      return workspace;
    }
    await new Promise((resolve) => setTimeout(resolve, pollIntervalMs));
  }
}

async function executeAuth(api, params) {
  const action = String(params.action || "status").trim();
  if (action === "status") {
    const cfg = resolveConfig(api);
    let authenticated = false;
    let authSource = cachedSession?.source || null;
    let boundDevice = null;
    let sessionUser = cachedSession?.user || null;
    try {
      await ensureAccessToken(api, params || {});
      authenticated = Boolean(cachedSession?.accessToken || cfg.accessToken);
      authSource = cachedSession?.source || (cfg.accessToken ? "config_access_token" : null);
      sessionUser = cachedSession?.user || null;
      boundDevice = await resolveBoundOmniBullDevice(api, params || {});
    } catch {
      boundDevice = null;
    }
    const headlessAgentSessionActive = authSource === "local_agent_session";
    return toolResult({
      authenticated,
      authSource,
      supportsHeadlessAgentSession: true,
      headlessAgentSessionActive,
      manualCredentialsConfigured: Boolean(cfg.accessToken || (cfg.email && cfg.password)),
      manualCredentialsRequired: !authenticated && !headlessAgentSessionActive,
      hasLocalDeviceCode: Boolean(cfg.localDeviceCode),
      cachedSession: cachedSession
        ? {
            email: cachedSession.email || null,
            source: cachedSession.source || null,
            loggedInAt: cachedSession.loggedInAt || null,
          }
        : null,
      sessionUser: sessionUser
        ? {
            id: sessionUser.id || null,
            email: sessionUser.email || null,
            name: sessionUser.name || null,
          }
        : null,
      baseUrl: cfg.baseUrl,
      localOmniBullBaseUrl: cfg.localOmniBullBaseUrl,
      boundDevice: boundDevice
        ? {
            id: boundDevice.id,
            deviceCode: boundDevice.deviceCode,
            name: boundDevice.name,
            isEnabled: boundDevice.isEnabled,
            defaultChatModel: boundDevice.defaultChatModel || null,
            defaultImageModel: boundDevice.defaultImageModel || null,
            defaultVideoModel: boundDevice.defaultVideoModel || null,
          }
        : null,
    });
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
      boundDevice: boundDevice
        ? {
            id: boundDevice.id,
            deviceCode: boundDevice.deviceCode,
            name: boundDevice.name,
            defaultChatModel: boundDevice.defaultChatModel || null,
            defaultImageModel: boundDevice.defaultImageModel || null,
            defaultVideoModel: boundDevice.defaultVideoModel || null,
          }
        : null,
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
      boundDevice: boundDevice
        ? {
            id: boundDevice.id,
            deviceCode: boundDevice.deviceCode,
            name: boundDevice.name,
            defaultChatModel: boundDevice.defaultChatModel || null,
            defaultImageModel: boundDevice.defaultImageModel || null,
            defaultVideoModel: boundDevice.defaultVideoModel || null,
          }
        : null,
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

async function executeChat(api, params) {
  const cfg = resolveConfig(api);
  const prompt = typeof params.prompt === "string" ? params.prompt.trim() : "";
  const messages = Array.isArray(params.messages) ? params.messages : null;
  ensure(prompt || (messages && messages.length > 0), "chat 需要 prompt 或 messages");
  const boundDevice = await resolveBoundOmniBullDevice(api, params || {});
  if (params.deviceId && String(params.deviceId).trim() !== String(boundDevice.id || "").trim()) {
    throw new Error("OpenClaw OmniDrive chat 只能使用当前本机已绑定的 OmniBull 设备");
  }

  const payload = {
    source: "openclaw_skill",
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
    return toolResult({ job, nextStep: "使用 omnidrive_job_detail 查询结果" });
  }

  const workspace = await pollWorkspaceUntilFinal(api, job.id, params || {});
  return toolResult({
    job,
    workspace: summarizeWorkspace(workspace),
  });
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
    return toolResult({ job, nextStep: "使用 omnidrive_job_detail 查询结果" });
  }

  const workspace = await pollWorkspaceUntilFinal(api, job.id, {
    ...params,
    timeoutMs: params.timeoutMs || 120000,
    pollIntervalMs: params.pollIntervalMs || 3000,
  });
  return toolResult({
    job,
    workspace: summarizeWorkspace(workspace),
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
    },
  };

  const job = await createAIJob(api, payload, params || {});
  const wait = params.wait === true;
  if (!wait) {
    return toolResult({ job, nextStep: "视频默认异步生成，请使用 omnidrive_job_detail 轮询结果" });
  }

  const workspace = await pollWorkspaceUntilFinal(api, job.id, {
    ...params,
    timeoutMs: params.timeoutMs || 600000,
    pollIntervalMs: params.pollIntervalMs || 5000,
  });
  return toolResult({
    job,
    workspace: summarizeWorkspace(workspace),
  });
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

  if (wait) {
    workspace = await pollWorkspaceUntilFinal(api, jobId, params || {});
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
  return toolResult(result);
}

const plugin = {
  id: "omnidrive",
  name: "OmniDrive",
  description: "OmniDrive cloud auth and AI tools for OpenClaw",
  register(api) {
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
