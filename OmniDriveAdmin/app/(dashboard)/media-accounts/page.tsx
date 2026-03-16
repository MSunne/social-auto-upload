import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function MediaAccountsPage() {
  return (
    <PagePlaceholder
      title="媒体账号管理"
      subtitle="聚合所有用户的媒体账号、认证状态、登录会话与异常信息。"
      focusPoints={[
        "账号有效性与最近认证时间",
        "远程扫码登录记录",
        "账号关联任务和失败原因"
      ]}
    />
  );
}
