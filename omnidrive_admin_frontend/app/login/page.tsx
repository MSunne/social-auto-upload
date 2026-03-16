export default function LoginPage() {
  return (
    <main className="flex min-h-screen items-center justify-center px-6 py-10">
      <div className="w-full max-w-md rounded-[28px] border border-white/10 bg-[rgba(8,12,18,0.82)] p-8 shadow-[0_20px_80px_rgba(0,0,0,0.4)] backdrop-blur">
        <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/70">
          OmniDriveAdmin
        </p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight text-white">
          后台登录
        </h1>
        <p className="mt-3 text-sm leading-7 text-slate-300">
          这里保留给 Claude 完成正式登录页。后端应提供独立的 admin auth 与 RBAC。
        </p>
        <div className="mt-6 rounded-2xl border border-dashed border-white/10 bg-white/4 px-4 py-5 text-sm text-slate-300">
          TODO: admin login form, MFA state, permission bootstrap, audit banner.
        </div>
      </div>
    </main>
  );
}

