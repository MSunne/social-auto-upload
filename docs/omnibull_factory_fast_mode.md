# OmniBull Factory Fast Mode

这份文档对应“供应商出厂烧录”场景，不再以完整 `Live ISO` 作为主交付物，而是改成：

- 一份供应商恢复包目录
- 一套基础分区归档
- 一个恢复脚本

目标是让供应商在出厂前，把你已经调好的系统和 SAU 设置快速恢复到设备内部磁盘，用户开机即可使用。

## 方案摘要

推荐交付物不是 `.iso`，而是一个目录，例如：

```text
omnibull-factory-bundle-20260330-160000/
├── archives/
│   ├── efi.tar.zst
│   ├── boot.tar.zst
│   ├── root.tar.zst
│   └── persistent.tar.zst
├── meta/
│   ├── capture.env
│   ├── disk-layout.sfdisk
│   ├── partition-map.tsv
│   ├── source-fstab.txt
│   ├── source-findmnt.txt
│   └── source-lsblk.txt
├── provisioning/
│   └── README.txt
├── restore.sh
├── README_FACTORY.md
└── SHA256SUMS
```

供应商拿到 U 盘后，不是“复制文件到系统目录”，而是：

1. 用一个 Linux 维护环境或恢复环境启动机器
2. 把恢复包目录挂载出来
3. 执行 `restore.sh`
4. 把恢复包里的分区内容写回设备内部磁盘
5. 重启交付

## 为什么不用完整 ISO

完整 `Live ISO` 适合安装盘、恢复盘，不适合作为量产主交付物。原因：

- 你当前的 SAU 环境跑在完整 Deepin 系统上，打整机 `Live ISO` 会把大量系统组件、缓存、日志和运行期噪音一起带进去
- 构建慢，体积大
- 对出厂烧录场景来说，供应商真正需要的是“把一套已经调好的系统快速恢复到盘里”

出厂烧录的主交付物更适合：

- 文件级恢复包
- 或块级磁盘镜像

这个仓库当前落地的是前者。

## 当前 Deepin 机器的建议做法

对 `192.168.1.11` 这类 Deepin 机器，推荐采集这几部分：

- EFI 分区
- `/boot`
- 根分区原始内容
- `/persistent`

原因是这类系统往往不是一个简单的“单分区普通 Linux”：

- `/` 是独立根分区
- `/persistent` 保存大量实际改动和用户数据
- `/etc`、`/opt`、`/usr` 可能通过 overlay 和 `/persistent` 组合工作
- `/home`、`/root` 常常来自 `/persistent` 的 bind mount

所以恢复时必须同时恢复：

- `root`
- `persistent`

否则 SAU 的运行配置、用户数据、OpenClaw 插件、前端构建产物、数据库等都会不完整。

## 采集脚本

仓库里新增了：

- `scripts/factory_capture_bundle.sh`

它会：

1. 停掉 `sau-stack.service`
2. 读取源机分区布局
3. 采集 `EFI / boot / root / persistent`
4. 生成压缩归档和元数据
5. 把恢复脚本和说明一起打包成供应商恢复目录
6. 完成后重新启动被停掉的服务

默认输出位置：

- `${HOME}/Desktop/omnibull-factory-bundle-时间戳`

建议：

- 在采集前先关闭浏览器、停止额外桌面应用、确认 SAU 状态稳定
- 尽量减少运行中的缓存写入

## 恢复脚本

仓库里新增了：

- `scripts/factory_restore_bundle.sh`

恢复逻辑：

1. 读取恢复包里的 `disk-layout.sfdisk`
2. 重建目标磁盘分区表
3. 重新格式化 `EFI / boot / root / persistent / swap`
4. 解压各归档到对应分区
5. 按新 UUID 修正 `fstab`
6. 默认删除设备身份文件，让克隆出来的机器首启自动生成新的 `deviceCode / agentKey / localApiKey`
7. 重新安装 `grub`

默认恢复目标：

- `/dev/sda`

可通过环境变量覆盖：

```bash
OMNIBULL_FACTORY_TARGET_DISK=/dev/nvme0n1 ./restore.sh
```

## 设备身份和云端绑定

默认情况下，恢复脚本会清理这些身份信息：

- `/etc/omnibull/device.json`
- 任意 `runtime/device.identity.json`
- `machine-id`

这样同一份母盘恢复到多台机器后：

- 每台机器首次启动会自动生成新的本地设备身份
- 避免所有机器共用同一个 `deviceCode / agentKey`

如果你明确就是要做“完全同一身份复制”，可以在恢复时设置：

```bash
OMNIBULL_FACTORY_PRESERVE_DEVICE_IDENTITY=1 ./restore.sh
```

不建议量产这样做。

## 供应商标准流程

建议给供应商的流程：

1. 准备一个 Linux 维护 U 盘
2. 把 `omnibull-factory-bundle-*` 整个目录拷进去
3. 开机从 U 盘进入维护系统
4. 挂载 U 盘
5. 进入恢复包目录
6. 执行：

```bash
sudo bash ./restore.sh /dev/sda
```

7. 等待完成
8. 关机或重启
9. 用户开机即进入已预装的 SAU 环境

## 体积预估

如果源机当前使用量大致是：

- 根分区已用：`6G - 8G`
- persistent 已用：`15G - 18G`

在清理缓存和日志后，供应商恢复包通常落在：

- `8G - 12G`

具体体积取决于：

- `persistent` 里缓存是否清掉
- Chromium / Playwright 浏览器缓存大小
- `omnidriveSync` 和日志目录大小

## 可选的供应商覆盖文件

恢复包中包含：

- `provisioning/`

可选支持：

- `provisioning/rootfs/`
  - 恢复结束后，这个目录的内容会叠加复制到目标系统
- `provisioning/post-restore.sh`
  - 恢复结束、卸载前，会在目标系统里 chroot 执行

这适合做：

- 客户专属配置
- 设备标签
- 不同批次的默认参数差异

## 适用边界

这个“出厂快速模式”适合：

- 同型号或足够接近的硬件
- 相同启动模式，当前默认按 UEFI 处理
- 相同或更大的目标磁盘

如果供应商设备硬件差异很大，建议改成：

- 一份基础系统镜像
- 一份 SAU 安装脚本
- 一份首启 provisioning 脚本

而不是直接恢复整个运行态 bundle。
