"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Sidebar } from "./sidebar";
import { useAuthStore } from "@/lib/store";

export function AppShell({ children }: { children: React.ReactNode }) {
  const { token, hydrate } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();

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
        <div className="mx-auto max-w-[1440px] px-6 py-6 lg:px-8 lg:py-8">
          {children}
        </div>
      </main>
    </div>
  );
}
