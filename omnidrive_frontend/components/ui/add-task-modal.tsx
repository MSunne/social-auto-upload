"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  X,
  Search,
  Zap,
  Sun,
  Sparkles,
  PlusCircle,
  Clock,
  CalendarDays,
  Info,
  ChevronRight,
  Brain,
  Video,
  Image as ImageIcon,
  FileText,
} from "lucide-react";
import type { Skill } from "@/lib/types";

interface AddTaskModalProps {
  isOpen: boolean;
  onClose: () => void;
  skills: Skill[];
  accountName: string;
}

export function AddTaskModal({
  isOpen,
  onClose,
  skills,
  accountName,
}: AddTaskModalProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedSkillId, setSelectedSkillId] = useState<string | null>(null);
  
  // Controls the slide-over detail panel
  const [detailSkillId, setDetailSkillId] = useState<string | null>(null);
  // Controls image preview modal
  const [previewImageUrl, setPreviewImageUrl] = useState<string | null>(null);

  const [timeSlots, setTimeSlots] = useState<string[]>(["09:00 AM", "06:30 PM"]);
  const [repeatDaily, setRepeatDaily] = useState(true);

  if (!isOpen) return null;

  const filteredSkills = skills.filter(
    (s) =>
      s.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      s.description.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const detailSkill = skills.find((s) => s.id === detailSkillId);

  const handleCreate = () => {
    console.log("Creating task for", accountName, "with skill", selectedSkillId);
    onClose();
  };

  return (
    <AnimatePresence>
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6 sm:px-12 md:px-24">
        {/* Backdrop */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
          className="fixed inset-0 bg-black/60 backdrop-blur-md"
          onClick={onClose}
        />

        {/* Modal Dialog Container - relative for slide-over positioning */}
        <div className="relative w-full max-w-5xl h-full max-h-[85vh] flex justify-center items-center gap-4 py-8">
          
          {/* Main Modal Area */}
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 16 }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
            className={`relative z-10 w-full flex flex-col overflow-hidden rounded-3xl border border-white/10 bg-[#0A0A14] bg-opacity-95 shadow-[0_0_80px_rgba(177,73,255,0.15)] transition-[max-width] duration-300 ease-in-out h-full max-h-full ${detailSkillId ? 'max-w-xl' : 'max-w-2xl'}`}
          >
            {/* Header */}
            <div className="relative border-b border-white/5 bg-gradient-to-r from-accent/10 to-transparent px-8 py-6">
              <div className="absolute inset-0 bg-noise opacity-[0.03] mix-blend-overlay pointer-events-none" />
              <div className="flex items-center justify-between relative z-10">
                <div>
                  <h3 className="text-2xl font-black text-white flex items-center gap-3 tracking-wide">
                    配置新任务 
                    <span className="rounded-full bg-accent/20 px-2.5 py-0.5 text-[10px] font-bold text-accent uppercase tracking-widest border border-accent/20">
                      Add Task
                    </span>
                  </h3>
                  <p className="mt-2 text-sm text-text-muted/80 font-medium">
                    正在为 <span className="text-transparent bg-clip-text bg-gradient-to-r from-cyan to-accent font-bold">@{accountName}</span> 配置自动化技能
                  </p>
                </div>
                <button
                  onClick={onClose}
                  className="rounded-full bg-white/5 p-2.5 text-text-muted transition-all hover:bg-red-500/20 hover:text-red-400 hover:rotate-90"
                >
                  <X className="h-5 w-5" />
                </button>
              </div>
            </div>

            {/* Scrollable Content Range */}
            <div className="flex-1 px-8 py-8 overflow-y-auto custom-scroll relative">
              <div className="absolute inset-x-0 top-0 h-10 bg-gradient-to-b from-[#0A0A14] to-transparent pointer-events-none z-10" />

              {/* SELECT SKILLS SECTION */}
              <div className="mb-10 relative z-0">
                <div className="flex items-center gap-3 mb-5">
                  <div className="h-6 w-1.5 rounded-full bg-gradient-to-b from-cyan to-accent" />
                  <h4 className="text-base font-black tracking-widest text-white uppercase drop-shadow-[0_0_10px_rgba(0,245,212,0.3)]">
                    第一步：选择核心技能
                  </h4>
                </div>

                {/* Search Bar */}
                <div className="relative mb-5 group">
                  <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 transition-transform group-focus-within:scale-110">
                    <Search className="h-4 w-4 text-cyan" />
                  </div>
                  <input
                    type="text"
                    placeholder="输入关键词搜索现有技能..."
                    className="w-full rounded-2xl border border-white/10 bg-white/5 py-3.5 pl-12 pr-4 text-sm text-white placeholder-text-muted/50 transition-all focus:border-cyan/50 focus:bg-white/10 focus:outline-none focus:ring-4 focus:ring-cyan/10"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                </div>

                {/* Skills List */}
                <div className="space-y-4">
                  {filteredSkills.map((skill, index) => {
                    const isSelected = selectedSkillId === skill.id;
                    const isDetailOpen = detailSkillId === skill.id;
                    
                    const icons = [
                      <Zap key="1" className="h-5 w-5 text-white" />,
                      <Sun key="2" className="h-5 w-5 text-white" />,
                      <Sparkles key="3" className="h-5 w-5 text-white" />
                    ];
                    const IconNode = icons[index % icons.length];
                    const gradientClass = index % 2 === 0 
                      ? "from-accent to-fuchsia-500" 
                      : "from-cyan to-blue-500";
                    const shadowGlow = index % 2 === 0 
                      ? "shadow-[0_0_20px_rgba(177,73,255,0.25)]" 
                      : "shadow-[0_0_20px_rgba(0,245,212,0.25)]";

                    return (
                      <div
                        key={skill.id}
                        onClick={() => setSelectedSkillId(skill.id)}
                        className={`group relative flex cursor-pointer rounded-2xl border transition-all duration-300 overflow-hidden ${
                          isSelected
                            ? `border-white/20 bg-white/10 scale-[1.02] ${shadowGlow}`
                            : "border-white/5 bg-white/5 hover:bg-white/10 hover:border-white/15"
                        }`}
                      >
                        {/* Selected Background Gradient Reveal */}
                        {isSelected && (
                          <div className={`absolute inset-0 opacity-10 bg-gradient-to-r ${gradientClass} pointer-events-none`} />
                        )}

                        <div className="flex w-full p-5 gap-5 relative z-10">
                          {/* Radio button area */}
                          <div className="pt-1 flex-shrink-0">
                            <div
                              className={`flex h-6 w-6 items-center justify-center rounded-full border-2 transition-all duration-300 ${
                                isSelected
                                  ? `border-transparent bg-gradient-to-br ${gradientClass}`
                                  : "border-text-muted/30 group-hover:border-text-muted/60"
                              }`}
                            >
                              {isSelected && (
                                <motion.div 
                                  initial={{ scale: 0 }} 
                                  animate={{ scale: 1 }} 
                                  className="h-2.5 w-2.5 rounded-full bg-white shadow-md" 
                                />
                              )}
                            </div>
                          </div>

                          <div className="flex-1 min-w-0">
                            <div className="flex items-start justify-between">
                              <div>
                                <h5 className={`font-black text-lg truncate transition-colors ${isSelected ? 'text-white drop-shadow-md' : 'text-white/90'}`}>
                                  {skill.name}
                                </h5>
                                <div className="mt-1 flex flex-wrap items-center gap-2">
                                  <span className="text-[10px] font-mono text-text-muted/50 drop-shadow-sm">{skill.id.replace('sk_', 'SK-884')}</span>
                                  {skill.outputType === "image_text" ? (
                                    <span className="rounded bg-gradient-to-r from-cyan/20 to-blue-500/20 px-2 py-0.5 text-[10px] font-bold text-cyan border border-cyan/20 shadow-sm">
                                      图文模式
                                    </span>
                                  ) : (
                                    <span className="rounded bg-gradient-to-r from-purple-500/20 to-pink-500/20 px-2 py-0.5 text-[10px] font-bold text-purple-300 border border-purple-500/20 shadow-sm">
                                      视频模式
                                    </span>
                                  )}
                                </div>
                              </div>
                              <div className={`flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-gradient-to-br ${gradientClass} shadow-lg ml-4`}>
                                {IconNode}
                              </div>
                            </div>
                            
                            <p className="mt-3 text-sm text-text-muted/70 leading-relaxed line-clamp-2 pr-4 transition-colors group-hover:text-text-muted/90">
                              {skill.description}
                            </p>

                            {/* View Details Target Area - Separate from Selection */}
                            <div className="mt-4 flex items-center justify-between border-t border-white/5 pt-4">
                              <div className="flex -space-x-2">
                                {/* Decorative mockup generic thumbnails */}
                                <div className={`h-8 w-8 rounded-lg border-2 border-[#0A0A14] bg-gradient-to-br from-gray-700 to-gray-900 shadow-sm`} />
                                <div className={`h-8 w-8 rounded-lg border-2 border-[#0A0A14] bg-gradient-to-br from-indigo-800 to-purple-900 shadow-sm`} />
                                <div className={`h-8 w-8 rounded-lg border-2 border-[#0A0A14] flex items-center justify-center bg-white/10 backdrop-blur-sm shadow-sm text-[10px] text-white font-bold`}>+3</div>
                              </div>
                              
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setDetailSkillId(isDetailOpen ? null : skill.id);
                                }}
                                className={`inline-flex items-center gap-1.5 rounded-full px-4 py-1.5 text-xs font-bold transition-all ${
                                  isDetailOpen 
                                  ? "bg-white text-[#0A0A14] shadow-md"
                                  : "bg-white/10 text-white hover:bg-white/20 hover:scale-105"
                                }`}
                              >
                                <Info className="h-3.5 w-3.5" />
                                技能详解
                                <ChevronRight className={`h-3.5 w-3.5 transition-transform duration-300 ${isDetailOpen ? 'rotate-180' : ''}`} />
                              </button>
                            </div>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* EXECUTION TIME SECTION */}
              <div className="mb-8">
                <div className="flex items-center justify-between mb-5">
                  <div className="flex items-center gap-3">
                    <div className="h-6 w-1.5 rounded-full bg-gradient-to-b from-fuchsia-500 to-rose-500" />
                    <h4 className="text-base font-black tracking-widest text-white uppercase drop-shadow-[0_0_10px_rgba(236,72,153,0.3)]">
                      第二步：配置调度策略
                    </h4>
                  </div>
                </div>

                <div className="rounded-2xl border border-white/10 bg-white/5 p-5 backdrop-blur-sm relative overflow-hidden">
                  <div className="absolute top-0 right-0 w-32 h-32 bg-fuchsia-500/10 rounded-full blur-3xl" />
                  
                  <div className="flex items-center justify-between mb-4 relative z-10">
                    <span className="text-sm font-bold text-white">执行时间槽位 (Time Slots)</span>
                    <button className="flex items-center gap-1.5 rounded-full bg-fuchsia-500/20 px-3 py-1 text-[11px] font-bold text-fuchsia-300 transition-all hover:bg-fuchsia-500/30 hover:scale-105 border border-fuchsia-500/30">
                      <PlusCircle className="h-3 w-3" />
                      增加时间
                    </button>
                  </div>

                  <div className="flex flex-wrap gap-3 relative z-10">
                    {timeSlots.map((time, idx) => (
                      <div key={idx} className="group flex items-center gap-2 rounded-xl border border-fuchsia-500/30 bg-black/40 px-4 py-2 text-sm font-bold text-fuchsia-100 shadow-inner transition-colors hover:border-fuchsia-400">
                        <Clock className="h-4 w-4 text-fuchsia-400" />
                        {time}
                        <button 
                          onClick={() => setTimeSlots(slots => slots.filter((_, i) => i !== idx))}
                          className="ml-2 rounded-full p-0.5 opacity-40 transition-all hover:bg-red-500/20 hover:text-red-400 hover:opacity-100"
                        >
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              {/* REPEAT DAILY TOGGLE */}
              <div className="mb-4">
                <div className="flex items-center justify-between rounded-2xl border border-white/10 bg-gradient-to-r from-white/5 to-transparent p-6 shadow-sm">
                  <div className="flex items-center gap-4">
                    <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-orange-400 to-rose-500 shadow-lg shadow-orange-500/20">
                      <CalendarDays className="h-6 w-6 text-white" />
                    </div>
                    <div>
                      <h5 className="font-black text-white text-base">每天自动循环 (Repeat Daily)</h5>
                      <p className="text-xs font-medium text-text-muted mt-1">开启后，系统将在所选时间点每日自动执行此技能</p>
                    </div>
                  </div>
                  
                  {/* Toggle Switch */}
                  <button
                    type="button"
                    role="switch"
                    aria-checked={repeatDaily}
                    onClick={() => setRepeatDaily(!repeatDaily)}
                    className={`relative inline-flex h-7 w-14 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-300 ease-in-out focus:outline-none focus:ring-4 focus:ring-rose-500/30 ${
                      repeatDaily ? "bg-gradient-to-r from-orange-400 to-rose-500 shadow-[0_0_15px_rgba(244,63,94,0.4)]" : "bg-white/10"
                    }`}
                  >
                    <span
                      aria-hidden="true"
                      className={`inline-block h-6 w-6 transform rounded-full bg-white shadow-md ring-0 transition duration-300 ease-bounce ${
                        repeatDaily ? "translate-x-7" : "translate-x-0"
                      }`}
                    />
                  </button>
                </div>
              </div>
            </div>

            {/* Footer actions */}
            <div className="relative border-t border-white/5 bg-black/80 px-8 py-6 flex items-center justify-end gap-x-6 backdrop-blur-xl z-20 shadow-[0_-10px_40px_rgba(0,0,0,0.5)]">
              <button
                onClick={onClose}
                className="rounded-full px-6 py-2.5 text-sm font-bold text-text-muted hover:text-white hover:bg-white/10 transition-all border border-transparent hover:border-white/10"
              >
                取消 (Cancel)
              </button>
              <button
                onClick={handleCreate}
                disabled={!selectedSkillId}
                className={`group relative overflow-hidden rounded-full px-10 py-3 text-sm font-black shadow-2xl transition-all duration-300 ${
                  selectedSkillId 
                  ? "bg-gradient-to-r from-accent via-cyan to-accent bg-[length:200%_auto] text-white hover:scale-[1.05] hover:shadow-[0_0_40px_rgba(0,245,212,0.6)] animate-gradient" 
                  : "bg-white/5 border border-white/10 text-white/30 cursor-not-allowed"
                }`}
              >
                {selectedSkillId && (
                  <div className="absolute inset-0 bg-white/20 opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none mix-blend-overlay" />
                )}
                <span className="relative z-10 drop-shadow-md">确认并创建任务</span>
              </button>
            </div>
          </motion.div>

          {/* Slide-over Detail Panel */}
          <AnimatePresence>
            {detailSkillId && detailSkill && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 20 }}
                transition={{ type: "spring", stiffness: 300, damping: 30 }}
                className="relative z-0 h-full w-full max-w-md rounded-3xl border border-white/10 bg-[#12121A] shadow-2xl overflow-y-auto custom-scroll flex flex-col"
              >
                <div className="p-8">
                  <div className="mb-6 flex items-center justify-between">
                    <h3 className="text-xl font-black text-white">技能详情</h3>
                    <button onClick={() => setDetailSkillId(null)} className="rounded-full bg-white/5 p-2 hover:bg-white/10 text-white transition-colors">
                      <X className="h-4 w-4" />
                    </button>
                  </div>

                  <div className="space-y-6">
                    {/* Materials (Images & Text) moved to top */}
                    {detailSkill.materials && detailSkill.materials.length > 0 && (
                      <div className="rounded-2xl bg-white/5 border border-white/10 shadow-lg">
                        <div className="px-4 py-3 border-b border-white/10 flex items-center gap-2 bg-gradient-to-r from-accent/20 to-transparent rounded-t-2xl">
                          <ImageIcon className="h-4 w-4 text-accent" />
                          <span className="text-[11px] font-black text-white tracking-widest uppercase shadow-sm">知识库与参考素材 / {detailSkill.materials.length} ITEMS</span>
                        </div>
                        {/* Thumbnails row */}
                        <div className="px-4 pt-4 pb-2 flex flex-wrap gap-3">
                          {detailSkill.materials.map((mat) => (
                            <div key={mat.id} className="relative group/thumb shrink-0">
                              {mat.type === 'image' && mat.previewUrl ? (
                                <div 
                                  className={`h-14 w-14 rounded-xl overflow-hidden relative shadow-md cursor-pointer transition-all duration-200 border-2 ${
                                    previewImageUrl === mat.previewUrl 
                                      ? 'border-[#ae66ff] shadow-[0_0_16px_rgba(174,102,255,0.4)] scale-105' 
                                      : 'border-white/10 hover:border-[#ae66ff]/50'
                                  }`}
                                  onClick={() => setPreviewImageUrl(previewImageUrl === mat.previewUrl ? null : (mat.previewUrl || null))}
                                >
                                  <img src={mat.previewUrl} alt={mat.fileName} className="w-full h-full object-cover" />
                                </div>
                              ) : (
                                <div className="h-14 w-14 rounded-xl border-2 border-white/10 border-dashed bg-white/5 flex flex-col items-center justify-center p-1.5 cursor-pointer hover:bg-white/10 transition-all shadow-md">
                                  <FileText className="h-4 w-4 text-cyan mb-0.5 opacity-70" />
                                  <span className="text-[6px] text-text-muted text-center leading-tight line-clamp-2 w-full font-bold">{mat.fileName || '文本'}</span>
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                        {/* Inline preview area — only shown when an image is selected */}
                        {previewImageUrl && (
                          <div className="px-4 pb-4">
                            <div className="relative w-full rounded-xl overflow-hidden border border-[#ae66ff]/30 bg-black/30 shadow-[0_0_24px_rgba(174,102,255,0.15)]">
                              <img 
                                src={previewImageUrl} 
                                alt="Preview" 
                                className="w-full max-h-48 object-contain bg-black/20" 
                              />
                              <button 
                                onClick={() => setPreviewImageUrl(null)}
                                className="absolute top-2 right-2 rounded-full bg-black/60 backdrop-blur-sm p-1.5 text-white/70 hover:text-white hover:bg-black/80 transition-colors"
                              >
                                <X className="h-3.5 w-3.5" />
                              </button>
                            </div>
                          </div>
                        )}
                      </div>
                    )}

                    <div>
                      <h4 className="text-lg font-black text-white mb-2">{detailSkill.name}</h4>
                      <p className="text-sm text-text-muted leading-relaxed font-medium">
                        {detailSkill.description}
                      </p>
                    </div>

                    <div className="h-px w-full bg-white/10" />

                    <div className="grid grid-cols-2 gap-4">
                      {/* Generative Model */}
                      <div className="col-span-2 rounded-xl bg-gradient-to-br from-white/5 to-transparent p-4 border border-white/5">
                        <span className="block text-[10px] font-black tracking-widest text-cyan uppercase mb-2 drop-shadow-sm flex items-center gap-1.5 opacity-80">
                          <Zap className="h-3 w-3" /> 生成主模型 (CORE MODEL)
                        </span>
                        <span className="text-base font-bold text-white flex items-center gap-2">
                          {detailSkill.modelName}
                        </span>
                      </div>
                      
                      {/* Reasoning Model (Optional) */}
                      {detailSkill.reasoningModel && (
                        <div className="rounded-xl bg-white/5 p-4 border border-white/5 shadow-inner">
                          <span className="block text-[10px] font-black tracking-widest text-accent uppercase mb-2 drop-shadow-sm flex items-center gap-1.5 opacity-80">
                            <Brain className="h-3 w-3" /> 推理逻辑模型
                          </span>
                          <span className="text-sm font-bold text-white leading-tight">
                            {detailSkill.reasoningModel}
                          </span>
                        </div>
                      )}

                      {/* Video Model (Optional) */}
                      {detailSkill.videoModel && (
                        <div className="rounded-xl bg-white/5 p-4 border border-white/5 shadow-inner">
                          <span className="block text-[10px] font-black tracking-widest text-[#fb7185] uppercase mb-2 drop-shadow-sm flex items-center gap-1.5 opacity-80">
                            <Video className="h-3 w-3" /> 视频生成引擎
                          </span>
                          <span className="text-sm font-bold text-white leading-tight">
                            {detailSkill.videoModel}
                          </span>
                        </div>
                      )}
                      
                      {/* Status */}
                      <div className={`rounded-xl bg-white/5 p-4 border border-white/5 shadow-inner ${(!detailSkill.reasoningModel && !detailSkill.videoModel) ? 'col-span-2' : 'col-span-2'}`}>
                        <span className="block text-[10px] font-black tracking-widest text-emerald-400 uppercase mb-2 drop-shadow-sm opacity-80">当前状态 (STATUS)</span>
                        <span className={`text-sm font-bold flex items-center gap-2 ${detailSkill.isEnabled ? 'text-emerald-400' : 'text-amber-400'}`}>
                          <div className={`h-2.5 w-2.5 rounded-full ${detailSkill.isEnabled ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.8)]' : 'bg-amber-400 shadow-[0_0_8px_rgba(251,191,36,0.8)] animate-pulse'}`} />
                          {detailSkill.isEnabled ? '已启用 - 运行良好' : '已禁用 - 需重置'}
                        </span>
                      </div>
                    </div>


                    {detailSkill.promptTemplate && (
                      <div className="rounded-2xl border border-white/10 bg-black/40 overflow-hidden">
                        <div className="bg-white/5 px-4 py-2 border-b border-white/10 flex items-center gap-2">
                          <div className="flex gap-1.5">
                            <div className="h-2.5 w-2.5 rounded-full bg-red-400/80" />
                            <div className="h-2.5 w-2.5 rounded-full bg-yellow-400/80" />
                            <div className="h-2.5 w-2.5 rounded-full bg-green-400/80" />
                          </div>
                          <span className="text-[10px] font-bold text-white/50 tracking-widest uppercase ml-2">PROMPT TEMPLATE</span>
                        </div>
                        <div className="p-4">
                          <p className="text-xs text-fuchsia-200/80 font-mono leading-relaxed break-words whitespace-pre-wrap">
                            {detailSkill.promptTemplate}
                          </p>
                        </div>
                      </div>
                    )}

                    <div className="pt-4 flex items-center justify-between text-[11px] font-bold text-text-muted/40 uppercase tracking-widest border-t border-white/10">
                      <span>Last Updated</span>
                      <span>{new Date(detailSkill.updatedAt).toLocaleDateString('en-US')}</span>
                    </div>
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>

        </div>
      </div>
      
      {/* Fullscreen preview removed — inline preview is inside the detail panel */}

    </AnimatePresence>
  );
}
