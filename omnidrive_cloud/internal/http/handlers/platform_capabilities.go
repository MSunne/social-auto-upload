package handlers

import (
	"sort"
	"strings"

	"omnidrive_cloud/internal/domain"
)

const defaultPlatformCapabilitiesRevision = "beta-platform-capabilities-v1"

// 处理strPtr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func strPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// 处理默认设备平台能力相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func defaultDevicePlatformCapabilities() []domain.PlatformCapability {
	return []domain.PlatformCapability{
		{
			PlatformType:   3,
			Slug:           "douyin",
			Label:          "抖音",
			DisplayOrder:   10,
			Visible:        true,
			LoginEnabled:   true,
			PublishEnabled: true,
		},
		{
			PlatformType:   4,
			Slug:           "kuaishou",
			Label:          "快手",
			DisplayOrder:   20,
			Visible:        true,
			LoginEnabled:   true,
			PublishEnabled: true,
		},
		{
			PlatformType:   2,
			Slug:           "wechat_channel",
			Label:          "视频号",
			DisplayOrder:   30,
			Visible:        true,
			LoginEnabled:   true,
			PublishEnabled: true,
		},
		{
			PlatformType:   1,
			Slug:           "xiaohongshu",
			Label:          "小红书",
			DisplayOrder:   40,
			Visible:        true,
			LoginEnabled:   false,
			PublishEnabled: false,
			DisabledReason: strPtr("本期未开放"),
		},
	}
}

// 应用生效设备平台能力，把外部输入转换为当前链路的最终状态变更。
func applyEffectiveDevicePlatformCapabilities(device *domain.Device) {
	if device == nil {
		return
	}
	if len(device.PlatformCapabilities) == 0 {
		device.PlatformCapabilities = defaultDevicePlatformCapabilities()
		if device.PlatformCapabilitiesRevision == nil || strings.TrimSpace(*device.PlatformCapabilitiesRevision) == "" {
			revision := defaultPlatformCapabilitiesRevision
			device.PlatformCapabilitiesRevision = &revision
		}
		return
	}

	sort.SliceStable(device.PlatformCapabilities, func(i, j int) bool {
		if device.PlatformCapabilities[i].DisplayOrder == device.PlatformCapabilities[j].DisplayOrder {
			return device.PlatformCapabilities[i].PlatformType < device.PlatformCapabilities[j].PlatformType
		}
		return device.PlatformCapabilities[i].DisplayOrder < device.PlatformCapabilities[j].DisplayOrder
	})

	if device.PlatformCapabilitiesRevision == nil || strings.TrimSpace(*device.PlatformCapabilitiesRevision) == "" {
		revision := "custom-platform-capabilities"
		device.PlatformCapabilitiesRevision = &revision
	}
}
