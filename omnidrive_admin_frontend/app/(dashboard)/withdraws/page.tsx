import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function WithdrawsPage() {
  return (
    <PagePlaceholder
      eyebrow="Withdrawals"
      title="提现管理"
      description="用于处理推广人的提现申请、审核状态、打款记录和驳回原因。"
      apiGroup="/api/admin/v1/withdrawals/*"
      checklist={[
        "提现申请队列",
        "审核通过与驳回",
        "打款凭证和人工确认",
        "关联结算与佣金明细",
      ]}
    />
  );
}

