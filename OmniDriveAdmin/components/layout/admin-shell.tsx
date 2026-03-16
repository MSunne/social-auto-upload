"use client";

import { Sidebar } from "./sidebar";

export function AdminShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="admin-shell grid min-h-screen grid-cols-1 lg:grid-cols-[280px_minmax(0,1fr)]">
      <Sidebar />
      <main className="min-w-0 px-4 py-4 lg:px-6 lg:py-6">
        {children}
      </main>
    </div>
  );
}
