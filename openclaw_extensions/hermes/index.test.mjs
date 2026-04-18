import test from "node:test";
import assert from "node:assert/strict";

import { buildRunSkillPayload, normalizeSourceCategory } from "./index.js";

test("normalizeSourceCategory defaults to openclaw_via_hermes", () => {
  assert.equal(normalizeSourceCategory(""), "openclaw_via_hermes");
  assert.equal(normalizeSourceCategory(undefined), "openclaw_via_hermes");
  assert.equal(normalizeSourceCategory("hermes_direct"), "hermes_direct");
});

test("buildRunSkillPayload normalizes gateway payload for Hermes bridge", () => {
  const payload = buildRunSkillPayload({
    skill: "omnibull-publish",
    action: "enqueue",
    title: "demo",
  });

  assert.equal(payload.taskType, "run_skill");
  assert.equal(payload.skillName, "omnibull-publish");
  assert.equal(payload.source, "openclaw_via_hermes");
});
