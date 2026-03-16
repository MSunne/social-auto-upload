import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function PricingPage() {
  return (
    <PagePlaceholder
      eyebrow="Pricing"
      title="充值套餐管理"
      description="定义充值套餐、标准积分、赠送积分、排序与启停状态。"
      apiGroup="/api/admin/v1/packages/*"
      checklist={[
        "套餐 CRUD",
        "赠送积分规则",
        "支付渠道可见性",
        "排序和启停控制",
      ]}
    />
  );
}

