ALTER TABLE releases
    ADD COLUMN IF NOT EXISTS title_i18n JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS summary_i18n JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS sections_i18n JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS required_actions_i18n JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS known_issues_i18n JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE UNIQUE INDEX IF NOT EXISTS notifications_release_published_once_idx
    ON notifications (recipient_entity_id, ((data->>'release_id')))
    WHERE kind = 'release.published';

UPDATE releases
SET
    title_i18n = jsonb_build_object(
        'en', 'Structured mention assignment and agent adapter updates',
        'zh-CN', '结构化 @ 指派与 Agent 适配器更新'
    ),
    summary_i18n = jsonb_build_object(
        'en', 'This release introduces explicit mention context and assignment semantics across ANI, Web, SDKs, and agent adapters.',
        'zh-CN', '本版本为 ANI、Web、SDK 和 Agent 适配器引入明确的 @ 上下文与任务指派语义。'
    ),
    sections_i18n = jsonb_build_object(
        'en', sections,
        'zh-CN', '[
          {"kind":"new","title":"结构化 @ 指派","items":["消息现在可以区分可见的 @ 上下文与真正需要执行任务的 Agent。","Web 的 @ 芯片可以切换为派活或仅通知。"]},
          {"kind":"improved","title":"Agent 适配器","items":["OpenClaw、Zebra 和 Hermes 适配器会在可用时转发 public mention assignment 元数据。","Python 和 JavaScript SDK 支持 public conversation send 与 assignment 字段。"]},
          {"kind":"fixed","title":"生产发布","items":["生产后端与 Web 已部署新消息字段。","iOS Expo 推送凭据在部署后已修复。"]}
        ]'::jsonb
    ),
    required_actions_i18n = jsonb_build_object(
        'en', required_actions,
        'zh-CN', '[
          {"component":"openclaw_plugin","title":"升级 OpenClaw ANI 插件","body":"运行标准 OpenClaw ANI installer update，并重启 gateway。"},
          {"component":"hermes_adapter","title":"升级 Hermes ANI 适配器","body":"更新 hermes-ani-adapter 仓库，重新安装到 Hermes agent checkout，并重启 Hermes gateway。"},
          {"component":"zebra_adapter","title":"升级 Zebra ANI 适配器","body":"升级 Zebra 到包含 ANI adapter 1.3.0 的版本，并重启 zebra-gateway。"}
        ]'::jsonb
    ),
    known_issues_i18n = jsonb_build_object('en', known_issues, 'zh-CN', '[]'::jsonb)
WHERE version = '2026.5.14';

UPDATE releases
SET
    title_i18n = jsonb_build_object(
        'en', 'Feedback fixes, bot runtime visibility, and recalled message resend improvements',
        'zh-CN', '反馈修复、Bot 运行时可视化与撤回消息重发改进'
    ),
    summary_i18n = jsonb_build_object(
        'en', 'This release summarizes ANI product improvements shipped after 2026.5.14, including Web interaction fixes, backend stability fixes, bot runtime diagnostics, and updated agent adapters.',
        'zh-CN', '本版本汇总 2026.5.14 之后发布的 ANI 产品改进，包括 Web 交互修复、后端稳定性修复、Bot 运行时诊断，以及 Agent 适配器更新。'
    ),
    sections_i18n = jsonb_build_object(
        'en', sections,
        'zh-CN', '[
          {"kind":"new","title":"Bot 运行时可视化","items":["Bot 详情页现在展示 client type、client version、adapter/extension name、adapter/extension version 与活跃 WebSocket 设备诊断。","OpenClaw、Zebra 和 Hermes 集成可以向 ANI 上报运行时版本，方便运维确认数字员工是否运行预期的 adapter 或 extension。"]},
          {"kind":"improved","title":"好友与 Bot 体验","items":["桌面端 Friends 页面改为响应式卡片网格，并支持从好友详情卡分享名片。","Bot 头像弹出卡片操作精简为消息、分享、详情。","Bot 列表排序现在稳定，不再因 presence 刷新而跳动。","Bot 详情页的低频访问策略和运维控制默认折叠，优先展示身份、运行时、好友与会话信息。"]},
          {"kind":"improved","title":"聊天输入与撤回流程","items":["切换会话时会保留文本、@ 提及和已上传附件草稿，只有发送成功后才清空。","自己撤回的消息可以编辑并重新发送，而不是原地恢复旧消息。","撤回提醒已改为 ANI modal，不再使用浏览器原生 alert。","撤回后编辑重发会恢复 mention、assigned mention 元数据，以及已有 URL 的上传附件。"]},
          {"kind":"fixed","title":"反馈与后端修复","items":["好友发现限流从 3/min 放宽到 30/min，登录和注册限流仍保持严格。","重复接受好友请求现在是幂等的，不会产生重复关系状态。","参与者管理和相关群组操作继续迁移到 public UUID，避免暴露内部数字 ID。","HTML 与宽泛文件上传、@ 候选菜单体验、会话草稿保留等反馈已在验收后关闭。"]},
          {"kind":"fixed","title":"Agent 集成发布","items":["Alice 主机的 OpenClaw ANI extension 已通过标准 installer 路径升级到 2026.5.15。","已部署的 Zebra 数字员工主机升级到 ani-platform 1.4.1。","生产日志确认 Zebra 连接上报 adapter_version 1.4.1，Alice OpenClaw 上报 extension_version 2026.5.15。"]}
        ]'::jsonb
    ),
    required_actions_i18n = jsonb_build_object(
        'en', required_actions,
        'zh-CN', '[
          {"component":"openclaw_plugin","title":"升级 OpenClaw ANI extension","body":"运行 npx -y @wzfukui/openclaw-ani-installer update，然后重启或重连 OpenClaw gateway。期望 extension version 为 2026.5.15 或更新。"},
          {"component":"zebra_adapter","title":"升级 Zebra ANI adapter","body":"升级 Zebra 到包含 ani-platform 1.4.1 或更新版本的构建，然后重启 zebra-gateway。"},
          {"component":"hermes_adapter","title":"使用 Hermes 时升级 Hermes ANI adapter","body":"拉取最新 hermes-ani-adapter，重新安装到当前 Hermes checkout，并重启 Hermes gateway，以保持 runtime metadata 与 mention 行为一致。"}
        ]'::jsonb
    ),
    known_issues_i18n = jsonb_build_object(
        'en', known_issues,
        'zh-CN', '[
          "撤回后编辑重发只恢复已有稳定 URL 的上传附件。没有 URL 的附件不会提供编辑重发，避免静默丢失数据。",
          "撤回只改变 ANI 会话展示状态；如果 AI agent 已经开始执行任务，除非该 agent 自己支持 stop 或 interrupt 命令，否则无法中断。",
          "部分移动端客户端可能通过 OTA 时机接收 Web 更新，而不是每次冷启动后立刻更新。"
        ]'::jsonb
    )
WHERE version = '2026.5.16';

WITH target_release AS (
    SELECT id, public_id, version, title, title_i18n, summary_i18n
    FROM releases
    WHERE version = '2026.5.16'
    ORDER BY published_at DESC, id DESC
    LIMIT 1
),
target_users AS (
    SELECT id
    FROM entities
    WHERE entity_type = 'user' AND status = 'active'
)
INSERT INTO notifications (
    recipient_entity_id,
    kind,
    status,
    title,
    body,
    data,
    created_at,
    updated_at
)
SELECT
    target_users.id,
    'release.published',
    'unread',
    'ANI 更新说明 ' || target_release.version,
    target_release.title,
    jsonb_build_object(
        'release_id', target_release.id,
        'release_public_id', target_release.public_id,
        'version', target_release.version,
        'path', '/settings/releases',
        'title_i18n', target_release.title_i18n,
        'body_i18n', target_release.summary_i18n
    ),
    NOW(),
    NOW()
FROM target_release
CROSS JOIN target_users
ON CONFLICT DO NOTHING;
