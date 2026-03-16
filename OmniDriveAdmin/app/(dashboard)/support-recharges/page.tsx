import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function SupportRechargesPage() {
  return (
    <PagePlaceholder
      title="客服充值审核"
      subtitle="后台重点模块，用于审核客服充值、赠送积分和入账操作。"
      focusPoints={[
        "审核通过 / 驳回 / 已入账状态流",
        "充值凭证和审核备注",
        "赠送积分和幂等入账"
      ]}
    />
  );
}
