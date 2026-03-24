import type { AIJob } from "@/lib/types";

export interface JobScheduleMeta {
  generateAt?: string;
  publishAt?: string;
  repeatDaily: boolean;
  timeOfDay?: string;
  scheduleKey?: string;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function asString(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed || undefined;
}

export function extractJobScheduleMeta(job: AIJob): JobScheduleMeta {
  const payload = asRecord(job.inputPayload);
  const scheduleConfig = asRecord(payload?.scheduleConfig);

  return {
    generateAt: job.runAt || asString(payload?.runAt),
    publishAt: asString(payload?.publishAt),
    repeatDaily: scheduleConfig?.repeatDaily === true,
    timeOfDay: asString(scheduleConfig?.timeOfDay),
    scheduleKey: asString(scheduleConfig?.scheduleKey),
  };
}

export function describeJobSchedule(meta: JobScheduleMeta): string | null {
  if (!meta.repeatDaily) {
    return null;
  }
  if (meta.timeOfDay) {
    return `同一循环任务，每天 ${meta.timeOfDay} 发布`;
  }
  return "同一循环任务，每天按时发布";
}
