"use client";

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  AlertTriangle,
  Bot,
  CheckCircle2,
  Clock3,
  Coins,
  FileText,
  Image as ImageIcon,
  Loader2,
  Paperclip,
  Plus,
  Search,
  Send,
  ChevronDown,
  MessageSquare,
  Square,
  User,
  X,
} from "lucide-react";
import { API_BASE_URL } from "@/lib/api";
import { safeLocalStorageGet } from "@/lib/browser-storage";
import {
  buildFileAccept,
  formatSupportedFileTypes,
  isFileSupportedByTypes,
  resolveSupportedFileTypes,
} from "@/lib/ai-file-types";
import { getAIJob, getAIJobArtifacts, listAIJobs, listAIModels } from "@/lib/services";
import { getModelDisplayName } from "@/lib/model-display";
import type { AIJob, AIJobArtifact, AIModel } from "@/lib/types";
import { cn } from "@/lib/utils";
import { ChatMarkdown } from "@/components/ui/chat-markdown";

type ChatAttachmentKind = "image" | "text" | "file";

type ChatAttachment = {
  id: string;
  fileName: string;
  mimeType: string;
  kind: ChatAttachmentKind;
  dataUrl?: string;
  publicUrl?: string | null;
  textContent?: string | null;
  sizeBytes?: number | null;
  removable?: boolean;
};

type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: string;
  state?: "pending" | "streaming" | "done" | "error";
  isSeed?: boolean;
  modelName?: string | null;
  rawContent?: unknown;
  attachments?: ChatAttachment[];
  jobId?: string | null;
  completionState?: string | null;
  warningCodes?: string[];
  warningMessage?: string | null;
};

type StreamEventPayload = {
  jobId?: string;
  modelName?: string;
  delta?: string;
  text?: string;
  role?: string;
  finishReason?: string;
  completionState?: string;
  warningCodes?: string[];
  warningMessage?: string;
  protocolFamily?: string;
  streamDiagnostics?: Record<string, unknown>;
  done?: boolean;
  error?: string;
  progressed?: boolean;
  ping?: boolean;
};

type StreamReadState = {
  sawDone: boolean;
  sawError: boolean;
};

const DEFAULT_CHAT_MAX_TOKENS = 2048;
const ATTACHMENT_HEAVY_CHAT_MAX_TOKENS = 3200;
const LONG_FORM_CHAT_MAX_TOKENS = 4800;
const HIGH_OUTPUT_CHAT_MAX_TOKENS = 4096;
const HIGH_OUTPUT_LONG_FORM_CHAT_MAX_TOKENS = 6400;
const CHAT_MAX_TOKEN_HARD_CAP = 8192;
const CHAT_MODELS_STALE_TIME = 5 * 60 * 1000;
const CHAT_HISTORY_STALE_TIME = 15 * 1000;

const INITIAL_MESSAGES: ChatMessage[] = [
  {
    id: "seed-assistant",
    role: "assistant",
    content:
      "你好，我是 OmniDrive 聊天助手。这里是纯模型对话，不接知识库。你可以直接聊脚本、创意、任务思路，也可以把图片或文本附件一起发给我。",
    timestamp: "",
    state: "done",
    isSeed: true,
  },
];

function buildConversationMessages(history: ChatMessage[], nextUserMessage: string) {
  const messages = history
    .filter((item) => !item.isSeed)
    .filter((item) => item.state !== "pending")
    .filter((item) => item.rawContent || item.content.trim())
    .map((item) => ({
      role: item.role,
      content: item.rawContent ?? item.content.trim(),
    }));

  messages.push({
    role: "user" as const,
    content: nextUserMessage.trim(),
  });
  return messages;
}

function extractContentText(content: unknown): string {
  if (typeof content === "string") {
    return content;
  }
  if (Array.isArray(content)) {
    return content.map((item) => extractContentText(item)).join("\n");
  }
  if (content && typeof content === "object") {
    const record = content as Record<string, unknown>;
    if (typeof record.text === "string") {
      return record.text;
    }
    return "";
  }
  return "";
}

function estimateConversationChars(messages: Array<{ role: string; content: unknown }>) {
  return messages.reduce((total, item) => total + extractContentText(item.content).length, 0);
}

function isLongFormChatPrompt(text: string) {
  const normalized = text.trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return [
    "详细",
    "做表",
    "表格",
    "逐步",
    "展开",
    "完整",
    "分析",
    "测算",
    "收入",
    "计算",
    "列出",
    "对比",
    "方案",
    "markdown table",
    "table",
    "breakdown",
    "step by step",
    "analysis",
    "detailed",
    "compare",
    "plan",
  ].some((keyword) => normalized.includes(keyword));
}

function isHighOutputChatModel(modelName?: string | null) {
  const normalized = (modelName || "").trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return ["gemini", "claude", "opus", "deepseek", "r1", "o1", "o3", "o4"].some((marker) =>
    normalized.includes(marker),
  );
}

function roundChatTokenBudget(value: number) {
  const step = 256;
  return Math.ceil(value / step) * step;
}

function resolveRequestedChatMaxTokens(
  model: AIModel | null | undefined,
  history: ChatMessage[],
  nextUserMessage: string,
  attachments: ChatAttachment[],
) {
  const conversationMessages = buildConversationMessages(history, nextUserMessage);
  const conversationChars = estimateConversationChars(conversationMessages);
  const longForm = isLongFormChatPrompt(nextUserMessage);
  const highOutputModel = isHighOutputChatModel(model?.modelName);

  let budget = attachments.length > 0 ? ATTACHMENT_HEAVY_CHAT_MAX_TOKENS : DEFAULT_CHAT_MAX_TOKENS;
  if (conversationChars >= 800) {
    budget = Math.max(budget, 3072);
  }
  if (longForm) {
    budget = Math.max(budget, LONG_FORM_CHAT_MAX_TOKENS);
  }
  if (highOutputModel && (longForm || conversationChars >= 600)) {
    budget = Math.max(budget, HIGH_OUTPUT_CHAT_MAX_TOKENS);
  }
  if (highOutputModel && longForm) {
    budget = Math.max(budget, HIGH_OUTPUT_LONG_FORM_CHAT_MAX_TOKENS);
  }
  if (conversationChars >= 2400) {
    budget += 1024;
  } else if (conversationChars >= 1200) {
    budget += 512;
  }
  return Math.min(CHAT_MAX_TOKEN_HARD_CAP, roundChatTokenBudget(budget));
}

function createConversationId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `chat-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function extractConversationId(job?: AIJob | null) {
  const inputPayload = (job?.inputPayload || {}) as Record<string, unknown>;
  return typeof inputPayload.conversationId === "string" ? inputPayload.conversationId : "";
}

function getConversationKey(job: AIJob) {
  return extractConversationId(job) || job.id;
}

function groupHistoryJobs(historyJobs: AIJob[]) {
  const grouped = new Map<string, AIJob>();

  for (const job of historyJobs) {
    const key = getConversationKey(job);
    if (!grouped.has(key)) {
      grouped.set(key, job);
    }
  }

  return Array.from(grouped.values());
}

function formatMessageTime(value?: string | null) {
  if (!value) {
    return "--:--";
  }
  return new Date(value).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatHistoryTime(value?: string | null) {
  if (!value) {
    return "";
  }
  return new Date(value).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatModelPrice(model?: AIModel | null) {
  if (!model) {
    return "价格待配置";
  }
  if (
    typeof model.chatInputBillingAmount === "number" &&
    typeof model.chatOutputBillingAmount === "number"
  ) {
    return `输入 ${model.chatInputBillingAmount.toFixed(2)} / 输出 ${model.chatOutputBillingAmount.toFixed(2)}`;
  }
  if (typeof model.chatInputBillingAmount === "number") {
    return `输入 ${model.chatInputBillingAmount.toFixed(2)}`;
  }
  if (typeof model.chatOutputBillingAmount === "number") {
    return `输出 ${model.chatOutputBillingAmount.toFixed(2)}`;
  }
  const amount = typeof model.billingAmount === "number" ? model.billingAmount : model.rawRate;
  if (typeof amount !== "number" || Number.isNaN(amount)) {
    return "价格待配置";
  }
  return `参考价 ${amount.toFixed(2)}`;
}

function buildUnsupportedFilesMessage(fileNames: string[], supportedTypes: string[]) {
  if (fileNames.length === 0) {
    return "";
  }
  return `当前模型不支持这些文件：${fileNames.join("、")}。允许类型：${formatSupportedFileTypes(supportedTypes)}`;
}

function matchesModelQuery(model: AIModel, query: string) {
  const normalized = query.trim().toLowerCase();
  if (!normalized) {
    return true;
  }
  return [model.modelAlias, model.modelName, model.description]
    .filter((item): item is string => typeof item === "string" && item.trim().length > 0)
    .some((item) => item.toLowerCase().includes(normalized));
}

function sortChatModels(items: AIModel[]) {
  return [...items].sort((left, right) => {
    if (left.isEnabled !== right.isEnabled) {
      return left.isEnabled ? -1 : 1;
    }
    return getModelDisplayName(left).localeCompare(
      getModelDisplayName(right),
      "zh-CN",
    );
  });
}

function toErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

function parseSSEBlock(block: string) {
  const lines = block.split(/\r?\n/);
  let event = "message";
  const dataLines: string[] = [];

  for (const line of lines) {
    if (line.startsWith("event:")) {
      event = line.slice(6).trim();
      continue;
    }
    if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).trimStart());
    }
  }

  if (dataLines.length === 0) {
    return null;
  }

  const payloadText = dataLines.join("\n");
  try {
    return {
      event,
      payload: JSON.parse(payloadText) as StreamEventPayload,
    };
  } catch {
    return {
      event,
      payload: { error: payloadText } satisfies StreamEventPayload,
    };
  }
}

async function readStream(
  stream: ReadableStream<Uint8Array>,
  onEvent: (event: string, payload: StreamEventPayload) => void,
): Promise<StreamReadState> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const state: StreamReadState = {
    sawDone: false,
    sawError: false,
  };

  while (true) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });

    const parts = buffer.split("\n\n");
    buffer = parts.pop() || "";

    for (const part of parts) {
      const parsed = parseSSEBlock(part);
      if (parsed) {
        if (parsed.event === "done") {
          state.sawDone = true;
        }
        if (parsed.event === "error") {
          state.sawError = true;
        }
        onEvent(parsed.event, parsed.payload);
      }
    }

    if (done) {
      if (buffer.trim()) {
        const parsed = parseSSEBlock(buffer);
        if (parsed) {
          if (parsed.event === "done") {
            state.sawDone = true;
          }
          if (parsed.event === "error") {
            state.sawError = true;
          }
          onEvent(parsed.event, parsed.payload);
        }
      }
      break;
    }
  }

  return state;
}

function emitBufferedSSEBlocks(
  rawText: string,
  onEvent: (event: string, payload: StreamEventPayload) => void,
): StreamReadState {
  const state: StreamReadState = {
    sawDone: false,
    sawError: false,
  };

  for (const part of rawText.split(/\r?\n\r?\n/)) {
    const parsed = parseSSEBlock(part);
    if (!parsed) {
      continue;
    }
    if (parsed.event === "done") {
      state.sawDone = true;
    }
    if (parsed.event === "error") {
      state.sawError = true;
    }
    onEvent(parsed.event, parsed.payload);
  }

  return state;
}

async function readChatResponse(
  response: Response,
  onEvent: (event: string, payload: StreamEventPayload) => void,
): Promise<StreamReadState> {
  const bufferedResponse = typeof response.clone === "function" ? response.clone() : null;

  if (
    !response.body ||
    typeof response.body.getReader !== "function" ||
    typeof TextDecoder !== "function"
  ) {
    return emitBufferedSSEBlocks(await response.text(), onEvent);
  }

  try {
    return await readStream(response.body, onEvent);
  } catch (error) {
    if (bufferedResponse) {
      return emitBufferedSSEBlocks(await bufferedResponse.text(), onEvent);
    }
    throw error;
  }
}

function scrollChatViewportToBottom(container: HTMLDivElement | null, behavior: ScrollBehavior) {
  if (!container) {
    return;
  }
  const targetTop = container.scrollHeight;
  if (behavior === "auto") {
    container.scrollTop = targetTop;
    return;
  }
  try {
    container.scrollTo({ top: targetTop, behavior });
  } catch {
    container.scrollTop = targetTop;
  }
}

function detectAttachmentKind(mimeType: string, fileName: string): ChatAttachmentKind {
  const normalizedMime = mimeType.trim().toLowerCase();
  if (normalizedMime.startsWith("image/")) {
    return "image";
  }
  if (normalizedMime.startsWith("text/")) {
    return "text";
  }
  const lowerName = fileName.toLowerCase();
  if (
    [".txt", ".md", ".markdown", ".json", ".csv", ".tsv", ".yaml", ".yml", ".xml", ".html", ".htm"].some((ext) =>
      lowerName.endsWith(ext),
    )
  ) {
    return "text";
  }
  return "file";
}

function isTextLikeFile(file: File) {
  return detectAttachmentKind(file.type || "", file.name) === "text";
}

function readFileAsDataURL(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(reader.error || new Error("读取文件失败"));
    reader.readAsDataURL(file);
  });
}

function readFileAsText(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(reader.error || new Error("读取文件失败"));
    reader.readAsText(file);
  });
}

async function fileToAttachment(file: File): Promise<ChatAttachment> {
  const [dataUrl, textContent] = await Promise.all([
    readFileAsDataURL(file),
    isTextLikeFile(file) ? readFileAsText(file) : Promise.resolve(""),
  ]);

  return {
    id: `${file.name}-${file.size}-${file.lastModified}`,
    fileName: file.name,
    mimeType: file.type || "application/octet-stream",
    kind: detectAttachmentKind(file.type || "", file.name),
    dataUrl,
    textContent: textContent.trim() || undefined,
    sizeBytes: file.size,
    removable: true,
  };
}

function serializeAttachments(items: ChatAttachment[]) {
  return items.map((item) => ({
    fileName: item.fileName,
    mimeType: item.mimeType,
    dataUrl: item.dataUrl,
    textContent: item.textContent || undefined,
    sizeBytes: item.sizeBytes || undefined,
  }));
}

function extractDisplayText(rawContent: unknown) {
  if (typeof rawContent === "string") {
    return rawContent;
  }
  if (!Array.isArray(rawContent)) {
    return "";
  }
  return rawContent
    .map((item) => {
      if (!item || typeof item !== "object") {
        return "";
      }
      const part = item as Record<string, unknown>;
      if (part.type === "text" && typeof part.text === "string") {
        return part.text;
      }
      if (part.type === "image_url") {
        return "";
      }
      return "";
    })
    .filter(Boolean)
    .join("\n\n")
    .trim();
}

function toAttachmentMapFromHistory(job?: AIJob | null, artifacts: AIJobArtifact[] = []) {
  const map = new Map<number, ChatAttachment[]>();

  const pushAttachment = (messageIndex: number, attachment: ChatAttachment) => {
    const current = map.get(messageIndex) || [];
    current.push(attachment);
    map.set(messageIndex, current);
  };

  const inputPayload = (job?.inputPayload || {}) as Record<string, unknown>;
  const rawRefs = Array.isArray(inputPayload.attachments) ? inputPayload.attachments : [];
  for (const item of rawRefs) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const ref = item as Record<string, unknown>;
    const messageIndex =
      typeof ref.messageIndex === "number" && Number.isFinite(ref.messageIndex) ? Number(ref.messageIndex) : -1;
    if (messageIndex < 0) {
      continue;
    }
    const fileName = typeof ref.fileName === "string" ? ref.fileName : "附件";
    const mimeType = typeof ref.mimeType === "string" ? ref.mimeType : "application/octet-stream";
    pushAttachment(messageIndex, {
      id: `${messageIndex}-${fileName}`,
      fileName,
      mimeType,
      kind: detectAttachmentKind(mimeType, fileName),
      publicUrl: typeof ref.publicUrl === "string" ? ref.publicUrl : null,
      textContent: typeof ref.textContent === "string" ? ref.textContent : undefined,
      sizeBytes: typeof ref.sizeBytes === "number" ? ref.sizeBytes : undefined,
      removable: false,
    });
  }

  for (const artifact of artifacts) {
    if (artifact.artifactType !== "chat_attachment") {
      continue;
    }
    const payload = (artifact.payload || {}) as Record<string, unknown>;
    const messageIndex =
      typeof payload.messageIndex === "number" && Number.isFinite(payload.messageIndex)
        ? Number(payload.messageIndex)
        : rawRefs.length > 0
          ? -1
          : 0;
    if (messageIndex < 0) {
      continue;
    }
    const fileName = artifact.fileName || artifact.title || artifact.artifactKey;
    pushAttachment(messageIndex, {
      id: artifact.id,
      fileName,
      mimeType: artifact.mimeType || "application/octet-stream",
      kind: detectAttachmentKind(artifact.mimeType || "", fileName),
      publicUrl: artifact.publicUrl,
      textContent: artifact.textContent || undefined,
      sizeBytes: artifact.sizeBytes || undefined,
      removable: false,
    });
  }

  return map;
}

function buildMessagesFromHistory(job?: AIJob | null, artifacts: AIJobArtifact[] = []) {
  if (!job) {
    return INITIAL_MESSAGES;
  }

  const inputPayload = (job.inputPayload || {}) as Record<string, unknown>;
  const rawMessages = Array.isArray(inputPayload.messages) ? inputPayload.messages : [];
  const attachmentsByMessageIndex = toAttachmentMapFromHistory(job, artifacts);

  const historyMessages: ChatMessage[] = [];
  rawMessages.forEach((item, index) => {
    if (!item || typeof item !== "object") {
      return;
    }
    const message = item as Record<string, unknown>;
    const role = message.role === "assistant" ? "assistant" : "user";
    const rawContent = message.content;
    historyMessages.push({
      id: `${job.id}-${index}-${role}`,
      role,
      content: extractDisplayText(rawContent),
      rawContent,
      timestamp: job.updatedAt,
      state: "done",
      modelName: role === "assistant" ? getModelDisplayName(job) : null,
      attachments: attachmentsByMessageIndex.get(index) || [],
      jobId: job.id,
    });
  });

  const outputPayload = (job.outputPayload || {}) as Record<string, unknown>;
  const completionWarning = resolveCompletionWarning(outputPayload);
  const outputText =
    (typeof outputPayload.text === "string" && outputPayload.text.trim()) ||
    artifacts.find((item) => item.artifactType === "chat_response")?.textContent ||
    "";
  const fallbackStatusText =
    typeof job.message === "string" && job.message.trim() && job.message !== "聊天生成中"
      ? job.message.trim()
      : job.status === "failed"
        ? "本次对话失败，请重试。"
        : "聊天已中止。";
  const assistantContent = outputText.trim() || fallbackStatusText;

  if (assistantContent.trim()) {
    historyMessages.push({
      id: `${job.id}-assistant-final`,
      role: "assistant",
      content: assistantContent,
      rawContent: assistantContent,
      timestamp: job.finishedAt || job.updatedAt,
      state: job.status === "failed" ? "error" : "done",
      modelName: getModelDisplayName(job),
      jobId: job.id,
      completionState: completionWarning.completionState || null,
      warningCodes: completionWarning.warningCodes,
      warningMessage: completionWarning.warningMessage || null,
    });
  }

  return historyMessages.length > 0 ? historyMessages : INITIAL_MESSAGES;
}

function summarizeHistory(job: AIJob) {
  const sanitizeHistoryLine = (value: string) =>
    value
      .replace(/^Sender\s*\(untrusted metadata\):\s*/i, "")
      .replace(/^```[\w-]*$/g, "")
      .replace(/^\[[^\]]+\]\s*/, "")
      .replace(/^[>\-*]\s+/, "")
      .replace(/^[`]+|[`]+$/g, "")
      .trim();

  const isMetadataLikeLine = (value: string) => {
    const normalized = value.trim();
    if (!normalized) {
      return true;
    }
    if (/^(json|yaml|xml|markdown)$/i.test(normalized)) {
      return true;
    }
    if (/^[{}\[\],:]+$/.test(normalized)) {
      return true;
    }
    if (/^"[^"]+"\s*:/.test(normalized)) {
      return true;
    }
    if (/^[\[{].*[\]}]$/.test(normalized) && normalized.length < 120) {
      return true;
    }
    return false;
  };

  const summarizeText = (value: string) => {
    const cleaned = value
      .split(/\r?\n+/)
      .map(sanitizeHistoryLine)
      .filter((line) => line && !isMetadataLikeLine(line));
    if (cleaned.length === 0) {
      return "";
    }
    return cleaned.join(" ").replace(/\s+/g, " ").trim();
  };

  const inputPayload = (job.inputPayload || {}) as Record<string, unknown>;
  const rawMessages = Array.isArray(inputPayload.messages) ? inputPayload.messages : [];
  const lastUser = [...rawMessages]
    .reverse()
    .find((item) => item && typeof item === "object" && (item as Record<string, unknown>).role === "user");

  if (lastUser && typeof lastUser === "object") {
    const content = summarizeText(extractDisplayText((lastUser as Record<string, unknown>).content));
    if (content) {
      return content;
    }
  }
  if (job.prompt?.trim()) {
    const prompt = summarizeText(job.prompt);
    if (prompt) {
      return prompt;
    }
  }
  return `${getModelDisplayName(job, "聊天模型")} 对话`;
}

function historyJobNeedsArtifactHydration(job?: AIJob | null) {
  if (!job) {
    return false;
  }

  const inputPayload = (job.inputPayload || {}) as Record<string, unknown>;
  const outputPayload = (job.outputPayload || {}) as Record<string, unknown>;
  const rawRefs = Array.isArray(inputPayload.attachments) ? inputPayload.attachments : [];
  const hasOutputText = typeof outputPayload.text === "string" && outputPayload.text.trim().length > 0;

  if (!hasOutputText) {
    return true;
  }
  if (rawRefs.length > 0) {
    return false;
  }

  const rawMessages = Array.isArray(inputPayload.messages) ? inputPayload.messages : [];
  return rawMessages.some((item) => {
    if (!item || typeof item !== "object") {
      return false;
    }
    const content = (item as Record<string, unknown>).content;
    if (!Array.isArray(content)) {
      return false;
    }
    return content.some((part) => {
      if (!part || typeof part !== "object") {
        return false;
      }
      const typedPart = part as Record<string, unknown>;
      if (typedPart.type === "image_url") {
        return true;
      }
      return typedPart.type === "file";
    });
  });
}

function attachmentIcon(kind: ChatAttachmentKind) {
  if (kind === "image") {
    return ImageIcon;
  }
  return FileText;
}

function appendStreamError(existingContent: string, nextError: string) {
  const normalizedContent = existingContent.trim();
  const normalizedError = nextError.trim();
  if (!normalizedContent) {
    return normalizedError;
  }
  if (normalizedContent.includes(normalizedError)) {
    return normalizedContent;
  }
  return `${normalizedContent}\n\n[流式连接已中断] ${normalizedError}`;
}

function toWarningCodes(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
}

function defaultCompletionWarningMessage(warningCodes: string[]) {
  if (warningCodes.includes("reasoning_only_output")) {
    return "仅收到思考过程，未收到最终答复";
  }
  return "本次回复已结束，但模型未返回完整终止信号，内容可能不完整";
}

function resolveCompletionWarning(payload: Record<string, unknown>) {
  const completionState =
    typeof payload.completionState === "string" ? payload.completionState.trim() : "";
  const warningCodes = toWarningCodes(payload.warningCodes);
  const warningMessage =
    typeof payload.warningMessage === "string" ? payload.warningMessage.trim() : "";
  if (completionState !== "incomplete") {
    return {
      completionState: completionState || undefined,
      warningCodes,
      warningMessage: warningMessage || undefined,
    };
  }
  return {
    completionState,
    warningCodes,
    warningMessage: warningMessage || defaultCompletionWarningMessage(warningCodes),
  };
}

function parseThinkContent(fullText: string) {
  const thinkStart = fullText.indexOf("<think>");
  if (thinkStart === -1) {
    return { think: null, text: fullText };
  }
  const thinkEnd = fullText.indexOf("</think>", thinkStart);
  if (thinkEnd === -1) {
    return {
      think: fullText.slice(thinkStart + 7),
      text: fullText.slice(0, thinkStart),
    };
  }
  return {
    think: fullText.slice(thinkStart + 7, thinkEnd),
    text: fullText.slice(0, thinkStart) + fullText.slice(thinkEnd + 8),
  };
}

function ThinkBlock({ content, isStreaming }: { content: string; isStreaming: boolean }) {
  const [collapsed, setCollapsed] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isStreaming) {
      const frame = window.requestAnimationFrame(() => setCollapsed(true));
      return () => window.cancelAnimationFrame(frame);
    }
    return undefined;
  }, [isStreaming]);

  useEffect(() => {
    if (isStreaming && !collapsed && containerRef.current) {
      const el = containerRef.current;
      el.scrollTop = el.scrollHeight;
    }
  }, [content, isStreaming, collapsed]);

  return (
    <div className="mb-4 overflow-hidden rounded-xl border border-border/80 bg-surface/30">
      <button
        type="button"
        onClick={() => setCollapsed(!collapsed)}
        className="flex w-full items-center gap-2 px-3 py-2 text-[11px] font-medium text-text-muted transition-colors hover:bg-surface-hover/50 hover:text-text-primary"
      >
        <span>{isStreaming ? "思考中..." : "思考结束"}</span>
        <ChevronDown
          className={cn("h-3.5 w-3.5 transition-transform", collapsed ? "" : "rotate-180")}
        />
      </button>

      <AnimatePresence initial={false}>
        {!collapsed && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="border-t border-border/50"
          >
            <div
              ref={containerRef}
              className="custom-scrollbar max-h-40 overflow-y-auto px-4 py-3 text-xs leading-[1.65] text-text-secondary/80"
            >
              <span className="whitespace-pre-wrap">{content}</span>
              {isStreaming && <span className="chat-streaming-cursor inline-block ml-1" />}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function ChatThinkingIndicator() {
  return (
    <div className="flex items-center gap-2 px-1 py-1.5">
      <span className="chat-thinking-dot" />
      <span className="chat-thinking-dot" />
      <span className="chat-thinking-dot" />
      <span className="ml-1.5 text-xs text-text-muted/70">AI正在思考中</span>
    </div>
  );
}

function AttachmentList({
  attachments,
  onRemove,
  compact = false,
}: {
  attachments: ChatAttachment[];
  onRemove?: (attachmentId: string) => void;
  compact?: boolean;
}) {
  if (!attachments.length) {
    return null;
  }

  return (
    <div
      className={cn(
        "mt-3",
        compact ? "mt-2 grid gap-2 sm:grid-cols-2 xl:grid-cols-3" : "flex flex-wrap gap-2",
      )}
    >
      {attachments.map((attachment) => {
        const Icon = attachmentIcon(attachment.kind);
        const href = attachment.publicUrl || attachment.dataUrl;
        return (
          <div
            key={attachment.id}
            className={cn(
              "group flex min-w-0 items-center gap-2 rounded-2xl border border-border/80 bg-background/60 px-2.5 py-2 text-left",
              compact && "w-full bg-background/45",
            )}
          >
            {attachment.kind === "image" && href ? (
              <a href={href} target="_blank" rel="noreferrer" className="shrink-0">
                <AttachmentThumbnail href={href} fileName={attachment.fileName} />
              </a>
            ) : (
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-white/10 text-text-muted">
                <Icon className="h-4 w-4" />
              </div>
            )}

            <div className="min-w-0 flex-1">
              {href ? (
                <a
                  href={href}
                  target="_blank"
                  rel="noreferrer"
                  className="block truncate text-xs font-semibold text-text-primary hover:text-accent"
                >
                  {attachment.fileName}
                </a>
              ) : (
                <div className="truncate text-xs font-semibold text-text-primary">{attachment.fileName}</div>
              )}
              <div className="mt-0.5 text-[11px] text-text-muted">
                {attachment.kind === "image" ? "图片附件" : attachment.kind === "text" ? "文本附件" : "文件附件"}
              </div>
            </div>

            {attachment.removable && onRemove ? (
              <button
                type="button"
                onClick={() => onRemove(attachment.id)}
                className="shrink-0 rounded-full p-1 text-text-muted transition-colors hover:bg-white/10 hover:text-text-primary"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

function AttachmentThumbnail({ href, fileName }: { href: string; fileName: string }) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-white/10 text-text-muted">
        <ImageIcon className="h-4 w-4" />
      </div>
    );
  }

  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={href}
      alt={fileName}
      className="h-11 w-11 rounded-xl object-cover"
      onError={() => setFailed(true)}
    />
  );
}

export default function ChatPage() {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const messagesViewportRef = useRef<HTMLDivElement>(null);
  const nextScrollBehaviorRef = useRef<ScrollBehavior>("smooth");
  const streamAbortRef = useRef<AbortController | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const [messages, setMessages] = useState<ChatMessage[]>(INITIAL_MESSAGES);
  const [draft, setDraft] = useState("");
  const [selectedModelName, setSelectedModelName] = useState("");
  const [modelQuery, setModelQuery] = useState("");
  const [selectedJobId, setSelectedJobId] = useState("");
  const [submitError, setSubmitError] = useState("");
  const [sending, setSending] = useState(false);
  const [modelDropdownOpen, setModelDropdownOpen] = useState(false);
  const [draftAttachments, setDraftAttachments] = useState<ChatAttachment[]>([]);
  const [pendingHydrationJobId, setPendingHydrationJobId] = useState("");
  const [autoSelectLatestHistory, setAutoSelectLatestHistory] = useState(true);
  const [conversationId, setConversationId] = useState("");

  const {
    data: rawModels = [],
    isLoading: modelsLoading,
  } = useQuery<AIModel[], Error>({
    queryKey: ["aiModels", "chat"],
    queryFn: () => listAIModels({ category: "chat" }),
    staleTime: CHAT_MODELS_STALE_TIME,
  });

  const {
    data: historyJobs = [],
    isLoading: historyLoading,
  } = useQuery<AIJob[], Error>({
    queryKey: ["aiJobs", "chat", "history"],
    queryFn: () =>
      listAIJobs({
        jobType: "chat",
        source: "omnidrive_chat",
        payloadMode: "summary",
        limit: 30,
      }),
    staleTime: CHAT_HISTORY_STALE_TIME,
  });

  const { data: selectedJob } = useQuery<AIJob, Error>({
    queryKey: ["aiJob", "historyDetail", selectedJobId],
    queryFn: () => getAIJob(selectedJobId, { payloadMode: "history_detail" }),
    enabled: Boolean(selectedJobId),
    staleTime: CHAT_HISTORY_STALE_TIME,
  });

  const selectedJobNeedsArtifacts = useMemo(
    () => historyJobNeedsArtifactHydration(selectedJob),
    [selectedJob],
  );

  const { data: selectedJobArtifacts = [] } = useQuery<AIJobArtifact[], Error>({
    queryKey: ["aiJobArtifacts", "chat", selectedJobId, selectedJobNeedsArtifacts ? "chat" : "skip"],
    queryFn: () => getAIJobArtifacts(selectedJobId, { artifactMode: "chat" }),
    enabled: Boolean(selectedJobId) && selectedJobNeedsArtifacts,
    staleTime: CHAT_HISTORY_STALE_TIME,
  });

  const chatModels = useMemo(() => {
    const enabledModels = rawModels.filter((item) => item.category === "chat" && item.isEnabled);
    const fallbackModels = rawModels.filter((item) => item.category === "chat");
    return sortChatModels(enabledModels.length > 0 ? enabledModels : fallbackModels);
  }, [rawModels]);

  const filteredModels = useMemo(() => {
    return chatModels.filter((item) => matchesModelQuery(item, modelQuery));
  }, [chatModels, modelQuery]);
  const groupedHistoryJobs = useMemo(() => groupHistoryJobs(historyJobs), [historyJobs]);

  const activeModel = useMemo(() => {
    return chatModels.find((item) => item.modelName === selectedModelName) || chatModels[0] || null;
  }, [chatModels, selectedModelName]);
  const activeSupportedFileTypes = useMemo(() => resolveSupportedFileTypes(activeModel), [activeModel]);
  const activeSupportedFileTypesLabel = useMemo(
    () => formatSupportedFileTypes(activeSupportedFileTypes),
    [activeSupportedFileTypes],
  );
  const activeFileAccept = useMemo(() => buildFileAccept(activeSupportedFileTypes), [activeSupportedFileTypes]);

  useEffect(() => {
    if (!selectedModelName && chatModels.length > 0) {
      setSelectedModelName(chatModels[0].modelName);
      return;
    }
    if (
      selectedModelName &&
      !chatModels.some((item) => item.modelName === selectedModelName) &&
      chatModels.length > 0
    ) {
      setSelectedModelName(chatModels[0].modelName);
    }
  }, [chatModels, selectedModelName]);

  useEffect(() => {
    if (autoSelectLatestHistory && !selectedJobId && groupedHistoryJobs.length > 0 && !sending) {
      const latestJob = groupedHistoryJobs[0];
      setSelectedJobId(latestJob.id);
      setPendingHydrationJobId(latestJob.id);
      setConversationId(getConversationKey(latestJob));
      setAutoSelectLatestHistory(false);
    }
  }, [autoSelectLatestHistory, groupedHistoryJobs, selectedJobId, sending]);

  useEffect(() => {
    if (!selectedJob || sending || pendingHydrationJobId !== selectedJob.id) {
      return;
    }
    nextScrollBehaviorRef.current = "auto";
    setMessages(buildMessagesFromHistory(selectedJob, selectedJobArtifacts));
    setDraft("");
    setDraftAttachments([]);
    setSubmitError("");
    setPendingHydrationJobId("");
    if (selectedJob.modelName) {
      setSelectedModelName(selectedJob.modelName);
    }
  }, [selectedJob, selectedJobArtifacts, pendingHydrationJobId, sending]);

  useEffect(() => {
    if (!selectedJob) {
      return;
    }
    setConversationId((previous) => previous || getConversationKey(selectedJob));
  }, [selectedJob]);

  useLayoutEffect(() => {
    scrollChatViewportToBottom(messagesViewportRef.current, nextScrollBehaviorRef.current);
    nextScrollBehaviorRef.current = "smooth";
  }, [messages]);

  useEffect(() => {
    return () => {
      streamAbortRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setModelDropdownOpen(false);
      }
    }
    if (modelDropdownOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [modelDropdownOpen]);

  const openFilePicker = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const removeDraftAttachment = useCallback((attachmentId: string) => {
    setDraftAttachments((previous) => previous.filter((item) => item.id !== attachmentId));
  }, []);

  const handleFilesSelected = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const files = Array.from(event.target.files || []);
      event.target.value = "";
      if (files.length === 0) {
        return;
      }

      const supportedFiles = files.filter((file) => isFileSupportedByTypes(file, activeSupportedFileTypes));
      const unsupportedFiles = files.filter((file) => !isFileSupportedByTypes(file, activeSupportedFileTypes));
      if (unsupportedFiles.length > 0) {
        setSubmitError(
          buildUnsupportedFilesMessage(
            unsupportedFiles.map((item) => item.name),
            activeSupportedFileTypes,
          ),
        );
      }
      if (supportedFiles.length === 0) {
        return;
      }

      try {
        const nextAttachments = await Promise.all(supportedFiles.map((file) => fileToAttachment(file)));
        setDraftAttachments((previous) => {
          const seen = new Set(previous.map((item) => item.id));
          const merged = [...previous];
          for (const item of nextAttachments) {
            if (!seen.has(item.id)) {
              merged.push(item);
            }
          }
          return merged;
        });
        if (unsupportedFiles.length === 0) {
          setSubmitError("");
        }
      } catch (error) {
        setSubmitError(toErrorMessage(error, "附件读取失败，请重新选择文件"));
      }
    },
    [activeSupportedFileTypes],
  );

  async function handleSend() {
    const fallbackPrompt = "请先阅读我上传的附件，并根据这些内容给我回复。";
    const nextUserMessage = draft.trim() || (draftAttachments.length > 0 ? fallbackPrompt : "");
    if ((!nextUserMessage && draftAttachments.length === 0) || !activeModel || sending) {
      return;
    }
    const unsupportedDrafts = draftAttachments.filter(
      (item) => !isFileSupportedByTypes({ name: item.fileName, type: item.mimeType }, activeSupportedFileTypes),
    );
    if (unsupportedDrafts.length > 0) {
      setSubmitError(
        buildUnsupportedFilesMessage(
          unsupportedDrafts.map((item) => item.fileName),
          activeSupportedFileTypes,
        ),
      );
      return;
    }

    const token = safeLocalStorageGet("omnidrive_token");
    if (!token) {
      setSubmitError("登录已失效，请重新登录后再聊天");
      return;
    }

    const now = new Date().toISOString();
    const userMessage: ChatMessage = {
      id: `user-${Date.now()}`,
      role: "user",
      content: nextUserMessage,
      rawContent: nextUserMessage,
      timestamp: now,
      state: "done",
      attachments: draftAttachments.map((item) => ({ ...item, removable: false })),
    };
    const assistantMessageId = `assistant-${Date.now() + 1}`;
    const assistantMessage: ChatMessage = {
      id: assistantMessageId,
      role: "assistant",
      content: "",
      rawContent: "",
      timestamp: now,
      state: "pending",
      modelName: getModelDisplayName(activeModel),
    };

    const controller = new AbortController();
    streamAbortRef.current = controller;

    const outboundAttachments = serializeAttachments(draftAttachments);
    const requestedMaxTokens = resolveRequestedChatMaxTokens(
      activeModel,
      messages,
      nextUserMessage,
      draftAttachments,
    );
    const activeConversationId = conversationId || (selectedJob ? getConversationKey(selectedJob) : "") || createConversationId();
    setDraft("");
    setDraftAttachments([]);
    setSubmitError("");
    setSending(true);
    setConversationId(activeConversationId);
    setMessages((previous) => [...previous, userMessage, assistantMessage]);

    let createdJobId = "";
    try {
      const response = await fetch(`${API_BASE_URL}/ai/chat/stream`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          modelName: activeModel.modelName,
          prompt: nextUserMessage,
          inputPayload: {
            prompt: nextUserMessage,
            temperature: 0.5,
            maxTokens: requestedMaxTokens,
            conversationId: activeConversationId,
            messages: buildConversationMessages(messages, nextUserMessage),
            attachments: outboundAttachments,
          },
        }),
        signal: controller.signal,
      });

      if (!response.ok) {
        const payloadText = await response.text();
        let nextMessage = payloadText || "聊天请求失败，请稍后再试";
        try {
          const parsed = JSON.parse(payloadText) as { error?: string };
          if (parsed?.error) {
            nextMessage = parsed.error;
          }
        } catch {}
        throw new Error(nextMessage);
      }

      let receivedText = "";
      const streamState = await readChatResponse(response, (event, payload) => {
        if (event === "meta") {
          if (payload.jobId) {
            createdJobId = payload.jobId;
            setSelectedJobId(payload.jobId);
            setPendingHydrationJobId("");
          }
          return;
        }

        if (event === "ping") {
          return;
        }

        if (event === "delta" && payload.delta) {
          receivedText += payload.delta;
          setMessages((previous) =>
            previous.map((item) =>
              item.id === assistantMessageId
                ? {
                    ...item,
                    content: receivedText,
                    rawContent: receivedText,
                    state: "streaming",
                    modelName: item.modelName,
                    jobId: createdJobId || item.jobId,
                  }
                : item,
            ),
          );
          return;
        }

        if (event === "progress") {
          setMessages((previous) =>
            previous.map((item) =>
              item.id === assistantMessageId
                ? {
                    ...item,
                    state: "streaming",
                    modelName: item.modelName,
                    jobId: createdJobId || item.jobId,
                  }
                : item,
            ),
          );
          return;
        }

        if (event === "done") {
          const finalText = (payload.text || receivedText).trim();
          const completionWarning = resolveCompletionWarning(payload as Record<string, unknown>);
          setMessages((previous) =>
            previous.map((item) =>
              item.id === assistantMessageId
                ? {
                    ...item,
                    content: finalText || "模型已完成，但没有返回可展示的文本内容。",
                    rawContent: finalText || "模型已完成，但没有返回可展示的文本内容。",
                    state: "done",
                    timestamp: new Date().toISOString(),
                    jobId: createdJobId || item.jobId,
                    completionState: completionWarning.completionState || null,
                    warningCodes: completionWarning.warningCodes,
                    warningMessage: completionWarning.warningMessage || null,
                  }
                : item,
            ),
          );
          return;
        }

        if (event === "error") {
          const nextMessage = payload.error || "本次对话失败，请稍后重试。";
          setSubmitError(nextMessage);
          setMessages((previous) =>
            previous.map((item) =>
              item.id === assistantMessageId
                ? {
                    ...item,
                    content: appendStreamError(item.content, nextMessage),
                    rawContent: appendStreamError(String(item.rawContent || item.content || ""), nextMessage),
                    state: "error",
                    timestamp: new Date().toISOString(),
                    jobId: createdJobId || item.jobId,
                  }
                : item,
            ),
          );
        }
      });
      if (!streamState.sawDone && !streamState.sawError) {
        throw new Error(
          receivedText.trim()
            ? "聊天流意外中断，当前回复只收到了一部分，请重新发送或继续追问。"
            : "聊天流未正常结束，请稍后重试。",
        );
      }
    } catch (error) {
      if ((error as Error)?.name === "AbortError") {
        setMessages((previous) =>
          previous.map((item) =>
            item.id === assistantMessageId
              ? {
                  ...item,
                  content: item.content.trim() || "已停止当前回复。",
                  rawContent: item.content.trim() || "已停止当前回复。",
                  state: item.content.trim() ? "done" : "error",
                  timestamp: new Date().toISOString(),
                }
              : item,
          ),
        );
      } else {
        const nextError = toErrorMessage(error, "聊天请求失败，请稍后再试");
        setSubmitError(nextError);
        setMessages((previous) =>
          previous.map((item) =>
            item.id === assistantMessageId
              ? {
                  ...item,
                  content: appendStreamError(item.content, nextError),
                  rawContent: appendStreamError(String(item.rawContent || item.content || ""), nextError),
                  state: "error",
                  timestamp: new Date().toISOString(),
                  jobId: createdJobId || item.jobId,
                }
              : item,
          ),
        );
      }
    } finally {
      setSending(false);
      if (streamAbortRef.current === controller) {
        streamAbortRef.current = null;
      }
      void queryClient.invalidateQueries({ queryKey: ["aiJobs", "chat", "history"] });
      void queryClient.invalidateQueries({ queryKey: ["billingSummary"] });
      void queryClient.invalidateQueries({ queryKey: ["walletLedger"] });
      const historyJobId = createdJobId || selectedJobId;
      if (historyJobId) {
        void queryClient.invalidateQueries({ queryKey: ["aiJob", "historyDetail", historyJobId] });
        void queryClient.invalidateQueries({ queryKey: ["aiJobArtifacts", "chat", historyJobId] });
      }
    }
  }

  function stopStreaming() {
    streamAbortRef.current?.abort();
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      void handleSend();
    }
  }

  function startNewConversation() {
    stopStreaming();
    setSending(false);
    nextScrollBehaviorRef.current = "auto";
    setSelectedJobId("");
    setPendingHydrationJobId("");
    setAutoSelectLatestHistory(false);
    setConversationId(createConversationId());
    setMessages(INITIAL_MESSAGES);
    setDraft("");
    setDraftAttachments([]);
    setSubmitError("");
  }

  const selectedConversationKey = conversationId || (selectedJob ? getConversationKey(selectedJob) : "");

  return (
    <div className="grid h-[calc(100vh-2rem)] grid-cols-1 gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
      {/* ── Sidebar: History-first ── */}
      <aside className="flex min-h-0 flex-col overflow-hidden rounded-3xl border border-border bg-surface">
        <div className="border-b border-border px-4 py-4">
          <button type="button" onClick={startNewConversation} className="flex w-full items-center justify-center gap-2 rounded-2xl bg-gradient-to-r from-accent to-cyan px-4 py-3 text-sm font-semibold text-background transition-all hover:shadow-lg hover:shadow-accent/25">
            <Plus className="h-4 w-4" />
            新对话
          </button>
        </div>

        <div ref={dropdownRef} className="relative border-b border-border px-4 py-3">
          <button type="button" onClick={() => setModelDropdownOpen(!modelDropdownOpen)} className="flex w-full items-center justify-between rounded-xl border border-border bg-surface-hover/70 px-3 py-2.5 text-left transition-all hover:border-accent/30">
            <div className="flex items-center gap-2 min-w-0">
              <Bot className="h-4 w-4 shrink-0 text-accent" />
              <span className="truncate text-sm font-medium text-text-primary">{getModelDisplayName(activeModel, "选择模型")}</span>
            </div>
            <ChevronDown className={cn("h-4 w-4 shrink-0 text-text-muted transition-transform", modelDropdownOpen && "rotate-180")} />
          </button>
          <AnimatePresence>
            {modelDropdownOpen && (
              <motion.div initial={{ opacity: 0, y: -8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -8 }} transition={{ duration: 0.15 }} className="absolute left-4 right-4 top-full z-50 mt-1 max-h-64 overflow-y-auto rounded-2xl border border-border bg-surface-elevated shadow-2xl shadow-black/40">
                <div className="p-2">
                  <div className="relative mb-2">
                    <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-muted" />
                    <input value={modelQuery} onChange={(e) => setModelQuery(e.target.value)} placeholder="搜索模型" className="w-full rounded-xl border border-border bg-background/60 py-2 pl-8 pr-3 text-xs text-text-primary outline-none focus:border-accent/40" />
                  </div>
                  {modelsLoading ? (
                    <div className="flex items-center gap-2 px-3 py-4 text-xs text-text-muted"><Loader2 className="h-3.5 w-3.5 animate-spin" /> 加载中</div>
                  ) : filteredModels.length === 0 ? (
                    <div className="px-3 py-4 text-xs text-text-muted">无匹配模型</div>
                  ) : (
                    filteredModels.map((model) => {
                      const selected = activeModel?.modelName === model.modelName;
                      return (
                        <button key={model.id} type="button" onClick={() => { setSelectedModelName(model.modelName); setModelDropdownOpen(false); }} className={cn("flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-left transition-all", selected ? "bg-accent/10 text-accent" : "text-text-primary hover:bg-surface-hover")}>
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              {selected && <CheckCircle2 className="h-3 w-3 text-accent" />}
                              <span className="truncate text-xs font-semibold">{getModelDisplayName(model)}</span>
                            </div>
                            <div className="mt-0.5 truncate text-[11px] text-text-muted">{model.description || "聊天模型"}</div>
                          </div>
                          <span className="shrink-0 text-[10px] text-text-muted">{formatModelPrice(model)}</span>
                        </button>
                      );
                    })
                  )}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-3 py-3">
          <div className="mb-2 px-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-text-muted">历史记录</div>
          {historyLoading ? (
            <div className="flex items-center gap-2 px-3 py-6 text-sm text-text-muted"><Loader2 className="h-4 w-4 animate-spin" /> 加载中</div>
          ) : groupedHistoryJobs.length === 0 ? (
            <div className="flex flex-col items-center gap-3 px-4 py-10 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-surface-hover"><MessageSquare className="h-5 w-5 text-text-muted" /></div>
              <p className="text-sm text-text-muted">还没有聊天记录</p>
              <p className="text-xs text-text-muted/60">发送第一条消息后会自动保存</p>
            </div>
          ) : (
            <div className="space-y-1">
              {groupedHistoryJobs.map((job) => {
                const active = selectedConversationKey === getConversationKey(job);
                return (
                  <button key={getConversationKey(job)} type="button" onClick={() => { stopStreaming(); setSending(false); nextScrollBehaviorRef.current = "auto"; setAutoSelectLatestHistory(false); setConversationId(getConversationKey(job)); setSelectedJobId(job.id); setPendingHydrationJobId(job.id); }} className={cn("group w-full rounded-xl px-3 py-2.5 text-left transition-all", active ? "bg-accent/10 border border-accent/30 shadow-sm shadow-accent/10" : "border border-transparent hover:bg-surface-hover/80")}>
                    <div className="line-clamp-2 text-sm font-medium leading-5 text-text-primary">{summarizeHistory(job)}</div>
                    <div className="mt-1.5 flex items-center gap-1.5 text-[11px] text-text-muted">
                      <Clock3 className="h-3 w-3" />
                      <span>{formatHistoryTime(job.updatedAt)}</span>
                      <span className="text-text-muted/40">·</span>
                      <span className="truncate">{getModelDisplayName(job)}</span>
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </aside>

      {/* ── Chat Area ── */}
      <section className="flex min-h-0 flex-col overflow-hidden rounded-3xl border border-border bg-surface">
        <div className="border-b border-border px-6 py-3">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-accent to-cyan text-background"><Bot className="h-4 w-4" /></div>
            <div className="min-w-0 flex-1">
              <h2 className="text-sm font-semibold text-text-primary">OmniDrive Chat</h2>
              <div className="flex items-center gap-2 text-[11px] text-text-muted">
                <span className="truncate">{getModelDisplayName(activeModel, "未选择模型")}</span>
                {activeModel && (<><span className="text-text-muted/30">·</span><span className="flex items-center gap-1"><Coins className="h-3 w-3" />{formatModelPrice(activeModel)}</span></>)}
              </div>
              <div className="mt-1 space-y-1">
                <p className="line-clamp-2 text-[11px] text-text-muted">
                  {activeModel?.description || "模型说明将从后台配置实时读取。"}
                </p>
                <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-text-muted">
                  <Paperclip className="h-3 w-3" />
                  <span>支持文件：{activeSupportedFileTypesLabel}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div ref={messagesViewportRef} className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
          <div className="flex h-full w-full flex-col">
            <div className="space-y-4">
              {messages.map((message) => {
                const parsedContent =
                  message.role === "assistant"
                    ? parseThinkContent(message.content)
                    : { think: null, text: message.content };
                const isThinkStreaming =
                  message.state === "streaming" &&
                  message.content.includes("<think>") &&
                  !message.content.includes("</think>");

                return (
                <motion.div key={message.id} initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25, ease: "easeOut" }} className={cn("flex gap-3", message.role === "user" ? "justify-end" : "justify-start")}>
                  {message.role === "assistant" && (
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-accent to-cyan text-background">
                      {message.state === "pending" ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Bot className="h-3.5 w-3.5" />}
                    </div>
                  )}
                  <div className={cn("max-w-[90%] rounded-2xl px-4 py-3 shadow-sm", message.role === "user" ? "rounded-tr-md bg-gradient-to-r from-accent to-cyan text-background" : "rounded-tl-md border border-border bg-surface-hover text-text-primary", message.state === "error" && "border-red-500/30 bg-red-500/10 text-red-100")}>
                    {message.state === "pending" || (message.state === "streaming" && message.role === "assistant" && !message.content.trim()) ? (
                      <ChatThinkingIndicator />
                    ) : (
                      <div className="text-sm leading-7">
                        {message.role === "assistant" ? (
                          <>
                            {parsedContent.think != null && (
                              <ThinkBlock
                                content={parsedContent.think.trim()}
                                isStreaming={isThinkStreaming}
                              />
                            )}
                            {parsedContent.text.trim() ? (
                              <ChatMarkdown
                                content={parsedContent.text.trim()}
                                allowMermaid={message.state === "done"}
                              />
                            ) : null}
                            {message.state === "done" &&
                            message.completionState === "incomplete" &&
                            message.warningMessage ? (
                              <div className="mt-3 flex items-start gap-2 rounded-xl border border-amber-500/20 bg-amber-500/10 px-3 py-2 text-xs leading-6 text-amber-200">
                                <AlertTriangle className="mt-0.5 h-3.5 w-3.5 flex-shrink-0" />
                                <span>{message.warningMessage}</span>
                              </div>
                            ) : null}
                          </>
                        ) : (
                          <span className="whitespace-pre-wrap">{message.content}</span>
                        )}
                        {message.state === "streaming" && parsedContent.text.trim() && !isThinkStreaming && <span className="chat-streaming-cursor inline-block ml-1" />}
                      </div>
                    )}
                    <AttachmentList attachments={message.attachments || []} compact />
                    <div className={cn("mt-2 flex items-center gap-2 text-[11px]", message.role === "user" ? "text-white/60" : "text-text-muted")}>
                      <span>{message.isSeed ? "欢迎语" : formatMessageTime(message.timestamp)}</span>
                      {message.modelName && <span>{message.modelName}</span>}
                    </div>
                  </div>
                  {message.role === "user" && (
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl border border-border bg-surface-hover text-text-primary"><User className="h-3.5 w-3.5" /></div>
                  )}
                </motion.div>
              );
            })}
              {submitError && (
                <div className="rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-200">
                  <div className="flex items-center gap-2"><AlertTriangle className="h-4 w-4" />{submitError}</div>
                </div>
              )}
            </div>
            <div className="h-2" />
          </div>
        </div>

        <div className="border-t border-border px-6 py-4">
          <div>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept={activeFileAccept || undefined}
              className="hidden"
              onChange={handleFilesSelected}
            />
            <AttachmentList attachments={draftAttachments} onRemove={removeDraftAttachment} />
            <div className="flex items-end gap-2 rounded-2xl border border-border bg-background/80 px-3 py-2.5 shadow-sm transition-colors focus-within:border-accent/40 focus-within:shadow-accent/10">
              <button
                type="button"
                onClick={openFilePicker}
                disabled={!activeModel || sending || activeSupportedFileTypes.length === 0}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-text-muted transition-colors hover:bg-surface-hover hover:text-text-primary disabled:opacity-40"
              >
                <Paperclip className="h-4 w-4" />
              </button>
              <textarea value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={handleKeyDown} placeholder={activeModel ? "输入消息..." : "请先选择模型"} disabled={!activeModel || sending} rows={1} className="max-h-32 min-h-[36px] flex-1 resize-none border-none bg-transparent py-1.5 text-sm leading-6 text-text-primary outline-none placeholder:text-text-muted/60 disabled:cursor-not-allowed disabled:opacity-50" style={{ height: "36px" }} onInput={(e) => { const t = e.target as HTMLTextAreaElement; t.style.height = "36px"; t.style.height = Math.min(t.scrollHeight, 128) + "px"; }} />
              {sending ? (
                <button type="button" onClick={stopStreaming} className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-red-500/20 text-red-300 transition-colors hover:bg-red-500/30"><Square className="h-4 w-4" /></button>
              ) : (
                <button type="button" onClick={() => void handleSend()} disabled={(!draft.trim() && draftAttachments.length === 0) || !activeModel} className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gradient-to-r from-accent to-cyan text-background transition-all hover:shadow-lg hover:shadow-accent/25 disabled:opacity-40 disabled:hover:shadow-none"><Send className="h-4 w-4" /></button>
              )}
            </div>
            <div className="mt-2 flex items-center justify-between px-1 text-[11px] text-text-muted/60">
              <span>{sending ? "正在接收回复..." : "Enter 发送 · Shift+Enter 换行"}</span>
              <span>{activeSupportedFileTypes.length > 0 ? `可上传：${activeSupportedFileTypesLabel}` : "当前模型未配置可上传文件"}</span>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
