"use client";

import { useDashboardSummary } from "@/lib/hooks/useDashboard";
import { MetricTile } from "@/components/ui/common";
import { 
  BarChart3, 
  Coins, 
  ReceiptText, 
  Users, 
  MonitorSmartphone,
  Wallet
} from "lucide-react";
import { useMemo } from "react";

export function OverviewCards() {
  const { data, isLoading, error } = useDashboardSummary();

  const metrics = useMemo(() => {
    if (!data) return null;
    return [
      {
        label: "总用户数 / 活跃",
        value: `${data.metrics.userCount} / ${data.metrics.activeUserCount}`,
        hint: "平台注册用户及近期活跃状态",
        icon: <Users className="h-5 w-5" />
      },
      {
        label: "管控设备 (在线)",
        value: `${data.metrics.deviceCount} (${data.metrics.onlineDeviceCount})`,
        hint: "当前绑定的真实手机设备数量",
        icon: <MonitorSmartphone className="h-5 w-5" />
      },
      {
        label: "客服充值待审核",
        value: data.finance.pendingSupportRechargeCount.toString(),
        hint: "需要财务人工核对转账凭证的请求",
        icon: <ReceiptText className="h-5 w-5" />
      },
      {
        label: "待结算佣金",
        value: `¥ ${(data.distribution.pendingSettlementAmountCents / 100).toFixed(2)}`,
        hint: "按消费释放后进入待结算池的金额",
        icon: <Coins className="h-5 w-5" />
      },
      {
        label: "异常任务/AI队列",
        value: `${data.metrics.failedPublishTaskCount} / ${data.queues.pendingAiJobCount}`,
        hint: `发布失败:${data.metrics.failedPublishTaskCount} | AI排队:${data.queues.pendingAiJobCount}`,
        icon: <BarChart3 className="h-5 w-5" />
      },
      {
        label: "财务入账总额",
        value: `¥ ${(data.finance.rechargeAmountCents / 100).toFixed(2)}`,
        hint: "包含支付宝与线下打款等所有渠道",
        icon: <Wallet className="h-5 w-5" />
      }
    ];
  }, [data]);

  if (isLoading) {
    return (
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="h-[120px] rounded-xl bg-[var(--color-bg-secondary)] border border-[var(--color-border)] animate-pulse" />
        ))}
      </section>
    );
  }

  if (error || !metrics) {
    return (
      <div className="p-4 rounded-xl bg-red-500/10 border border-red-500/20 text-red-500 text-sm">
        获取仪表盘数据失败，请检查网络或刷新重试。
      </div>
    );
  }

  return (
    <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {metrics.map((metric, idx) => (
        <MetricTile 
          key={idx}
          label={metric.label}
          value={metric.value}
          hint={metric.hint}
          icon={metric.icon}
        />
      ))}
    </section>
  );
}
