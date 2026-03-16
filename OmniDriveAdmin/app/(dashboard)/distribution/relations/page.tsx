import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DistributionRelationsPage() {
  return (
    <PagePlaceholder
      title="分销关系管理"
      subtitle="维护推广人与被邀请用户的绑定关系、有效状态和冻结控制。"
      focusPoints={[
        "邀请关系列表与详情",
        "生效 / 冻结 / 作废",
        "查看下游用户充值与消费表现"
      ]}
    />
  );
}
