"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { PageHeader } from "@/components/ui/common";
import { Send, Loader2, Bot, User, Trash2, Plus } from "lucide-react";

interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  createdAt: Date;
}

const API_BASE =
  process.env.NEXT_PUBLIC_OMNIDRIVE_ADMIN_API_BASE_URL?.replace(/\/+$/, "") ||
  "http://127.0.0.1:8410";
const CHAT_STREAM_URL = `${API_BASE}/api/admin/v1/ai/chat/stream`;

export function AdminChatView() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  const scrollToBottom = useCallback(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  const handleSend = async () => {
    const trimmed = input.trim();
    if (!trimmed || isStreaming) return;

    const userMsg: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content: trimmed,
      createdAt: new Date(),
    };

    const assistantMsg: ChatMessage = {
      id: crypto.randomUUID(),
      role: "assistant",
      content: "",
      createdAt: new Date(),
    };

    setMessages((prev) => [...prev, userMsg, assistantMsg]);
    setInput("");
    setIsStreaming(true);

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const token =
        typeof window !== "undefined"
          ? localStorage.getItem("admin_token")
          : null;

      const chatHistory = [...messages, userMsg].map((m) => ({
        role: m.role,
        content: m.content,
      }));

      const response = await fetch(CHAT_STREAM_URL, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          messages: chatHistory,
        }),
        signal: controller.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const reader = response.body?.getReader();
      const decoder = new TextDecoder();

      if (!reader) throw new Error("No response body");

      let accumulated = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const chunk = decoder.decode(value, { stream: true });
        const lines = chunk.split("\n");

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const data = line.slice(6);
            if (data === "[DONE]") continue;
            try {
              const parsed = JSON.parse(data);
              const delta =
                parsed.choices?.[0]?.delta?.content ||
                parsed.content ||
                parsed.text ||
                "";
              if (delta) {
                accumulated += delta;
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === assistantMsg.id
                      ? { ...m, content: accumulated }
                      : m
                  )
                );
              }
            } catch {
              // If it's not JSON, treat the raw data as text
              if (data && data !== "[DONE]") {
                accumulated += data;
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === assistantMsg.id
                      ? { ...m, content: accumulated }
                      : m
                  )
                );
              }
            }
          }
        }
      }
    } catch (err) {
      if ((err as Error).name === "AbortError") return;
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMsg.id
            ? { ...m, content: `⚠️ 请求失败: ${(err as Error).message}` }
            : m
        )
      );
    } finally {
      setIsStreaming(false);
      abortRef.current = null;
    }
  };

  const handleStop = () => {
    abortRef.current?.abort();
    setIsStreaming(false);
  };

  const handleClear = () => {
    if (isStreaming) handleStop();
    setMessages([]);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex flex-col h-[calc(100vh-6rem)]">
      <PageHeader
        title="AI 助手"
        subtitle="与 AI 模型实时对话，进行系统管理辅助。"
        actions={
          <button
            onClick={handleClear}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] border border-[var(--color-border)] rounded-lg hover:bg-[var(--color-bg-secondary)] transition-colors"
          >
            <Plus className="h-3.5 w-3.5" /> 新对话
          </button>
        }
      />

      {/* Messages Area */}
      <div
        ref={scrollRef}
        className="flex-1 overflow-y-auto rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4 space-y-4"
      >
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="h-16 w-16 rounded-2xl bg-[var(--color-primary)]/10 flex items-center justify-center mb-4">
              <Bot className="h-8 w-8 text-[var(--color-primary)]" />
            </div>
            <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">
              管理员 AI 助手
            </h3>
            <p className="mt-2 text-sm text-[var(--color-text-secondary)] max-w-md">
              可以帮助你查询系统数据、分析运营指标、生成报告等。输入消息开始对话。
            </p>
          </div>
        )}

        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`flex gap-3 ${
              msg.role === "user" ? "justify-end" : "justify-start"
            }`}
          >
            {msg.role === "assistant" && (
              <div className="flex-shrink-0 h-8 w-8 rounded-full bg-[var(--color-primary)]/10 flex items-center justify-center">
                <Bot className="h-4 w-4 text-[var(--color-primary)]" />
              </div>
            )}
            <div
              className={`max-w-[70%] rounded-xl px-4 py-3 text-sm leading-relaxed ${
                msg.role === "user"
                  ? "bg-[var(--color-primary)] text-white"
                  : "bg-[var(--color-bg-secondary)] border border-[var(--color-border)] text-[var(--color-text-primary)]"
              }`}
            >
              {msg.content || (
                <span className="inline-flex items-center gap-1.5 text-[var(--color-text-secondary)]">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  思考中...
                </span>
              )}
            </div>
            {msg.role === "user" && (
              <div className="flex-shrink-0 h-8 w-8 rounded-full bg-[var(--color-bg-secondary)] border border-[var(--color-border)] flex items-center justify-center">
                <User className="h-4 w-4 text-[var(--color-text-secondary)]" />
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Input Area */}
      <div className="mt-4 flex items-end gap-3">
        <div className="flex-1 relative">
          <textarea
            ref={inputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="输入消息，Shift+Enter 换行..."
            rows={1}
            className="w-full resize-none rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-4 py-3 pr-12 text-sm focus:outline-none focus:border-[var(--color-primary)] transition-all max-h-32 overflow-y-auto"
            style={{
              height: "auto",
              minHeight: "44px",
            }}
            onInput={(e) => {
              const target = e.target as HTMLTextAreaElement;
              target.style.height = "auto";
              target.style.height = `${Math.min(target.scrollHeight, 128)}px`;
            }}
          />
        </div>
        {isStreaming ? (
          <button
            onClick={handleStop}
            className="flex-shrink-0 h-11 w-11 rounded-xl bg-red-500/10 border border-red-500/30 flex items-center justify-center text-red-400 hover:bg-red-500/20 transition-colors"
          >
            <div className="h-3.5 w-3.5 rounded-sm bg-red-400" />
          </button>
        ) : (
          <button
            onClick={handleSend}
            disabled={!input.trim()}
            className="flex-shrink-0 h-11 w-11 rounded-xl bg-[var(--color-primary)] flex items-center justify-center text-white disabled:opacity-40 hover:opacity-90 transition-all"
          >
            <Send className="h-4 w-4" />
          </button>
        )}
      </div>
    </div>
  );
}
