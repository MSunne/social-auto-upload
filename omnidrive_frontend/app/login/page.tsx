"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import { ArrowRight, Eye, EyeOff, Sparkles, Zap, Bot, Video, Share2, Activity } from "lucide-react";
import {
  getCurrentUser,
  loginWithPassword,
  loginWithPhone,
  register,
  sendRegisterSMSCode,
} from "@/lib/services";
import { useAuthStore } from "@/lib/store";
import { mockToken, mockUser } from "@/lib/mock-data";
import type { LoginResponse } from "@/lib/types";

function isValidPhone(value: string) {
  const digits = value.replace(/\D/g, "");
  return /^1\d{10}$/.test(digits);
}

function isValidCountryCode(value: string) {
  const digits = value.replace(/\D/g, "");
  return digits.length > 0;
}

const inputClassName =
  "w-full rounded-xl border border-white/10 bg-black/20 px-3.5 py-2.5 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/40 focus:ring-1 focus:ring-accent/40";
const labelClassName = "mb-1 block text-sm font-medium text-text-secondary";
const tabBaseClassName =
  "rounded-full px-4 py-2 text-sm font-medium transition-all w-1/2 text-center";

export default function LoginPage() {
  const router = useRouter();
  const { setAuth } = useAuthStore();

  const [mode, setMode] = useState<"login" | "register">("login");
  const [loginMethod, setLoginMethod] = useState<"phone" | "password">("phone");
  const [account, setAccount] = useState("");
  const [phone, setPhone] = useState("");
  const [countryCode, setCountryCode] = useState("86");
  const [name, setName] = useState("");
  const [smsCode, setSMSCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [partnerCode, setPartnerCode] = useState("");
  const [showPwd, setShowPwd] = useState(false);
  const [showConfirmPwd, setShowConfirmPwd] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [sendingCode, setSendingCode] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const [codeHint, setCodeHint] = useState("");

  const isRegister = mode === "register";

  useEffect(() => {
    if (countdown <= 0) {
      return undefined;
    }
    const timer = window.setTimeout(() => setCountdown((current) => current - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [countdown]);

  function resetTransientState() {
    setError("");
    setCodeHint("");
    setCountdown(0);
    setShowPwd(false);
    setShowConfirmPwd(false);
  }

  function handleModeChange(nextMode: "login" | "register") {
    if (mode === nextMode) {
      return;
    }
    setMode(nextMode);
    resetTransientState();
    if (nextMode === "login") {
      setLoginMethod("phone");
    }
  }

  function handleLoginMethodChange(nextMethod: "phone" | "password") {
    if (loginMethod === nextMethod) {
      return;
    }
    setLoginMethod(nextMethod);
    setError("");
    if (nextMethod === "password" && !account && phone) {
      setAccount(phone);
    }
    if (nextMethod === "phone" && !phone && account) {
      setPhone(account.replace(/\D/g, ""));
    }
  }

  async function handleSendCode() {
    setError("");
    setCodeHint("");

    if (!isValidCountryCode(countryCode)) {
      setError("请输入正确的国家区号");
      return;
    }
    if (!isValidPhone(phone)) {
      setError("请输入正确的手机号");
      return;
    }

    setSendingCode(true);
    try {
      const response = await sendRegisterSMSCode({ phone, countryCode });
      setCountdown(response.cooldownSeconds);
      setCodeHint(`验证码已发送，有效期 ${response.validMinutes} 分钟`);
      if (response.defaultCountryCode) {
        setCountryCode(response.defaultCountryCode);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "验证码发送失败，请稍后重试");
    } finally {
      setSendingCode(false);
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      let resp: LoginResponse;

      if (isRegister) {
        if (!isValidCountryCode(countryCode)) {
          throw new Error("请输入正确的国家区号");
        }
        if (!isValidPhone(phone)) {
          throw new Error("请输入正确的手机号");
        }
        if (!name.trim()) {
          throw new Error("请输入用户名");
        }
        if (!partnerCode.trim()) {
          throw new Error("请输入专属客服码");
        }
        if (!smsCode.trim()) {
          throw new Error("请输入短信验证码");
        }
        if (password !== confirmPassword) {
          throw new Error("两次输入的密码不一致");
        }

        await register({
          phone,
          countryCode,
          name: name.trim(),
          password,
          smsCode,
          partnerCode,
        });
        resp = await loginWithPhone({ phone, countryCode, password });
      } else if (loginMethod === "phone") {
        if (!isValidCountryCode(countryCode)) {
          throw new Error("请输入正确的国家区号");
        }
        if (!isValidPhone(phone)) {
          throw new Error("请输入正确的手机号");
        }
        resp = await loginWithPhone({ phone, countryCode, password });
      } else {
        if (!account.trim()) {
          throw new Error("请输入手机号");
        }
        resp = await loginWithPassword({ account: account.trim(), password });
      }

      const user = resp.user ?? await getCurrentUser();
      setAuth(user, resp.accessToken);
      router.push("/dashboard");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "操作失败，请检查网络连接";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  function renderPasswordField(options: {
    label: string;
    placeholder: string;
    value: string;
    onChange: (value: string) => void;
    visible: boolean;
    onToggle: () => void;
    autoComplete: string;
  }) {
    return (
      <div>
        <label className={labelClassName}>{options.label}</label>
        <div className="relative">
          <input
            type={options.visible ? "text" : "password"}
            value={options.value}
            onChange={(e) => options.onChange(e.target.value)}
            placeholder={options.placeholder}
            className={`${inputClassName} pr-11`}
            autoComplete={options.autoComplete}
            required
          />
          <button
            type="button"
            onClick={options.onToggle}
            className="absolute right-4 top-1/2 -translate-y-1/2 text-text-muted transition-colors hover:text-text-primary"
          >
            {options.visible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </button>
        </div>
      </div>
    );
  }

  function renderPhoneFields() {
    return (
      <div>
        <label className={labelClassName}>手机号</label>
        <div className="grid grid-cols-[92px_minmax(0,1fr)] gap-3">
          <input
            type="text"
            value={countryCode}
            onChange={(e) => setCountryCode(e.target.value.replace(/\D/g, ""))}
            placeholder="86"
            className={inputClassName}
            inputMode="numeric"
            autoComplete="tel-country-code"
            required
          />
          <input
            type="text"
            value={phone}
            onChange={(e) => setPhone(e.target.value.replace(/\D/g, ""))}
            placeholder="请输入手机号"
            className={inputClassName}
            inputMode="numeric"
            autoComplete="tel-national"
            required
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen bg-background text-text-primary">
      {/* Left Landing Showcase Panel (Desktop Only) */}
      <div className="hidden lg:flex relative w-1/2 flex-col items-start justify-center overflow-hidden border-r border-white/5 bg-black/40 px-16 xl:px-24">
        {/* Background Gradients */}
        <div className="pointer-events-none absolute left-0 top-1/2 h-[800px] w-[800px] -translate-y-1/2 -translate-x-1/4 rounded-full bg-accent/15 blur-[120px]" />
        <div className="pointer-events-none absolute bottom-0 right-0 h-[600px] w-[600px] rounded-full bg-cyan/10 blur-[120px]" />
        
        {/* Logo & Intro */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
          className="relative z-10 w-full"
        >
          <div className="inline-flex items-center gap-3 mb-8">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-gradient-to-br from-accent via-pink to-cyan shadow-[0_0_30px_rgba(177,73,255,0.4)]">
              <Zap className="h-6 w-6 text-white" />
            </div>
            <span className="text-2xl font-bold tracking-tight text-white">OmniDrive Matrix</span>
          </div>

          <h1 className="text-[2.75rem] font-extrabold tracking-tight text-white mb-6 leading-[1.15]">
            全自动自媒体制作发布 <br />
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-accent via-pink to-cyan">全平台 AI 生态引擎</span>
          </h1>
          <p className="text-lg text-text-secondary leading-relaxed max-w-lg mb-12">
            深度集成世界领先的 <strong>OpenClaw Agent 框架</strong>，实现从灵感到分发的一站式全自动流程。代理执行自媒体运营任务，24小时不间断为您创造内容生产力。
          </p>
        </motion.div>

        {/* Feature Cards Grid */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.2 }}
          className="relative z-10 grid grid-cols-2 gap-5 w-full max-w-2xl"
        >
          {/* Feature 1 */}
          <div className="rounded-2xl border border-white/10 bg-white/5 p-5 backdrop-blur-sm transition-all hover:bg-white/10 hover:-translate-y-1 hover:shadow-[0_10px_40px_-10px_rgba(177,73,255,0.3)]">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-accent/20 text-accent">
              <Bot className="h-5 w-5" />
            </div>
            <h3 className="text-base font-semibold text-white mb-1.5">OpenClaw Agent 引擎</h3>
            <p className="text-sm text-text-muted leading-relaxed">
              内嵌强大的底层智能代理系统，自主推理、自动规划与执行您指派的自媒体复杂运营任务。
            </p>
          </div>

          {/* Feature 2 */}
          <div className="rounded-2xl border border-white/10 bg-white/5 p-5 backdrop-blur-sm transition-all hover:bg-white/10 hover:-translate-y-1 hover:shadow-[0_10px_40px_-10px_rgba(255,107,158,0.3)]">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-pink/20 text-pink">
              <Video className="h-5 w-5" />
            </div>
            <h3 className="text-base font-semibold text-white mb-1.5">自动化图文/视频产出</h3>
            <p className="text-sm text-text-muted leading-relaxed">
              彻底告别繁琐渲染，AI 脚本自动生成、图片/视频极速合成，1分钟输出高质量爆款原生物料。
            </p>
          </div>

          {/* Feature 3 */}
          <div className="rounded-2xl border border-white/10 bg-white/5 p-5 backdrop-blur-sm transition-all hover:bg-white/10 hover:-translate-y-1 hover:shadow-[0_10px_40px_-10px_rgba(6,182,212,0.3)]">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-cyan/20 text-cyan">
              <Share2 className="h-5 w-5" />
            </div>
            <h3 className="text-base font-semibold text-white mb-1.5">多社交平台一键分发</h3>
            <p className="text-sm text-text-muted leading-relaxed">
              无缝对接小红书、抖音、B站、视频号等主流社媒，内容定时发送，多账号全场景自动铺开。
            </p>
          </div>

          {/* Feature 4 */}
          <div className="rounded-2xl border border-white/10 bg-white/5 p-5 backdrop-blur-sm transition-all hover:bg-white/10 hover:-translate-y-1 hover:shadow-[0_10px_40px_-10px_rgba(74,222,128,0.2)]">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-green-500/20 text-green-400">
              <Activity className="h-5 w-5" />
            </div>
            <h3 className="text-base font-semibold text-white mb-1.5">24 小时全自动运行</h3>
            <p className="text-sm text-text-muted leading-relaxed">
              无需人工盯盘跟进，全生命周期无人值守工作流。打造真正属于云端的自动化发布流水线。
            </p>
          </div>
        </motion.div>
      </div>

      {/* Right Form Panel */}
      <div className="relative flex w-full lg:w-1/2 items-center justify-center px-6 py-4 sm:px-12">
        <div className="pointer-events-none absolute left-1/2 top-0 h-72 w-72 -translate-x-1/2 rounded-full bg-accent/15 blur-[120px] lg:hidden" />
        <div className="pointer-events-none absolute bottom-0 right-0 h-72 w-72 rounded-full bg-cyan/12 blur-[120px] lg:hidden" />
        
        <div className="relative z-10 w-full max-w-[380px]">
          {/* Mobile Logo */}
          <div className="mb-4 flex flex-col items-center lg:hidden">
            <div className="mb-2 flex h-12 w-12 items-center justify-center rounded-[18px] bg-gradient-to-br from-accent via-pink to-cyan shadow-[0_0_24px_rgba(177,73,255,0.26)]">
              <Zap className="h-6 w-6 text-white" />
            </div>
            <h1 className="text-xl font-bold tracking-tight text-white">OmniDrive Matrix</h1>
          </div>

          <div className="mb-5 text-center lg:text-left">
            <h2 className="text-2xl font-bold tracking-tight text-white mb-1.5">
              {isRegister ? "创建账户" : "欢迎回来"}
            </h2>
            <p className="text-xs text-text-secondary">
              {isRegister
                ? "注册 OmniDrive 开始你的自动化矩阵"
                : "请登录你的账户以继续"}
            </p>
          </div>

          <div className="mb-5 flex rounded-full border border-white/10 bg-black/20 p-1">
            <button
              type="button"
              onClick={() => handleModeChange("login")}
              className={`${tabBaseClassName} ${
                !isRegister
                  ? "bg-white/10 text-white shadow-sm"
                  : "text-text-secondary hover:text-white"
              }`}
            >
              登录
            </button>
            <button
              type="button"
              onClick={() => handleModeChange("register")}
              className={`${tabBaseClassName} ${
                isRegister
                  ? "bg-white/10 text-white shadow-sm"
                  : "text-text-secondary hover:text-white"
              }`}
            >
              注册
            </button>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <motion.div
              key={`${mode}-${loginMethod}`}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25, ease: "easeOut" }}
              className="space-y-3"
            >
              {isRegister ? (
                <>
                  <div>
                    <label className={labelClassName}>用户名</label>
                    <input
                      type="text"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="请输入用户名或昵称"
                      className={inputClassName}
                      autoComplete="nickname"
                      required
                    />
                  </div>

                  <div>
                    <label className={labelClassName}>
                      专属客服码
                      <span className="ml-1 text-danger">*</span>
                    </label>
                    <input
                      type="text"
                      value={partnerCode}
                      onChange={(e) => setPartnerCode(e.target.value)}
                      placeholder="请输入专属客服码"
                      className={`${inputClassName} uppercase`}
                      autoComplete="off"
                      required
                    />
                  </div>

                  {renderPhoneFields()}

                  <div>
                    <label className={labelClassName}>短信验证码</label>
                    <div className="grid grid-cols-[minmax(0,1fr)_120px] gap-3">
                      <input
                        type="text"
                        value={smsCode}
                        onChange={(e) => setSMSCode(e.target.value.replace(/[^\dA-Za-z]/g, ""))}
                        placeholder="输入验证码"
                        className={inputClassName}
                        inputMode="numeric"
                        autoComplete="one-time-code"
                      />
                      <button
                        type="button"
                        onClick={handleSendCode}
                        disabled={sendingCode || countdown > 0}
                        className="rounded-xl border border-white/10 bg-white/5 text-sm font-medium text-text-secondary transition-all hover:bg-white/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {sendingCode ? "发送中..." : countdown > 0 ? `${countdown}s` : "获取验证码"}
                      </button>
                    </div>
                    {codeHint && <p className="mt-2 text-xs text-text-muted">{codeHint}</p>}
                  </div>

                  {renderPasswordField({
                    label: "密码",
                    placeholder: "至少 6 位，建议包含字母和数字",
                    value: password,
                    onChange: setPassword,
                    visible: showPwd,
                    onToggle: () => setShowPwd((current) => !current),
                    autoComplete: "new-password",
                  })}

                  {renderPasswordField({
                    label: "确认密码",
                    placeholder: "请再次输入密码",
                    value: confirmPassword,
                    onChange: setConfirmPassword,
                    visible: showConfirmPwd,
                    onToggle: () => setShowConfirmPwd((current) => !current),
                    autoComplete: "new-password",
                  })}
                </>
              ) : loginMethod === "phone" ? (
                <>
                  {renderPhoneFields()}
                  {renderPasswordField({
                    label: "密码",
                    placeholder: "请输入登录密码",
                    value: password,
                    onChange: setPassword,
                    visible: showPwd,
                    onToggle: () => setShowPwd((current) => !current),
                    autoComplete: "current-password",
                  })}
                </>
              ) : (
                <>
                  <div>
                    <label className={labelClassName}>手机号</label>
                    <input
                      type="text"
                      value={account}
                      onChange={(e) => setAccount(e.target.value)}
                      placeholder="请输入手机号"
                      className={inputClassName}
                      autoComplete="username"
                      inputMode="numeric"
                      required
                    />
                  </div>

                  {renderPasswordField({
                    label: "密码",
                    placeholder: "请输入登录密码",
                    value: password,
                    onChange: setPassword,
                    visible: showPwd,
                    onToggle: () => setShowPwd((current) => !current),
                    autoComplete: "current-password",
                  })}
                </>
              )}
            </motion.div>

            {!isRegister && (
              <div className="flex justify-end">
                <button
                  type="button"
                  onClick={() => handleLoginMethodChange(loginMethod === "phone" ? "password" : "phone")}
                  className="text-sm font-medium text-text-muted transition-colors hover:text-white"
                >
                  {loginMethod === "phone" ? "用账户密码登录" : "用手机号验证登录"}
                </button>
              </div>
            )}

            {error && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger"
              >
                {error}
              </motion.div>
            )}

            <div className="space-y-2.5 pt-1">
              <button
                type="submit"
                disabled={loading}
                className="group flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-accent/20 transition-all hover:opacity-90 hover:shadow-accent/30 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <span>{loading ? "处理中..." : isRegister ? "注册并登录" : "登录"}</span>
                {!loading && <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-1" />}
              </button>

              <button
                type="button"
                onClick={() => {
                  setAuth(mockUser, mockToken);
                  router.push("/dashboard");
                }}
                className="flex w-full items-center justify-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm font-semibold text-text-secondary transition-all hover:bg-white/10 hover:text-white"
              >
                <Sparkles className="h-4 w-4" />
                Demo 体验
              </button>
            </div>
          </form>

          <p className="mt-5 text-center text-sm text-text-muted">
            {isRegister ? "已经有账户了？" : "还没有账户？"}
            <button
              type="button"
              onClick={() => handleModeChange(isRegister ? "login" : "register")}
              className="ml-1 font-medium text-white transition-colors hover:text-accent"
            >
              {isRegister ? "返回登录" : "立即注册"}
            </button>
          </p>
        </div>
      </div>
    </div>
  );
}
