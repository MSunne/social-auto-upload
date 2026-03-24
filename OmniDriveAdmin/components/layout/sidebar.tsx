"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { NAV_GROUPS } from "@/lib/routes";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/hooks/useAuth";
import { LogOut } from "lucide-react";

const SIDEBAR_SCROLL_STORAGE_KEY = "omnidrive-admin-sidebar-scroll-top";

export function Sidebar() {
  const pathname = usePathname();
  const { logout } = useAuth();
  const navRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const navElement = navRef.current;
    if (!navElement || typeof window === "undefined") {
      return;
    }

    const savedScrollTop = window.sessionStorage.getItem(SIDEBAR_SCROLL_STORAGE_KEY);
    if (savedScrollTop) {
      navElement.scrollTop = Number(savedScrollTop) || 0;
    }

    const handleScroll = () => {
      window.sessionStorage.setItem(SIDEBAR_SCROLL_STORAGE_KEY, String(navElement.scrollTop));
    };

    navElement.addEventListener("scroll", handleScroll, { passive: true });
    return () => navElement.removeEventListener("scroll", handleScroll);
  }, []);

  useEffect(() => {
    const navElement = navRef.current;
    if (!navElement || typeof window === "undefined") {
      return;
    }

    const savedScrollTop = window.sessionStorage.getItem(SIDEBAR_SCROLL_STORAGE_KEY);
    if (savedScrollTop) {
      navElement.scrollTop = Number(savedScrollTop) || 0;
    }
  }, [pathname]);

  return (
    <aside className="flex flex-col border-b border-[var(--color-border)] bg-[var(--color-sidebar)] px-4 py-5 text-white lg:sticky lg:top-0 lg:h-screen lg:border-b-0 lg:border-r">
      <div className="mb-6 flex items-center justify-between lg:mb-10">
        <div>
          <p className="text-[11px] uppercase tracking-[0.32em] text-white/45">
            OmniDrive
          </p>
          <h1 className="mt-2 text-xl font-semibold">Admin</h1>
        </div>
        <div className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-[11px] uppercase tracking-[0.2em] text-white/55">
          internal
        </div>
      </div>

      <nav ref={navRef} className="min-h-0 flex-1 space-y-6 overflow-y-auto pr-1">
        {NAV_GROUPS.map((group) => (
          <div key={group.label}>
            <p className="mb-2 px-3 text-[11px] uppercase tracking-[0.24em] text-white/35">
              {group.label}
            </p>
            <div className="space-y-1">
              {group.items.map((item) => {
                const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
                const Icon = item.icon;

                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={cn(
                      "flex items-center gap-3 rounded-2xl px-3 py-2.5 text-sm transition-colors",
                      active
                        ? "bg-white/10 text-white"
                        : "text-white/62 hover:bg-white/6 hover:text-white"
                    )}
                  >
                    <Icon className="h-4 w-4 shrink-0" />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="mt-auto pt-6 border-t border-white/10">
        <button
          onClick={logout}
          className="flex w-full items-center gap-3 rounded-2xl px-3 py-2.5 text-sm transition-colors text-red-300 w-full hover:bg-white/6 hover:text-red-200"
        >
          <LogOut className="h-4 w-4 shrink-0" />
          <span>注销登录</span>
        </button>
      </div>
    </aside>
  );
}
