import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function UsersPage() {
  return (
    <PagePlaceholder
      eyebrow="Users"
      title="用户管理"
      description="查看用户基本信息、钱包状态、设备与任务关联，并支持冻结、禁用、风控备注等内部操作。"
      apiGroup="/api/admin/v1/users/*"
      checklist={[
        "用户列表与多条件筛选",
        "钱包、设备、任务聚合详情",
        "冻结与禁用操作",
        "内部标签、备注、风控历史",
      ]}
    />
  );
}

