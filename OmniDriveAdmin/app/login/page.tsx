"use client";

import { useState } from "react";
import { useAuth } from "@/lib/hooks/useAuth";
import { Loader2, Lock, Mail, ArrowRight } from "lucide-react";
import { useRouter } from "next/navigation";

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errorBuf, setErrorBuf] = useState("");
  const { login, isLoading } = useAuth();
  const router = useRouter();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorBuf("");
    
    if (!email || !password) {
      setErrorBuf("请输入邮箱和密码");
      return;
    }

    try {
      const user = await login(email, password);
      if (user) {
        router.push("/dashboard"); // Redirect to main dashboard
      } else {
        setErrorBuf("登录失败，请检查账号密码是否正确");
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        // @ts-expect-error - axios response type is unknown in this catch block
        setErrorBuf(err.response?.data?.error || err.message || "登录请求异常");
      } else {
        setErrorBuf("登录请求异常");
      }
    }
  };

  return (
    <main className="min-h-screen bg-[var(--color-bg-secondary)] px-6 py-10 flex items-center justify-center font-sans">
      <div className="w-full max-w-md">
        
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-[var(--color-primary)] text-white mb-4 shadow-lg shadow-[var(--color-primary)]/30">
            <Lock className="w-6 h-6" />
          </div>
          <h1 className="text-2xl font-bold text-[var(--color-text-primary)] tracking-tight">
            OmniDrive Admin
          </h1>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            内部运营后台管理系统，请验证您的身份
          </p>
        </div>

        <div className="bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-2xl shadow-xl shadow-black/5 p-8">
          <form onSubmit={handleLogin} className="space-y-5 flex flex-col">
            
            {errorBuf && (
              <div className="p-3 text-sm bg-red-500/10 text-red-500 rounded-lg border border-red-500/20 text-center">
                {errorBuf}
              </div>
            )}

            <div>
              <label className="block text-xs font-semibold text-[var(--color-text-secondary)] uppercase tracking-wider mb-1.5">
                管理员邮箱 Address
              </label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-text-secondary)]" />
                <input 
                  type="email" 
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  disabled={isLoading}
                  placeholder="admin@example.com"
                  className="w-full pl-9 pr-4 py-2.5 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]/20 focus:border-[var(--color-primary)] transition-all font-medium disabled:opacity-50" 
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--color-text-secondary)] uppercase tracking-wider mb-1.5 flex justify-between">
                安全密码 Password
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-text-secondary)]" />
                <input 
                  type="password" 
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  disabled={isLoading}
                  placeholder="••••••••"
                  className="w-full pl-9 pr-4 py-2.5 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]/20 focus:border-[var(--color-primary)] transition-all font-medium disabled:opacity-50" 
                />
              </div>
            </div>

            <button 
              type="submit" 
              disabled={isLoading}
              className="mt-2 w-full flex items-center justify-center gap-2 py-3 bg-[var(--color-primary)] text-white rounded-xl text-sm font-semibold hover:brightness-110 active:scale-[0.98] transition-all disabled:opacity-50 shadow-md shadow-[var(--color-primary)]/20">
              {isLoading ? <Loader2 className="w-5 h-5 animate-spin" /> : (
                <>安全进入系统 <ArrowRight className="w-4 h-4" /></>
              )}
            </button>
          </form>
        </div>

        <p className="mt-8 text-center text-xs text-[var(--color-text-secondary)]">
          &copy; {new Date().getFullYear()} OmniDrive Cloud System. All rights reserved.
        </p>
      </div>
    </main>
  );
}
