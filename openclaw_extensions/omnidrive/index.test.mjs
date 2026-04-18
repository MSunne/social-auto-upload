import test from "node:test";
import assert from "node:assert/strict";

import {
  buildMediaToolResultContent,
  buildPublishMetadataInputPayload,
  getLongJobObservationRemainingMs,
  pollWorkspaceUntilFinal,
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
