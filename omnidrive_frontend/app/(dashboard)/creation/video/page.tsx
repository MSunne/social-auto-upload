"use client";

import { useState } from "react";
import { motion } from "framer-motion";
import {
  Video,
  Wand2,
  Sparkles,
  Play,
  Download,
  RotateCcw,
  Clock,
  Upload,
} from "lucide-react";
import { PageHeader } from "@/components/ui/common";

const MODELS = [
  { id: "veo3", name: "VEO 3", badge: "推荐" },
  { id: "sora2", name: "Sora 2", badge: null },
  { id: "runway", name: "Runway Gen-4", badge: null },
  { id: "kling", name: "Kling 2.1", badge: "快速" },
];

const DURATIONS = ["5s", "10s", "15s", "30s", "60s"];

const RESOLUTIONS = [
  { label: "720p", desc: "快速预览" },
  { label: "1080p", desc: "标准质量" },
  { label: "4K", desc: "超高清" },
];

const mockVideos = [
  {
    id: 1,
    prompt: "城市天际线的延时摄影，从日落过渡到夜景",
    model: "VEO 3",
    duration: "10s",
    status: "done" as const,
    thumbnail: "https://placehold.co/480x270/1a1a2e/a855f7?text=City+Timelapse",
  },
  {
    id: 2,
    prompt: "一只金毛犬在海滩上奔跑，慢动作效果",
    model: "Sora 2",
    duration: "5s",
    status: "done" as const,
    thumbnail: "https://placehold.co/480x270/1a1a2e/06b6d4?text=Dog+Beach",
  },
  {
    id: 3,
    prompt: "科幻空间站内部穿行镜头",
    model: "Runway Gen-4",
    duration: "15s",
    status: "processing" as const,
    thumbnail: null,
  },
];

export default function VideoCreationPage() {
  const [prompt, setPrompt] = useState("");
  const [selectedModel, setSelectedModel] = useState("veo3");
  const [selectedDuration, setSelectedDuration] = useState("10s");
  const [selectedRes, setSelectedRes] = useState("1080p");
  const [generating, setGenerating] = useState(false);
  const [keyframe, setKeyframe] = useState<string | null>(null);

  function handleGenerate() {
    if (!prompt.trim()) return;
    setGenerating(true);
    setTimeout(() => setGenerating(false), 4000);
  }

  return (
    <>
      <PageHeader
        title="视频制作"
        subtitle="AI 驱动的视频生成引擎，从文字描述到高画质视频"
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* ── Left: Controls ── */}
        <div className="lg:col-span-1 space-y-5">
          {/* Prompt */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className="glass-card p-5"
          >
            <label className="mb-2 block text-xs font-semibold text-text-muted uppercase tracking-wider">
              视频描述
            </label>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="描述你希望生成的视频内容，包括镜头运动、风格和氛围..."
              rows={5}
              className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none resize-none focus:border-accent/50 focus:ring-2 focus:ring-accent/20 transition-all"
            />
            <div className="mt-2 flex items-center justify-between">
              <span className="text-xs text-text-muted">
                {prompt.length} / 4000
              </span>
              <button className="flex items-center gap-1 text-xs text-accent hover:text-accent-strong transition-colors">
                <Sparkles className="h-3 w-3" />
                AI 优化描述
              </button>
            </div>
          </motion.div>

          {/* Keyframe Upload */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.05 }}
            className="glass-card p-5"
          >
            <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
              参考关键帧（可选）
            </label>
            {keyframe ? (
              <div className="relative rounded-xl overflow-hidden border border-border">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={keyframe} alt="keyframe" className="w-full h-32 object-cover" />
                <button
                  onClick={() => setKeyframe(null)}
                  className="absolute top-2 right-2 rounded-lg bg-black/60 p-1.5 text-white hover:bg-black/80 transition-colors"
                >
                  <RotateCcw className="h-3 w-3" />
                </button>
              </div>
            ) : (
              <button
                onClick={() =>
                  setKeyframe(
                    "https://placehold.co/600x200/1a1a2e/a855f7?text=Keyframe",
                  )
                }
                className="flex w-full flex-col items-center gap-2 rounded-xl border-2 border-dashed border-border py-8 text-text-muted transition-colors hover:border-accent/30 hover:text-accent"
              >
                <Upload className="h-6 w-6" />
                <span className="text-xs">点击上传或拖拽图片</span>
              </button>
            )}
          </motion.div>

          {/* Model */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="glass-card p-5"
          >
            <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
              生成模型
            </label>
            <div className="grid grid-cols-2 gap-2">
              {MODELS.map((m) => (
                <button
                  key={m.id}
                  onClick={() => setSelectedModel(m.id)}
                  className={`relative rounded-xl border px-3 py-2.5 text-sm font-medium transition-all ${
                    selectedModel === m.id
                      ? "border-accent/60 bg-accent/10 text-accent shadow-sm shadow-accent/10"
                      : "border-border bg-surface text-text-secondary hover:bg-surface-hover"
                  }`}
                >
                  {m.name}
                  {m.badge && (
                    <span className="absolute -top-2 -right-2 rounded-md bg-accent px-1.5 py-0.5 text-[10px] font-bold text-background">
                      {m.badge}
                    </span>
                  )}
                </button>
              ))}
            </div>
          </motion.div>

          {/* Duration & Resolution */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.15 }}
            className="glass-card p-5 space-y-4"
          >
            <div>
              <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
                <Clock className="inline h-3 w-3 mr-1" />
                时长
              </label>
              <div className="flex gap-2">
                {DURATIONS.map((d) => (
                  <button
                    key={d}
                    onClick={() => setSelectedDuration(d)}
                    className={`flex-1 rounded-lg border py-2 text-xs font-semibold transition-all ${
                      selectedDuration === d
                        ? "border-cyan/50 bg-cyan/10 text-cyan"
                        : "border-border text-text-muted hover:text-text-secondary"
                    }`}
                  >
                    {d}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
                分辨率
              </label>
              <div className="flex gap-2">
                {RESOLUTIONS.map((r) => (
                  <button
                    key={r.label}
                    onClick={() => setSelectedRes(r.label)}
                    className={`flex-1 rounded-lg border py-2.5 text-center transition-all ${
                      selectedRes === r.label
                        ? "border-accent/50 bg-accent/10"
                        : "border-border hover:bg-surface-hover"
                    }`}
                  >
                    <div
                      className={`text-xs font-bold ${selectedRes === r.label ? "text-accent" : "text-text-secondary"}`}
                    >
                      {r.label}
                    </div>
                    <div className="text-[10px] text-text-muted">{r.desc}</div>
                  </button>
                ))}
              </div>
            </div>
          </motion.div>

          {/* Generate */}
          <motion.button
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2 }}
            onClick={handleGenerate}
            disabled={!prompt.trim() || generating}
            className="w-full rounded-xl bg-gradient-to-r from-accent to-cyan py-3.5 text-sm font-bold text-background shadow-lg shadow-accent/25 transition-all hover:shadow-xl hover:shadow-accent/35 disabled:opacity-40 flex items-center justify-center gap-2"
          >
            {generating ? (
              <>
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-background/30 border-t-background" />
                视频生成中...
              </>
            ) : (
              <>
                <Wand2 className="h-4 w-4" />
                开始生成视频
              </>
            )}
          </motion.button>
        </div>

        {/* ── Right: Results ── */}
        <div className="lg:col-span-2 space-y-5">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="glass-card p-5"
          >
            <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-text-primary">
              <Video className="h-4 w-4 text-accent" />
              最近生成
            </h3>
            <div className="space-y-4">
              {mockVideos.map((v) => (
                <div
                  key={v.id}
                  className="group flex gap-4 rounded-xl border border-border bg-surface p-3 transition-all hover:border-accent/30"
                >
                  {/* Thumbnail */}
                  <div className="relative flex-shrink-0 w-48 h-28 rounded-lg overflow-hidden bg-surface-hover">
                    {v.status === "done" && v.thumbnail ? (
                      <>
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={v.thumbnail}
                          alt={v.prompt}
                          className="h-full w-full object-cover"
                        />
                        <div className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 group-hover:opacity-100 transition-opacity">
                          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-accent/90 text-background shadow-lg">
                            <Play className="h-5 w-5 ml-0.5" />
                          </div>
                        </div>
                      </>
                    ) : (
                      <div className="flex h-full w-full flex-col items-center justify-center gap-2">
                        <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/30 border-t-accent" />
                        <span className="text-[10px] text-text-muted">
                          渲染中…
                        </span>
                      </div>
                    )}
                    {/* Duration badge */}
                    <span className="absolute bottom-1.5 right-1.5 rounded bg-black/70 px-1.5 py-0.5 text-[10px] font-bold text-white">
                      {v.duration}
                    </span>
                  </div>
                  {/* Info */}
                  <div className="flex flex-1 flex-col justify-between min-w-0">
                    <div>
                      <p className="text-sm font-medium text-text-primary line-clamp-2">
                        {v.prompt}
                      </p>
                      <p className="mt-1 text-xs text-text-muted">
                        模型：{v.model}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      {v.status === "done" ? (
                        <>
                          <button className="flex items-center gap-1 rounded-lg border border-border px-2.5 py-1 text-xs text-text-secondary hover:border-accent/30 hover:text-accent transition-all">
                            <Download className="h-3 w-3" />
                            下载
                          </button>
                          <button className="flex items-center gap-1 rounded-lg border border-border px-2.5 py-1 text-xs text-text-secondary hover:border-accent/30 hover:text-accent transition-all">
                            <RotateCcw className="h-3 w-3" />
                            重新生成
                          </button>
                        </>
                      ) : (
                        <span className="flex items-center gap-1.5 text-xs text-amber-400">
                          <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber-400" />
                          处理中…
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </motion.div>
        </div>
      </div>
    </>
  );
}
