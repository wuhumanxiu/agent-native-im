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
    published_at
) VALUES (
    'd9b49927-87da-4a73-87b2-9a6ad7f89a01',
    '2026.5.16',
    'platform',
    'all',
    'production',
    'Feedback fixes, bot runtime visibility, and recalled message resend improvements',
    'This release summarizes ANI product improvements shipped after 2026.5.14, including Web interaction fixes, backend stability fixes, bot runtime diagnostics, and updated agent adapters.',
    '[
      {"kind":"new","title":"Bot runtime visibility","items":["Bot details now show runtime integration information including client type, client version, adapter or extension name, adapter or extension version, and active WebSocket device diagnostics.","OpenClaw, Zebra, and Hermes integrations can now report their runtime versions to ANI so operators can verify whether a digital employee is running the expected adapter or extension."]},
      {"kind":"improved","title":"Friends and bot experience","items":["The desktop Friends page now uses a responsive card grid and supports profile-card sharing from friend detail cards.","Bot avatar popover actions were shortened to compact labels for message, share, and details.","Bot list ordering is now stable and no longer jumps during presence refresh.","Low-frequency Bot detail sections, including access policy and operations controls, are collapsed by default so high-frequency identity, runtime, friends, and conversation information is visible first."]},
      {"kind":"improved","title":"Chat composer and recall flow","items":["Conversation drafts now persist across conversation switching for text, mentions, and uploaded attachments, and clear only after a successful send.","Self-recalled messages can be edited and resent without restoring the old message in place.","The recall warning now uses the ANI modal component instead of a browser-native alert.","Recalled edit/resend now restores mention IDs, assigned mention metadata, and already-uploaded attachments with URLs."]},
      {"kind":"fixed","title":"Feedback and backend fixes","items":["Friend discovery rate limits were relaxed from 3/min to 30/min while login and registration limits remain strict.","Repeated friend-request accept is now idempotent and no longer creates duplicate friendship state.","Participant management and related group operations continue moving to public UUID identifiers instead of exposing internal numeric IDs.","HTML and broad file upload support, mention candidate menu behavior, and conversation draft retention feedback were closed after verification."]},
      {"kind":"fixed","title":"Agent integration rollout","items":["OpenClaw ANI extension was upgraded through the standard installer path to 2026.5.15 on the Alice host.","Zebra ANI adapter was upgraded to ani-platform 1.4.1 on the deployed Zebra digital employee hosts.","Production logs confirmed upgraded Zebra connections report adapter_version 1.4.1 and Alice OpenClaw reports extension_version 2026.5.15."]}
    ]'::jsonb,
    '[
      {"component":"openclaw_plugin","title":"Upgrade OpenClaw ANI extension","body":"Run npx -y @wuhumanxiu/openclaw-ani-installer update, then restart or reconnect the OpenClaw gateway. Expected extension version: 2026.5.15 or newer."},
      {"component":"zebra_adapter","title":"Upgrade Zebra ANI adapter","body":"Upgrade Zebra to a build that includes ani-platform 1.4.1 or newer, then restart zebra-gateway."},
      {"component":"hermes_adapter","title":"Upgrade Hermes ANI adapter when using Hermes","body":"Pull the latest hermes-ani-adapter, reinstall it into the active Hermes checkout, and restart the Hermes gateway so runtime metadata and mention behavior stay aligned."}
    ]'::jsonb,
    '[
      "Recall edit/resend restores uploaded attachments that already have stable URLs. Attachments without URLs are intentionally not offered for edit/resend to avoid silent data loss.",
      "Recall only changes ANI conversation display state; it cannot interrupt an AI agent task that has already started unless that agent provides its own stop or interrupt command.",
      "Some mobile clients may receive Web changes through OTA timing rather than immediately on cold start."
    ]'::jsonb,
    '2026-05-16T09:05:00Z'
) ON CONFLICT (public_id) DO NOTHING;
