import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DevicesPage() {
  return (
    <PagePlaceholder
      title="设备管理"
      subtitle="管理 OmniBull 设备、心跳状态、工作负载和强制运维操作。"
      focusPoints={[
        "在线状态与心跳时间",
        "设备认领、禁用、解绑、默认模型",
        "卡死任务释放和同步状态检查"
      ]}
    />
  );
}
