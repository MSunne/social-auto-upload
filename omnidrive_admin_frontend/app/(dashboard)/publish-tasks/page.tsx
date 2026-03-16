import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function PublishTasksPage() {
  return (
    <PagePlaceholder
      eyebrow="Publish Tasks"
      title="发布任务管理"
      description="这里会承载内部任务干预工作台，包括失败重试、人工结单、证据查看和 readiness 问题排查。"
      apiGroup="/api/admin/v1/publish-tasks/*"
      checklist={[
        "任务列表、状态和来源过滤",
        "详情抽屉与 artifact 证据",
        "retry / cancel / manual resolve",
        "账号、素材、技能漂移排查",
      ]}
    />
  );
}

