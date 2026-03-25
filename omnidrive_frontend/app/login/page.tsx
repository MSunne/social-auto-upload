"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import { ArrowRight, Eye, EyeOff, Sparkles, Zap } from "lucide-react";
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
      {/* Left Branding Panel (Desktop Only) */}
      <div className="hidden lg:flex relative w-1/2 flex-col items-center justify-center overflow-hidden border-r border-white/5 bg-black/40">
        <div className="pointer-events-none absolute left-1/2 top-1/2 h-[600px] w-[600px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-accent/20 blur-[120px]" />
        <div className="pointer-events-none absolute bottom-0 right-0 h-96 w-96 rounded-full bg-cyan/15 blur-[120px]" />
        
        <div className="relative z-10 flex flex-col items-center text-center">
          <div className="mb-8 flex h-24 w-24 items-center justify-center rounded-[32px] bg-gradient-to-br from-accent via-pink to-cyan shadow-[0_0_50px_rgba(177,73,255,0.3)]">
            <Zap className="h-12 w-12 text-white" />
          </div>
          <h1 className="mb-4 text-4xl font-bold tracking-tight text-white">OmniDrive</h1>
          <p className="max-w-md text-lg text-text-secondary leading-relaxed">
            下一代智能云存储平台，为您的数据提供安全、高效的管理体验。
          </p>
        </div>
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
            <h1 className="text-xl font-bold tracking-tight text-white">OmniDrive</h1>
          </div>

          <div className="mb-5 text-center lg:text-left">
            <h2 className="text-2xl font-bold tracking-tight text-white mb-1.5">
              {isRegister ? "创建账户" : "欢迎回来"}
            </h2>
            <p className="text-xs text-text-secondary">
              {isRegister
                ? "注册 OmniDrive 开始你的云端之旅"
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
