export default function LoginPage() {
  return (
    <main className="min-h-screen bg-[var(--color-background)] px-6 py-10 text-[var(--color-text-primary)]">
      <div className="mx-auto flex min-h-[80vh] max-w-5xl items-center">
        <section className="grid w-full gap-8 lg:grid-cols-[1.2fr_0.8fr]">
          <div className="rounded-[28px] border border-[var(--color-border-strong)] bg-[var(--color-panel)] p-10 shadow-[0_30px_100px_rgba(15,23,42,0.28)]">
            <p className="text-xs uppercase tracking-[0.3em] text-[var(--color-text-muted)]">
              OmniDriveAdmin
            </p>
            <h1 className="mt-5 max-w-xl text-4xl font-semibold leading-tight">
              内部运营、财务、分销与审核后台
            </h1>
            <p className="mt-4 max-w-2xl text-base leading-7 text-[var(--color-text-secondary)]">
              这个目录已经建立完成，Claude 可以在此基础上继续实现正式登录页、审核运营台、
              财务工作台和分销结算界面。
            </p>
          </div>
          <div className="rounded-[28px] border border-[var(--color-border)] bg-[var(--color-surface)] p-8">
            <div className="rounded-[22px] border border-dashed border-[var(--color-border-strong)] bg-[var(--color-panel-muted)] p-6">
              <h2 className="text-lg font-semibold">待实现</h2>
              <p className="mt-3 text-sm leading-7 text-[var(--color-text-secondary)]">
                这里建议由 Claude 实现真正的管理员登录、双因素认证、角色切换和会话过期处理。
              </p>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
