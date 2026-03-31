# OmniBull 供应商出厂恢复指南

这份文档给供应商使用，目标只有一个：

- 在出厂前，把已经准备好的 OmniBull / SAU 系统恢复到设备内部磁盘
- 设备交付给最终用户后，开机即可进入预装好的系统和 SAU 环境

## 当前交付物

供应商会拿到一只已经做好的恢复 U 盘。

这只 U 盘有两部分：

- 启动区：Deepin Live 恢复环境
- 数据区：`OMNIFACTORY`

启动菜单默认第一项是：

- `OmniBull Factory Restore`

数据区根目录包含：

- `START_HERE.txt`
- `omnibull-factory-bundle-时间戳/`

恢复包目录内包含：

- `restore.sh`
- `README_FACTORY.md`
- `SHA256SUMS`
- `archives/`
- `meta/`
- `provisioning/`

## 供应商标准流程

1. 把恢复 U 盘插入待交付设备。
2. 开机进入 BIOS / UEFI 启动菜单。
3. 选择从这只 U 盘启动。
4. 启动后保持默认菜单项 `OmniBull Factory Restore`，不要进入 Deepin 安装项。
5. 恢复环境启动后，会自动查找数据区标签 `OMNIFACTORY`。
6. 程序会列出可恢复的目标磁盘。
7. 确认目标磁盘是设备内部磁盘，不是恢复 U 盘本身。
8. 在确认提示里输入 `RESTORE`。
9. 等待恢复完成。
10. 按提示重启设备，必要时拔掉 U 盘。

## 操作员判断标准

正常情况下，操作员会看到：

- 标题 `OmniBull Factory Recovery`
- 自动识别到 bundle 分区
- 自动列出候选目标盘，例如 `/dev/sda`、`/dev/nvme0n1`
- 明确提示“将清空目标磁盘”
- 需要手工输入 `RESTORE`

如果界面里没有看到这些，而是看到 Deepin 桌面或安装器界面，说明没有进入正确菜单项，应重启后重新选择：

- `OmniBull Factory Restore`

## 重要注意事项

- 不要把恢复目标选成 U 盘本身。
- 不要进入 `Install Deepin` 安装流程。
- 恢复过程中不要断电。
- 目标磁盘容量必须不小于母盘所要求的容量。
- 恢复完成后，第一次开机时间可能比平时略长，这是正常现象。

## 恢复完成后的验收点

设备重启后，应满足：

- 能正常进入预装系统
- SAU / OmniBull 环境已存在
- 本地服务按预设自动启动
- 无需最终用户手工安装系统

## 失败时的处理

如果自动恢复流程没有找到 bundle 分区，先检查：

- U 盘是否完整插好
- 启动时是否从正确的恢复 U 盘进入
- 数据分区标签是否仍为 `OMNIFACTORY`

如果自动流程没有执行，但已经进入 Linux 恢复环境，可手工执行：

```bash
sudo mkdir -p /mnt/omnifactory
sudo mount /dev/sdb3 /mnt/omnifactory
cd /mnt/omnifactory/omnibull-factory-bundle-*/
sudo bash ./restore.sh /dev/sda
```

如果目标盘不是 `/dev/sda`，改成实际设备路径，例如：

```bash
sudo bash ./restore.sh /dev/nvme0n1
```

## 供应商不需要做的事

- 不需要安装 Windows PE
- 不需要进入 Deepin 安装器
- 不需要手工解压归档文件
- 不需要手工分区
- 不需要手工安装 SAU

## 内部备注

当前恢复盘设计是：

- 启动区使用 Deepin Live 作为底座
- 数据区使用 `exfat`
- 自动恢复逻辑在启动后的恢复环境里执行

对供应商来说，只需要按“启动恢复盘 -> 选择目标盘 -> 输入 `RESTORE` -> 等待完成”执行即可。
