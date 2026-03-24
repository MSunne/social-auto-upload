"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import {
  ArrowRight,
  BadgeCheck,
  Eye,
  EyeOff,
  KeyRound,
  MessageSquareText,
  Phone,
  ShieldCheck,
  Sparkles,
  UserRound,
  Zap,
} from "lucide-react";
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
  "w-full rounded-2xl border border-border/80 bg-black/20 px-4 py-3.5 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20 focus:shadow-[0_0_0_1px_rgba(177,73,255,0.1),0_0_30px_rgba(177,73,255,0.08)]";
const labelClassName = "mb-2 block text-xs font-semibold uppercase tracking-[0.18em] text-text-muted";
const sectionClassName =
  "rounded-[28px] border border-white/8 bg-white/[0.03] p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]";

const registerBenefits = [
  {
    icon: MessageSquareText,
    title: "短信注册",
    description: "手机号验证完成后立即创建云端账户，首次路径更顺手。",
  },
  {
    icon: BadgeCheck,
    title: "失败返还积分",
    description: "按次任务如果执行失败，会把已扣积分自动返还回钱包。",
  },
  {
    icon: ShieldCheck,
    title: "账本自动绑定",
    description: "填写合作伙伴客服码后，后续消费和分销链路会自动关联。",
  },
];

const loginBenefits = [
  "支持手机号登录和账户密码登录两种入口。",
  "账户密码登录当前使用手机号作为账号。",
  "注册完成后可直接进入创作和任务中心。",
];

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
  const isPhoneLogin = mode === "login" && loginMethod === "phone";
  const showPhoneIdentity = isRegister || isPhoneLogin;

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
      setPhone(account.replace(/[^\d]/g, ""));
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
      const response = await sendRegisterSMSCode({
        phone,
        countryCode,
      });
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

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-6 sm:px-6 lg:px-8">
      <div className="pointer-events-none absolute left-1/2 top-0 h-[540px] w-[540px] -translate-x-1/2 rounded-full bg-accent/15 blur-[160px]" />
      <div className="pointer-events-none absolute -left-20 top-10 h-[360px] w-[360px] rounded-full bg-pink/15 blur-[120px]" />
      <div className="pointer-events-none absolute -bottom-20 right-0 h-[360px] w-[360px] rounded-full bg-cyan/15 blur-[120px]" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.03)_1px,transparent_1px)] bg-[size:44px_44px] opacity-[0.06]" />

      <motion.div
        initial={{ opacity: 0, y: 18, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.45, ease: "easeOut" }}
        className="glass-card-elevated relative w-full max-w-6xl overflow-hidden"
      >
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(177,73,255,0.16),transparent_28%),radial-gradient(circle_at_bottom_right,rgba(0,245,212,0.12),transparent_30%)]" />

        <div className="relative grid lg:grid-cols-[1.02fr_minmax(430px,1fr)]">
          <motion.section
            initial={{ opacity: 0, x: -18 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.4, delay: 0.05 }}
            className="order-2 border-t border-border/60 p-6 sm:p-8 lg:order-1 lg:border-r lg:border-t-0 lg:p-10"
          >
            <div className="inline-flex items-center gap-3 rounded-full border border-white/10 bg-white/[0.03] px-4 py-2 text-xs font-semibold uppercase tracking-[0.22em] text-text-secondary">
              <span className="h-2 w-2 rounded-full bg-cyan shadow-[0_0_12px_rgba(0,245,212,0.7)]" />
              OmniDrive Cloud Console
            </div>

            <div className="mt-8 flex items-center gap-4">
              <div className="flex h-16 w-16 items-center justify-center rounded-[22px] bg-gradient-to-br from-accent via-pink to-cyan shadow-[0_0_40px_rgba(177,73,255,0.32)]">
                <Zap className="h-7 w-7 text-background" />
              </div>
              <div>
                <div className="text-3xl font-bold tracking-tight text-text-primary sm:text-4xl">
                  OmniDrive
                </div>
                <div className="mt-1 text-base text-text-secondary sm:text-lg">
                  {isRegister ? "Register & Connect" : "Access Your Workspace"}
                </div>
              </div>
            </div>

            <div className="mt-10 max-w-xl">
              <div className="inline-flex items-center gap-2 rounded-full border border-accent/20 bg-accent/10 px-3 py-1.5 text-xs font-semibold text-accent">
                <Sparkles className="h-3.5 w-3.5" />
                {isRegister ? "两步完成注册与账本绑定" : "手机号与账户密码双入口"}
              </div>
              <h1 className="mt-5 text-3xl font-bold leading-tight text-text-primary sm:text-[2.6rem] sm:leading-[1.05]">
                {isRegister
                  ? "把第一次注册，也做成一眼就会用的正式入口。"
                  : "回到你的 OmniDrive 工作台，继续管理任务与创作流程。"}
              </h1>
              <p className="mt-4 max-w-lg text-base leading-7 text-text-secondary">
                {isRegister
                  ? "手机号验证、合作绑定、密码设置放在同一条顺滑路径里，信息关系更清楚，注册后会直接进入系统。"
                  : "登录页已经支持手机号登录和账户密码登录。账户密码入口当前使用手机号作为账号，不再出现邮箱导向的错误提示。"}
              </p>
            </div>

            <div className="mt-8 rounded-[24px] border border-white/8 bg-white/[0.04] p-4 sm:hidden">
              <div className="text-sm font-semibold text-text-primary">
                {isRegister ? "注册后自动登录，并保留合作绑定关系。" : "当前默认手机号识别，不再误导到邮箱输入。"}
              </div>
              <p className="mt-2 text-sm leading-6 text-text-secondary">
                {isRegister
                  ? "手机号验证、密码设置和客服码绑定在同一条流程里完成。"
                  : "登录入口已经拆分清楚，手机号登录和账户密码登录可以直接切换。"}
              </p>
            </div>

            <div className="mt-8 hidden gap-3 sm:grid sm:grid-cols-3 lg:grid-cols-1">
              {registerBenefits.map((item) => {
                const Icon = item.icon;
                return (
                  <div
                    key={item.title}
                    className="rounded-[24px] border border-white/8 bg-white/[0.04] p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
                  >
                    <div className="flex items-center gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-accent/20 to-cyan/20 text-text-primary">
                        <Icon className="h-4.5 w-4.5" />
                      </div>
                      <div className="text-sm font-semibold text-text-primary">{item.title}</div>
                    </div>
                    <p className="mt-3 text-sm leading-6 text-text-secondary">{item.description}</p>
                  </div>
                );
              })}
            </div>

            <div className="mt-8 hidden rounded-[28px] border border-white/8 bg-[linear-gradient(135deg,rgba(177,73,255,0.14),rgba(0,245,212,0.08))] p-5 sm:block">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <div className="text-xs font-semibold uppercase tracking-[0.22em] text-text-muted">
                    登录说明
                  </div>
                  <div className="mt-2 text-lg font-semibold text-text-primary">
                    {isRegister ? "注册完成后自动登录" : "当前账号默认以手机号识别"}
                  </div>
                </div>
                <div className="rounded-full border border-white/10 bg-black/20 px-3 py-1.5 text-xs font-semibold text-cyan">
                  Secure Access
                </div>
              </div>
              <div className="mt-4 space-y-2">
                {loginBenefits.map((item) => (
                  <div key={item} className="flex items-start gap-3 text-sm text-text-secondary">
                    <span className="mt-1 h-1.5 w-1.5 rounded-full bg-accent shadow-[0_0_10px_rgba(177,73,255,0.7)]" />
                    <span>{item}</span>
                  </div>
                ))}
              </div>
            </div>
          </motion.section>

          <motion.section
            initial={{ opacity: 0, x: 18 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.4, delay: 0.12 }}
            className="order-1 p-5 sm:p-8 lg:order-2 lg:p-10"
          >
            <div className="mb-6 flex items-center gap-3 lg:hidden">
              <div className="flex h-12 w-12 items-center justify-center rounded-[18px] bg-gradient-to-br from-accent via-pink to-cyan shadow-[0_0_26px_rgba(177,73,255,0.28)]">
                <Zap className="h-5 w-5 text-background" />
              </div>
              <div>
                <div className="text-lg font-bold tracking-tight text-text-primary">OmniDrive</div>
                <div className="text-sm text-text-secondary">
                  {isRegister ? "Create & Connect" : "Secure Sign In"}
                </div>
              </div>
            </div>

            <div className="rounded-full border border-white/8 bg-black/20 p-1">
              <div className="grid grid-cols-2 gap-2">
                {(["login", "register"] as const).map((item) => {
                  const active = mode === item;
                  return (
                    <button
                      key={item}
                      type="button"
                      onClick={() => handleModeChange(item)}
                      className={`rounded-full px-4 py-3 text-sm font-semibold transition-all ${
                        active
                          ? "bg-gradient-to-r from-accent via-pink/80 to-cyan text-background shadow-[0_0_24px_rgba(177,73,255,0.28)]"
                          : "text-text-secondary hover:bg-white/[0.04] hover:text-text-primary"
                      }`}
                    >
                      {item === "login" ? "登录" : "注册"}
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="mt-8">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.24em] text-text-muted">
                    {isRegister ? "Create Account" : "Sign In"}
                  </p>
                  <h2 className="mt-3 text-3xl font-bold tracking-tight text-text-primary">
                    {isRegister ? "创建你的云端账户" : "欢迎回来"}
                  </h2>
                  <p className="mt-3 text-sm leading-6 text-text-secondary">
                    {isRegister
                      ? "手机号注册、合作绑定和密码设置一次完成。"
                      : "请选择合适的登录方式。账户密码登录当前请输入手机号。"}
                  </p>
                </div>
                <div className="hidden rounded-2xl border border-white/8 bg-white/[0.03] px-4 py-3 text-right sm:block">
                  <div className="text-xs uppercase tracking-[0.22em] text-text-muted">Status</div>
                  <div className="mt-2 text-sm font-semibold text-text-primary">
                    {isRegister ? "Ready To Register" : loginMethod === "phone" ? "Phone Login" : "Password Login"}
                  </div>
                </div>
              </div>

              {mode === "login" && (
                <div className="mt-6 rounded-[26px] border border-white/8 bg-white/[0.03] p-1.5">
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => handleLoginMethodChange("phone")}
                      className={`rounded-[20px] px-4 py-3 text-sm font-semibold transition-all ${
                        loginMethod === "phone"
                          ? "bg-gradient-to-r from-accent to-cyan text-background shadow-[0_0_20px_rgba(177,73,255,0.2)]"
                          : "text-text-secondary hover:bg-white/[0.04] hover:text-text-primary"
                      }`}
                    >
                      手机号登录
                    </button>
                    <button
                      type="button"
                      onClick={() => handleLoginMethodChange("password")}
                      className={`rounded-[20px] px-4 py-3 text-sm font-semibold transition-all ${
                        loginMethod === "password"
                          ? "bg-gradient-to-r from-accent to-cyan text-background shadow-[0_0_20px_rgba(177,73,255,0.2)]"
                          : "text-text-secondary hover:bg-white/[0.04] hover:text-text-primary"
                      }`}
                    >
                      账户密码登录
                    </button>
                  </div>
                </div>
              )}

              {mode === "register" && (
                <div className="mt-6 grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[24px] border border-accent/15 bg-accent/8 p-4">
                    <div className="text-xs font-semibold uppercase tracking-[0.2em] text-accent">Step 1</div>
                    <div className="mt-2 text-base font-semibold text-text-primary">账户资料</div>
                    <p className="mt-1 text-sm text-text-secondary">填写昵称和合作伙伴客服码。</p>
                  </div>
                  <div className="rounded-[24px] border border-cyan/15 bg-cyan/8 p-4">
                    <div className="text-xs font-semibold uppercase tracking-[0.2em] text-cyan">Step 2</div>
                    <div className="mt-2 text-base font-semibold text-text-primary">安全验证</div>
                    <p className="mt-1 text-sm text-text-secondary">完成手机号验证并设置登录密码。</p>
                  </div>
                </div>
              )}
            </div>

            <form onSubmit={handleSubmit} className="mt-8 space-y-5">
              <motion.div
                key={`${mode}-${loginMethod}`}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.28, ease: "easeOut" }}
                className="space-y-5"
              >
                {isRegister ? (
                  <div className="grid gap-5 xl:grid-cols-[0.92fr_1.08fr]">
                    <section className={sectionClassName}>
                      <div className="flex items-center gap-3">
                        <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-accent/12 text-accent">
                          <UserRound className="h-5 w-5" />
                        </div>
                        <div>
                          <div className="text-sm font-semibold text-text-primary">基础资料</div>
                          <div className="text-xs text-text-muted">先把账户身份和合作来源配置清楚。</div>
                        </div>
                      </div>

                      <div className="mt-5">
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

                      <div className="mt-4">
                        <label className={labelClassName}>
                          专属客服码
                          <span className="ml-2 text-[11px] tracking-normal text-text-muted">选填</span>
                        </label>
                        <input
                          type="text"
                          value={partnerCode}
                          onChange={(e) => setPartnerCode(e.target.value)}
                          placeholder="如有合作伙伴提供的客服码，请填写"
                          className={`${inputClassName} uppercase`}
                          autoComplete="off"
                        />
                        <div className="mt-3 rounded-2xl border border-white/6 bg-black/20 px-4 py-3 text-sm leading-6 text-text-secondary">
                          填写后，消费、分销和账本会自动绑定到对应合作关系。
                        </div>
                      </div>
                    </section>

                    <section className={sectionClassName}>
                      <div className="flex items-center gap-3">
                        <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-cyan/12 text-cyan">
                          <ShieldCheck className="h-5 w-5" />
                        </div>
                        <div>
                          <div className="text-sm font-semibold text-text-primary">安全验证</div>
                          <div className="text-xs text-text-muted">完成手机号验证，后续直接用密码登录。</div>
                        </div>
                      </div>

                      <div className="mt-5">
                        <label className={labelClassName}>手机号</label>
                        <div className="grid gap-3 sm:grid-cols-[96px_minmax(0,1fr)]">
                          <input
                            type="text"
                            value={countryCode}
                            onChange={(e) => setCountryCode(e.target.value.replace(/[^\d]/g, ""))}
                            placeholder="86"
                            className={inputClassName}
                            inputMode="numeric"
                            autoComplete="tel-country-code"
                            required
                          />
                          <input
                            type="text"
                            value={phone}
                            onChange={(e) => setPhone(e.target.value.replace(/[^\d]/g, ""))}
                            placeholder="请输入手机号"
                            className={inputClassName}
                            inputMode="numeric"
                            autoComplete="tel-national"
                            required
                          />
                        </div>
                      </div>

                      <div className="mt-4">
                        <label className={labelClassName}>短信验证码</label>
                        <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_168px]">
                          <input
                            type="text"
                            value={smsCode}
                            onChange={(e) => setSMSCode(e.target.value.replace(/[^\dA-Za-z]/g, ""))}
                            placeholder="输入短信验证码"
                            className={inputClassName}
                            inputMode="numeric"
                            autoComplete="one-time-code"
                          />
                          <button
                            type="button"
                            onClick={handleSendCode}
                            disabled={sendingCode || countdown > 0}
                            className="rounded-2xl border border-accent/25 bg-accent/10 px-4 py-3 text-sm font-semibold text-accent transition-all hover:border-accent/45 hover:bg-accent/14 disabled:cursor-not-allowed disabled:opacity-55"
                          >
                            {sendingCode ? "发送中..." : countdown > 0 ? `${countdown}s 后重试` : "获取验证码"}
                          </button>
                        </div>
                        {codeHint && <p className="mt-2 text-sm text-text-muted">{codeHint}</p>}
                      </div>

                      <div className="mt-4">
                        <label className={labelClassName}>密码</label>
                        <div className="relative">
                          <input
                            type={showPwd ? "text" : "password"}
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            placeholder="至少 6 位，建议包含字母和数字"
                            className={`${inputClassName} pr-11`}
                            autoComplete="new-password"
                            required
                          />
                          <button
                            type="button"
                            onClick={() => setShowPwd((current) => !current)}
                            className="absolute right-4 top-1/2 -translate-y-1/2 text-text-muted transition-colors hover:text-text-primary"
                          >
                            {showPwd ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                          </button>
                        </div>
                      </div>

                      <div className="mt-4">
                        <label className={labelClassName}>确认密码</label>
                        <div className="relative">
                          <input
                            type={showConfirmPwd ? "text" : "password"}
                            value={confirmPassword}
                            onChange={(e) => setConfirmPassword(e.target.value)}
                            placeholder="请再次输入密码"
                            className={`${inputClassName} pr-11`}
                            autoComplete="new-password"
                            required
                          />
                          <button
                            type="button"
                            onClick={() => setShowConfirmPwd((current) => !current)}
                            className="absolute right-4 top-1/2 -translate-y-1/2 text-text-muted transition-colors hover:text-text-primary"
                          >
                            {showConfirmPwd ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                          </button>
                        </div>
                      </div>
                    </section>
                  </div>
                ) : (
                  <section className={sectionClassName}>
                    <div className="flex items-center gap-3">
                      <div
                        className={`flex h-11 w-11 items-center justify-center rounded-2xl ${
                          showPhoneIdentity ? "bg-cyan/12 text-cyan" : "bg-accent/12 text-accent"
                        }`}
                      >
                        {showPhoneIdentity ? <Phone className="h-5 w-5" /> : <KeyRound className="h-5 w-5" />}
                      </div>
                      <div>
                        <div className="text-sm font-semibold text-text-primary">
                          {showPhoneIdentity ? "手机号登录" : "账户密码登录"}
                        </div>
                        <div className="text-xs text-text-muted">
                          {showPhoneIdentity
                            ? "输入区号、手机号和密码后即可进入控制台。"
                            : "账户密码登录当前使用手机号作为账号。"}
                        </div>
                      </div>
                    </div>

                    {showPhoneIdentity ? (
                      <div className="mt-5 space-y-4">
                        <div>
                          <label className={labelClassName}>手机号</label>
                          <div className="grid gap-3 sm:grid-cols-[96px_minmax(0,1fr)]">
                            <input
                              type="text"
                              value={countryCode}
                              onChange={(e) => setCountryCode(e.target.value.replace(/[^\d]/g, ""))}
                              placeholder="86"
                              className={inputClassName}
                              inputMode="numeric"
                              autoComplete="tel-country-code"
                              required
                            />
                            <input
                              type="text"
                              value={phone}
                              onChange={(e) => setPhone(e.target.value.replace(/[^\d]/g, ""))}
                              placeholder="请输入手机号"
                              className={inputClassName}
                              inputMode="numeric"
                              autoComplete="tel-national"
                              required
                            />
                          </div>
                        </div>

                        <div>
                          <label className={labelClassName}>密码</label>
                          <div className="relative">
                            <input
                              type={showPwd ? "text" : "password"}
                              value={password}
                              onChange={(e) => setPassword(e.target.value)}
                              placeholder="请输入登录密码"
                              className={`${inputClassName} pr-11`}
                              autoComplete="current-password"
                              required
                            />
                            <button
                              type="button"
                              onClick={() => setShowPwd((current) => !current)}
                              className="absolute right-4 top-1/2 -translate-y-1/2 text-text-muted transition-colors hover:text-text-primary"
                            >
                              {showPwd ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            </button>
                          </div>
                        </div>
                      </div>
                    ) : (
                      <div className="mt-5 space-y-4">
                        <div className="rounded-2xl border border-accent/14 bg-accent/7 px-4 py-3 text-sm leading-6 text-text-secondary">
                          这里不再提示邮箱，当前请输入手机号与密码登录。
                        </div>

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

                        <div>
                          <label className={labelClassName}>密码</label>
                          <div className="relative">
                            <input
                              type={showPwd ? "text" : "password"}
                              value={password}
                              onChange={(e) => setPassword(e.target.value)}
                              placeholder="请输入登录密码"
                              className={`${inputClassName} pr-11`}
                              autoComplete="current-password"
                              required
                            />
                            <button
                              type="button"
                              onClick={() => setShowPwd((current) => !current)}
                              className="absolute right-4 top-1/2 -translate-y-1/2 text-text-muted transition-colors hover:text-text-primary"
                            >
                              {showPwd ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            </button>
                          </div>
                        </div>
                      </div>
                    )}
                  </section>
                )}
              </motion.div>

              {error && (
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  className="rounded-2xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger"
                >
                  {error}
                </motion.div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="group relative flex w-full items-center justify-center gap-2 overflow-hidden rounded-[24px] bg-gradient-to-r from-accent via-pink/70 to-cyan px-6 py-4 text-sm font-semibold text-background shadow-[0_18px_45px_rgba(177,73,255,0.28)] transition-all hover:-translate-y-0.5 hover:shadow-[0_22px_55px_rgba(177,73,255,0.34)] disabled:cursor-not-allowed disabled:opacity-55"
              >
                <span>{loading ? "处理中..." : isRegister ? "注册并进入控制台" : "登录"}</span>
                {!loading && <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />}
              </button>

              <button
                type="button"
                onClick={() => {
                  setAuth(mockUser, mockToken);
                  router.push("/dashboard");
                }}
                className="flex w-full items-center justify-center gap-2 rounded-[24px] border border-accent/20 bg-accent/8 px-6 py-4 text-sm font-semibold text-accent transition-all hover:border-accent/35 hover:bg-accent/12"
              >
                <Sparkles className="h-4 w-4" />
                Demo 体验登录（无需后端）
              </button>
            </form>

            <div className="mt-6 flex flex-wrap items-center justify-between gap-3 rounded-[24px] border border-white/8 bg-white/[0.03] px-4 py-4 text-sm text-text-secondary">
              <div>
                {isRegister ? "已经有账户了？" : "还没有账户？"}
                <button
                  type="button"
                  onClick={() => handleModeChange(isRegister ? "login" : "register")}
                  className="ml-2 font-semibold text-accent transition-colors hover:text-accent-strong"
                >
                  {isRegister ? "返回登录" : "立即注册"}
                </button>
              </div>
              <div className="text-xs uppercase tracking-[0.2em] text-text-muted">
                {isRegister ? "Auto Login After Register" : "Phone First"}
              </div>
            </div>
          </motion.section>
        </div>
      </motion.div>
    </div>
  );
}
