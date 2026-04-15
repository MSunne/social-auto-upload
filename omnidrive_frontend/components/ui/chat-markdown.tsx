"use client";

import { type ReactNode, Children, isValidElement, useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { Check, Clipboard, Loader2 } from "lucide-react";
import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneDark } from "react-syntax-highlighter/dist/esm/styles/prism";
import { normalizeChatMarkdown } from "@/components/ui/chat-markdown-helpers";

type ChatMarkdownProps = {
  content: string;
  allowMermaid?: boolean;
};

type MarkdownCodeProps = {
  children?: ReactNode;
  className?: string;
};

type MarkdownCodeBlockProps = {
  code: string;
  language?: string;
  note?: string;
};

let mermaidLoader: Promise<typeof import("mermaid").default> | null = null;

function loadMermaid() {
  if (!mermaidLoader) {
    mermaidLoader = import("mermaid").then((module) => {
      module.default.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        theme: "dark",
      });
      return module.default;
    });
  }
  return mermaidLoader;
}

function stringifyCodeChildren(children: ReactNode): string {
  return Children.toArray(children)
    .map((child) => {
      if (typeof child === "string") {
        return child;
      }
      if (typeof child === "number") {
        return String(child);
      }
      if (isValidElement<{ children?: ReactNode }>(child)) {
        return stringifyCodeChildren(child.props.children);
      }
      return "";
    })
    .join("");
}

function normalizeCodeString(value: string) {
  return value.replace(/\n$/, "");
}

function extractPreCodeProps(children: ReactNode): MarkdownCodeProps | null {
  if (Children.count(children) !== 1) {
    return null;
  }
  const onlyChild = Children.only(children);
  if (!isValidElement<MarkdownCodeProps>(onlyChild)) {
    return null;
  }
  if (onlyChild.type !== "code") {
    return null;
  }
  return {
    className: onlyChild.props.className,
    children: onlyChild.props.children,
  };
}

function writeTextToClipboard(text: string) {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    return navigator.clipboard.writeText(text);
  }

  return new Promise<void>((resolve, reject) => {
    if (typeof document === "undefined") {
      reject(new Error("当前环境不支持复制"));
      return;
    }

    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "true");
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    textarea.style.pointerEvents = "none";
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();

    try {
      const copied = document.execCommand("copy");
      document.body.removeChild(textarea);
      if (!copied) {
        reject(new Error("复制失败"));
        return;
      }
      resolve();
    } catch (error) {
      document.body.removeChild(textarea);
      reject(error instanceof Error ? error : new Error("复制失败"));
    }
  });
}

function CodeCopyButton({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(() => {
    void writeTextToClipboard(code)
      .then(() => {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 2000);
      })
      .catch(() => {
        setCopied(false);
      });
  }, [code]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="inline-flex items-center gap-1 rounded-lg bg-white/10 px-2 py-1 text-[11px] font-medium text-text-muted transition-colors hover:bg-white/20 hover:text-text-primary"
    >
      {copied ? <Check className="h-3 w-3" /> : <Clipboard className="h-3 w-3" />}
      {copied ? "已复制" : "复制"}
    </button>
  );
}

function MarkdownCodeBlock({ code, language, note }: MarkdownCodeBlockProps) {
  const displayLanguage = (language || "text").toLowerCase();

  return (
    <div className="group/code my-3 overflow-hidden rounded-xl border border-border bg-[#1e1e2e]">
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className="text-[11px] font-medium uppercase tracking-wider text-text-muted">
            {displayLanguage}
          </span>
          {note ? <span className="text-[11px] text-amber-300/90">{note}</span> : null}
        </div>
        <CodeCopyButton code={code} />
      </div>
      {language ? (
        <SyntaxHighlighter
          style={oneDark}
          language={language}
          PreTag="div"
          customStyle={{
            margin: 0,
            padding: "1rem",
            background: "transparent",
            fontSize: "0.8125rem",
            lineHeight: "1.7",
          }}
        >
          {code}
        </SyntaxHighlighter>
      ) : (
        <pre className="overflow-x-auto p-4 text-[0.8125rem] leading-7 text-text-primary">
          <code>{code}</code>
        </pre>
      )}
    </div>
  );
}

function MermaidBlock({ code }: { code: string }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const diagramId = useId().replace(/[:]/g, "-");
  const [svg, setSvg] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    void loadMermaid()
      .then(async (mermaid) => {
        const rendered = await mermaid.render(`chat-mermaid-${diagramId}`, code);
        if (cancelled) {
          return;
        }
        setSvg(rendered.svg);
        setLoading(false);
        if (rendered.bindFunctions && containerRef.current) {
          rendered.bindFunctions(containerRef.current);
        }
      })
      .catch((reason) => {
        if (cancelled) {
          return;
        }
        const nextError = reason instanceof Error ? reason.message : "图表渲染失败";
        setError(nextError);
        setSvg("");
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [code, diagramId]);

  if (error) {
    return <MarkdownCodeBlock code={code} language="mermaid" note="图表渲染失败，已回退源码显示" />;
  }

  if (loading) {
    return (
      <div className="my-3 overflow-hidden rounded-xl border border-border bg-[#1e1e2e]">
        <div className="flex items-center justify-between border-b border-white/10 px-4 py-2">
          <span className="text-[11px] font-medium uppercase tracking-wider text-text-muted">mermaid</span>
          <CodeCopyButton code={code} />
        </div>
        <div className="flex items-center justify-center gap-2 px-4 py-10 text-sm text-text-muted">
          <Loader2 className="h-4 w-4 animate-spin" />
          <span>流程图渲染中...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="my-3 overflow-hidden rounded-xl border border-border bg-[#1e1e2e]">
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-2">
        <span className="text-[11px] font-medium uppercase tracking-wider text-text-muted">mermaid</span>
        <CodeCopyButton code={code} />
      </div>
      <div
        ref={containerRef}
        className="overflow-x-auto p-4 [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full"
        dangerouslySetInnerHTML={{ __html: svg }}
      />
    </div>
  );
}

export function ChatMarkdown({ content, allowMermaid = false }: ChatMarkdownProps) {
  const normalized = useMemo(() => normalizeChatMarkdown(content), [content]);

  const components = useMemo<Components>(
    () => ({
      pre({ children }) {
        const codeProps = extractPreCodeProps(children);
        if (!codeProps) {
          return (
            <pre className="my-3 overflow-x-auto rounded-xl border border-border bg-[#1e1e2e] p-4 text-[0.8125rem] leading-7 text-text-primary">
              {children}
            </pre>
          );
        }

        const codeString = normalizeCodeString(stringifyCodeChildren(codeProps.children));
        const match = /language-([\w-]+)/.exec(codeProps.className || "");
        const language = match?.[1]?.toLowerCase();

        if (language === "mermaid") {
          if (!allowMermaid) {
            return <MarkdownCodeBlock code={codeString} language="mermaid" />;
          }
          return <MermaidBlock key={codeString} code={codeString} />;
        }

        if (!language) {
          return <MarkdownCodeBlock code={codeString} />;
        }

        return <MarkdownCodeBlock code={codeString} language={language} />;
      },
      code({ className, children, ...rest }) {
        return (
          <code
            className={className || "rounded-md bg-white/10 px-1.5 py-0.5 text-[0.8125rem] font-mono text-accent"}
            {...rest}
          >
            {children}
          </code>
        );
      },
      p({ children }) {
        return <p className="my-2 leading-7">{children}</p>;
      },
      h1({ children }) {
        return <h1 className="mb-3 mt-5 text-lg font-bold text-text-primary">{children}</h1>;
      },
      h2({ children }) {
        return <h2 className="mb-2 mt-4 text-base font-bold text-text-primary">{children}</h2>;
      },
      h3({ children }) {
        return <h3 className="mb-2 mt-3 text-sm font-bold text-text-primary">{children}</h3>;
      },
      ul({ children }) {
        return <ul className="my-2 list-disc space-y-1 pl-5">{children}</ul>;
      },
      ol({ children }) {
        return <ol className="my-2 list-decimal space-y-1 pl-5">{children}</ol>;
      },
      li({ children }) {
        return <li className="leading-7">{children}</li>;
      },
      blockquote({ children }) {
        return (
          <blockquote className="my-3 border-l-3 border-accent/60 pl-4 text-text-muted italic">
            {children}
          </blockquote>
        );
      },
      table({ children }) {
        return (
          <div className="my-3 overflow-x-auto rounded-xl border border-border">
            <table className="w-full text-sm">{children}</table>
          </div>
        );
      },
      thead({ children }) {
        return <thead className="border-b border-border bg-white/5">{children}</thead>;
      },
      th({ children }) {
        return (
          <th className="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wider text-text-muted">
            {children}
          </th>
        );
      },
      td({ children }) {
        return <td className="border-t border-border/50 px-3 py-2">{children}</td>;
      },
      a({ href, children }) {
        return (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-accent underline decoration-accent/40 underline-offset-2 transition-colors hover:text-accent/80"
          >
            {children}
          </a>
        );
      },
      strong({ children }) {
        return <strong className="font-semibold text-text-primary">{children}</strong>;
      },
      hr() {
        return <hr className="my-4 border-border" />;
      },
    }),
    [allowMermaid],
  );

  return (
    <ReactMarkdown remarkPlugins={[remarkGfm, remarkBreaks]} components={components}>
      {normalized}
    </ReactMarkdown>
  );
}
