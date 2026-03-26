"use client";

import { useMemo, useState } from "react";
import {
  CalendarClock,
  Check,
  ChevronRight,
  Loader2,
  Plus,
  Repeat,
  Search,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import type { AIJob, AccountSkillScheduleSlot, Skill } from "@/lib/types";
import { getModelDisplayName } from "@/lib/model-display";
import { cn } from "@/lib/utils";
import { normalizeSkillOutputLabel } from "@/lib/workflow";

type AccountSkillRunModalProps = {
  isOpen: boolean;
  accountName: string;
  job?: AIJob | null;
  skills: Skill[];
  submitting?: boolean;
  onClose: () => void;
  onSubmit: (payload: {
    skillId: string;
    scheduleSlots: AccountSkillScheduleSlot[];
  }) => Promise<void> | void;
};

const DEFAULT_GENERATION_LEAD_MINUTES = 0;

function normalizeGenerationLeadMinutes(value?: number | null) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) {
    return DEFAULT_GENERATION_LEAD_MINUTES;
  }
  return Math.min(24 * 60, Math.round(numeric));
}

function inferGenerationLeadMinutes(
  publishAt?: string | null,
  generateAt?: string | null,
) {
  if (!publishAt || !generateAt) {
    return DEFAULT_GENERATION_LEAD_MINUTES;
  }
  const publishDate = new Date(publishAt);
  const generateDate = new Date(generateAt);
  if (
    Number.isNaN(publishDate.getTime()) ||
    Number.isNaN(generateDate.getTime())
  ) {
    return DEFAULT_GENERATION_LEAD_MINUTES;
  }
  return normalizeGenerationLeadMinutes(
    (publishDate.getTime() - generateDate.getTime()) / 60000,
  );
}

function toTimeInputValue(value?: string | null) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

function buildDefaultTimeOfDay() {
  const date = new Date(Date.now() + 60 * 60 * 1000);
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

function normalizeTimeOfDay(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  return trimmed.length === 5 ? `${trimmed}:00` : trimmed;
}

function buildDefaultScheduleSlot(): AccountSkillScheduleSlot {
  return {
    timeOfDay: buildDefaultTimeOfDay(),
    repeatDaily: true,
    generationLeadMinutes: DEFAULT_GENERATION_LEAD_MINUTES,
  };
}

function resolveJobScheduleSlot(job?: AIJob | null): AccountSkillScheduleSlot {
  if (!job) {
    return buildDefaultScheduleSlot();
  }
  const payload = (job.inputPayload || {}) as Record<string, unknown>;
  const scheduleConfig =
    payload.scheduleConfig && typeof payload.scheduleConfig === "object"
      ? (payload.scheduleConfig as Record<string, unknown>)
      : null;
  const timeOfDay =
    typeof scheduleConfig?.timeOfDay === "string"
      ? normalizeTimeOfDay(scheduleConfig.timeOfDay)
      : normalizeTimeOfDay(
          toTimeInputValue(
            typeof payload.publishAt === "string"
              ? payload.publishAt
              : job.runAt,
          ),
        );
  return {
    scheduleKey:
      typeof scheduleConfig?.scheduleKey === "string"
        ? scheduleConfig.scheduleKey
        : undefined,
    timeOfDay: timeOfDay || buildDefaultTimeOfDay(),
    repeatDaily: Boolean(scheduleConfig?.repeatDaily),
    timezone:
      typeof scheduleConfig?.timezone === "string"
        ? scheduleConfig.timezone
        : Intl.DateTimeFormat().resolvedOptions().timeZone,
    generationLeadMinutes:
      typeof scheduleConfig?.generationLeadMinutes === "number"
        ? normalizeGenerationLeadMinutes(scheduleConfig.generationLeadMinutes)
        : inferGenerationLeadMinutes(
            typeof payload.publishAt === "string" ? payload.publishAt : null,
            typeof payload.runAt === "string" ? payload.runAt : job.runAt,
          ),
  };
}

function TimeInput({
  value,
  onChange,
}: {
  value: string;
  onChange: (val: string) => void;
}) {
  const normalizedValue = normalizeTimeOfDay(value) || "00:00:00";

  return (
    <div className="flex h-11 items-center rounded-2xl border border-white/10 bg-white/6 px-4 text-base font-semibold text-white transition-all focus-within:border-accent/40 focus-within:bg-white/8 focus-within:ring-4 focus-within:ring-accent/10">
      <input
        type="time"
        step={1}
        value={normalizedValue}
        onChange={(event) => onChange(normalizeTimeOfDay(event.target.value))}
        className="w-[160px] bg-transparent text-center outline-none [color-scheme:dark] [&::-webkit-calendar-picker-indicator]:hidden"
        aria-label="发布时间"
      />
    </div>
  );
}

export function AccountSkillRunModal({
  isOpen,
  accountName,
  job,
  skills,
  submitting,
  onClose,
  onSubmit,
}: AccountSkillRunModalProps) {
  const isEditing = Boolean(job);
  const enabledSkills = useMemo(
    () => skills.filter((item) => item.isEnabled),
    [skills],
  );
  const modalSkills = useMemo(() => {
    if (!isEditing) {
      return enabledSkills;
    }
    const currentSkill = skills.find((item) => item.id === job?.skillId);
    return currentSkill ? [currentSkill] : [];
  }, [enabledSkills, isEditing, job?.skillId, skills]);
  const initialSkillId = isEditing
    ? job?.skillId || ""
    : enabledSkills[0]?.id || "";
  const [selectedSkillId, setSelectedSkillId] = useState(initialSkillId);
  const [scheduleSlots, setScheduleSlots] = useState<
    AccountSkillScheduleSlot[]
  >(isEditing ? [resolveJobScheduleSlot(job)] : [buildDefaultScheduleSlot()]);

  const [skillSearch, setSkillSearch] = useState("");
  const effectiveSelectedSkillId =
    selectedSkillId || (!isEditing ? enabledSkills[0]?.id || "" : "");
  const selectedSkill =
    modalSkills.find((item) => item.id === effectiveSelectedSkillId) || null;
  const repeatCount = scheduleSlots.filter((item) => item.repeatDaily).length;
  const filteredSkills = useMemo(() => {
    const q = skillSearch.trim().toLowerCase();
    if (!q) return modalSkills;
    return modalSkills.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        (s.description || "").toLowerCase().includes(q),
    );
  }, [modalSkills, skillSearch]);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 px-4 py-8 backdrop-blur-md">
      <div className="flex w-full max-w-3xl flex-col overflow-hidden rounded-[28px] border border-white/10 bg-[#09111f] shadow-[0_30px_90px_rgba(0,0,0,0.45)] sm:max-h-[calc(100vh-4rem)]">
        <div className="flex shrink-0 items-start justify-between border-b border-white/10 px-6 py-5">
          <div>
            <p className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/6 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-text-muted">
              <Sparkles className="h-3.5 w-3.5 text-accent" />
              Account Skill Run
            </p>
            <h3 className="mt-3 text-2xl font-semibold text-white">
              {isEditing ? "修改发布时间" : "为账号创建任务"}
            </h3>
            <p className="mt-2 text-sm leading-6 text-text-secondary">
              当前账号是{" "}
              <span className="font-medium text-white">@{accountName}</span>。
              {isEditing
                ? " 这次只修改当前账号任务的发布时间，不会影响别的账号。"
                : " 先选技能，再给这次任务一个发布时间。"}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-white/20 hover:bg-white/10 hover:text-white"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-6">
          <div className="space-y-6">
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-sm font-medium text-white">
                  <Check className="h-4 w-4 text-cyan" />
                  {isEditing ? "当前技能" : "选择技能"}
                </div>
                {!isEditing && modalSkills.length > 4 ? (
                  <span className="text-xs text-text-muted">
                    {filteredSkills.length}/{modalSkills.length} 条技能
                  </span>
                ) : null}
              </div>

              {!isEditing && modalSkills.length > 4 ? (
                <div className="relative">
                  <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
                  <input
                    type="text"
                    value={skillSearch}
                    onChange={(e) => setSkillSearch(e.target.value)}
                    placeholder="搜索技能名称或描述..."
                    className="w-full rounded-2xl border border-white/10 bg-white/6 py-2.5 pl-10 pr-4 text-sm text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                  />
                </div>
              ) : null}

              <div
                className={cn(
                  "space-y-1.5 overflow-y-auto rounded-[20px] border border-white/10 bg-[#0d1729] p-2",
                  !isEditing && modalSkills.length > 4 ? "max-h-[200px]" : "",
                )}
              >
                {filteredSkills.map((skill) => {
                  const selected = skill.id === effectiveSelectedSkillId;
                  return (
                    <button
                      key={skill.id}
                      type="button"
                      onClick={() => {
                        if (isEditing) return;
                        setSelectedSkillId(skill.id);
                        setScheduleSlots([buildDefaultScheduleSlot()]);
                      }}
                      className={cn(
                        "flex w-full items-center gap-3 rounded-2xl border px-3.5 py-3 text-left transition-all",
                        selected
                          ? "border-accent/45 bg-accent/12"
                          : "border-transparent hover:border-white/10 hover:bg-white/[0.04]",
                        isEditing ? "cursor-default" : "",
                      )}
                    >
                      <span
                        className={cn(
                          "inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full border transition-all",
                          selected
                            ? "border-accent bg-accent text-white"
                            : "border-white/20 bg-white/5 text-transparent",
                        )}
                      >
                        <Check className="h-3 w-3" />
                      </span>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium text-white">
                          {skill.name}
                        </p>
                        <p className="mt-0.5 truncate text-xs text-text-secondary">
                          {normalizeSkillOutputLabel(skill.outputType)}
                          {skill.description ? ` · ${skill.description}` : ""}
                        </p>
                      </div>
                      <ChevronRight
                        className={cn(
                          "h-4 w-4 shrink-0 text-white/20 transition-colors",
                          selected && "text-accent/60",
                        )}
                      />
                    </button>
                  );
                })}
                {filteredSkills.length === 0 ? (
                  <p className="px-4 py-6 text-center text-sm text-text-secondary">
                    {skillSearch
                      ? "没有匹配的技能"
                      : "当前没有可用技能，请先返回技能中心启用或创建技能。"}
                  </p>
                ) : null}
              </div>
              {selectedSkill ? (
                <div className="animate-in fade-in slide-in-from-top-2 rounded-[20px] border border-accent/20 bg-accent/[0.06] p-4">
                  <div className="flex items-start gap-3">
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-accent/15 text-accent">
                      <Sparkles className="h-4 w-4" />
                    </div>
                    <div className="min-w-0 flex-1 space-y-2">
                      <p className="text-sm font-semibold text-white">
                        {selectedSkill.name}
                      </p>
                      <p className="text-sm leading-6 text-text-secondary">
                        {selectedSkill.description || "暂无技能说明。"}
                      </p>
                      <div className="flex flex-wrap gap-2 pt-1">
                        <span className="inline-flex items-center gap-1 rounded-lg bg-white/8 px-2 py-1 text-[11px] font-medium text-text-secondary">
                          输出{" "}
                          <span className="text-white">
                            {normalizeSkillOutputLabel(
                              selectedSkill.outputType,
                            )}
                          </span>
                        </span>
                        <span className="inline-flex items-center gap-1 rounded-lg bg-white/8 px-2 py-1 text-[11px] font-medium text-text-secondary">
                          模型{" "}
                          <span className="text-white">
                            {getModelDisplayName(selectedSkill)}
                          </span>
                        </span>
                        <span className="inline-flex items-center gap-1 rounded-lg bg-white/8 px-2 py-1 text-[11px] font-medium text-text-secondary">
                          分镜{" "}
                          <span className="text-white">
                            {selectedSkill.storyboardEnabled === false
                              ? "关闭"
                              : "启用"}
                          </span>
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              ) : null}
            </div>

            <div className="grid gap-4 rounded-[24px] border border-white/10 bg-white/[0.04] p-5 md:grid-cols-[1.1fr_0.9fr]">
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm font-medium text-white">
                  <CalendarClock className="h-4 w-4 text-amber-300" />
                  时间计划
                </div>
                <div className="space-y-3">
                  {scheduleSlots.map((slot, index) => (
                    <div
                      key={slot.scheduleKey || `${index}-${slot.timeOfDay}`}
                      className="rounded-[20px] border border-white/10 bg-[#0d1729] p-4"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="text-sm font-medium text-white">
                            时间 {index + 1}
                          </p>
                          <p className="mt-1 text-xs text-text-secondary">
                            只选时分秒，不选年月日。系统会自动算出下一次执行日期。
                          </p>
                        </div>
                        {!isEditing && scheduleSlots.length > 1 ? (
                          <button
                            type="button"
                            onClick={() =>
                              setScheduleSlots((current) =>
                                current.filter(
                                  (_, itemIndex) => itemIndex !== index,
                                ),
                              )
                            }
                            className="inline-flex h-9 w-9 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-danger hover:bg-danger/10 hover:text-danger"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        ) : null}
                      </div>
                      <div className="mt-5 flex flex-wrap items-center gap-3">
                        <TimeInput
                          value={slot.timeOfDay}
                          onChange={(nextValue) => {
                            setScheduleSlots((current) =>
                              current.map((item, itemIndex) =>
                                itemIndex === index
                                  ? { ...item, timeOfDay: nextValue }
                                  : item,
                              ),
                            );
                          }}
                        />
                        <button
                          type="button"
                          onClick={() =>
                            setScheduleSlots((current) =>
                              current.map((item, itemIndex) =>
                                itemIndex === index
                                  ? { ...item, repeatDaily: !item.repeatDaily }
                                  : item,
                              ),
                            )
                          }
                          className={cn(
                            "inline-flex h-11 items-center gap-2 rounded-2xl border px-4 text-sm font-medium transition-all",
                            slot.repeatDaily
                              ? "border-cyan/35 bg-cyan/12 text-white"
                              : "border-white/10 bg-white/5 text-text-secondary hover:border-white/20 hover:text-white",
                          )}
                        >
                          <Repeat className="h-4 w-4" />
                          {slot.repeatDaily ? "每天重复" : "只执行一次"}
                        </button>
                        <label className="flex h-11 items-center gap-3 rounded-2xl border border-white/10 bg-white/5 px-4 text-sm text-text-secondary transition-all focus-within:border-accent/40 focus-within:bg-white/8 focus-within:ring-4 focus-within:ring-accent/10">
                          <span className="font-medium">生成提前</span>
                          <input
                            type="number"
                            min={0}
                            max={1440}
                            step={5}
                            value={normalizeGenerationLeadMinutes(
                              slot.generationLeadMinutes,
                            )}
                            onChange={(event) => {
                              const nextValue = normalizeGenerationLeadMinutes(
                                event.target.valueAsNumber,
                              );
                              setScheduleSlots((current) =>
                                current.map((item, itemIndex) =>
                                  itemIndex === index
                                    ? {
                                        ...item,
                                        generationLeadMinutes: nextValue,
                                      }
                                    : item,
                                ),
                              );
                            }}
                            className="w-12 bg-transparent p-0 text-center font-semibold text-white outline-none"
                          />
                          <span className="font-medium">分钟</span>
                        </label>
                      </div>
                    </div>
                  ))}
                </div>
                {!isEditing ? (
                  <button
                    type="button"
                    onClick={() =>
                      setScheduleSlots((current) => [
                        ...current,
                        buildDefaultScheduleSlot(),
                      ])
                    }
                    className="mt-3 inline-flex items-center gap-2 rounded-2xl border border-dashed border-white/15 bg-white/[0.03] px-4 py-3 text-sm font-medium text-text-primary transition-all hover:border-accent/40 hover:text-white"
                  >
                    <Plus className="h-4 w-4" />
                    再加一个时间
                  </button>
                ) : null}
                <p className="text-xs leading-5 text-text-secondary">
                  每条时间都属于当前账号自己。勾选“每天重复”后，系统会在这个时间点每天自动续排下一次计划。
                </p>
              </div>

              <div className="rounded-[20px] border border-white/10 bg-[#0d1729] p-4">
                <p className="text-xs uppercase tracking-[0.22em] text-white/45">
                  当前摘要
                </p>
                <div className="mt-3 space-y-2 text-sm text-text-secondary">
                  <p>
                    技能：
                    <span className="font-medium text-white">
                      {selectedSkill?.name || "未选择"}
                    </span>
                  </p>
                  <p>
                    模型：
                    <span className="font-medium text-white">
                      {getModelDisplayName(selectedSkill, "未选择")}
                    </span>
                  </p>
                  <p>
                    时间：
                    <span className="font-medium text-white">
                      {scheduleSlots.length} 条
                    </span>
                  </p>
                  <p>
                    重复：
                    <span className="font-medium text-white">
                      {repeatCount} 条每天重复
                    </span>
                  </p>
                  <p>
                    生成提前：
                    <span className="font-medium text-white">
                      {scheduleSlots[0]
                        ? `${normalizeGenerationLeadMinutes(scheduleSlots[0].generationLeadMinutes)} 分钟`
                        : "0 分钟"}
                    </span>
                  </p>
                  <p>
                    分镜：
                    <span className="font-medium text-white">
                      {selectedSkill?.storyboardEnabled === false
                        ? "关闭"
                        : "启用"}
                    </span>
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="flex shrink-0 items-center justify-end gap-3 border-t border-white/10 bg-[#09111f] px-6 py-4">
          <button
            type="button"
            onClick={onClose}
            className="rounded-2xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm font-medium text-text-primary transition-all hover:border-white/20 hover:bg-white/8 hover:text-white"
          >
            取消
          </button>
          <button
            type="button"
            disabled={
              !effectiveSelectedSkillId ||
              scheduleSlots.some((item) => !item.timeOfDay.trim()) ||
              submitting
            }
            onClick={async () => {
              if (
                !effectiveSelectedSkillId ||
                scheduleSlots.some((item) => !item.timeOfDay.trim())
              ) {
                return;
              }
              await onSubmit({
                skillId: effectiveSelectedSkillId,
                scheduleSlots: scheduleSlots.map((item) => ({
                  scheduleKey: item.scheduleKey,
                  timeOfDay: normalizeTimeOfDay(item.timeOfDay),
                  repeatDaily: item.repeatDaily,
                  generationLeadMinutes: normalizeGenerationLeadMinutes(
                    item.generationLeadMinutes,
                  ),
                })),
              });
            }}
            className="inline-flex items-center gap-2 rounded-2xl bg-gradient-to-r from-accent to-cyan px-5 py-2.5 text-sm font-semibold text-white shadow-[0_16px_40px_rgba(177,73,255,0.2)] transition-all hover:scale-[1.01] disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            {isEditing ? "保存计划" : "创建计划"}
          </button>
        </div>
      </div>
    </div>
  );
}
