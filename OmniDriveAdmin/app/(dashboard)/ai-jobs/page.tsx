import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function AIJobsPage() {
  return (
    <PagePlaceholder
      title="AI 作业管理"
      subtitle="面向运营和技术支持，查看 AI 任务、模型消耗、产物与异常。"
      focusPoints={[
        "模型、来源、状态、费用",
        "失败重试和错误详情",
        "产物文件与 S3 状态"
      ]}
    />
  );
}
