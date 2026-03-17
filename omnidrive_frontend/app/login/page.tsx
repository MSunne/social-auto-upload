"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import { Zap, Eye, EyeOff, Sparkles } from "lucide-react";
import { login, register, getCurrentUser } from "@/lib/services";
import { useAuthStore } from "@/lib/store";
import { mockUser, mockToken } from "@/lib/mock-data";

export default function LoginPage() {
  const router = useRouter();
  const { setAuth } = useAuthStore();

  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [showPwd, setShowPwd] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      if (mode === "register") {
        await register(email, name, password);
      }
      const resp = await login(email, password);
      // Backend may return user in the login response, or we fetch it separately
      const user = resp.user ?? await getCurrentUser();
      setAuth(user, resp.accessToken);
      router.push("/dashboard");
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "操作失败，请检查网络连接";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden px-4">
      {/* Decorative background blurs */}
      <div className="pointer-events-none absolute -left-40 -top-40 h-[500px] w-[500px] rounded-full bg-accent/20 blur-[120px]" />
      <div className="pointer-events-none absolute -bottom-40 -right-40 h-[400px] w-[400px] rounded-full bg-cyan/15 blur-[100px]" />

      <motion.div
        initial={{ opacity: 0, y: 20, scale: 0.96 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.5, ease: "easeOut" }}
        className="glass-card-elevated w-full max-w-md p-8"
      >
        {/* Logo */}
        <div className="mb-8 flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-gradient-to-br from-accent to-cyan shadow-lg shadow-accent/20">
            <Zap className="h-5 w-5 text-background" />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tight text-text-primary">
              OmniDrive
            </h1>
            <p className="text-xs text-text-muted">Cloud Console</p>
          </div>
        </div>

        {/* Title */}
        <h2 className="mb-1 text-lg font-semibold text-text-primary">
          {mode === "login" ? "欢迎回来" : "创建账户"}
        </h2>
        <p className="mb-6 text-sm text-text-secondary">
          {mode === "login"
            ? "使用您的云端账户登录控制台"
            : "注册一个新的 OmniDrive 云端账户"}
        </p>

        <form onSubmit={handleSubmit} className="space-y-4">
          {mode === "register" && (
            <div>
              <label className="mb-1.5 block text-xs font-medium text-text-secondary">
                用户名
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="输入用户名"
                className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
                required
              />
            </div>
          )}

          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-secondary">
              手机号
            </label>
            <input
              type="text"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="请输入手机号码"
              className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
              required
            />
          </div>

          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-secondary">
              密码
            </label>
            <div className="relative">
              <input
                type={showPwd ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full rounded-xl border border-border bg-surface px-4 py-3 pr-10 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
                required
              />
              <button
                type="button"
                onClick={() => setShowPwd(!showPwd)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary"
              >
                {showPwd ? (
                  <EyeOff className="h-4 w-4" />
                ) : (
                  <Eye className="h-4 w-4" />
                )}
              </button>
            </div>
          </div>

          {error && (
            <motion.div
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger"
            >
              {error}
            </motion.div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-xl bg-gradient-to-r from-accent to-cyan py-3 text-sm font-semibold text-background shadow-lg shadow-accent/25 transition-all hover:shadow-xl hover:shadow-accent/30 disabled:opacity-50"
          >
            {loading
              ? "处理中..."
              : mode === "login"
                ? "登录"
                : "注册并登录"}
          </button>

          {/* Demo 登录按钮 */}
          <button
            type="button"
            onClick={() => {
              setAuth(mockUser, mockToken);
              router.push("/dashboard");
            }}
            className="w-full rounded-xl border border-accent/30 bg-accent/5 py-3 text-sm font-semibold text-accent transition-all hover:bg-accent/10 hover:border-accent/50 flex items-center justify-center gap-2"
          >
            <Sparkles className="h-4 w-4" />
            Demo 体验登录（无需后端）
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-text-muted">
          {mode === "login" ? "还没有账户？" : "已有账户？"}
          <button
            onClick={() => {
              setMode(mode === "login" ? "register" : "login");
              setError("");
            }}
            className="ml-1 font-medium text-accent hover:text-accent-strong transition-colors"
          >
            {mode === "login" ? "立即注册" : "返回登录"}
          </button>
        </p>
      </motion.div>
    </div>
  );
}
