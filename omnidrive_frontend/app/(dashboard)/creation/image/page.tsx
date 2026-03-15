"use client";

import { useState, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  ImagePlus,
  Wand2,
  Layers,
  Download,
  RotateCcw,
  Sparkles,
  ChevronDown,
  Copy,
  X,
  Upload,
  Info,
  Check
} from "lucide-react";
import { cn } from "@/lib/utils";

const MODELS = [
  { 
    id: "imagen3", 
    name: "Imagen 3", 
    badge: "推荐",
    desc: "Google最新旗舰模型，极高的细节还原度与文本理解能力，卓越的光影表现。" 
  },
  { 
    id: "dalle4", 
    name: "DALL·E 4", 
    badge: null,
    desc: "OpenAI新一代模型，擅长超现实主义创意融合与极高难度的空间构图。" 
  },
  { 
    id: "midjourney", 
    name: "Midjourney v7", 
    badge: null,
    desc: "艺术表现力天花板，在插画、摄影级人像美学上具有统治级统治力。" 
  },
  { 
    id: "flux", 
    name: "Flux Pro 1.1", 
    badge: "快速",
    desc: "以极高的新图生成速度著称，适合电商修图与大量素材批量产出。" 
  },
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
    imageUrl: "https://images.unsplash.com/photo-1535295972055-1c762f4483e5?w=800&q=80",
  },
  {
    id: 2,
    prompt: "日出时分的富士山雪景，写实风格",
    model: "DALL·E 4",
    style: "写实摄影",
    status: "done",
    imageUrl: "https://images.unsplash.com/photo-1490806843957-31f4c9a91c65?w=800&q=80",
  },
  {
    id: 3,
    prompt: "极简主义科技产品宣传图",
    model: "Flux Pro 1.1",
    style: "扁平设计",
    status: "generating",
    imageUrl: null,
  },
  {
    id: 4,
    prompt: "未来城市的空中列车穿梭在高楼之间",
    model: "Midjourney v7",
    style: "3D 渲染",
    status: "done",
    imageUrl: "https://images.unsplash.com/photo-1541888086925-920a0b2d6add?w=800&q=80",
  },
];

export default function ImageCreationPage() {
  const [prompt, setPrompt] = useState("");
  const [selectedModel, setSelectedModel] = useState("imagen3");
  const [selectedStyle, setSelectedStyle] = useState("写实摄影");
  const [selectedRatio, setSelectedRatio] = useState("1:1");
  const [count, setCount] = useState(1);
  const [generating, setGenerating] = useState(false);
  const [refImages, setRefImages] = useState<string[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);
  
  // Custom dropdown state
  const [isModelDropdownOpen, setIsModelDropdownOpen] = useState(false);
  
  // Right panel state
  const [selectedHistoryId, setSelectedHistoryId] = useState<number>(mockHistory[0].id);

  function handleGenerate() {
    if (!prompt.trim()) return;
    setGenerating(true);
    // Simulate generation by selecting the "generating" item
    setSelectedHistoryId(3); 
    setTimeout(() => {
      setGenerating(false);
      // Let's pretend it finished and flip back to history 1 for demo
      setSelectedHistoryId(1);
    }, 4000);
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    if (e.target.files) {
      const newImgs = Array.from(e.target.files).map(f => URL.createObjectURL(f));
      setRefImages(prev => [...prev, ...newImgs].slice(0, 4)); // Max 4 ref images
    }
  }

  function removeRefImage(idx: number) {
    setRefImages(prev => prev.filter((_, i) => i !== idx));
  }

  const activeModel = MODELS.find(m => m.id === selectedModel) || MODELS[0];
  const selectedHistoryItem = mockHistory.find(h => h.id === selectedHistoryId);

  return (
    <div className="grid h-[calc(100vh-theme(spacing.16))] grid-cols-1 gap-5 lg:grid-cols-12 pb-4">
      
      {/* ── Left: Controls ── */}
      <div className="flex h-full flex-col gap-3 lg:col-span-4 xl:col-span-3 overflow-y-auto pr-1 pb-4">
        
        {/* Reference Images Upload */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="glass-card p-4"
        >
          <div className="mb-2 flex items-center justify-between">
            <label className="text-xs font-semibold text-text-muted uppercase tracking-wider">
              参考图 (最多4张)
            </label>
            <span className="text-[10px] text-text-muted">{refImages.length}/4</span>
          </div>
          
          <div className="grid grid-cols-4 gap-2">
            <AnimatePresence>
              {refImages.map((img, idx) => (
                <motion.div
                  key={idx}
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.8 }}
                  className="group relative aspect-square overflow-hidden rounded-lg bg-surface-hover border border-border"
                >
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={img} alt="ref" className="h-full w-full object-cover" />
                  <button
                    onClick={() => removeRefImage(idx)}
                    className="absolute -right-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full bg-danger text-white opacity-0 transition-opacity group-hover:opacity-100 shadow-md"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </motion.div>
              ))}
            </AnimatePresence>
            
            {refImages.length < 4 && (
              <button
                onClick={() => fileInputRef.current?.click()}
                className="flex aspect-square flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed border-border bg-surface-hover/50 text-text-muted transition-colors hover:border-accent/50 hover:text-accent hover:bg-accent/5"
              >
                <Upload className="h-4 w-4" />
                <span className="text-[10px]">上传</span>
              </button>
            )}
            <input 
              type="file" 
              multiple 
              accept="image/*" 
              className="hidden" 
              ref={fileInputRef}
              onChange={handleFileSelect}
            />
          </div>
        </motion.div>

        {/* Prompt */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.05 }}
          className="glass-card p-4"
        >
          <label className="mb-2 block text-xs font-semibold text-text-muted uppercase tracking-wider">
            创作提示词
          </label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="描述你想要生成的图片内容，例如：一座未来科技城市的黄昏景色，霓虹灯光穿梭..."
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

        {/* Compact Model Selector */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="glass-card p-4"
        >
          <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
            生成模型
          </label>
          <div className="relative">
            <button
              onClick={() => setIsModelDropdownOpen(!isModelDropdownOpen)}
              className="flex w-full items-center justify-between rounded-xl border border-border bg-surface px-4 py-3 text-sm font-medium transition-all hover:border-accent/50 focus:border-accent"
            >
              <div className="flex items-center gap-2">
                <span className="text-text-primary text-glow">{activeModel.name}</span>
                {activeModel.badge && (
                  <span className="rounded bg-accent/20 px-1.5 py-0.5 text-[10px] text-accent">
                    {activeModel.badge}
                  </span>
                )}
              </div>
              <ChevronDown className={cn("h-4 w-4 text-text-muted transition-transform", isModelDropdownOpen && "rotate-180")} />
            </button>
            
            {/* Model Dropdown List */}
            <AnimatePresence>
              {isModelDropdownOpen && (
                <motion.div
                  initial={{ opacity: 0, y: -5 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -5 }}
                  className="absolute left-0 right-0 top-full mt-2 z-50 overflow-hidden rounded-xl border border-border-strong bg-surface-elevated shadow-xl backdrop-blur-3xl"
                >
                  {MODELS.map((m) => (
                    <button
                      key={m.id}
                      onClick={() => {
                        setSelectedModel(m.id);
                        setIsModelDropdownOpen(false);
                      }}
                      className="flex w-full items-center justify-between border-b border-border/50 px-4 py-3 last:border-0 hover:bg-surface-hover transition-colors"
                    >
                      <span className={cn("text-sm", selectedModel === m.id ? "text-accent font-bold" : "text-text-primary")}>
                        {m.name}
                      </span>
                      {selectedModel === m.id && <Check className="h-4 w-4 text-accent" />}
                    </button>
                  ))}
                </motion.div>
              )}
            </AnimatePresence>
          </div>
          
          {/* Active Model Description */}
          <div className="mt-3 flex items-start gap-2 rounded-lg bg-surface-hover p-3 text-xs text-text-muted border border-border/50">
            <Info className="h-3.5 w-3.5 mt-0.5 shrink-0 text-cyan" />
            <p className="leading-snug">{activeModel.desc}</p>
          </div>
        </motion.div>

        {/* Style, Ratio & Count */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15 }}
          className="glass-card p-4 space-y-4"
        >
          <div>
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
                      ? "border-cyan/50 bg-cyan/10 text-cyan text-glow-cyan"
                      : "border-border bg-transparent text-text-muted hover:text-text-secondary hover:border-border"
                  }`}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-2 block text-xs font-semibold text-text-muted uppercase tracking-wider">
                画幅比例
              </label>
              <select 
                value={selectedRatio}
                onChange={(e) => setSelectedRatio(e.target.value)}
                className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-primary outline-none focus:border-accent"
              >
                {RATIOS.map(r => <option key={r.label} value={r.label}>{r.label} ({r.w}x{r.h})</option>)}
              </select>
            </div>
            <div>
              <label className="mb-2 block text-xs font-semibold text-text-muted uppercase tracking-wider">
                生成数量
              </label>
              <select 
                value={count}
                onChange={(e) => setCount(Number(e.target.value))}
                className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-primary outline-none focus:border-accent"
              >
                {[1, 2, 4].map(n => <option key={n} value={n}>{n} 张</option>)}
              </select>
            </div>
          </div>
        </motion.div>

        <div className="flex-1" />

        {/* Generate Button — Glassmorphism Glow (matching video page) */}
        <motion.button
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          onClick={handleGenerate}
          disabled={!prompt.trim() || generating}
          className="group relative w-full overflow-hidden rounded-2xl py-5 text-sm font-bold disabled:opacity-40 flex flex-col items-center justify-center gap-1 transition-all bg-gradient-to-r from-accent via-pink to-cyan shadow-[0_0_30px_rgba(177,73,255,0.3),0_0_60px_rgba(0,245,212,0.15)] hover:shadow-[0_0_40px_rgba(177,73,255,0.5),0_0_80px_rgba(0,245,212,0.25)] hover:scale-[1.02] active:scale-[0.98] shrink-0 mt-2"
        >
          {/* Glassmorphism inner overlay */}
          <div className="absolute inset-[1px] rounded-2xl bg-background/60 backdrop-blur-xl" />
          
          <div className="relative z-10">
            {generating ? (
              <div className="flex items-center gap-2 text-white">
                <div className="h-5 w-5 animate-spin rounded-full border-2 border-white/30 border-t-white" />
                <span className="tracking-wider">生成中...</span>
              </div>
            ) : (
              <div className="flex items-center gap-2 text-white">
                <Wand2 className="h-5 w-5 drop-shadow-[0_0_6px_rgba(255,255,255,0.6)]" />
                <span className="tracking-[0.15em] text-[15px] drop-shadow-[0_0_8px_rgba(255,255,255,0.4)]">开始构图</span>
              </div>
            )}
          </div>
        </motion.button>
      </div>

      {/* ── Right: Preview & History ── */}
      <div className="flex h-full flex-col gap-4 lg:col-span-8 xl:col-span-9 overflow-hidden">
        
        {/* Top: Main Preview Area — fixed height so it never pushes history out */}
        <motion.div
          initial={{ opacity: 0, scale: 0.98 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ delay: 0.1 }}
          className="glow-border relative h-[calc(100%-200px)] overflow-hidden rounded-2xl bg-surface-elevated cyber-grid border border-border shadow-2xl flex items-center justify-center"
        >
          <AnimatePresence mode="wait">
            {selectedHistoryItem?.status === "generating" || generating ? (
              <motion.div 
                key="generating"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="flex flex-col items-center justify-center gap-6"
              >
                <div className="relative h-24 w-24">
                  <div className="absolute inset-0 rounded-full border-4 border-surface border-t-accent animate-spin" />
                  <div className="absolute inset-2 rounded-full border-4 border-surface border-b-cyan animate-[spin_1.5s_reverse_infinite]" />
                  <div className="absolute inset-0 flex items-center justify-center">
                    <Sparkles className="h-6 w-6 text-accent animate-pulse" />
                  </div>
                </div>
                <div className="text-center space-y-2">
                  <h3 className="text-xl font-bold tracking-widest text-glow text-accent-strong uppercase">
                    Model Computing
                  </h3>
                  <p className="text-sm text-text-muted">正在连接神经网络节点渲染图像细节区...</p>
                </div>
              </motion.div>
            ) : selectedHistoryItem?.imageUrl ? (
              <motion.div
                key={selectedHistoryItem.id}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="group relative h-full w-full"
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img 
                  src={selectedHistoryItem.imageUrl} 
                  alt="Preview" 
                  className="h-full w-full object-contain p-4"
                />
                
                {/* Floating Toolbar inside preview */}
                <div className="absolute bottom-6 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-2xl border border-white/10 bg-black/40 p-2 backdrop-blur-xl opacity-0 transition-opacity group-hover:opacity-100 shadow-2xl">
                  <button className="flex h-10 w-10 items-center justify-center rounded-xl text-white hover:bg-white/20 transition-colors tooltip" title="下载原图">
                    <Download className="h-5 w-5" />
                  </button>
                  <div className="h-6 w-px bg-white/20" />
                  <button className="flex h-10 w-10 items-center justify-center rounded-xl text-white hover:bg-white/20 transition-colors tooltip" title="复制链接">
                    <Copy className="h-5 w-5" />
                  </button>
                  <button className="flex h-10 flex-col items-center justify-center px-4 rounded-xl text-white hover:bg-white/20 transition-colors tooltip" title="相似重绘">
                    <RotateCcw className="h-4 w-4 mb-0.5" />
                    <span className="text-[10px]">重绘</span>
                  </button>
                </div>
                
                {/* Status Badge inside preview */}
                <div className="absolute left-6 top-6 flex items-center gap-2 rounded-lg border border-white/10 bg-black/40 px-3 py-1.5 backdrop-blur-md">
                  <span className="flex h-2 w-2 rounded-full bg-success pulse-online" />
                  <span className="text-xs font-medium text-white">{selectedHistoryItem.model} • 渲染完成</span>
                </div>
              </motion.div>
            ) : (
              <motion.div
                key="empty"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="flex flex-col items-center justify-center text-text-muted opacity-50"
              >
                <ImagePlus className="mb-4 h-16 w-16" />
                <p>等待模型接收指令输出</p>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>

        {/* Bottom: History Thumbnails */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="h-[180px] shrink-0"
        >
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-medium text-text-secondary flex items-center gap-2">
              <Layers className="h-4 w-4" /> 历史记录库
            </h3>
            <span className="text-xs text-text-muted cursor-pointer hover:text-accent transition-colors">查看全部 →</span>
          </div>
          
          <div className="flex h-[140px] gap-4 overflow-x-auto pb-2 scrollbar-hide">
            {mockHistory.map((item) => (
              <button
                key={item.id}
                onClick={() => setSelectedHistoryId(item.id)}
                className={cn(
                  "group relative aspect-[4/3] h-full shrink-0 overflow-hidden rounded-xl border-2 transition-all",
                  selectedHistoryId === item.id 
                    ? "border-accent shadow-[0_0_15px_rgba(177,73,255,0.3)]" 
                    : "border-border hover:border-accent/50 opacity-70 hover:opacity-100"
                )}
              >
                {item.imageUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={item.imageUrl} alt="" className="h-full w-full object-cover" />
                ) : (
                  <div className="flex h-full w-full items-center justify-center bg-surface-hover cyber-grid">
                     <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/30 border-t-accent" />
                  </div>
                )}
                
                {/* Meta overlay for thumbnail */}
                <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent p-2 text-left">
                  <p className="line-clamp-1 text-[10px] text-white/90">{item.prompt}</p>
                </div>
                
                {/* Status indicator badge */}
                {item.status === "generating" && (
                  <div className="absolute top-2 right-2 rounded bg-black/60 px-1.5 py-0.5 backdrop-blur-sm text-[9px] text-warning flex items-center gap-1">
                    <span className="h-1.5 w-1.5 rounded-full bg-warning animate-pulse" /> 生成中
                  </div>
                )}
              </button>
            ))}
          </div>
        </motion.div>
      </div>
    </div>
  );
}
