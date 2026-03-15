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
        <div className="px-4 py-4 lg:px-5 lg:py-5">
          {children}
        </div>
      </main>
    </div>
  );
}
