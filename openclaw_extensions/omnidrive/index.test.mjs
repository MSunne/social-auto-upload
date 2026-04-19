import test from "node:test";
import assert from "node:assert/strict";

import {
  buildMediaToolResultContent,
  buildAuthStatusPayload,
  buildMixVideoCreateInput,
  buildPublishMetadataInputPayload,
  clearCachedSession,
  getLongJobObservationRemainingMs,
  pollWorkspaceUntilFinal,
  requestJson,
  summarizeMixVideoToolPayload,
  summarizeMediaToolPayload,
} from "./index.js";

test("buildPublishMetadataInputPayload returns normalized publish metadata", () => {
  const payload = buildPublishMetadataInputPayload({
    accountId: "account-1",
    platform: "douyin",
    accountName: "demo-account",
    publishAt: "2026-04-14T12:00:00Z",
  });

  assert.equal(payload.accountId, "account-1");
  assert.equal(payload.platform, "douyin");
  assert.equal(payload.accountName, "demo-account");
  assert.equal(payload.publishAt, "2026-04-14T12:00:00Z");
  assert.deepEqual(payload.publishPayload.targets, [
    {
      accountId: "account-1",
      platform: "douyin",
      accountName: "demo-account",
    },
  ]);
});

test("buildPublishMetadataInputPayload rejects partial publish metadata", () => {
  assert.throws(
    () =>
      buildPublishMetadataInputPayload({
        accountId: "account-1",
        platform: "douyin",
      }),
    /自动发布需要同时提供/,
  );
});

test("getLongJobObservationRemainingMs expires long-running observation window", () => {
  const nowMs = Date.parse("2026-04-14T10:00:00Z");
  const remainingMs = getLongJobObservationRemainingMs(
    {
      jobType: "video",
      createdAt: "2026-04-14T09:53:00Z",
    },
    nowMs,
  );

  assert.equal(remainingMs, 0);
});

test("pollWorkspaceUntilFinal returns early when long-running observation window is exhausted", async () => {
  let calls = 0;
  const createdAt = new Date(Date.now() - 7 * 60 * 1000).toISOString();
  const workspace = {
    job: {
      id: "job-1",
      jobType: "video",
      status: "running",
      createdAt,
    },
    artifacts: [],
  };

  const result = await pollWorkspaceUntilFinal(
    {},
    "job-1",
    {
      timeoutMs: 600000,
    },
    async () => {
      calls += 1;
      return workspace;
    },
  );

  assert.equal(calls, 1);
  assert.equal(result.terminal, false);
  assert.equal(result.waitExpired, true);
  assert.equal(result.observationExpired, true);
  assert.equal(result.workspace.job.id, "job-1");
});

test("summarizeMediaToolPayload deduplicates artifact urls for video results", () => {
  const summary = summarizeMediaToolPayload("video", {
    job: {
      id: "job-video-1",
      jobType: "video",
      modelName: "veo-3.1-fast-fl",
      status: "success",
    },
    workspace: {
      jobId: "job-video-1",
      jobType: "video",
      modelName: "veo-3.1-fast-fl",
      status: "success",
      publicUrls: ["https://cdn.example.com/video.mp4"],
      artifacts: [
        {
          artifactType: "video",
          fileName: "demo.mp4",
          mimeType: "video/mp4",
          publicUrl: "https://cdn.example.com/video.mp4",
        },
      ],
    },
  });

  assert.equal(summary.jobType, "video");
  assert.deepEqual(summary.details.publicUrls, ["https://cdn.example.com/video.mp4"]);
  assert.match(summary.summaryText, /视频已生成完成/);
  assert.match(summary.summaryText, /video\/mp4/);
});

test("buildMediaToolResultContent embeds generated image previews", async () => {
  const content = await buildMediaToolResultContent(
    "image",
    {
      job: {
        id: "job-image-1",
        jobType: "image",
        modelName: "gemini-3-pro-image-preview",
        status: "success",
      },
      workspace: {
        jobId: "job-image-1",
        jobType: "image",
        modelName: "gemini-3-pro-image-preview",
        status: "success",
        publicUrls: ["https://cdn.example.com/result.png"],
        artifacts: [
          {
            artifactType: "image",
            fileName: "result.png",
            mimeType: "image/png",
            publicUrl: "https://cdn.example.com/result.png",
          },
        ],
      },
    },
    {
      fetchImpl: async () => ({
        ok: true,
        headers: {
          get(name) {
            if (name === "content-type") {
              return "image/png";
            }
            return null;
          },
        },
        async arrayBuffer() {
          return Uint8Array.from([137, 80, 78, 71]).buffer;
        },
      }),
    },
  );

  assert.equal(content[0].type, "text");
  assert.match(content[0].text, /https:\/\/cdn\.example\.com\/result\.png/);
  assert.equal(content[1].type, "image");
  assert.equal(content[1].mimeType, "image/png");
  assert.ok(content[1].data.length > 0);
});

test("buildMediaToolResultContent keeps video results as text-only content", async () => {
  const content = await buildMediaToolResultContent("video", {
    job: {
      id: "job-video-2",
      jobType: "video",
      modelName: "veo-3.1-fast-fl",
      status: "running",
    },
    nextStep: "视频默认异步生成，请使用 omnidrive_job_detail 轮询结果",
  });

  assert.equal(content.length, 1);
  assert.equal(content[0].type, "text");
  assert.match(content[0].text, /视频任务已提交/);
  assert.match(content[0].text, /轮询结果/);
});

test("buildMixVideoCreateInput normalizes local and remote assets", () => {
  const payload = buildMixVideoCreateInput({
    scriptText: "请生成混剪视频",
    sourceVideos: [
      { absolutePath: "/tmp/source-1.mp4" },
      { url: "https://cdn.example.com/source-2.mp4", fileName: "source-2.mp4" },
    ],
    refAudio: {
      url: "https://cdn.example.com/audio.m4a",
      fileName: "audio.m4a",
    },
    accountId: "account-1",
    platform: "douyin",
    accountName: "demo-account",
    publishAt: "2026-04-14T12:00:00Z",
  });

  assert.equal(payload.scriptText, "请生成混剪视频");
  assert.equal(payload.sourceVideos.length, 2);
  assert.equal(payload.sourceVideos[0].absolutePath, "/tmp/source-1.mp4");
  assert.equal(payload.sourceVideos[1].url, "https://cdn.example.com/source-2.mp4");
  assert.equal(payload.refAudio.url, "https://cdn.example.com/audio.m4a");
  assert.equal(payload.publish.accountId, "account-1");
  assert.equal(payload.publish.publishAt, "2026-04-14T12:00:00Z");
});

test("summarizeMixVideoToolPayload renders task detail summary", () => {
  const summary = summarizeMixVideoToolPayload("task_detail", {
    id: "mix-task-1",
    status: "completed",
    scriptText: "最终脚本",
    resultAsset: {
      publicUrl: "https://cdn.example.com/result.mp4",
      fileName: "result.mp4",
    },
    platform: "抖音",
    accountName: "账号A",
  });

  assert.match(summary.summaryText, /混剪任务详情/);
  assert.match(summary.summaryText, /mix-task-1/);
  assert.match(summary.summaryText, /https:\/\/cdn\.example\.com\/result\.mp4/);
  assert.match(summary.summaryText, /抖音/);
});

test("requestJson retries with refreshed local session after 401 without manual credentials", async () => {
  clearCachedSession();
  const originalFetch = global.fetch;
  const requests = [];
  let sessionCalls = 0;

  global.fetch = async (url, options = {}) => {
    requests.push({
      url: String(url),
      authorization: options.headers?.Authorization || "",
    });
    if (String(url) === "http://127.0.0.1:5409/api/skill/omnidrive/session") {
      sessionCalls += 1;
      return {
        ok: true,
        status: 200,
        async text() {
          return JSON.stringify({
            data: {
              accessToken: sessionCalls === 1 ? "stale-local-token" : "fresh-local-token",
              apiBaseUrl: "https://cloud.example.com",
              user: { id: "user-1", name: "禾硕AI" },
            },
          });
        },
      };
    }
    if (String(url) === "https://cloud.example.com/api/v1/auth/me") {
      if (options.headers?.Authorization === "Bearer stale-local-token") {
        return {
          ok: false,
          status: 401,
          async text() {
            return JSON.stringify({ error: "invalid access token or token expired" });
          },
        };
      }
      return {
        ok: true,
        status: 200,
        async text() {
          return JSON.stringify({ id: "user-1", name: "禾硕AI" });
        },
      };
    }
    throw new Error(`unexpected request: ${url}`);
  };

  try {
    const payload = await requestJson(
      { pluginConfig: { baseUrl: "https://cloud.example.com", localOmniBullBaseUrl: "http://127.0.0.1:5409" } },
      "/api/v1/auth/me",
      { method: "GET" },
      {},
    );

    assert.equal(payload.id, "user-1");
    assert.equal(sessionCalls, 2);
    assert.deepEqual(
      requests
        .filter((item) => item.url === "https://cloud.example.com/api/v1/auth/me")
        .map((item) => item.authorization),
      ["Bearer stale-local-token", "Bearer fresh-local-token"],
    );
  } finally {
    global.fetch = originalFetch;
    clearCachedSession();
  }
});

test("buildAuthStatusPayload reports local agent session and available skills", async () => {
  clearCachedSession();
  const originalFetch = global.fetch;

  global.fetch = async (url, options = {}) => {
    if (String(url) === "http://127.0.0.1:5409/api/skill/omnidrive/session") {
      return {
        ok: true,
        status: 200,
        async text() {
          return JSON.stringify({
            data: {
              accessToken: "fresh-local-token",
              apiBaseUrl: "https://cloud.example.com",
              user: { id: "user-1", name: "禾硕AI", email: "demo@example.com" },
              device: { id: "device-1", deviceCode: "device-code-1", name: "Factory OmniBull" },
              authState: "authorized",
              reason: "",
              availableSkills: ["omnidrive_auth", "omnidrive_chat", "omnibull_status"],
            },
          });
        },
      };
    }
    if (String(url) === "http://127.0.0.1:5409/api/skill/status") {
      return {
        ok: true,
        status: 200,
        async text() {
          return JSON.stringify({ data: { deviceCode: "device-code-1" } });
        },
      };
    }
    if (String(url) === "https://cloud.example.com/api/v1/devices") {
      assert.equal(options.headers?.Authorization, "Bearer fresh-local-token");
      return {
        ok: true,
        status: 200,
        async text() {
          return JSON.stringify([
            { id: "device-1", deviceCode: "device-code-1", name: "Factory OmniBull", isEnabled: true },
          ]);
        },
      };
    }
    throw new Error(`unexpected request: ${url}`);
  };

  try {
    const payload = await buildAuthStatusPayload(
      { pluginConfig: { baseUrl: "https://cloud.example.com", localOmniBullBaseUrl: "http://127.0.0.1:5409" } },
      {},
    );

    assert.equal(payload.authenticated, true);
    assert.equal(payload.authSource, "local_agent_session");
    assert.equal(payload.headlessAgentSessionActive, true);
    assert.equal(payload.boundDevice.deviceCode, "device-code-1");
    assert.deepEqual(payload.availableSkills, ["omnidrive_auth", "omnidrive_chat", "omnibull_status"]);
  } finally {
    global.fetch = originalFetch;
    clearCachedSession();
  }
});
