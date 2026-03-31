"use client";

import { useEffect, useMemo, useSyncExternalStore, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { X } from "lucide-react";
import { Sidebar } from "./sidebar";
import { getBillingSummary } from "@/lib/services";
import { useAuthStore } from "@/lib/store";

const BILLING_ALERT_DISMISSED_STORAGE_KEY = "omnidrive_billing_alert_dismissed";
const BILLING_ALERT_STORAGE_EVENT = "omnidrive-billing-alert-storage-change";

function readDismissedBillingAlertKey() {
  if (typeof window === "undefined") {
    return null;
  }
  return localStorage.getItem(BILLING_ALERT_DISMISSED_STORAGE_KEY);
}

function notifyBillingAlertStorageChange() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new Event(BILLING_ALERT_STORAGE_EVENT));
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const { token, hydrate } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();
  const dismissedAlertKey = useSyncExternalStore(
    (onStoreChange) => {
      if (typeof window === "undefined") {
        return () => undefined;
      }
      const handleStorageChange = () => onStoreChange();
      window.addEventListener("storage", handleStorageChange);
      window.addEventListener(BILLING_ALERT_STORAGE_EVENT, handleStorageChange);
      return () => {
        window.removeEventListener("storage", handleStorageChange);
        window.removeEventListener(BILLING_ALERT_STORAGE_EVENT, handleStorageChange);
      };
    },
    readDismissedBillingAlertKey,
    () => null,
  );
  const { data: billingSummary } = useQuery({
    queryKey: ["billing-summary-global-reminder"],
    queryFn: getBillingSummary,
    enabled: Boolean(token),
    staleTime: 15 * 1000,
  });

  const billingAlertKey = useMemo(() => {
    if (!billingSummary?.needsRecharge) {
      return "";
    }
    return [
      billingSummary.rechargeAlertReason || "",
      billingSummary.rechargeAlertMessage || "",
      billingSummary.lastBillingFailedAt || "",
    ].join("::");
  }, [
    billingSummary?.lastBillingFailedAt,
    billingSummary?.needsRecharge,
    billingSummary?.rechargeAlertMessage,
    billingSummary?.rechargeAlertReason,
  ]);

  const shouldShowBillingAlert = Boolean(
    billingSummary?.needsRecharge &&
      billingAlertKey &&
      dismissedAlertKey !== billingAlertKey,
  );

  const [mounted, setMounted] = useState(false);
  const [authChecked, setAuthChecked] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (typeof window === "undefined" || billingSummary?.needsRecharge) {
      return;
    }
    localStorage.removeItem(BILLING_ALERT_DISMISSED_STORAGE_KEY);
    notifyBillingAlertStorageChange();
  }, [billingSummary?.needsRecharge]);

  useEffect(() => {
    if (!mounted) return;
    hydrate();
    
    const localToken = localStorage.getItem("omnidrive_token");
    const hasAuth = Boolean(token || localToken);

    if (!hasAuth && pathname !== "/login" && pathname !== "/register") {
      router.replace("/login");
    } else {
      setAuthChecked(true);
    }
  }, [mounted, hydrate, token, pathname, router]);

  const dismissBillingAlert = () => {
    if (!billingAlertKey || typeof window === "undefined") {
      return;
    }
    localStorage.setItem(BILLING_ALERT_DISMISSED_STORAGE_KEY, billingAlertKey);
    notifyBillingAlertStorageChange();
  };

  if (!authChecked) {
    return <div className="min-h-screen bg-background" />;
  }

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="ml-[260px] flex-1 overflow-x-hidden">
        <div className="px-4 py-4 lg:px-5 lg:py-5">
          {shouldShowBillingAlert ? (
            <div className="mb-4 rounded-2xl border border-amber-400/30 bg-gradient-to-r from-amber-500/14 via-orange-500/10 to-transparent px-4 py-4 shadow-[0_0_30px_rgba(245,158,11,0.08)]">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div className="flex-1">
                  <p className="text-sm font-semibold text-amber-200">
                    积分不足提醒
                  </p>
                  <p className="mt-1 text-sm text-amber-50/90">
                    {billingSummary?.rechargeAlertMessage || "当前可用积分不足，请先充值后继续使用扣费能力。"}
                  </p>
                </div>
                <div className="flex items-center gap-3 self-end lg:self-auto">
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
                  <button
                    type="button"
                    onClick={dismissBillingAlert}
                    aria-label="关闭积分不足提醒"
                    className="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-amber-300/20 text-amber-100 transition hover:bg-white/5 hover:text-white"
                  >
                    <X className="h-4 w-4" />
                  </button>
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
