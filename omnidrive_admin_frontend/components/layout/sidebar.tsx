"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { adminNavigation } from "@/lib/navigation";
import { cn } from "@/lib/utils";

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="hidden w-72 shrink-0 border-r border-white/10 bg-[rgba(8,12,18,0.88)] px-5 py-6 backdrop-blur xl:flex xl:flex-col">
      <div className="mb-8">
        <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/70">
          OmniDrive
        </p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-white">
          Admin Console
        </h1>
        <p className="mt-2 text-sm leading-6 text-slate-400">
          内部运营、财务、审核与风控管理工作台。
        </p>
      </div>

      <div className="space-y-8 overflow-y-auto pr-1">
        {adminNavigation.map((section) => (
          <section key={section.title}>
            <p className="mb-3 text-xs font-medium uppercase tracking-[0.18em] text-slate-500">
              {section.title}
            </p>
            <div className="space-y-1.5">
              {section.items.map((item) => {
                const active =
                  pathname === item.href ||
                  (item.href !== "/dashboard" && pathname.startsWith(item.href));
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={cn(
                      "flex items-center gap-3 rounded-2xl px-3 py-3 text-sm transition-all",
                      active
                        ? "bg-cyan-400/12 text-white shadow-[inset_0_0_0_1px_rgba(103,232,249,0.22)]"
                        : "text-slate-400 hover:bg-white/5 hover:text-slate-100",
                    )}
                  >
                    <Icon className="h-4 w-4 shrink-0" />
                    <span>{item.title}</span>
                  </Link>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </aside>
  );
}

