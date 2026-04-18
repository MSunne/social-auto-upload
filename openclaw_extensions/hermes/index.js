const DEFAULT_LOCAL_OMNIBULL_BASE_URL = "http://127.0.0.1:5409";
const DEFAULT_LOCAL_OMNIBULL_TIMEOUT_MS = 15000;
const DEFAULT_SOURCE_CATEGORY = "openclaw_via_hermes";

function resolveConfig(api) {
  const pluginConfig = api.pluginConfig || {};
  return {
    localOmniBullBaseUrl: String(
      pluginConfig.localOmniBullBaseUrl || process.env.OMNIBULL_BASE_URL || DEFAULT_LOCAL_OMNIBULL_BASE_URL,
    ).replace(/\/+$/, ""),
    localOmniBullApiKey: String(pluginConfig.localOmniBullApiKey || process.env.OMNIBULL_API_KEY || "").trim(),
    localOmniBullTimeoutMs: Number(
      pluginConfig.localOmniBullTimeoutMs || process.env.OMNIBULL_TIMEOUT_MS || DEFAULT_LOCAL_OMNIBULL_TIMEOUT_MS,
    ),
  };
}

async function requestLocalBridge(api, path, options = {}) {
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
      payload = { code: response.status, msg: text, data: null };
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

function normalizeSourceCategory(sourceCategory) {
  const value = String(sourceCategory || "").trim();
  return value || DEFAULT_SOURCE_CATEGORY;
}

function buildRunSkillPayload(params = {}) {
  const payload = {
    ...params,
    taskType: "run_skill",
    source: normalizeSourceCategory(params.source || params.sourceCategory),
  };
  if (payload.skillName === undefined && payload.skill !== undefined) {
    payload.skillName = payload.skill;
  }
  delete payload.sourceCategory;
  return payload;
}

async function executeStatus(api) {
  const response = await requestLocalBridge(api, "/api/hermes/status");
  return toolResult(response.payload || response);
}

async function executeChat(api, params) {
  const payload = {
    ...params,
    source: normalizeSourceCategory(params?.source || params?.sourceCategory),
  };
  delete payload.sourceCategory;
  const response = await requestLocalBridge(api, "/api/hermes/chat", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  return toolResult(response.payload || response);
}

async function executeRunSkill(api, params) {
  const response = await requestLocalBridge(api, "/api/hermes/task/run", {
    method: "POST",
    body: JSON.stringify(buildRunSkillPayload(params || {})),
  });
  return toolResult(response.payload || response);
}

const plugin = {
  id: "hermes",
  name: "Hermes",
  description: "OpenClaw plugin for Hermes bridge chat and skill cooperation through local OmniBull",
  register(api) {
    api.registerGatewayMethod("hermes.status", async ({ respond }) => {
      try {
        const response = await requestLocalBridge(api, "/api/hermes/status");
        respond(response.ok, response.payload || response);
      } catch (error) {
        respond(false, { ok: false, error: String(error?.message || error) });
      }
    });

    api.registerGatewayMethod("hermes.chat", async ({ respond, ...request }) => {
      try {
        const params = request?.params || request?.payload || request?.input || request?.arguments || {};
        const payload = {
          ...params,
          source: normalizeSourceCategory(params?.source || params?.sourceCategory),
        };
        delete payload.sourceCategory;
        const response = await requestLocalBridge(api, "/api/hermes/chat", {
          method: "POST",
          body: JSON.stringify(payload),
        });
        respond(response.ok, response.payload || response);
      } catch (error) {
        respond(false, { ok: false, error: String(error?.message || error) });
      }
    });

    api.registerGatewayMethod("hermes.run_skill", async ({ respond, ...request }) => {
      try {
        const params = request?.params || request?.payload || request?.input || request?.arguments || {};
        const response = await requestLocalBridge(api, "/api/hermes/task/run", {
          method: "POST",
          body: JSON.stringify(buildRunSkillPayload(params || {})),
        });
        respond(response.ok, response.payload || response);
      } catch (error) {
        respond(false, { ok: false, error: String(error?.message || error) });
      }
    });

    api.registerTool({
      name: "hermes_status",
      description: "读取本地 Hermes bridge 状态、共享模型运行时配置和回退策略。",
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
      name: "hermes_chat",
      description: "通过本地 OmniBull 的 Hermes bridge 调用 Hermes 聊天；Hermes 不可用时由桥接层自动回退到 OmniDrive 直连聊天。",
      parameters: {
        type: "object",
        additionalProperties: false,
        properties: {
          prompt: { type: "string" },
          input: { type: "string" },
          messages: { type: "array", items: { type: "object" } },
          systemPrompt: { type: "string" },
          instructions: { type: "string" },
          conversation: { type: "string" },
          previousResponseId: { type: "string" },
          model: { type: "string" },
          source: { type: "string" },
          sourceCategory: { type: "string" },
          correlationId: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeChat(api, params || {});
      },
    });

    api.registerTool({
      name: "hermes_run_skill",
      description: "通过 Hermes bridge 执行本地 OmniBull/OmniDrive 薄技能，如账号、素材、发布、AI 任务等。",
      parameters: {
        type: "object",
        additionalProperties: true,
        properties: {
          skillName: { type: "string" },
          skill: { type: "string" },
          action: { type: "string" },
          source: { type: "string" },
          sourceCategory: { type: "string" },
          correlationId: { type: "string" },
        },
      },
      async execute(_id, params) {
        return executeRunSkill(api, params || {});
      },
    });
  },
};

export { buildRunSkillPayload, normalizeSourceCategory };
export default plugin;
