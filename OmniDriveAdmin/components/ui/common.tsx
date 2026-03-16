import { type ReactNode } from "react";

export function PageHeader({
  title,
  subtitle,
  actions
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <header className="mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
      <div>
        <p className="data-kicker">operations workspace</p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight">{title}</h1>
        {subtitle ? (
          <p className="mt-3 max-w-3xl text-sm leading-7 text-[var(--color-text-secondary)]">
            {subtitle}
          </p>
        ) : null}
      </div>
      {actions ? <div className="flex items-center gap-3">{actions}</div> : null}
    </header>
  );
}

export function MetricTile({
  label,
  value,
  hint,
  icon
}: {
  label: string;
  value: string;
  hint?: string;
  icon?: ReactNode;
}) {
  return (
    <article className="panel p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="data-kicker">{label}</p>
          <p className="mt-4 text-4xl font-semibold tracking-tight">{value}</p>
          {hint ? (
            <p className="mt-3 text-sm text-[var(--color-text-secondary)]">{hint}</p>
          ) : null}
        </div>
        {icon ? (
          <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] p-3 text-[var(--color-accent)]">
            {icon}
          </div>
        ) : null}
      </div>
    </article>
  );
}

export function SectionCard({
  title,
  subtitle,
  children
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
}) {
  return (
    <section className="panel p-6">
      <div className="mb-5">
        <h2 className="text-lg font-semibold">{title}</h2>
        {subtitle ? (
          <p className="mt-2 text-sm leading-7 text-[var(--color-text-secondary)]">
            {subtitle}
          </p>
        ) : null}
      </div>
      {children}
    </section>
  );
}
