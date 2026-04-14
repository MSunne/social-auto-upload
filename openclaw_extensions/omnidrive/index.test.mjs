import test from "node:test";
import assert from "node:assert/strict";

import {
  buildPublishMetadataInputPayload,
  getLongJobObservationRemainingMs,
  pollWorkspaceUntilFinal,
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
