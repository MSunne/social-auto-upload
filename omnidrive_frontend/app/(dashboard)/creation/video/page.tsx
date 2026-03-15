"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Video,
  Wand2,
  Sparkles,
  Play,
  Download,
  Layers,
  RotateCcw,
  Clock,
  Upload,
  ChevronDown,
  Info,
  Check,
  Coins
} from "lucide-react";
import { cn } from "@/lib/utils";

const TOTAL_CREDITS = 2500;

const MODELS = [
  { 
    id: "veo3", 
    name: "VEO 3", 
    badge: "推荐",
    desc: "目前动作一致性最优的模型，适合复杂物理规律与真实世界模拟。",
    cost: 50
  },
  { 
    id: "sora2", 
    name: "Sora 2", 
    badge: null,
    desc: "最强空间想象力，擅长长镜头、多机位切换及复杂视角运动。",
    cost: 80
  },
  { 
    id: "runway", 
    name: "Runway Gen-4", 
    badge: null,
    desc: "影视级美术控制，对艺术风格和色调把握极其精准。",
    cost: 40
  },
  { 
    id: "kling", 
    name: "Kling 2.1", 
    badge: "快速",
    desc: "本土化优化，生成速度快，适合短平快的二次元及特效视频。",
    cost: 20
  },
];

const DURATIONS = [
  { label: "5s", multiplier: 1 },
  { label: "10s", multiplier: 2 },
  { label: "15s", multiplier: 3 },
  { label: "30s", multiplier: 5 },
];

const RESOLUTIONS = [
  { label: "720p", desc: "快速预览", multiplier: 1 },
  { label: "1080p", desc: "标准质量", multiplier: 1.5 },
  { label: "4K", desc: "超高清", multiplier: 3 },
];

const mockVideos = [
  {
    id: 1,
    prompt: "城市天际线的延时摄影，从日落过渡到夜景，霓虹闪烁",
    model: "VEO 3",
    duration: "10s",
    status: "done" as const,
    thumbnail: "https://images.unsplash.com/photo-1477959858617-67f85cf4f1df?w=800&q=80",
  },
  {
    id: 2,
    prompt: "一只金毛犬在海滩上奔跑，慢动作效果，水花四溅",
    model: "Sora 2",
    duration: "5s",
    status: "done" as const,
    thumbnail: "https://images.unsplash.com/photo-1537151608804-ea6fac25d4c9?w=800&q=80",
  },
  {
    id: 3,
    prompt: "科幻空间站内部穿行镜头，失重状态下的水滴漂浮",
    model: "Runway Gen-4",
    duration: "15s",
    status: "generating" as const,
    thumbnail: null,
  },
  {
    id: 4,
    prompt: "航拍雪山之巅，云海翻腾",
    model: "Kling 2.1",
    duration: "10s",
    status: "done" as const,
    thumbnail: "https://images.unsplash.com/photo-1464822759023-fed622ff2c3b?w=800&q=80",
  },
];

export default function VideoCreationPage() {
  const [prompt, setPrompt] = useState("");
  const [selectedModel, setSelectedModel] = useState("veo3");
  const [selectedDuration, setSelectedDuration] = useState("10s");
  const [selectedRes, setSelectedRes] = useState("1080p");
  const [generating, setGenerating] = useState(false);
  const [keyframe, setKeyframe] = useState<string | null>(null);
  
  const [isModelDropdownOpen, setIsModelDropdownOpen] = useState(false);
  const [selectedHistoryId, setSelectedHistoryId] = useState<number>(mockVideos[0].id);

  const activeModel = MODELS.find(m => m.id === selectedModel) || MODELS[0];
  const durationConfig = DURATIONS.find(d => d.label === selectedDuration) || DURATIONS[0];
  const resConfig = RESOLUTIONS.find(r => r.label === selectedRes) || RESOLUTIONS[0];
  
  // Calculate dynamic credit cost
  const currentCost = Math.round(activeModel.cost * durationConfig.multiplier * resConfig.multiplier);

  function handleGenerate() {
    if (!prompt.trim()) return;
    setGenerating(true);
    setSelectedHistoryId(3); // Switch to the generating mock item
    setTimeout(() => {
      setGenerating(false);
      setSelectedHistoryId(1); // Back to a finished one
    }, 4000);
  }

  const selectedHistoryItem = mockVideos.find(h => h.id === selectedHistoryId);

  return (
    <div className="grid h-[calc(100vh-theme(spacing.16))] grid-cols-1 gap-5 lg:grid-cols-12 pb-4">
      
      {/* ── Left: Controls ── */}
      <div className="flex h-full flex-col gap-2.5 lg:col-span-4 xl:col-span-3 overflow-y-auto pr-1 pb-4">
        

        {/* Prompt Input */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.05 }}
          className="glass-card p-4"
        >
          <label className="mb-2 block text-xs font-semibold text-text-muted uppercase tracking-wider">
            视频描述指令
          </label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="描述镜头运动、灯光变化与动态细节..."
            rows={5}
            className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none resize-none focus:border-accent/50 focus:ring-2 focus:ring-accent/20 transition-all custom-scrollbar"
          />
          <div className="mt-2 flex items-center justify-between">
            <span className="text-xs text-text-muted">
              {prompt.length} / 4000
            </span>
            <button className="flex items-center gap-1 text-xs text-accent hover:text-accent-strong transition-colors">
              <Sparkles className="h-3 w-3" />
              智能扩写
            </button>
          </div>
        </motion.div>

        {/* Start / Keyframe Image */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="glass-card p-4"
        >
          <div className="mb-3 flex items-center justify-between">
            <label className="text-xs font-semibold text-text-muted uppercase tracking-wider">
              首尾帧参考 (图生视频)
            </label>
          </div>
          {keyframe ? (
            <div className="group relative rounded-xl overflow-hidden border border-border">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={keyframe} alt="keyframe" className="w-full h-24 object-cover" />
              <button
                onClick={() => setKeyframe(null)}
                className="absolute top-1.5 right-1.5 rounded-lg bg-black/60 p-1 text-white opacity-0 group-hover:opacity-100 transition-all hover:bg-danger text-[10px]"
              >
                移除
              </button>
            </div>
          ) : (
            <button
              onClick={() => setKeyframe("https://images.unsplash.com/photo-1550745165-9bc0b252726f?w=800&q=80")}
              className="flex w-full h-20 flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-border bg-surface-hover/50 text-text-muted transition-colors hover:border-accent/50 hover:text-accent hover:bg-accent/5"
            >
              <Upload className="h-4 w-4" />
              <span className="text-[10px]">上传参考图</span>
            </button>
          )}
        </motion.div>

        {/* Compact Model Selector */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15 }}
          className="glass-card p-4"
        >
          <label className="mb-3 block text-xs font-semibold text-text-muted uppercase tracking-wider">
            视频引擎
          </label>
          <div className="relative">
            <button
              onClick={() => setIsModelDropdownOpen(!isModelDropdownOpen)}
              className="flex w-full items-center justify-between rounded-xl border border-border bg-surface px-3 py-2.5 text-sm font-medium transition-all hover:border-accent/50 focus:border-accent"
            >
              <div className="flex items-center gap-2">
                <span className="text-text-primary text-glow-cyan">{activeModel.name}</span>
                {activeModel.badge && (
                  <span className="rounded bg-cyan/20 px-1.5 py-0.5 text-[10px] text-cyan">
                    {activeModel.badge}
                  </span>
                )}
              </div>
              <ChevronDown className={cn("h-4 w-4 text-text-muted transition-transform", isModelDropdownOpen && "rotate-180")} />
            </button>
            
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
                      <span className={cn("text-sm", selectedModel === m.id ? "text-cyan font-bold" : "text-text-primary")}>
                        {m.name}
                      </span>
                      {selectedModel === m.id && <Check className="h-4 w-4 text-cyan" />}
                    </button>
                  ))}
                </motion.div>
              )}
            </AnimatePresence>
          </div>
          
          <div className="mt-2 flex items-start gap-2 rounded-lg bg-surface-hover p-2 text-[11px] text-text-muted border border-border/50">
            <Info className="h-3.5 w-3.5 mt-0.5 shrink-0 text-accent" />
            <p className="leading-snug">{activeModel.desc}</p>
          </div>
        </motion.div>

        {/* Generate Settings */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="glass-card p-4 space-y-3"
        >
          <div>
            <label className="mb-2 block text-xs font-semibold text-text-muted uppercase tracking-wider">
              视频时长
            </label>
            <div className="grid grid-cols-4 gap-2">
              {DURATIONS.map((d) => (
                <button
                  key={d.label}
                  onClick={() => setSelectedDuration(d.label)}
                  className={`rounded-lg border py-1.5 text-xs font-medium transition-all ${
                    selectedDuration === d.label
                      ? "border-accent/50 bg-accent/10 text-accent text-glow"
                      : "border-border text-text-muted hover:text-text-secondary hover:border-border"
                  }`}
                >
                  {d.label}
                </button>
              ))}
            </div>
          </div>
          
          <div>
            <label className="mb-2 block text-xs font-semibold text-text-muted uppercase tracking-wider">
              渲染分辨率
            </label>
            <div className="grid grid-cols-3 gap-2">
              {RESOLUTIONS.map((r) => (
                <button
                  key={r.label}
                  onClick={() => setSelectedRes(r.label)}
                  className={`rounded-lg border py-1.5 flex items-center justify-center transition-all ${
                    selectedRes === r.label
                      ? "border-accent/50 bg-accent/10 border-b-2 border-b-accent"
                      : "border-border bg-surface hover:bg-surface-hover"
                  }`}
                >
                  <span className={`text-[13px] font-bold ${selectedRes === r.label ? "text-accent" : "text-text-secondary"}`}>
                    {r.label}
                  </span>
                </button>
              ))}
            </div>
          </div>
        </motion.div>

        <div className="flex-1" />

        {/* Generate Button — Glassmorphism Glow */}
        <motion.button
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.25 }}
          onClick={handleGenerate}
          disabled={!prompt.trim() || generating}
          className="group relative w-full overflow-hidden rounded-2xl py-3 text-sm font-bold disabled:opacity-40 flex flex-col items-center justify-center gap-1 transition-all bg-gradient-to-r from-accent via-pink to-cyan shadow-[0_0_30px_rgba(177,73,255,0.3),0_0_60px_rgba(0,245,212,0.15)] hover:shadow-[0_0_40px_rgba(177,73,255,0.5),0_0_80px_rgba(0,245,212,0.25)] hover:scale-[1.02] active:scale-[0.98] shrink-0 mt-1"
        >
          {/* Glassmorphism inner overlay */}
          <div className="absolute inset-[1px] rounded-2xl bg-background/60 backdrop-blur-xl" />
          
          {/* Content */}
          <div className="relative z-10">
            {generating ? (
              <div className="flex items-center gap-2 text-white">
                <div className="h-5 w-5 animate-spin rounded-full border-2 border-white/30 border-t-white" />
                <span className="tracking-wider">渲染引擎运转中...</span>
              </div>
            ) : (
              <>
                <div className="flex items-center gap-2 text-white">
                  <Wand2 className="h-5 w-5 drop-shadow-[0_0_6px_rgba(255,255,255,0.6)]" />
                  <span className="tracking-[0.15em] text-[15px] drop-shadow-[0_0_8px_rgba(255,255,255,0.4)]">启动渲染引擎</span>
                </div>
                <div className="flex items-center gap-1 text-[10px] text-white/60 font-normal">
                  预计消耗 <Coins className="h-2.5 w-2.5" /> <span className="font-semibold text-white/80">{currentCost}</span> 积分
                </div>
              </>
            )}
          </div>
        </motion.button>
      </div>

      {/* ── Middle: Main Video Player ── */}
      <div className="hidden lg:flex flex-col h-full lg:col-span-6 xl:col-span-7 pb-4">
        <motion.div
           initial={{ opacity: 0, scale: 0.98 }}
           animate={{ opacity: 1, scale: 1 }}
           transition={{ delay: 0.1 }}
           className="glow-border relative flex-1 min-h-0 w-full overflow-hidden rounded-2xl bg-surface-elevated cyber-grid border border-border shadow-2xl flex items-center justify-center"
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
                {/* Advanced Sci-Fi Loader */}
                <div className="relative h-32 w-32 flex items-center justify-center">
                  <div className="absolute inset-0 rounded-full border-[1px] border-cyan/20 animate-[spin_4s_linear_infinite]" />
                  <div className="absolute inset-2 rounded-full border-2 border-transparent border-t-cyan border-b-accent animate-[spin_2s_ease-in-out_infinite]" />
                  <div className="absolute inset-6 rounded-full border-[1px] border-dashed border-accent/40 animate-[spin_3s_reverse_infinite]" />
                  <Video className="h-8 w-8 text-cyan animate-pulse z-10 drop-shadow-[0_0_8px_rgba(0,245,212,0.8)]" />
                </div>
                
                <div className="text-center space-y-3">
                  <h3 className="text-2xl font-black tracking-[0.2em] text-cyan text-glow-cyan uppercase">
                    Synergizing
                  </h3>
                  <div className="flex flex-col items-center gap-1">
                    <p className="text-xs text-text-muted tracking-wider">正在进行物理规律模拟与场景构建</p>
                    <div className="w-48 h-1 bg-surface rounded-full overflow-hidden mt-2">
                      <div className="h-full bg-gradient-to-r from-accent to-cyan w-1/2 rounded-full animate-[pulse_2s_ease-in-out_infinite] shadow-[0_0_10px_rgba(0,245,212,0.5)]" />
                    </div>
                  </div>
                </div>
              </motion.div>
            ) : selectedHistoryItem?.thumbnail ? (
              <motion.div
                key={selectedHistoryItem.id}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="group relative h-full w-full bg-black/50"
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img 
                  src={selectedHistoryItem.thumbnail} 
                  alt="Video frame" 
                  className="h-full w-full object-contain p-2"
                />
                
                {/* Play Button Overlay */}
                <div className="absolute inset-0 flex items-center justify-center">
                  <button className="flex h-20 w-20 items-center justify-center rounded-full bg-black/40 border border-white/10 text-white backdrop-blur-md transition-all hover:bg-accent/80 hover:scale-110 shadow-[0_0_30px_rgba(0,0,0,0.5)]">
                    <Play className="h-8 w-8 ml-1" />
                  </button>
                </div>
                
                {/* Video Controls Bar */}
                <div className="absolute bottom-4 left-4 right-4 flex items-center justify-between rounded-xl border border-white/10 bg-black/60 px-4 py-3 backdrop-blur-xl opacity-0 transition-opacity group-hover:opacity-100">
                   <div className="flex items-center gap-3">
                     <span className="text-xs font-mono text-cyan">{selectedHistoryItem.duration}</span>
                     <div className="h-1 w-64 bg-white/20 rounded-full overflow-hidden">
                       <div className="h-full w-1/3 bg-cyan" />
                     </div>
                   </div>
                   <div className="flex gap-2">
                      <button className="h-8 w-8 flex items-center justify-center rounded-lg hover:bg-white/10 text-white"><RotateCcw className="h-4 w-4"/></button>
                      <button className="h-8 w-8 flex items-center justify-center rounded-lg hover:bg-white/10 text-white"><Download className="h-4 w-4"/></button>
                   </div>
                </div>
                
                {/* Top status */}
                <div className="absolute left-6 top-6 flex items-center gap-2 rounded-lg border border-white/10 bg-black/40 px-3 py-1.5 backdrop-blur-md">
                  <span className="flex h-2 w-2 rounded-full bg-success pulse-online" />
                  <span className="text-xs font-medium text-white">{selectedHistoryItem.model} • 渲染已就绪</span>
                </div>
              </motion.div>
            ) : null}
          </AnimatePresence>
        </motion.div>
      </div>

      {/* ── Right: History List (Span 2/3) ── */}
      <div className="flex h-full flex-col gap-4 lg:col-span-2 xl:col-span-2 overflow-hidden border-l border-border/50 pl-6 pb-4">
        <h3 className="shrink-0 text-sm font-semibold text-text-secondary flex items-center gap-2 uppercase tracking-widest border-b border-border/50 pb-3">
          <Layers className="h-4 w-4 text-accent" /> 视频素材库
        </h3>
        
        <div className="flex-1 overflow-y-auto space-y-3 pr-1 custom-scrollbar">
          {mockVideos.map((v) => (
             <button
               key={v.id}
               onClick={() => setSelectedHistoryId(v.id)}
               className={cn(
                 "group relative w-full overflow-hidden rounded-xl border-2 transition-all text-left flex flex-col",
                 selectedHistoryId === v.id 
                   ? "border-cyan bg-cyan/5 shadow-[0_0_15px_rgba(0,245,212,0.1)]" 
                   : "border-border bg-surface hover:border-cyan/40 hover:bg-surface-hover"
               )}
             >
               {/* Tiny THumbnail part */}
               <div className="relative w-full aspect-video bg-black shrink-0 border-b border-border/50">
                 {v.thumbnail ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={v.thumbnail} className="h-full w-full object-cover opacity-80 group-hover:opacity-100 transition-opacity" alt=""/>
                 ) : (
                    <div className="flex h-full w-full items-center justify-center cyber-grid bg-surface-elevated">
                       <Video className="h-5 w-5 text-text-muted/50" />
                    </div>
                 )}
                 {/* Duration / Status overlay */}
                 <div className="absolute bottom-1 right-1 px-1.5 py-0.5 rounded bg-black/80 backdrop-blur text-[9px] text-white">
                   {v.status === "generating" ? "渲染中..." : v.duration}
                 </div>
               </div>
               
               {/* Quick Info part */}
               <div className="p-2.5 flex-1 w-full">
                 <p className="text-[11px] leading-tight text-text-primary line-clamp-2 mb-1.5">
                   {v.prompt}
                 </p>
                 <div className="flex items-center justify-between">
                   <span className="text-[9px] text-text-muted uppercase">{v.model}</span>
                   {v.status === "generating" ? (
                      <span className="text-[9px] text-warning flex items-center gap-1"><div className="h-1 w-1 bg-warning rounded-full animate-pulse"/>执行中</span>
                   ) : (
                      <span className="text-[9px] text-success">已完成</span>
                   )}
                 </div>
               </div>
             </button>
          ))}
        </div>
      </div>
      
    </div>
  );
}
