import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function PublishTasksPage() {
  return (
    <PagePlaceholder
      title="发布任务管理"
      subtitle="查看全局发布任务、人工校验证据、失败原因与运维动作。"
      focusPoints={[
        "状态、平台、用户、设备多维筛选",
        "失败、待验证、人工结单",
        "截图、日志、素材、技能版本漂移"
      ]}
    />
  );
}
