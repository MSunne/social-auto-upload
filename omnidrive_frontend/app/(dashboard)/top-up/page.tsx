"use client";

import { PageHeader } from "@/components/ui/common";
import { motion } from "framer-motion";
import { Check, Crown, Zap, Building2 } from "lucide-react";
import { cn } from "@/lib/utils";

const plans = [
  {
    name: "基础版 (Basic)",
    price: "¥99",
    period: "/月",
    features: [
      "每月基计算额度 (1000点)",
      "标准生成模型可用",
      "基础模版流转处理",
    ],
    icon: <Zap className="h-5 w-5" />,
    highlight: false,
  },
  {
    name: "专业版 (Professional)",
    price: "¥299",
    period: "/月",
    features: [
      "高额计算额度 (5000点)",
      "优先生成队列 (无需等待)",
      "全部高级模型访问权",
      "新功能抢先体验",
    ],
    icon: <Crown className="h-5 w-5" />,
    highlight: true,
  },
  {
    name: "企业版 (Enterprise)",
    price: "¥999",
    period: "/月",
    features: [
      "不限额度定额生成",
      "24/7 专属在线支持",
      "自定义工作流集成",
      "独立算力资源预留池",
    ],
    icon: <Building2 className="h-5 w-5" />,
    highlight: false,
  },
];

export default function TopUpPage() {
  return (
    <>
      <PageHeader
        title="充值中心"
        subtitle="选择适合您的业务方案，开启高效 AI 创作运营"
      />

      <div className="mb-8 grid grid-cols-1 gap-6 lg:grid-cols-3">
        {plans.map((plan, i) => (
          <motion.div
            key={plan.name}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: i * 0.1 }}
            className={cn(
              "relative flex flex-col overflow-hidden rounded-2xl border p-6",
              plan.highlight
                ? "border-accent/40 bg-accent/5 shadow-xl shadow-accent/10"
                : "border-border glass-card",
            )}
          >
            {plan.highlight && (
              <div className="absolute right-4 top-4 rounded-full bg-accent px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-background">
                推荐
              </div>
            )}

            <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-xl bg-accent/10 text-accent">
              {plan.icon}
            </div>

            <h3 className="text-lg font-semibold text-text-primary">
              {plan.name}
            </h3>

            <div className="mt-3 flex items-baseline gap-1">
              <span className="text-3xl font-bold text-text-primary">
                {plan.price}
              </span>
              <span className="text-sm text-text-muted">{plan.period}</span>
            </div>

            <ul className="mt-5 flex-1 space-y-3">
              {plan.features.map((f) => (
                <li key={f} className="flex items-start gap-2 text-sm text-text-secondary">
                  <Check className="mt-0.5 h-4 w-4 shrink-0 text-success" />
                  {f}
                </li>
              ))}
            </ul>

            <button
              className={cn(
                "mt-6 w-full rounded-xl py-3 text-sm font-semibold transition-all",
                plan.highlight
                  ? "bg-gradient-to-r from-accent to-cyan text-background shadow-lg shadow-accent/25 hover:shadow-xl hover:shadow-accent/30"
                  : "border border-border bg-surface text-text-primary hover:border-accent/30 hover:text-accent",
              )}
            >
              {plan.highlight ? "立即订阅" : "了解详情"}
            </button>
          </motion.div>
        ))}
      </div>

      {/* Custom amount */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.3 }}
        className="glass-card p-6"
      >
        <h2 className="mb-4 text-base font-semibold text-text-primary">
          自定义金额充值
        </h2>
        <div className="flex flex-wrap items-center gap-3">
          <input
            placeholder="输入充值金额 (最低 ¥10)"
            className="w-64 rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
          />
          {[50, 100, 500, 1000].map((v) => (
            <button
              key={v}
              className="rounded-xl border border-border px-4 py-2.5 text-sm font-medium text-text-secondary transition-all hover:border-accent/30 hover:text-accent"
            >
              ¥{v}
            </button>
          ))}
        </div>
        <div className="mt-4 flex flex-wrap gap-3">
          <button className="rounded-xl bg-info/15 px-5 py-2.5 text-sm font-semibold text-info transition-all hover:bg-info/25">
            支付宝
          </button>
          <button className="rounded-xl bg-success/15 px-5 py-2.5 text-sm font-semibold text-success transition-all hover:bg-success/25">
            微信支付
          </button>
          <button className="rounded-xl bg-text-muted/15 px-5 py-2.5 text-sm font-semibold text-text-secondary transition-all hover:bg-text-muted/25">
            会员卡
          </button>
        </div>
      </motion.div>
    </>
  );
}
