WITH inserted_release AS (
    INSERT INTO releases (
        public_id,
        version,
        component,
        platform,
        channel,
        title,
        summary,
        sections,
        required_actions,
        known_issues,
        title_i18n,
        summary_i18n,
        sections_i18n,
        required_actions_i18n,
        known_issues_i18n,
        published_at
    ) VALUES (
        '40c8b557-24cc-4e51-acb7-daf1aae92967',
        '2026.5.17',
        'platform',
        'all',
        'production',
        'Conversation multi-select and forwarding',
        'This release adds conversation message multi-select, merged forwarding cards, per-message forwarding, and clearer forwarding previews across ANI Web and mobile clients.',
        '[
          {"kind":"new","title":"Conversation forwarding","items":["Web and mobile clients now support selecting multiple conversation messages and forwarding them to another conversation.","Users can choose merged forwarding or per-message forwarding, and can add extra text in the forwarding dialog before sending."]},
          {"kind":"new","title":"Merged chat record cards","items":["Merged forwarding now sends a structured ANI chat record card instead of plain copied text.","Opening the card shows the forwarded conversation history with sender avatars and left/right message layout for easier review."]},
          {"kind":"improved","title":"Forwarding dialog clarity","items":["The forwarding dialog now has stronger selected-state contrast and a clearer message preview list.","Per-message forwarding previews include dividers so each forwarded item is visually separated."]},
          {"kind":"improved","title":"Mention support while forwarding","items":["Additional text entered in the forwarding dialog can include @ mentions, which are resolved for the target conversation where supported."]},
          {"kind":"fixed","title":"Verified rollout","items":["The forwarding experience was validated on Web and mobile preview builds before this production release note was published."]}
        ]'::jsonb,
        '[]'::jsonb,
        '[
          "Some mobile clients may need to reopen the app or receive the latest OTA update before the forwarding UI appears.",
          "Merged forwarding preserves the selected message content and display metadata, but it does not create a live synchronized copy of the original conversation."
        ]'::jsonb,
        jsonb_build_object(
            'en', 'Conversation multi-select and forwarding',
            'zh-CN', '会话多选与转发'
        ),
        jsonb_build_object(
            'en', 'This release adds conversation message multi-select, merged forwarding cards, per-message forwarding, and clearer forwarding previews across ANI Web and mobile clients.',
            'zh-CN', '本版本为 ANI Web 和移动端新增会话消息多选、合并转发卡片、逐条转发，以及更清晰的转发预览。'
        ),
        jsonb_build_object(
            'en', '[
              {"kind":"new","title":"Conversation forwarding","items":["Web and mobile clients now support selecting multiple conversation messages and forwarding them to another conversation.","Users can choose merged forwarding or per-message forwarding, and can add extra text in the forwarding dialog before sending."]},
              {"kind":"new","title":"Merged chat record cards","items":["Merged forwarding now sends a structured ANI chat record card instead of plain copied text.","Opening the card shows the forwarded conversation history with sender avatars and left/right message layout for easier review."]},
              {"kind":"improved","title":"Forwarding dialog clarity","items":["The forwarding dialog now has stronger selected-state contrast and a clearer message preview list.","Per-message forwarding previews include dividers so each forwarded item is visually separated."]},
              {"kind":"improved","title":"Mention support while forwarding","items":["Additional text entered in the forwarding dialog can include @ mentions, which are resolved for the target conversation where supported."]},
              {"kind":"fixed","title":"Verified rollout","items":["The forwarding experience was validated on Web and mobile preview builds before this production release note was published."]}
            ]'::jsonb,
            'zh-CN', '[
              {"kind":"new","title":"会话消息转发","items":["Web 和移动端现在支持在会话中多选消息，并转发到其他会话。","转发时可以选择合并转发或逐条转发，并可在转发对话框中追加文字后再发送。"]},
              {"kind":"new","title":"合并聊天记录卡片","items":["合并转发现在发送结构化的 ANI 聊天记录卡片，不再只是复制为纯文本。","点击卡片可查看被转发的聊天历史，并按发送者头像与左右消息方向展示，方便回看上下文。"]},
              {"kind":"improved","title":"转发对话框可读性","items":["转发对话框强化了选中态颜色和对比度，并提供更清楚的消息预览列表。","逐条转发预览增加分割线，每条被转发消息更容易区分。"]},
              {"kind":"improved","title":"转发时支持 @ 提及","items":["在转发对话框追加的文字可以包含 @ 提及；在目标会话支持时会解析为对应 mention。"]},
              {"kind":"fixed","title":"已验证发布","items":["本次转发体验已在 Web 与移动端 preview 构建中验证通过后发布生产更新说明。"]}
            ]'::jsonb
        ),
        jsonb_build_object('en', '[]'::jsonb, 'zh-CN', '[]'::jsonb),
        jsonb_build_object(
            'en', '[
              "Some mobile clients may need to reopen the app or receive the latest OTA update before the forwarding UI appears.",
              "Merged forwarding preserves the selected message content and display metadata, but it does not create a live synchronized copy of the original conversation."
            ]'::jsonb,
            'zh-CN', '[
              "部分移动端客户端可能需要重新打开 App，或等待收到最新 OTA 更新后才会看到转发入口。",
              "合并转发会保留所选消息内容和展示元数据，但不会创建与原会话实时同步的副本。"
            ]'::jsonb
        ),
        '2026-05-17T10:00:00Z'
    )
    ON CONFLICT (public_id) DO UPDATE SET
        version = EXCLUDED.version,
        component = EXCLUDED.component,
        platform = EXCLUDED.platform,
        channel = EXCLUDED.channel,
        title = EXCLUDED.title,
        summary = EXCLUDED.summary,
        sections = EXCLUDED.sections,
        required_actions = EXCLUDED.required_actions,
        known_issues = EXCLUDED.known_issues,
        title_i18n = EXCLUDED.title_i18n,
        summary_i18n = EXCLUDED.summary_i18n,
        sections_i18n = EXCLUDED.sections_i18n,
        required_actions_i18n = EXCLUDED.required_actions_i18n,
        known_issues_i18n = EXCLUDED.known_issues_i18n,
        published_at = EXCLUDED.published_at
    RETURNING id, public_id, version, title, title_i18n, summary_i18n
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
    'ANI 更新说明 ' || inserted_release.version,
    inserted_release.title,
    jsonb_build_object(
        'release_id', inserted_release.id,
        'release_public_id', inserted_release.public_id,
        'version', inserted_release.version,
        'path', '/settings/releases',
        'title_i18n', inserted_release.title_i18n,
        'body_i18n', inserted_release.summary_i18n
    ),
    NOW(),
    NOW()
FROM inserted_release
CROSS JOIN target_users
ON CONFLICT DO NOTHING;
