"use client";

import { useState } from "react";
import { motion } from "framer-motion";
import {
  ImagePlus,
  Wand2,
  Layers,
  Download,
  RotateCcw,
  Sparkles,
  ChevronDown,
  Copy,
} from "lucide-react";
import { PageHeader } from "@/components/ui/common";

const MODELS = [
  { id: "imagen3", name: "Imagen 3", badge: "推荐" },
  { id: "dalle4", name: "DALL·E 4", badge: null },
  { id: "midjourney", name: "Midjourney v7", badge: null },
  { id: "flux", name: "Flux Pro 1.1", badge: "快速" },
];

const STYLES = [
  "写实摄影",
  "插画风格",
  "赛博朋克",
  "水彩画",
  "3D 渲染",
  "扁平设计",
  "像素艺术",
  "油画风格",
];

const RATIOS = [
  { label: "1:1", w: 1024, h: 1024 },
  { label: "16:9", w: 1344, h: 768 },
  { label: "9:16", w: 768, h: 1344 },
  { label: "4:3", w: 1152, h: 896 },
  { label: "3:4", w: 896, h: 1152 },
];

const mockHistory = [
  {
    id: 1,
    prompt: "一只赛博朋克风格的机械猫咪，霓虹灯光照射",
    model: "Imagen 3",
    style: "赛博朋克",
    status: "done",
    imageUrl: "https://placehold.co/400x400/1a1a2e/a855f7?text=CyberCat",
  },
  {
    id: 2,
    prompt: "日出时分的富士山雪景，写实风格",
    model: "DALL·E 4",
    style: "写实摄影",
    status: "done",
    imageUrl: "https://placehold.co/400x400/1a1a2e/06b6d4?text=Fuji",
  },
  {
    id: 3,
    prompt: "极简主义科技产品宣传图",
    model: "Flux Pro 1.1",
    style: "扁平设计",
    status: "generating",
    imageUrl: null,
  },
];

export default function ImageCreationPage() {
  const [prompt, setPrompt] = useState("");
  const [selectedModel, setSelectedModel] = useState("imagen3");
  const [selectedStyle, setSelectedStyle] = useState("写实摄影");
  const [selectedRatio, setSelectedRatio] = useState("1:1");
  const [count, setCount] = useState(1);
  const [generating, setGenerating] = useState(false);

  function handleGenerate() {
    if (!prompt.trim()) return;
    setGenerating(true);
    setTimeout(() => setGenerating(false), 3000);
  }

  return (
    <>
      <PageHeader
        title="图片制作"
        subtitle="使用 AI 生成高质量图片素材，支持多种模型与风格"
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
              创作提示词
            </label>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="描述你想要生成的图片内容，例如：一座未来科技城市的黄昏景色..."
              rows={5}
              className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none resize-none focus:border-accent/50 focus:ring-2 focus:ring-accent/20 transition-all"
            />
            <div className="mt-2 flex items-center justify-between">
              <span className="text-xs text-text-muted">
                {prompt.length} / 2000
              </span>
              <button className="flex items-center gap-1 text-xs text-accent hover:text-accent-strong transition-colors">
                <Sparkles className="h-3 w-3" />
                AI 优化提示词
              </button>
            </div>
          </motion.div>

          {/* Model */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.05 }}
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
                      : "border-border bg-surface text-text-secondary hover:border-border hover:bg-surface-hover"
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

          {/* Style */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="glass-card p-5"
          >
            <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
              风格预设
            </label>
            <div className="flex flex-wrap gap-2">
              {STYLES.map((s) => (
                <button
                  key={s}
                  onClick={() => setSelectedStyle(s)}
                  className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition-all ${
                    selectedStyle === s
                      ? "border-cyan/50 bg-cyan/10 text-cyan"
                      : "border-border bg-transparent text-text-muted hover:text-text-secondary hover:border-border"
                  }`}
                >
                  {s}
                </button>
              ))}
            </div>
          </motion.div>

          {/* Ratio & Count */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.15 }}
            className="glass-card p-5"
          >
            <div className="mb-4">
              <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
                画幅比例
              </label>
              <div className="flex gap-2">
                {RATIOS.map((r) => (
                  <button
                    key={r.label}
                    onClick={() => setSelectedRatio(r.label)}
                    className={`flex-1 rounded-lg border py-2 text-xs font-semibold transition-all ${
                      selectedRatio === r.label
                        ? "border-accent/50 bg-accent/10 text-accent"
                        : "border-border text-text-muted hover:text-text-secondary"
                    }`}
                  >
                    {r.label}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
                生成数量
              </label>
              <div className="flex gap-2">
                {[1, 2, 4].map((n) => (
                  <button
                    key={n}
                    onClick={() => setCount(n)}
                    className={`flex-1 rounded-lg border py-2 text-xs font-semibold transition-all ${
                      count === n
                        ? "border-accent/50 bg-accent/10 text-accent"
                        : "border-border text-text-muted hover:text-text-secondary"
                    }`}
                  >
                    {n} 张
                  </button>
                ))}
              </div>
            </div>
          </motion.div>

          {/* Generate Button */}
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
                生成中...
              </>
            ) : (
              <>
                <Wand2 className="h-4 w-4" />
                开始生成
              </>
            )}
          </motion.button>
        </div>

        {/* ── Right: Results & History ── */}
        <div className="lg:col-span-2 space-y-5">
          {/* Recent Generations */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="glass-card p-5"
          >
            <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-text-primary">
              <Layers className="h-4 w-4 text-accent" />
              最近生成
            </h3>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {mockHistory.map((item) => (
                <div
                  key={item.id}
                  className="group relative overflow-hidden rounded-xl border border-border bg-surface transition-all hover:border-accent/30"
                >
                  {/* Image */}
                  <div className="aspect-square bg-surface-hover flex items-center justify-center">
                    {item.status === "done" && item.imageUrl ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        src={item.imageUrl}
                        alt={item.prompt}
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      <div className="flex flex-col items-center gap-2">
                        <div className="h-8 w-8 animate-spin rounded-full border-2 border-accent/30 border-t-accent" />
                        <span className="text-xs text-text-muted">
                          生成中…
                        </span>
                      </div>
                    )}
                  </div>
                  {/* Overlay actions */}
                  {item.status === "done" && (
                    <div className="absolute inset-0 flex items-end bg-gradient-to-t from-black/70 via-transparent to-transparent opacity-0 transition-opacity group-hover:opacity-100">
                      <div className="flex w-full items-center justify-between p-3">
                        <span className="text-xs text-white/80 line-clamp-1 flex-1 mr-2">
                          {item.prompt}
                        </span>
                        <div className="flex gap-1.5">
                          <button className="flex h-7 w-7 items-center justify-center rounded-lg bg-white/20 text-white backdrop-blur-sm hover:bg-white/30 transition-colors">
                            <Download className="h-3.5 w-3.5" />
                          </button>
                          <button className="flex h-7 w-7 items-center justify-center rounded-lg bg-white/20 text-white backdrop-blur-sm hover:bg-white/30 transition-colors">
                            <Copy className="h-3.5 w-3.5" />
                          </button>
                          <button className="flex h-7 w-7 items-center justify-center rounded-lg bg-white/20 text-white backdrop-blur-sm hover:bg-white/30 transition-colors">
                            <RotateCcw className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </div>
                    </div>
                  )}
                  {/* Info bar */}
                  <div className="border-t border-border px-3 py-2">
                    <div className="flex items-center justify-between">
                      <span className="text-[11px] text-text-muted">
                        {item.model}
                      </span>
                      <span className="rounded-md bg-accent/10 px-1.5 py-0.5 text-[10px] font-medium text-accent">
                        {item.style}
                      </span>
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
