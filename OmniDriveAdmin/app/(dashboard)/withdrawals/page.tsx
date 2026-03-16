import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function WithdrawalsPage() {
  return (
    <PagePlaceholder
      title="提现管理"
      subtitle="如果推广人支持提现，这里承接申请审核、打款确认和驳回处理。"
      focusPoints={[
        "提现申请审核",
        "打款状态和凭证",
        "驳回原因和风控检查"
      ]}
    />
  );
}
