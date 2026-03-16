import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function SupportRechargesPage() {
  return (
    <PagePlaceholder
      eyebrow="Support Recharge"
      title="客服充值审核"
      description="这是客服和财务的核心页面，用于审核线下或人工充值申请，并处理额外赠送积分。"
      apiGroup="/api/admin/v1/support-recharges/*"
      checklist={[
        "待审核、已通过、已驳回、已入账队列",
        "充值金额、赠送积分、凭证与备注",
        "审批、驳回、撤销与幂等入账",
        "关联钱包流水和审计记录",
      ]}
    />
  );
}

