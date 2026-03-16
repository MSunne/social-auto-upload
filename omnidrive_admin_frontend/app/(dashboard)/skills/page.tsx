import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function SkillsPage() {
  return (
    <PagePlaceholder
      eyebrow="Skills"
      title="技能与模型策略"
      description="内部视角下管理技能模板、模型启停、Provider 策略和业务默认项。"
      apiGroup="/api/admin/v1/skills/*"
      checklist={[
        "技能模板与资产列表",
        "模型启停与默认项",
        "Provider 健康状态",
        "被引用关系和影响范围",
      ]}
    />
  );
}

