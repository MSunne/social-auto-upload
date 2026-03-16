import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function SettingsPage() {
  return (
    <PagePlaceholder
      title="系统配置"
      subtitle="管理模型默认值、充值赠送、返佣规则、风控阈值和支付开关。"
      focusPoints={[
        "充值与赠送配置",
        "返佣比例和结算规则",
        "模型与 Provider 开关"
      ]}
    />
  );
}
