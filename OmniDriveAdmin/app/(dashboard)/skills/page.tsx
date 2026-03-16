import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function SkillsPage() {
  return (
    <PagePlaceholder
      title="技能与模型治理"
      subtitle="用于管理技能资产、模型启停、供应商健康状态与风险内容。"
      focusPoints={[
        "用户技能查看与停用",
        "模型注册表和默认模型",
        "Provider 配置与健康度"
      ]}
    />
  );
}
