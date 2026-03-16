import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DevicesPage() {
  return (
    <PagePlaceholder
      eyebrow="Devices"
      title="设备管理"
      description="管理 OmniBull 设备生命周期、运行状态、技能同步、离线异常和卡死任务的处理。"
      apiGroup="/api/admin/v1/devices/*"
      checklist={[
        "设备列表、在线状态、心跳与负载",
        "启用/禁用和解绑控制",
        "同步状态与最近错误",
        "卡死任务强制释放",
      ]}
    />
  );
}

