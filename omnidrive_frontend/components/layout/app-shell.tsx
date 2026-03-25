"use client";

import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Sidebar } from "./sidebar";
import { getBillingSummary } from "@/lib/services";
import { useAuthStore } from "@/lib/store";

export function AppShell({ children }: { children: React.ReactNode }) {
  const { token, hydrate } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();
  const { data: billingSummary } = useQuery({
    queryKey: ["billing-summary-global-reminder"],
    queryFn: getBillingSummary,
    enabled: Boolean(token),
    staleTime: 15 * 1000,
  });

  useEffect(() => {
    hydrate();
  }, [hydrate]);

  useEffect(() => {
    if (
      !token &&
      typeof window !== "undefined" &&
      !localStorage.getItem("omnidrive_token") &&
      pathname !== "/login" &&
      pathname !== "/register"
    ) {
      router.replace("/login");
    }
  }, [token, pathname, router]);

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="ml-[260px] flex-1 overflow-x-hidden">
        <div className="px-4 py-4 lg:px-5 lg:py-5">
          {billingSummary?.needsRecharge ? (
            <div className="mb-4 rounded-2xl border border-amber-400/30 bg-gradient-to-r from-amber-500/14 via-orange-500/10 to-transparent px-4 py-4 shadow-[0_0_30px_rgba(245,158,11,0.08)]">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                <div>
                  <p className="text-sm font-semibold text-amber-200">
                    积分不足提醒
                  </p>
                  <p className="mt-1 text-sm text-amber-50/90">
                    {billingSummary.rechargeAlertMessage || "当前可用积分不足，请先充值后继续使用扣费能力。"}
                  </p>
                </div>
                <div className="flex gap-3">
                  <Link
                    href="/finance"
                    className="inline-flex items-center justify-center rounded-xl border border-amber-300/25 px-4 py-2 text-sm font-medium text-amber-50 transition hover:bg-white/5"
                  >
                    查看明细
                  </Link>
                  <Link
                    href="/top-up"
                    className="inline-flex items-center justify-center rounded-xl bg-gradient-to-r from-amber-300 to-orange-400 px-4 py-2 text-sm font-semibold text-slate-950 transition hover:brightness-105"
                  >
                    立即充值
                  </Link>
                </div>
              </div>
            </div>
          ) : null}
          {children}
        </div>
      </main>
    </div>
  );
}
