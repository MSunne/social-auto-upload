import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function AIJobsPage() {
  return (
    <PagePlaceholder
      eyebrow="AI Jobs"
      title="AI 任务管理"
      description="用于查看图片、视频、聊天等 AI 任务的成本、状态、产物和失败恢复。"
      apiGroup="/api/admin/v1/ai/*"
      checklist={[
        "任务列表、模型、来源和状态过滤",
        "输入输出、成本和错误详情",
        "重试、取消和手工修复",
        "产物下载与 S3 状态检查",
      ]}
    />
  );
}

