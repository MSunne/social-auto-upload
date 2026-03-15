"use client";

import { PageHeader, EmptyState } from "@/components/ui/common";
import { MessageSquare } from "lucide-react";

export default function ChatPage() {
  return (
    <>
      <PageHeader
        title="聊天助手"
        subtitle="选择大语言模型进行聊天，获取灵感与创意"
      />
      <EmptyState
        icon={<MessageSquare className="h-6 w-6" />}
        title="聊天模块开发中"
        description="即将支持 Gemini 3.1 Pro Preview 等多模型对话。"
      />
    </>
  );
}
