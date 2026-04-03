"use client";

import { motion, AnimatePresence } from "framer-motion";
import { X, ExternalLink, ShieldCheck } from "lucide-react";
import type { LoginSession } from "@/lib/types";
import { useEffect } from "react";
import { toast } from "react-hot-toast";

interface AddAccountModalProps {
  isOpen: boolean;
  onClose: () => void;
  deviceId: string;
  initialSession?: LoginSession | null;
}

export function AddAccountModal({ isOpen, onClose, deviceId }: AddAccountModalProps) {
  useEffect(() => {
    if (isOpen) {
      toast("平台账号请前往本地 OmniBull 添加", { icon: "ℹ️" });
    }
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <AnimatePresence>
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
        {/* Backdrop */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onClick={onClose}
          className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        />

        {/* Modal */}
        <motion.div
          initial={{ opacity: 0, scale: 0.95, y: 20 }}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={{ opacity: 0, scale: 0.95, y: 20 }}
          transition={{ type: "spring", damping: 25, stiffness: 300 }}
          className="relative w-full max-w-lg overflow-hidden rounded-2xl border border-gray-800 bg-gray-900/90 shadow-2xl shadow-indigo-500/10 backdrop-blur-xl"
        >
          {/* Header */}
          <div className="flex items-center justify-between border-b border-gray-800 p-6 pb-4">
            <div>
              <h2 className="text-xl font-bold text-white tracking-wide flex items-center gap-2">
                <ShieldCheck className="w-5 h-5 text-indigo-400" />
                添加与验证账号
              </h2>
              <p className="mt-1 text-sm text-gray-400">目前平台账号仅支持在本地端管理，以确保安全与稳定。</p>
            </div>
            <button
              onClick={onClose}
              className="rounded-full p-2 text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          <div className="p-8 text-center space-y-6">
            <div className="mx-auto w-16 h-16 bg-indigo-500/10 border border-indigo-500/20 rounded-2xl flex items-center justify-center">
              <ExternalLink className="w-8 h-8 text-indigo-400" />
            </div>
            
            <div className="space-y-2">
              <h3 className="text-lg font-medium text-gray-200">
                请前往本地 OmniBull (SAU) 进行操作
              </h3>
              <p className="text-sm text-gray-400 leading-relaxed max-w-[280px] mx-auto">
                为了提升验证成功率并保证 Cookie 时效，所有主流社交平台账号必须通过本地工程添加。
              </p>
            </div>

            <div className="bg-black/40 border border-gray-800 rounded-xl p-4 inline-block text-left w-full mt-4">
               <ol className="list-decimal pl-5 text-sm text-gray-300 space-y-3">
                 <li>确保您的本地 <strong>OmniBull (SAU)</strong> 工程正在运行</li>
                 <li>在浏览器中打开 <span className="text-indigo-400 font-mono bg-indigo-500/10 px-1 py-0.5 rounded">http://localhost:5409</span></li>
                 <li>进入左侧菜单的 <strong>「账号管理」</strong> 页面</li>
                 <li>点击 <strong>添加账号</strong> 并完成各平台的扫码验证</li>
                 <li>添加成功后点击 <strong>同步至云端</strong>，即可在此处查看账号状态</li>
               </ol>
            </div>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end border-t border-gray-800 bg-black/20 p-6 pt-4">
            <button
              onClick={onClose}
              className="rounded-xl bg-gray-800 px-6 py-2.5 text-sm font-medium text-gray-300 transition-colors hover:bg-gray-700 w-full"
            >
              我知道了
            </button>
          </div>
        </motion.div>
      </div>
    </AnimatePresence>
  );
}
