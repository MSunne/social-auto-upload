"use client";

import { PageHeader, EmptyState } from "@/components/ui/common";
import { Wallet, TrendingUp, TrendingDown, DollarSign } from "lucide-react";
import { StatCard } from "@/components/ui/common";
import { motion } from "framer-motion";

export default function FinancePage() {
  return (
    <>
      <PageHeader
        title="财务管理"
        subtitle="实时监控您的账户余额与消费支出流水"
      />

      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          label="总余额"
          value="¥12,840.00"
          change="积分余额充足"
          changeType="positive"
          icon={<DollarSign className="h-5 w-5" />}
        />
        <StatCard
          label="本月消费"
          value="¥1,250.00"
          change="较上月 +8.5%"
          changeType="negative"
          icon={<TrendingDown className="h-5 w-5" />}
        />
        <StatCard
          label="累计消费"
          value="¥45,600.00"
          change="自注册以来"
          changeType="neutral"
          icon={<TrendingUp className="h-5 w-5" />}
        />
      </div>

      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.1 }}
        className="glass-card p-6"
      >
        <h2 className="mb-4 text-base font-semibold text-text-primary">
          财务流水明细
        </h2>
        <EmptyState
          icon={<Wallet className="h-6 w-6" />}
          title="流水明细连接后端后自动加载"
          description="后端接口就绪后将展示完整的消费充值流水记录。"
        />
      </motion.div>
    </>
  );
}
