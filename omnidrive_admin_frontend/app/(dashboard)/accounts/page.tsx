import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function AccountsPage() {
  return (
    <PagePlaceholder
      eyebrow="Accounts"
      title="媒体账号管理"
      description="提供媒体账号、认证状态、登录会话与二次验证问题的内部支持视图。"
      apiGroup="/api/admin/v1/accounts/*"
      checklist={[
        "账号列表与状态筛选",
        "登录会话和二次认证记录",
        "所属设备与任务关联",
        "账号异常与支持备注",
      ]}
    />
  );
}

