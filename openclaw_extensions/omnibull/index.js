const DEFAULT_BASE_URL = "http://127.0.0.1:5409";
const DEFAULT_TIMEOUT_MS = 15000;

const configSchema = {
  type: "object",
  additionalProperties: false,
  properties: {
    baseUrl: { type: "string" },
    apiKey: { type: "string" },
    timeoutMs: { type: "integer", minimum: 1000, maximum: 120000 },
  },
};

function resolveConfig(api) {
  const pluginConfig = api.pluginConfig || {};
  return {
    baseUrl: String(pluginConfig.baseUrl || process.env.OMNIBULL_BASE_URL || DEFAULT_BASE_URL).replace(/\/+$/, ""),
    apiKey: String(pluginConfig.apiKey || process.env.OMNIBULL_API_KEY || "").trim(),
    timeoutMs: Number(pluginConfig.timeoutMs || process.env.OMNIBULL_TIMEOUT_MS || DEFAULT_TIMEOUT_MS),
  };
}

async function requestJson(api, path, options = {}) {
  const cfg = resolveConfig(api);
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), cfg.timeoutMs);
  const headers = {
    Accept: "application/json",
    ...(options.headers || {}),
  };
  if (cfg.apiKey) {
    headers["X-Omnibull-Key"] = cfg.apiKey;
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
      payload = { code: response.status, msg: text, data: null };
    }
    if (!response.ok) {
      return {
        ok: false,
        status: response.status,
        payload,
      };
    }
    return {
      ok: true,
      status: response.status,
      payload,
    };
  } finally {
    clearTimeout(timeout);
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

function ensure(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
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

async function executeStatus(api) {
  const response = await requestJson(api, "/api/skill/status");
  return toolResult(response.payload || response);
}

async function executeAccounts(api, params) {
  const action = params.action || "list";
  if (action === "list") {
    const response = await requestJson(api, `/api/skill/accounts${buildQuery({ validate: params.validateCookies ? 1 : 0 })}`);
    return toolResult(response.payload || response);
  }
  if (action === "detail") {
    ensure(Number.isInteger(params.accountId), "detail 操作需要整数 accountId");
    const response = await requestJson(api, `/api/skill/accounts/${params.accountId}`);
    return toolResult(response.payload || response);
  }
  if (action === "validate") {
    const response = await requestJson(api, "/api/skill/accounts/validate", {
      method: "POST",
      body: JSON.stringify({
        accountId: params.accountId,
        accountIds: params.accountIds,
        validateAll: params.validateAll === true,
      }),
    });
    return toolResult(response.payload || response);
  }
  throw new Error(`不支持的 accounts action: ${action}`);
}

async function executeMaterials(api, params) {
  const action = params.action || "roots";
  if (action === "roots") {
    const response = await requestJson(api, "/api/skill/materials/roots");
    return toolResult(response.payload || response);
  }
  if (action === "list") {
    ensure(params.root, "list 操作需要 root");
    const response = await requestJson(
      api,
      `/api/skill/materials/list${buildQuery({ root: params.root, path: params.path, limit: params.limit })}`,
    );
    return toolResult(response.payload || response);
  }
  if (action === "read") {
    ensure(params.root, "read 操作需要 root");
    ensure(params.path, "read 操作需要 path");
    const response = await requestJson(
      api,
      `/api/skill/materials/file${buildQuery({ root: params.root, path: params.path, maxBytes: params.maxBytes })}`,
    );
    return toolResult(response.payload || response);
  }
  throw new Error(`不支持的 materials action: ${action}`);
}

async function executePublish(api, params) {
  const action = params.action || "tasks";
  if (action === "enqueue") {
    const response = await requestJson(api, "/api/skill/publish", {
      method: "POST",
      body: JSON.stringify({
        platformType: params.platformType,
        title: params.title,
        tags: params.tags || [],
        accountIds: params.accountIds,
        accountFilePaths: params.accountFilePaths,
        files: params.files || [],
        runAt: params.runAt,
        enableTimer: params.enableTimer,
        videosPerDay: params.videosPerDay,
        startDays: params.startDays,
        dailyTimes: params.dailyTimes,
        category: params.category,
        isDraft: params.isDraft,
        thumbnail: params.thumbnail,
        productLink: params.productLink,
        productTitle: params.productTitle,
      }),
    });
    return toolResult(response.payload || response);
  }
  if (action === "tasks") {
    const response = await requestJson(
      api,
      `/api/skill/publish/tasks${buildQuery({ status: params.status, limit: params.limit })}`,
    );
    return toolResult(response.payload || response);
  }
  if (action === "task_detail") {
    ensure(params.taskUuid, "task_detail 操作需要 taskUuid");
    const response = await requestJson(api, `/api/skill/publish/tasks/${encodeURIComponent(params.taskUuid)}`);
    return toolResult(response.payload || response);
  }
  throw new Error(`不支持的 publish action: ${action}`);
}

const plugin = {
  id: "omnibull",
  name: "OmniBull",
  description: "Local OmniBull account, material, and media publish tools",
  configSchema,
  register(api) {
    api.registerGatewayMethod("omnibull.status", async ({ respond }) => {
      try {
        const response = await requestJson(api, "/api/skill/status");
        respond(true, response.payload || response);
      } catch (error) {
        respond(false, { ok: false, error: String(error?.message || error) });
      }
    });

    api.registerTool({
      name: "omnibull_status",
      description: "读取本地 OmniBull 状态，包括设备信息、云端同步配置、账号数量和发布任务概况。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {},
      },
      async execute() {
        return executeStatus(api);
      },
    });

    api.registerTool({
      name: "omnibull_accounts",
      description: "管理 OmniBull 本地账号。支持 list、detail、validate 三种 action。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          action: { type: "string", enum: ["list", "detail", "validate"] },
          accountId: { type: "integer" },
          accountIds: { type: "array", items: { type: "integer" } },
          validateAll: { type: "boolean" },
          validateCookies: { type: "boolean" },
        },
      },
      async execute(_id, params) {
        return executeAccounts(api, params || {});
      },
    });

    api.registerTool({
      name: "omnibull_materials",
      description: "浏览 OmniBull 允许访问的本地素材目录，支持 roots、list、read 三种 action。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          action: { type: "string", enum: ["roots", "list", "read"] },
          root: { type: "string" },
          path: { type: "string" },
          limit: { type: "integer", minimum: 1, maximum: 1000 },
          maxBytes: { type: "integer", minimum: 1024, maximum: 1048576 },
        },
      },
      async execute(_id, params) {
        return executeMaterials(api, params || {});
      },
    });

    api.registerTool({
      name: "omnibull_publish",
      description: "向本地 OmniBull 提交媒体发布任务，或查询发布任务状态。支持 enqueue、tasks、task_detail 三种 action。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          action: { type: "string", enum: ["enqueue", "tasks", "task_detail"] },
          platformType: { type: "integer", enum: [2, 3, 4] },
          title: { type: "string" },
          tags: { type: "array", items: { type: "string" } },
          accountIds: { type: "array", items: { type: "integer" } },
          accountFilePaths: { type: "array", items: { type: "string" } },
          files: {
            type: "array",
            items: {
              type: "object",
              additionalProperties: false,
              properties: {
                root: { type: "string" },
                path: { type: "string" },
                absolutePath: { type: "string" },
              },
            },
          },
          runAt: { type: "string" },
          enableTimer: { type: "boolean" },
          videosPerDay: { type: "integer", minimum: 1, maximum: 24 },
          startDays: { type: "integer", minimum: 0, maximum: 365 },
          dailyTimes: {
            type: "array",
            items: {
              anyOf: [{ type: "string" }, { type: "integer" }],
            },
          },
          category: { type: "string" },
          isDraft: { type: "boolean" },
          thumbnail: {
            type: "object",
            additionalProperties: false,
            properties: {
              root: { type: "string" },
              path: { type: "string" },
              absolutePath: { type: "string" },
            },
          },
          productLink: { type: "string" },
          productTitle: { type: "string" },
          taskUuid: { type: "string" },
          status: { type: "string" },
          limit: { type: "integer", minimum: 1, maximum: 500 },
        },
      },
      async execute(_id, params) {
        return executePublish(api, params || {});
      },
    });
  },
};

export default plugin;
