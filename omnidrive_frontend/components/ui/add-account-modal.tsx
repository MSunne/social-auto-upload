"use client";

import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { X, Smartphone, QrCode, ShieldCheck, Loader2, CheckCircle2 } from "lucide-react";
import Image from "next/image";
import { createRemoteLogin, getLoginSession, createLoginAction } from "@/lib/services";
import type { LoginSession } from "@/lib/types";

interface AddAccountModalProps {
  isOpen: boolean;
  onClose: () => void;
  deviceId: string;
}

const PLATFORMS = [
  { id: "douyin", name: "抖音", icon: "🎵", color: "from-gray-800 to-black", border: "border-gray-700" },
  { id: "xiaohongshu", name: "小红书", icon: "📕", color: "from-red-500 to-red-700", border: "border-red-500/50" },
  { id: "kuaishou", name: "快手", icon: "🎬", color: "from-orange-400 to-orange-600", border: "border-orange-500/50" },
  { id: "wechat_channel", name: "视频号", icon: "💬", color: "from-emerald-400 to-emerald-600", border: "border-emerald-500/50" },
];

export function AddAccountModal({ isOpen, onClose, deviceId }: AddAccountModalProps) {
  const [step, setStep] = useState<"form" | "waiting" | "qr" | "verification" | "success" | "error">("form");
  const [selectedPlatform, setSelectedPlatform] = useState<string>("douyin");
  const [accountName, setAccountName] = useState("");
  const [errorMsg, setErrorMsg] = useState("");
  
  const [session, setSession] = useState<LoginSession | null>(null);

  // Reset state on open
  useEffect(() => {
    if (isOpen) {
      setStep("form");
      setAccountName("");
      setErrorMsg("");
      setSession(null);
    }
  }, [isOpen]);

  // Polling logic
  useEffect(() => {
    if (!session?.id || step === "success" || step === "error" || step === "form") return;

    let mounted = true;
    let pollTimeout: NodeJS.Timeout;

    const poll = async () => {
      try {
        const updatedSession = await getLoginSession(session.id);
        if (!mounted) return;

        setSession(updatedSession);

        switch (updatedSession.status) {
          case "pending":
          case "running":
            if (updatedSession.qrData) {
              setStep("qr");
            } else {
              setStep("waiting");
            }
            break;
          case "verification_required":
            setStep("verification");
            break;
          case "success":
            setStep("success");
            break;
          case "failed":
          case "cancelled":
            setErrorMsg(updatedSession.message || "登录流程已终止");
            setStep("error");
            break;
        }

        // Continue polling if not in a final state
        if (!["success", "failed", "cancelled"].includes(updatedSession.status)) {
          pollTimeout = setTimeout(poll, 2000);
        }
      } catch (err: any) {
        console.error("Polling error:", err);
        // Don't kill the flow on single network errors, try again
        pollTimeout = setTimeout(poll, 3000);
      }
    };

    poll();

    return () => {
      mounted = false;
      clearTimeout(pollTimeout);
    };
  }, [session?.id, step]);

  const handleStartLogin = async () => {
    if (!accountName.trim()) {
      setErrorMsg("请输入账号名称");
      return;
    }

    try {
      setStep("waiting");
      setErrorMsg("");
      const platformName = PLATFORMS.find(p => p.id === selectedPlatform)?.name || selectedPlatform;
      const newSession = await createRemoteLogin({
        deviceId, 
        platform: platformName, 
        accountName: accountName.trim()
      });
      setSession(newSession);
    } catch (err: any) {
      setErrorMsg(err.message || "创建登录会话失败");
      setStep("error");
    }
  };

  const handleVerificationSubmit = async (answer: string) => {
    if (!session?.id || !answer) return;
    try {
      setStep("waiting"); // Show waiting state while processing
      await createLoginAction(session.id, {
        actionType: "verify_code",
        payload: { code: answer }
      });
      // Polling loop will naturally pick up the next state
    } catch (err: any) {
      setErrorMsg(err.message || "提交验证码失败");
    }
  };

  const handleClose = () => {
    onClose();
    if (step === "success") {
      window.setTimeout(() => {
        window.location.reload();
      }, 300); // Give modal fade-out time before hard reload
    }
  };

  // Auto close on success
  useEffect(() => {
    if (step === "success") {
      const timer = setTimeout(() => {
        handleClose();
      }, 3000);
      return () => clearTimeout(timer);
    }
  }, [step]);


  if (!isOpen) return null;

  return (
    <AnimatePresence>
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6">
        {/* Backdrop */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
          className="fixed inset-0 bg-black/60 backdrop-blur-md"
          onClick={handleClose}
        />

        {/* Modal */}
        <motion.div
          initial={{ opacity: 0, scale: 0.95, y: 16 }}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={{ opacity: 0, scale: 0.95, y: 16 }}
          className="relative z-10 w-full max-w-md overflow-hidden rounded-3xl border border-white/10 bg-[#0A0A14]/95 backdrop-blur-xl shadow-2xl"
        >
          {/* Header */}
          <div className="relative border-b border-white/5 bg-gradient-to-r from-accent/10 to-transparent px-6 py-5">
            <div className="flex items-center justify-between relative z-10">
              <h3 className="text-xl font-black text-white flex items-center gap-2">
                <Smartphone className="h-5 w-5 text-accent" />
                添加远程账号
              </h3>
              <button
                onClick={handleClose}
                className="rounded-full bg-white/5 p-2 text-text-muted hover:bg-white/10 hover:text-white transition-colors"
                title="关闭 / 取消"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </div>

          <div className="p-6">
            {step === "form" && (
              <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-6">
                <div>
                  <label className="mb-2 block text-sm font-bold text-text-primary">
                    选择自媒体平台
                  </label>
                  <div className="grid grid-cols-2 gap-3">
                    {PLATFORMS.map((platform) => (
                      <button
                        key={platform.id}
                        onClick={() => setSelectedPlatform(platform.id)}
                        className={`flex items-center gap-3 rounded-xl border p-3 transition-all ${
                          selectedPlatform === platform.id
                            ? `bg-gradient-to-br ${platform.color} ${platform.border} shadow-lg ring-2 ring-white/20`
                            : "border-white/5 bg-white/5 hover:bg-white/10"
                        }`}
                      >
                        <span className="text-xl">{platform.icon}</span>
                        <span className="font-bold text-white text-sm">{platform.name}</span>
                      </button>
                    ))}
                  </div>
                </div>

                <div>
                  <label className="mb-2 block text-sm font-bold text-text-primary">
                    内部账号备注名
                  </label>
                  <input
                    type="text"
                    value={accountName}
                    onChange={(e) => setAccountName(e.target.value)}
                    placeholder="例如：官方主账号_01"
                    className="w-full rounded-xl border border-white/10 bg-black/50 px-4 py-3 text-sm text-white placeholder-text-muted/50 focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent"
                  />
                  <p className="mt-2 text-xs text-text-muted">此名称仅用于系统内部区分，不会影响您在平台上的实际显示名称。</p>
                </div>

                {errorMsg && (
                  <div className="rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-400">
                    {errorMsg}
                  </div>
                )}

                <button
                  onClick={handleStartLogin}
                  className="w-full rounded-xl bg-gradient-to-r from-accent to-cyan p-3.5 text-sm font-bold text-white shadow-lg shadow-accent/20 transition-all hover:shadow-accent/40 active:scale-[0.98]"
                >
                  发起设备内登录
                </button>
              </motion.div>
            )}

            {step === "waiting" && (
              <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="flex flex-col items-center justify-center py-10 space-y-6">
                <div className="relative">
                  <div className="absolute inset-0 rounded-full bg-accent/20 blur-xl animate-pulse" />
                  <div className="relative flex h-20 w-20 items-center justify-center rounded-full border border-white/10 bg-surface shadow-xl">
                    <Loader2 className="h-8 w-8 text-accent animate-spin" />
                  </div>
                </div>
                <div className="text-center">
                  <h4 className="text-lg font-bold text-white mb-2">正在连接目标设备</h4>
                  <p className="text-sm text-text-muted">
                    {session?.message || "等待设备端 SAU 程序响应并拉起浏览器..."}
                  </p>
                </div>
                <button
                  onClick={handleClose}
                  className="mt-2 text-sm text-text-muted/60 hover:text-white transition-colors underline underline-offset-4"
                >
                  取消等待
                </button>
              </motion.div>
            )}

            {step === "qr" && session?.qrData && (
              <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="flex flex-col items-center text-center space-y-5">
                <div className="rounded-2xl border border-white/10 bg-white p-4 shadow-xl">
                  <Image 
                    src={session.qrData} 
                    alt="登录二维码" 
                    width={220} 
                    height={220} 
                    unoptimized 
                    className="rounded-lg"
                  />
                </div>
                <div>
                  <h4 className="text-lg font-bold text-white flex items-center justify-center gap-2 mb-1">
                    <QrCode className="h-5 w-5 text-cyan" />
                    扫码登录
                  </h4>
                  <p className="text-sm text-text-muted">
                    {session.message || "请使用对应平台 App 扫描上方二维码"}
                  </p>
                </div>
                <div className="flex items-center gap-4 w-full pt-2">
                  <button
                    onClick={handleClose}
                    className="flex-1 rounded-xl border border-white/10 bg-white/5 p-3 text-sm font-bold text-text-muted transition-colors hover:bg-white/10 hover:text-white"
                  >
                    取消登录
                  </button>
                </div>
              </motion.div>
            )}

            {step === "verification" && session?.verificationPayload && (
              <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-5">
                <div className="text-center">
                  <h4 className="text-lg font-bold text-white flex items-center justify-center gap-2 mb-2">
                    <ShieldCheck className="h-5 w-5 text-amber-400" />
                    {(session.verificationPayload as any).title || "需要安全验证"}
                  </h4>
                  <p className="text-sm text-text-muted">
                    {(session.verificationPayload as any).message || "平台需要进行二次验证"}
                  </p>
                </div>
                
                {(session.verificationPayload as any).screenshotData && (
                   <div className="rounded-xl border border-white/10 overflow-hidden">
                     <Image 
                        src={(session.verificationPayload as any).screenshotData}
                        alt="验证码截图"
                        width={400}
                        height={300}
                        unoptimized
                        className="w-full object-contain max-h-[200px] bg-black/50"
                     />
                   </div>
                )}

                <form onSubmit={(e) => {
                  e.preventDefault();
                  const formData = new FormData(e.currentTarget);
                  handleVerificationSubmit(formData.get("code") as string);
                }}>
                  <input
                    type="text"
                    name="code"
                    required
                    placeholder="请输入验证码..."
                    className="w-full rounded-xl border border-white/10 bg-black/50 px-4 py-3 text-center text-lg font-mono text-white tracking-widest focus:border-accent focus:outline-none"
                    autoComplete="off"
                  />
                  <button
                    type="submit"
                    className="mt-4 w-full rounded-xl bg-amber-500/20 border border-amber-500/30 p-3.5 text-sm font-bold text-amber-400 transition-colors hover:bg-amber-500/30"
                  >
                    提交验证
                  </button>
                  <button
                    type="button"
                    onClick={handleClose}
                    className="mt-3 w-full rounded-xl border border-white/10 bg-transparent p-3.5 text-sm font-bold text-text-muted transition-colors hover:bg-white/10 hover:text-white"
                  >
                    取消验证
                  </button>
                </form>
              </motion.div>
            )}

            {step === "success" && (
              <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="flex flex-col items-center justify-center py-8 space-y-4">
                <div className="flex h-20 w-20 items-center justify-center rounded-full bg-emerald-500/20 border border-emerald-500/30">
                  <CheckCircle2 className="h-10 w-10 text-emerald-400" />
                </div>
                <div className="text-center">
                  <h4 className="text-xl font-bold text-white mb-2">账号添加成功</h4>
                  <p className="text-sm text-text-muted">账号数据已成功同步至云端和目标设备。</p>
                </div>
                <button
                  onClick={handleClose}
                  className="mt-4 w-full rounded-xl bg-white/10 p-3.5 text-sm font-bold text-white transition-colors hover:bg-white/20"
                >
                  关闭弹窗
                </button>
              </motion.div>
            )}

            {step === "error" && (
              <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="flex flex-col items-center justify-center py-8 space-y-4 text-center">
                <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-500/20 border border-red-500/30">
                  <X className="h-8 w-8 text-red-400" />
                </div>
                <div>
                  <h4 className="text-lg font-bold text-white mb-2">流程中断</h4>
                  <p className="text-sm text-red-300 px-4">{errorMsg}</p>
                </div>
                <button
                  onClick={() => setStep("form")}
                  className="mt-4 w-full rounded-xl bg-white/10 p-3.5 text-sm font-bold text-white transition-colors hover:bg-white/20"
                >
                  重新尝试
                </button>
              </motion.div>
            )}

          </div>
        </motion.div>
      </div>
    </AnimatePresence>
  );
}
