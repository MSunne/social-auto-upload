import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function OrdersPage() {
  return (
    <PagePlaceholder
      title="订单管理"
      subtitle="查看充值订单、回调状态、到账情况与异常对账。"
      focusPoints={[
        "支付宝、微信、客服渠道订单",
        "到账积分与幂等对账",
        "异常订单补单与回调重放"
      ]}
    />
  );
}
