"use client";

import { useState, useRef, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  MessageSquare,
  Send,
  Paperclip,
  Bot,
  User,
  Sparkles,
  Command,
  MoreVertical,
} from "lucide-react";

type Message = {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: string;
};

const INITIAL_MESSAGES: Message[] = [
  {
    id: "m1",
    role: "assistant",
    content:
      "你好！我是 OmniDrive 的智能助手。我可以帮你查阅知识库、生成文案、分析任务状态，或是解答任何关于内容分发和 AI 工具的问题。今天需要点什么？",
    timestamp: new Date().toISOString(),
  },
];

export default function ChatPage() {
  const [messages, setMessages] = useState<Message[]>(INITIAL_MESSAGES);
  const [input, setInput] = useState("");
  const [isTyping, setIsTyping] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isTyping]);

  function handleSend() {
    if (!input.trim() || isTyping) return;

    const newMsg: Message = {
      id: Date.now().toString(),
      role: "user",
      content: input.trim(),
      timestamp: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, newMsg]);
    setInput("");
    setIsTyping(true);

    // Mock API response
    setTimeout(() => {
      setMessages((prev) => [
        ...prev,
        {
          id: (Date.now() + 1).toString(),
          role: "assistant",
          content: "这是模拟的 AI 回复。在实际对接后，我会连接到后端配置的 LLM（如 GPT-4o 或 Claude 3.5），并结合您的 OpenClaw 数据提供上下文感知的回答。",
          timestamp: new Date().toISOString(),
        },
      ]);
      setIsTyping(false);
    }, 1500);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    // Enter to send, Shift+Enter for newline
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  return (
    <div className="flex h-[calc(100vh-2rem)] flex-col rounded-2xl border border-border bg-surface overflow-hidden">
      {/* ── Header ── */}
      <div className="flex items-center justify-between border-b border-border bg-surface/50 px-6 py-4 backdrop-blur-md">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-accent to-cyan">
            <Bot className="h-5 w-5 text-background" />
          </div>
          <div>
            <h1 className="text-base font-bold text-text-primary">
              OmniDrive Assistant
            </h1>
            <p className="text-xs text-text-muted flex items-center gap-1">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500"></span>
              </span>
              GPT-4o (知识库已连接)
            </p>
          </div>
        </div>
        <button className="flex h-9 w-9 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-hover hover:text-text-primary">
          <MoreVertical className="h-5 w-5" />
        </button>
      </div>

      {/* ── Messages Area ── */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        <AnimatePresence initial={false}>
          {messages.map((msg) => (
            <motion.div
              key={msg.id}
              initial={{ opacity: 0, y: 10, scale: 0.95 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              className={`flex gap-4 ${
                msg.role === "user" ? "flex-row-reverse" : "flex-row"
              }`}
            >
              {/* Avatar */}
              <div
                className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${
                  msg.role === "assistant"
                    ? "bg-gradient-to-br from-accent to-cyan text-background"
                    : "bg-surface-hover border border-border text-text-secondary"
                }`}
              >
                {msg.role === "assistant" ? (
                  <Bot className="h-5 w-5" />
                ) : (
                  <User className="h-4 w-4" />
                )}
              </div>

              {/* Bubble */}
              <div
                className={`group max-w-[75%] rounded-2xl px-5 py-3.5 text-sm leading-relaxed shadow-sm ${
                  msg.role === "user"
                    ? "bg-gradient-to-br from-accent to-indigo-600 text-white rounded-tr-none shadow-accent/10"
                    : "border border-border bg-surface-hover/50 text-text-primary rounded-tl-none"
                }`}
              >
                <div className="whitespace-pre-wrap">{msg.content}</div>
                <span
                  className={`mt-2 block text-[10px] font-medium opacity-0 transition-opacity group-hover:opacity-100 ${
                    msg.role === "user" ? "text-white/60" : "text-text-muted"
                  }`}
                >
                  {new Date(msg.timestamp).toLocaleTimeString("zh-CN", {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </span>
              </div>
            </motion.div>
          ))}
        </AnimatePresence>

        {isTyping && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className="flex gap-4"
          >
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-accent to-cyan text-background">
              <Sparkles className="h-4 w-4" />
            </div>
            <div className="max-w-[75%] rounded-2xl rounded-tl-none border border-border bg-surface-hover/50 px-5 py-4">
              <div className="flex items-center gap-1.5">
                <div className="h-1.5 w-1.5 animate-bounce rounded-full bg-accent" style={{ animationDelay: "0ms" }} />
                <div className="h-1.5 w-1.5 animate-bounce rounded-full bg-accent" style={{ animationDelay: "150ms" }} />
                <div className="h-1.5 w-1.5 animate-bounce rounded-full bg-accent" style={{ animationDelay: "300ms" }} />
              </div>
            </div>
          </motion.div>
        )}
        <div ref={messagesEndRef} className="h-1" />
      </div>

      {/* ── Input Area ── */}
      <div className="border-t border-border bg-surface/50 p-4 backdrop-blur-md">
        <div className="mx-auto max-w-4xl relative rounded-2xl border border-border bg-background shadow-sm transition-all focus-within:border-accent/50 focus-within:ring-2 focus-within:ring-accent/20 focus-within:shadow-accent/5">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="输入消息，或使用 '/' 唤起快捷指令 (Shift + Enter 换行)..."
            rows={1}
            className="w-full resize-none border-none bg-transparent py-4 pl-4 pr-[100px] text-sm text-text-primary placeholder-text-muted outline-none max-h-32 min-h-[56px] overflow-y-auto custom-scrollbar"
            style={{
              height: input ? `${Math.min(120, Math.max(56, input.split('\n').length * 20 + 36))}px` : '56px'
            }}
          />
          
          <div className="absolute bottom-2 right-2 flex items-center gap-1">
            <button
              title="上传文件 (暂未开放)"
              className="flex h-8 w-8 items-center justify-center rounded-xl text-text-muted transition-colors hover:bg-surface hover:text-text-primary"
            >
              <Paperclip className="h-4 w-4" />
            </button>
            <button
              title="快捷指令"
              className="flex h-8 w-8 items-center justify-center rounded-xl text-text-muted transition-colors hover:bg-surface hover:text-text-primary"
            >
              <Command className="h-4 w-4" />
            </button>
            <div className="w-px h-4 bg-border mx-1" />
            <button
              onClick={handleSend}
              disabled={!input.trim() || isTyping}
              className="flex h-8 w-8 items-center justify-center rounded-xl bg-accent text-background transition-all hover:bg-accent-strong disabled:opacity-30 disabled:hover:bg-accent"
            >
              <Send className="h-4 w-4 ml-0.5" />
            </button>
          </div>
        </div>
        <p className="mt-2 text-center text-[10px] text-text-muted">
          AI 生成的内容可能存在误差，请在发布前人工核实敏感信息。
        </p>
      </div>
    </div>
  );
}
