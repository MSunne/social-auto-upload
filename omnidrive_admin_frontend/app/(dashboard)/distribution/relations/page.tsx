import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DistributionRelationsPage() {
  return (
    <PagePlaceholder
      eyebrow="Distribution"
      title="分销关系管理"
      description="管理推广人、邀请关系和绑定状态，支撑后续佣金生命周期计算。"
      apiGroup="/api/admin/v1/distribution/relations/*"
      checklist={[
        "推广人列表和状态",
        "邀请关系与绑定时间",
        "冻结和失效控制",
        "关系链路追溯",
      ]}
    />
  );
}

