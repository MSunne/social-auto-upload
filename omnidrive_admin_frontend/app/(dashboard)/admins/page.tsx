import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function AdminsPage() {
  return (
    <PagePlaceholder
      eyebrow="Admins"
      title="管理员与角色"
      description="维护内部管理员、角色绑定和权限授权。"
      apiGroup="/api/admin/v1/admins/*"
      checklist={[
        "管理员账号列表",
        "角色授权和权限边界",
        "启停与状态管理",
        "最近登录和安全事件",
      ]}
    />
  );
}
