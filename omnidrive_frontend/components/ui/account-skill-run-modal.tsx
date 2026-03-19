"use client";

import { useEffect, useMemo, useState } from "react";
import { CalendarClock, Check, Loader2, Sparkles, X } from "lucide-react";
import type { Skill } from "@/lib/types";
import { cn } from "@/lib/utils";
import { normalizeSkillOutputLabel } from "@/lib/workflow";

type AccountSkillRunModalProps = {
  isOpen: boolean;
  accountName: string;
  skills: Skill[];
  submitting?: boolean;
  onClose: () => void;
  onSubmit: (payload: { skillId: string; publishAt: string }) => Promise<void> | void;
};

function toDatetimeLocalValue(value?: string | null) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const offset = date.getTimezoneOffset();
  const normalized = new Date(date.getTime() - offset * 60 * 1000);
  return normalized.toISOString().slice(0, 16);
}

function buildDefaultDatetimeLocal() {
  const date = new Date(Date.now() + 60 * 60 * 1000);
  const offset = date.getTimezoneOffset();
  const normalized = new Date(date.getTime() - offset * 60 * 1000);
  return normalized.toISOString().slice(0, 16);
}

export function AccountSkillRunModal({
  isOpen,
  accountName,
  skills,
  submitting,
  onClose,
  onSubmit,
}: AccountSkillRunModalProps) {
  const enabledSkills = useMemo(() => skills.filter((item) => item.isEnabled), [skills]);
  const [selectedSkillId, setSelectedSkillId] = useState("");
  const [publishAt, setPublishAt] = useState("");

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const defaultSkill = enabledSkills[0];
    setSelectedSkillId(defaultSkill?.id || "");
    setPublishAt(
      toDatetimeLocalValue(defaultSkill?.nextRunAt || defaultSkill?.executionTime) || buildDefaultDatetimeLocal(),
    );
  }, [enabledSkills, isOpen]);

  const selectedSkill = enabledSkills.find((item) => item.id === selectedSkillId) || null;

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 px-4 py-6 backdrop-blur-md">
      <div className="w-full max-w-3xl overflow-hidden rounded-[28px] border border-white/10 bg-[#09111f] shadow-[0_30px_90px_rgba(0,0,0,0.45)]">
        <div className="flex items-start justify-between border-b border-white/10 px-6 py-5">
          <div>
            <p className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/6 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-text-muted">
              <Sparkles className="h-3.5 w-3.5 text-accent" />
              Account Skill Run
            </p>
            <h3 className="mt-3 text-2xl font-semibold text-white">为账号创建任务</h3>
            <p className="mt-2 text-sm leading-6 text-text-secondary">
              当前账号是 <span className="font-medium text-white">@{accountName}</span>。先选技能，再给这次任务一个发布时间。
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-10 w-10 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-white/20 hover:bg-white/10 hover:text-white"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="space-y-6 px-6 py-6">
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-sm font-medium text-white">
              <Check className="h-4 w-4 text-cyan" />
              选择技能
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              {enabledSkills.map((skill) => {
                const selected = skill.id === selectedSkillId;
                return (
                  <button
                    key={skill.id}
                    type="button"
                    onClick={() => {
                      setSelectedSkillId(skill.id);
                      setPublishAt(
                        toDatetimeLocalValue(skill.nextRunAt || skill.executionTime) || buildDefaultDatetimeLocal(),
                      );
                    }}
                    className={cn(
                      "rounded-[24px] border px-4 py-4 text-left transition-all",
                      selected
                        ? "border-accent/45 bg-accent/12 shadow-[0_12px_35px_rgba(177,73,255,0.14)]"
                        : "border-white/10 bg-white/[0.04] hover:border-white/20 hover:bg-white/[0.06]",
                    )}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-white">{skill.name}</p>
                        <p className="mt-1 text-xs text-text-secondary">{normalizeSkillOutputLabel(skill.outputType)}</p>
                      </div>
                      <span
                        className={cn(
                          "inline-flex h-7 w-7 items-center justify-center rounded-full border transition-all",
                          selected ? "border-white/20 bg-white text-[#09111f]" : "border-white/10 bg-white/5 text-transparent",
                        )}
                      >
                        <Check className="h-3.5 w-3.5" />
                      </span>
                    </div>
                    <p className="mt-3 line-clamp-2 text-sm leading-6 text-text-secondary">{skill.description}</p>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="grid gap-4 rounded-[24px] border border-white/10 bg-white/[0.04] p-5 md:grid-cols-[1.1fr_0.9fr]">
            <div className="space-y-2">
              <div className="flex items-center gap-2 text-sm font-medium text-white">
                <CalendarClock className="h-4 w-4 text-amber-300" />
                发布时间
              </div>
              <input
                type="datetime-local"
                value={publishAt}
                onChange={(event) => setPublishAt(event.target.value)}
                className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm text-white outline-none transition-all focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
              />
              <p className="text-xs leading-5 text-text-secondary">
                系统会自动在发布时间前 30 分钟进入云端生成链路。
              </p>
            </div>

            <div className="rounded-[20px] border border-white/10 bg-[#0d1729] p-4">
              <p className="text-xs uppercase tracking-[0.22em] text-white/45">当前摘要</p>
              <div className="mt-3 space-y-2 text-sm text-text-secondary">
                <p>
                  技能：<span className="font-medium text-white">{selectedSkill?.name || "未选择"}</span>
                </p>
                <p>
                  模型：<span className="font-medium text-white">{selectedSkill?.modelName || "未选择"}</span>
                </p>
                <p>
                  分镜：<span className="font-medium text-white">{selectedSkill?.storyboardEnabled === false ? "关闭" : "启用"}</span>
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-white/10 px-6 py-4">
          <button
            type="button"
            onClick={onClose}
            className="rounded-2xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm font-medium text-text-primary transition-all hover:border-white/20 hover:bg-white/8 hover:text-white"
          >
            取消
          </button>
          <button
            type="button"
            disabled={!selectedSkillId || !publishAt || submitting}
            onClick={async () => {
              if (!selectedSkillId || !publishAt) {
                return;
              }
              await onSubmit({
                skillId: selectedSkillId,
                publishAt: new Date(publishAt).toISOString(),
              });
            }}
            className="inline-flex items-center gap-2 rounded-2xl bg-gradient-to-r from-accent to-cyan px-5 py-2.5 text-sm font-semibold text-white shadow-[0_16px_40px_rgba(177,73,255,0.2)] transition-all hover:scale-[1.01] disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            创建任务
          </button>
        </div>
      </div>
    </div>
  );
}
