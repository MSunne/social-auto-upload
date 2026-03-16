import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function AdminsPage() {
  return (
    <PagePlaceholder
      title="管理员与角色"
      subtitle="用于管理后台账号、角色、权限矩阵和会话安全。"
      focusPoints={[
        "管理员账号管理",
        "角色和权限矩阵",
        "会话失效与安全控制"
      ]}
    />
  );
}
