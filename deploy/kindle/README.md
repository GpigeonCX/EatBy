# Kindle 看板

适配已越狱 Kindle 8（Basic 2 / Heisenberg，固件 5.16.2.1.1，600×800）。
服务端生成四栏灰度 PNG；设备使用 eips 绘图、现有 FBInk 清残影，不替换系统 FBInk。

## 日常使用

- KUAL → `EatBy: pair device`：手机设置页输入设备显示的八位短码，十分钟有效。
- 手机 Kindle 设置：时间段、间隔、横竖屏、是否显示更新时间/采样电量。
- `START battery dashboard`：立即更新一次，随后按服务端时间表休眠/唤醒。
- `STOP battery dashboard`：先正常唤醒、进入 KUAL，停止后等恢复提示，再连接 USB。
- 临时断网且有可用缓存：保留旧看板，深睡 30 分钟后重试；恢复联网后重新遵循服务端计划。
  离线重试可能发生在正常刷新时段之外；离线期间无法得知服务端新设置。
- 凭据失效、图片损坏或离线无缓存仍退出；重启设备后需手工 START，没有开机自启。

默认：上海时区，工作日 18:00–24:00，周末 09:00–24:00，每 30 分钟；午夜不刷新。
更改设置在设备下次连接时生效，不能立即推送到休眠设备。显式 START 会在时段外刷新一次。

## 设备文件与安装

仓库脚本位于本目录，设备运行目录是 `/mnt/us/eatby/`；KUAL 的 menu.json/config.xml
放在 `/mnt/us/extensions/eatby/`。macOS USB 对应挂载路径通常是 `/Volumes/Kindle/`。
首次安装需自行具备越狱/KUAL、curl、timeout、eips 以及 `/mnt/us/libkh/bin/fbink`。
不要把该实验设备的脚本直接用于其他型号，也不要为安装修改系统根分区。

普通脚本升级须先 STOP 并确认恢复后复制。本次显示优化另提供
`APPLY offline recovery update`：从 `eatby/display-update/` 的 wake-test.sh、refresh.sh、
battery-agent.sh 及 SHA256SUMS 校验并安装，先停止当前 agent，保存旧脚本到 backup-network-v2；
此前运行则自动重启，此前停止则保持停止。该入口需事先放好更新包，不会从云端下载。

`image-url.txt`、`display-link.txt` 是只读设备凭据，不能提交或分享；撤销设备在手机端操作。
不要同步设备 cache/logs 或生产数据库到仓库。

## 当前验证边界

- 已验证短码绑定、实际 PNG 下载显示、状态栏遮盖处理、真实深睡及 RTC1 唤醒。
- 用户已观察连续两轮自动更新；尚无多日稳定性或续航测量。
- 显示优化：请求休眠后约两秒重画缓存，唤醒后先画缓存再联网；保留准备休眠时的兜底重画。
  日志确认此前优化已应用，且多轮深睡/缓存重画成功；不保证原生切换瞬间完全无闪屏。
- 2026-10-21 日志确认 DNS 失败后旧 agent 退出，是恢复系统屏保的明确原因之一。
  新增断网缓存休眠恢复逻辑已模拟测试，暂存设备待安装、实测。
- 保持 framework 运行，只暂时停止原本运行的 statusbar；退出由独立 guard 恢复。
- RTC1 用自身时钟而非 Unix 时间；只清理自己设置的闹钟，不修改系统时钟或休眠抑制设置。
- 间隔不足 150 秒时等待而不休眠；频繁短间隔测试不代表正常半小时计划的续航。

## 测试与排障

```bash
python3 -m unittest discover -s deploy/kindle -p 'test_*.py' -v
```

当前 36 项模拟测试；它们不能替代深睡和功耗实测。
停止后检查设备 eatby/logs/ 中的 agent.txt、agent-guard.txt、agent-wake.txt、
agent-wake.previous.txt、refresh.txt。若日志不全，KUAL → `inspect last cycle (read only)`。
日志先写 RAM，重启可能丢失；导出的文件会保留。不要在不了解用途时重复运行诊断性休眠实验。

详细实验记录保留在 [HISTORY.md](HISTORY.md)，其中旧章节的“未实现/待验证”是当时状态，
不代表当前状态。业务发布流程见 [DEPLOYMENT.md](../DEPLOYMENT.md)。
