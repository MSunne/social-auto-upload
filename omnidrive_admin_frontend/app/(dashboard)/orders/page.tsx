import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function OrdersPage() {
  return (
    <PagePlaceholder
      eyebrow="Orders"
      title="订单管理"
      description="统一查看支付订单、客服充值单、回调状态、异常单和到账结果。"
      apiGroup="/api/admin/v1/orders/*"
      checklist={[
        "支付宝、微信、客服充值统一订单流",
        "到账结果和异常单识别",
        "渠道筛选和幂等流水查询",
        "关联钱包流水与佣金投影",
      ]}
    />
  );
}

