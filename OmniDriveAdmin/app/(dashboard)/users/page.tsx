import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function UsersPage() {
  return (
    <PagePlaceholder
      title="用户管理"
      subtitle="查看用户资产、设备、任务、风险标记与操作历史。"
      focusPoints={[
        "用户状态与风控开关",
        "钱包、订单、消费、退款汇总",
        "关联设备、媒体账号、任务、AI 作业"
      ]}
    />
  );
}
