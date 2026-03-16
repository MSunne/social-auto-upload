import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function SettingsPage() {
  return (
    <PagePlaceholder
      eyebrow="Settings"
      title="系统配置"
      description="集中管理业务规则、默认模型、返佣比例、支付开关和风控阈值。"
      apiGroup="/api/admin/v1/settings/*"
      checklist={[
        "默认模型和 Provider 配置",
        "返佣和提现规则",
        "支付渠道开关",
        "高风险操作保护阈值",
      ]}
    />
  );
}

