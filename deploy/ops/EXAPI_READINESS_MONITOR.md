# ExAPI readiness monitor / ExAPI readiness 监控

The monitor is a workstation-side notification helper. It is independent of
the production container and does not change the ExAPI deployment. English is
the default operational language; the Chinese summary is below.

## Policy (English)

- Probe `https://sub2api.research.for-immortal.cn/ready` every 30 seconds.
- Require 3 consecutive failed JSON-contract probes before an unhealthy
  incident is confirmed.
- Require 2 consecutive healthy probes before recovery is confirmed. Startup
  health is silent (it cannot create a recovery notification).
- Retry a failed desktop delivery no faster than every 5 minutes. Alert and
  recovery deliveries have independent retry clocks.
- After a delivered incident recovers, suppress a new incident notification
  for 15 minutes. A persistent failure is delivered once the cooldown ends;
  short flaps produce no second alert or recovery notice.
- Reminders are disabled by default (`EXAPI_MONITOR_REMINDER_SECONDS=0`).
- A timer gap over 120 seconds resets only the candidate streak; it does not
  erase a confirmed incident.
- State, proof, and NDJSON events are written atomically with private modes.
  Events are capped at 4 MiB and 12,000 lines. Corrupt state/proof is moved to
  a `.corrupt-*` file before a clean state is started.
- `alert-delivery-evidence.json` intentionally remains schema version 1 for
  the rollout adapters. New monitor metadata is under `monitor_schema_version`
  and event records use schema version 2.

## Install / rollback

Run the isolated tests first:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest -v deploy/ops/test_exapi_readiness_monitor.py
```

Stage the monitor, inspect the generated backup under `tmp/`, then activate:

```bash
deploy/ops/install-exapi-readiness-monitor.sh
deploy/ops/install-exapi-readiness-monitor.sh --activate
```

The installer backs up the prior shell monitor and user units under the
project-local `tmp/offhost-monitor-backup/` directory. To roll back, copy the
desired backup files back to `~/.local/lib/exapi-monitor/` and
`~/.config/systemd/user/`, then run `systemctl --user daemon-reload` and
restart `exapi-readiness-monitor.timer`. Do not delete the state directory;
the Python monitor migrates the existing v1 state and proof in place.

Post-install checks:

```bash
systemctl --user status exapi-readiness-monitor.timer --no-pager
systemctl --user start exapi-readiness-monitor.service
systemctl --user status exapi-readiness-monitor.service --no-pager
jq '{schema_version,status,failure_streak,success_streak}' \\
  ~/.local/state/exapi-readiness-monitor/state.json
```

The service should execute `exapi_readiness_monitor.py`; no production
container or OPC deployment is involved.

## 策略（中文）

- 每 30 秒从工作站检查 readiness JSON 接口。
- 连续 3 次失败才确认故障；连续 2 次成功才确认恢复。启动阶段的成功
  不发送“恢复”通知。
- 通知失败至少间隔 5 分钟重试，故障通知和恢复通知分别计时。
- 已通知的故障恢复后 15 分钟内抑制再次告警；持续故障在冷却结束后只发一次。
  默认关闭长期提醒，因此短暂抖动不会产生告警风暴。
- 监控暂停超过 120 秒只清除候选计数，不会删除已确认的事件。
- 状态、proof 和事件日志使用原子写入及私有权限；日志上限为 4 MiB/12,000
  行，损坏文件会先改名保存。
- proof 保持 schema version 1，以兼容现有 rollout adapter；新增信息放在
  `monitor_schema_version` 和 schema version 2 的事件记录中。

该组件只负责本机通知，不会修改生产 ExAPI 容器。安装前运行隔离测试，安装
脚本会把旧版本备份到项目 `tmp/`，可按上面的步骤回滚。
