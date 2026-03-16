import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function PricingPage() {
  return (
    <PagePlaceholder
      title="充值套餐管理"
      subtitle="维护套餐价格、标准积分、赠送积分、渠道和显示排序。"
      focusPoints={[
        "套餐启停和排序",
        "标准积分与赠送积分",
        "支付渠道可用性"
      ]}
    />
  );
}
