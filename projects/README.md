# Projects Workspace

这是当前仓库里几个活跃工程的统一入口，方便快速定位：

- `omnibull_frontend`
  - 指向 `../sau_frontend`
  - 这是当前 `OmniBull / SAU / LocaWeb` 的本地前端工程
- `omnidrive_cloud`
  - 指向 `../omnidrive_cloud`
  - 这是 `OmniDrive` 的 Go 云端后端工程
- `omnidrive_frontend`
  - 指向 `../omnidrive_frontend`
  - 这是 `OmniDrive` 的云端前端工程
- `openclaw_omnibull_extension`
  - 指向 `../openclaw_extensions/omnibull`
  - 这是本地 `OpenClaw` 调用 `OmniBull / SAU` 的插件工程

补充说明：

- 根目录下的 `omnibull_frontend` 也是一个直达别名，方便快速找到 `sau_frontend`
- 当前这次整理是“非破坏式整理”，没有移动正在运行的工程目录
- 如果后面要彻底拆分成多仓或移出 `social-auto-upload` 根目录，建议等当前前后端都停掉之后再做
