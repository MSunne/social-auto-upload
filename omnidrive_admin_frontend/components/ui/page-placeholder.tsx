import Link from "next/link";

interface PagePlaceholderProps {
  eyebrow: string;
  title: string;
  description: string;
  apiGroup: string;
  checklist: string[];
}

export function PagePlaceholder({
  eyebrow,
  title,
  description,
  apiGroup,
  checklist,
}: PagePlaceholderProps) {
  return (
    <div className="space-y-6">
      <div className="rounded-[28px] border border-white/10 bg-white/5 p-8 shadow-[0_20px_80px_rgba(0,0,0,0.35)]">
        <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/70">
          {eyebrow}
        </p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight text-white">
          {title}
        </h1>
        <p className="mt-4 max-w-3xl text-sm leading-7 text-slate-300">
          {description}
        </p>
        <div className="mt-6 flex flex-wrap gap-3">
          <span className="rounded-full border border-cyan-300/20 bg-cyan-300/8 px-3 py-1 text-xs text-cyan-100">
            API: {apiGroup}
          </span>
          <Link
            href="https://github.com"
            className="rounded-full border border-white/10 px-3 py-1 text-xs text-slate-200 transition-colors hover:border-white/20 hover:bg-white/5"
          >
            Claude will refine this screen
          </Link>
        </div>
      </div>

      <div className="rounded-[24px] border border-white/8 bg-[rgba(8,12,18,0.72)] p-6">
        <h2 className="text-sm font-semibold uppercase tracking-[0.2em] text-slate-400">
          Screen TODO
        </h2>
        <div className="mt-4 grid gap-3 md:grid-cols-2">
          {checklist.map((item) => (
            <div
              key={item}
              className="rounded-2xl border border-white/8 bg-white/4 px-4 py-4 text-sm text-slate-200"
            >
              {item}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

