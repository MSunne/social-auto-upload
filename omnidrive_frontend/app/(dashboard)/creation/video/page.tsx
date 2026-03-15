"use client";

import { PageHeader, EmptyState } from "@/components/ui/common";
import { Video } from "lucide-react";

export default function VideoCreationPage() {
  return (
    <>
      <PageHeader
        title="视频制作"
        subtitle="使用 AI 大模型生成高质量视频内容，支持多种分辨率"
      />
      <EmptyState
        icon={<Video className="h-6 w-6" />}
        title="视频制作模块开发中"
        description="即将支持 VEO 3.1 Fast 等多模型视频生成。"
      />
    </>
  );
}
