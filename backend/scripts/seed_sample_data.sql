-- Seed script for Nova Simulators
-- Creates a sample session with mock data for Slack and Gmail simulators
--
-- Usage:
--   sqlite3 simulators.db < scripts/seed_sample_data.sql
--
-- Or using make (if added to Makefile):
--   make seed
--
-- Note: This script is idempotent - it will delete and recreate demo session data

-- Clean up any existing demo session data
DELETE FROM slack_messages WHERE session_id = 'demo-session-001';
DELETE FROM slack_files WHERE session_id = 'demo-session-001';
DELETE FROM slack_users WHERE session_id = 'demo-session-001';
DELETE FROM slack_channels WHERE session_id = 'demo-session-001';
DELETE FROM gmail_messages WHERE session_id = 'demo-session-001';
DELETE FROM sessions WHERE id = 'demo-session-001';

-- Also clean up legacy default channels/users with empty session_id (from old migrations)
DELETE FROM slack_channels WHERE session_id = '';
DELETE FROM slack_users WHERE session_id = '';

-- Create a sample session
INSERT INTO sessions (id, created_at, last_accessed) VALUES
    ('demo-session-001', unixepoch(), unixepoch());

-- ============================================================================
-- SLACK SIMULATOR DATA
-- ============================================================================

-- Insert Slack Channels
INSERT INTO slack_channels (id, name, created_at, session_id) VALUES
    ('C001', 'general', unixepoch() - 86400 * 30, 'demo-session-001'),
    ('C002', 'random', unixepoch() - 86400 * 25, 'demo-session-001'),
    ('C003', 'engineering', unixepoch() - 86400 * 20, 'demo-session-001'),
    ('C004', 'product', unixepoch() - 86400 * 15, 'demo-session-001'),
    ('C005', 'support', unixepoch() - 86400 * 10, 'demo-session-001');

-- Insert Slack Users
INSERT INTO slack_users (
    id, team_id, name, real_name, email, display_name,
    first_name, last_name, is_admin, is_owner, is_bot,
    timezone, timezone_label, timezone_offset,
    created_at, session_id
) VALUES
    (
        'U001', 'T021F9ZE2', 'alice', 'Alice Johnson', 'alice@example.com', 'alice',
        'Alice', 'Johnson', 1, 1, 0,
        'America/Los_Angeles', 'Pacific Standard Time', -28800,
        unixepoch() - 86400 * 30, 'demo-session-001'
    ),
    (
        'U002', 'T021F9ZE2', 'bob', 'Bob Smith', 'bob@example.com', 'bob',
        'Bob', 'Smith', 1, 0, 0,
        'America/New_York', 'Eastern Standard Time', -18000,
        unixepoch() - 86400 * 28, 'demo-session-001'
    ),
    (
        'U003', 'T021F9ZE2', 'charlie', 'Charlie Davis', 'charlie@example.com', 'charlie',
        'Charlie', 'Davis', 0, 0, 0,
        'America/Chicago', 'Central Standard Time', -21600,
        unixepoch() - 86400 * 25, 'demo-session-001'
    ),
    (
        'U004', 'T021F9ZE2', 'diana', 'Diana Prince', 'diana@example.com', 'diana',
        'Diana', 'Prince', 0, 0, 0,
        'Europe/London', 'Greenwich Mean Time', 0,
        unixepoch() - 86400 * 20, 'demo-session-001'
    ),
    (
        'U005', 'T021F9ZE2', 'slackbot', 'Slackbot', 'slackbot@example.com', 'slackbot',
        'Slack', 'Bot', 0, 0, 1,
        'America/Los_Angeles', 'Pacific Standard Time', -28800,
        unixepoch() - 86400 * 30, 'demo-session-001'
    );

-- Insert Slack Messages (realistic conversation)
INSERT INTO slack_messages (channel_id, type, user_id, text, timestamp, created_at, session_id) VALUES
    -- General channel
    ('C001', 'message', 'U001', 'Good morning everyone! Hope you all had a great weekend.', '1704110400.000100', unixepoch() - 86400 * 7, 'demo-session-001'),
    ('C001', 'message', 'U002', 'Morning Alice! Yes, it was relaxing. Ready for the week ahead.', '1704110460.000200', unixepoch() - 86400 * 7, 'demo-session-001'),
    ('C001', 'message', 'U003', 'Hey team! Just a reminder - our all-hands meeting is at 2 PM today.', '1704110520.000300', unixepoch() - 86400 * 7, 'demo-session-001'),
    ('C001', 'message', 'U004', 'Thanks for the reminder Charlie!', '1704110580.000400', unixepoch() - 86400 * 7, 'demo-session-001'),
    ('C001', 'message', 'U001', 'I''ll send out the agenda shortly.', '1704110640.000500', unixepoch() - 86400 * 7, 'demo-session-001'),

    -- Engineering channel
    ('C003', 'message', 'U002', 'Starting work on the new API endpoints today. Created ticket ENG-123.', '1704196800.000100', unixepoch() - 86400 * 6, 'demo-session-001'),
    ('C003', 'message', 'U003', 'Great! I can help with the database migrations if needed.', '1704196860.000200', unixepoch() - 86400 * 6, 'demo-session-001'),
    ('C003', 'message', 'U002', 'That would be awesome, thanks Charlie! I''ll push the initial schema soon.', '1704196920.000300', unixepoch() - 86400 * 6, 'demo-session-001'),
    ('C003', 'message', 'U004', 'Can we also add monitoring for these endpoints?', '1704196980.000400', unixepoch() - 86400 * 6, 'demo-session-001'),
    ('C003', 'message', 'U002', 'Absolutely, I''ll include Datadog metrics in the implementation.', '1704197040.000500', unixepoch() - 86400 * 6, 'demo-session-001'),

    -- Product channel
    ('C004', 'message', 'U001', 'Quick update: We''ve received great feedback on the dashboard redesign!', '1704283200.000100', unixepoch() - 86400 * 5, 'demo-session-001'),
    ('C004', 'message', 'U004', 'That''s fantastic news! What were the main highlights?', '1704283260.000200', unixepoch() - 86400 * 5, 'demo-session-001'),
    ('C004', 'message', 'U001', 'Users love the new data visualizations and the improved navigation.', '1704283320.000300', unixepoch() - 86400 * 5, 'demo-session-001'),
    ('C004', 'message', 'U001', 'Next sprint, we should focus on mobile responsiveness.', '1704283380.000400', unixepoch() - 86400 * 5, 'demo-session-001'),

    -- Random channel
    ('C002', 'message', 'U003', 'Anyone tried the new coffee shop downtown?', '1704369600.000100', unixepoch() - 86400 * 4, 'demo-session-001'),
    ('C002', 'message', 'U004', 'Yes! Their espresso is amazing ☕', '1704369660.000200', unixepoch() - 86400 * 4, 'demo-session-001'),
    ('C002', 'message', 'U003', 'Perfect, I''ll check it out tomorrow!', '1704369720.000300', unixepoch() - 86400 * 4, 'demo-session-001'),

    -- Support channel
    ('C005', 'message', 'U004', 'Customer reported an issue with login on mobile devices.', '1704456000.000100', unixepoch() - 86400 * 3, 'demo-session-001'),
    ('C005', 'message', 'U002', 'Looking into it. Seems to be affecting iOS users primarily.', '1704456060.000200', unixepoch() - 86400 * 3, 'demo-session-001'),
    ('C005', 'message', 'U002', 'Found the issue - it''s related to cookie handling. Deploying fix now.', '1704456720.000300', unixepoch() - 86400 * 3, 'demo-session-001'),
    ('C005', 'message', 'U004', 'Excellent work Bob! I''ll update the customer.', '1704456780.000400', unixepoch() - 86400 * 3, 'demo-session-001'),

    -- Recent messages
    ('C001', 'message', 'U001', 'Happy Friday everyone! 🎉', '1704542400.000100', unixepoch() - 86400, 'demo-session-001'),
    ('C001', 'message', 'U002', 'Have a great weekend team!', '1704542460.000200', unixepoch() - 86400, 'demo-session-001'),
    ('C001', 'message', 'U003', 'See you all on Monday!', '1704542520.000300', unixepoch() - 86400, 'demo-session-001');

-- Insert Slack Files
INSERT INTO slack_files (
    id, filename, title, filetype, size, upload_url,
    channel_id, user_id, created_at, session_id
) VALUES
    (
        'F001', 'architecture-diagram.png', 'System Architecture', 'png', 245678,
        'https://files.slack.com/files-pri/T021F9ZE2-F001/architecture-diagram.png',
        'C003', 'U002', unixepoch() - 86400 * 6, 'demo-session-001'
    ),
    (
        'F002', 'Q4-roadmap.pdf', 'Q4 Product Roadmap', 'pdf', 1234567,
        'https://files.slack.com/files-pri/T021F9ZE2-F002/q4-roadmap.pdf',
        'C004', 'U001', unixepoch() - 86400 * 5, 'demo-session-001'
    ),
    (
        'F003', 'meeting-notes.md', 'All-Hands Meeting Notes', 'md', 12345,
        'https://files.slack.com/files-pri/T021F9ZE2-F003/meeting-notes.md',
        'C001', 'U001', unixepoch() - 86400 * 7, 'demo-session-001'
    ),
    (
        'F004', 'error-screenshot.png', 'iOS Login Error', 'png', 89012,
        'https://files.slack.com/files-pri/T021F9ZE2-F004/error-screenshot.png',
        'C005', 'U004', unixepoch() - 86400 * 3, 'demo-session-001'
    );

-- ============================================================================
-- GMAIL SIMULATOR DATA
-- ============================================================================

-- Insert Gmail Messages (realistic email thread)
INSERT INTO gmail_messages (
    id, thread_id, from_email, to_email, subject,
    body_plain, body_html, raw_message, snippet,
    label_ids, internal_date, size_estimate, created_at, session_id
) VALUES
    -- Welcome email thread
    (
        '18c5f2e8a9b1d3f4', '18c5f2e8a9b1d3f4',
        'support@example.com', 'alice@example.com',
        'Welcome to Nova Simulators!',
        'Hi Alice,

Welcome to Nova Simulators! We''re excited to have you on board.

Here are some quick tips to get started:
1. Check out our documentation at docs.example.com
2. Join our community Slack workspace
3. Reach out to support@example.com if you need help

Best regards,
The Nova Team',
        '<html><body><p>Hi Alice,</p><p>Welcome to Nova Simulators! We''re excited to have you on board.</p><p>Here are some quick tips to get started:<br>1. Check out our documentation at docs.example.com<br>2. Join our community Slack workspace<br>3. Reach out to support@example.com if you need help</p><p>Best regards,<br>The Nova Team</p></body></html>',
        'From: support@example.com\r\nTo: alice@example.com\r\nSubject: Welcome to Nova Simulators!\r\n\r\nHi Alice...',
        'Hi Alice, Welcome to Nova Simulators! We''re excited to have you on board...',
        'INBOX,IMPORTANT',
        unixepoch() - 86400 * 30,
        2345,
        unixepoch() - 86400 * 30,
        'demo-session-001'
    ),

    -- Meeting invitation
    (
        '18c5f3a2b4c6d7e8', '18c5f3a2b4c6d7e8',
        'bob@example.com', 'alice@example.com',
        'Re: Weekly Sync - API Development',
        'Hey Alice,

Let''s sync up on the new API endpoints tomorrow at 2 PM.

Agenda:
- Review current progress
- Discuss blockers
- Plan for next sprint

See you then!
Bob',
        '<html><body><p>Hey Alice,</p><p>Let''s sync up on the new API endpoints tomorrow at 2 PM.</p><p>Agenda:<br>- Review current progress<br>- Discuss blockers<br>- Plan for next sprint</p><p>See you then!<br>Bob</p></body></html>',
        'From: bob@example.com\r\nTo: alice@example.com\r\nSubject: Re: Weekly Sync - API Development\r\n\r\nHey Alice...',
        'Hey Alice, Let''s sync up on the new API endpoints tomorrow at 2 PM...',
        'INBOX,CATEGORY_UPDATES',
        unixepoch() - 86400 * 7,
        1890,
        unixepoch() - 86400 * 7,
        'demo-session-001'
    ),

    -- Customer inquiry
    (
        '18c5f4b3c5d8e9f0', '18c5f4b3c5d8e9f0',
        'customer@client.com', 'support@example.com',
        'Question about API rate limits',
        'Hi Nova Team,

I''m integrating with your API and wanted to clarify the rate limits.
The documentation mentions 1000 requests per hour, but I''m seeing 429 errors at around 800 requests.

Can you help clarify?

Thanks,
Jane Doe
Senior Engineer @ Client Corp',
        '<html><body><p>Hi Nova Team,</p><p>I''m integrating with your API and wanted to clarify the rate limits.<br>The documentation mentions 1000 requests per hour, but I''m seeing 429 errors at around 800 requests.</p><p>Can you help clarify?</p><p>Thanks,<br>Jane Doe<br>Senior Engineer @ Client Corp</p></body></html>',
        'From: customer@client.com\r\nTo: support@example.com\r\nSubject: Question about API rate limits\r\n\r\nHi Nova Team...',
        'Hi Nova Team, I''m integrating with your API and wanted to clarify the rate limits...',
        'INBOX,CATEGORY_FORUMS',
        unixepoch() - 86400 * 5,
        2156,
        unixepoch() - 86400 * 5,
        'demo-session-001'
    ),

    -- Response to customer
    (
        '18c5f5c4d6e9f1a2', '18c5f4b3c5d8e9f0',
        'support@example.com', 'customer@client.com',
        'Re: Question about API rate limits',
        'Hi Jane,

Thanks for reaching out! The 1000 requests/hour limit is per API key.

However, there''s also a burst limit of 100 requests per minute to prevent sudden spikes. That''s likely what you''re hitting.

We recommend implementing exponential backoff in your client. Here''s a code example:
https://docs.example.com/rate-limiting-best-practices

Let me know if you need further assistance!

Best,
Diana
Customer Support @ Nova',
        '<html><body><p>Hi Jane,</p><p>Thanks for reaching out! The 1000 requests/hour limit is per API key.</p><p>However, there''s also a burst limit of 100 requests per minute to prevent sudden spikes. That''s likely what you''re hitting.</p><p>We recommend implementing exponential backoff in your client. Here''s a code example:<br><a href="https://docs.example.com/rate-limiting-best-practices">Rate Limiting Best Practices</a></p><p>Let me know if you need further assistance!</p><p>Best,<br>Diana<br>Customer Support @ Nova</p></body></html>',
        'From: support@example.com\r\nTo: customer@client.com\r\nSubject: Re: Question about API rate limits\r\n\r\nHi Jane...',
        'Hi Jane, Thanks for reaching out! The 1000 requests/hour limit is per API key...',
        'SENT,CATEGORY_FORUMS',
        unixepoch() - 86400 * 4,
        2890,
        unixepoch() - 86400 * 4,
        'demo-session-001'
    ),

    -- Product update
    (
        '18c5f6d5e7f0a3b4', '18c5f6d5e7f0a3b4',
        'product@example.com', 'all-users@example.com',
        'New Feature: Real-time Event Streaming',
        'Hello Nova Users,

We''re excited to announce our latest feature: Real-time Event Streaming!

🎉 What''s New:
- Server-Sent Events (SSE) support
- Live updates for all simulator activities
- Zero polling, reduced latency
- Works with all 14 simulators

📚 Documentation: docs.example.com/sse-streaming
🎮 Try it now: app.example.com/simulators

As always, we welcome your feedback!

The Nova Product Team',
        '<html><body><h2>Hello Nova Users,</h2><p>We''re excited to announce our latest feature: Real-time Event Streaming!</p><h3>🎉 What''s New:</h3><ul><li>Server-Sent Events (SSE) support</li><li>Live updates for all simulator activities</li><li>Zero polling, reduced latency</li><li>Works with all 14 simulators</li></ul><p>📚 Documentation: <a href="https://docs.example.com/sse-streaming">docs.example.com/sse-streaming</a><br>🎮 Try it now: <a href="https://app.example.com/simulators">app.example.com/simulators</a></p><p>As always, we welcome your feedback!</p><p>The Nova Product Team</p></body></html>',
        'From: product@example.com\r\nTo: all-users@example.com\r\nSubject: New Feature: Real-time Event Streaming\r\n\r\nHello Nova Users...',
        'Hello Nova Users, We''re excited to announce our latest feature: Real-time Event Streaming!...',
        'INBOX,IMPORTANT,CATEGORY_PROMOTIONS',
        unixepoch() - 86400 * 2,
        3456,
        unixepoch() - 86400 * 2,
        'demo-session-001'
    ),

    -- System notification
    (
        '18c5f7e6f8a1b4c5', '18c5f7e6f8a1b4c5',
        'noreply@example.com', 'alice@example.com',
        'Your API key will expire in 30 days',
        'Hi Alice,

This is a friendly reminder that your API key (ending in ...abc123) will expire in 30 days.

To avoid service interruption, please generate a new key at:
https://app.example.com/settings/api-keys

Need help? Contact support@example.com

This is an automated message. Please do not reply.

Nova Security Team',
        '<html><body><p>Hi Alice,</p><p>This is a friendly reminder that your API key (ending in ...abc123) will expire in 30 days.</p><p>To avoid service interruption, please generate a new key at:<br><a href="https://app.example.com/settings/api-keys">API Key Settings</a></p><p>Need help? Contact support@example.com</p><p><em>This is an automated message. Please do not reply.</em></p><p>Nova Security Team</p></body></html>',
        'From: noreply@example.com\r\nTo: alice@example.com\r\nSubject: Your API key will expire in 30 days\r\n\r\nHi Alice...',
        'Hi Alice, This is a friendly reminder that your API key will expire in 30 days...',
        'INBOX,CATEGORY_UPDATES',
        unixepoch() - 86400,
        1678,
        unixepoch() - 86400,
        'demo-session-001'
    );

-- Print completion message
SELECT 'Sample data seeded successfully!' as status;
SELECT 'Session ID: demo-session-001' as session_info;
SELECT 'Slack channels: ' || COUNT(*) as slack_channels FROM slack_channels WHERE session_id = 'demo-session-001';
SELECT 'Slack users: ' || COUNT(*) as slack_users FROM slack_users WHERE session_id = 'demo-session-001';
SELECT 'Slack messages: ' || COUNT(*) as slack_messages FROM slack_messages WHERE session_id = 'demo-session-001';
SELECT 'Slack files: ' || COUNT(*) as slack_files FROM slack_files WHERE session_id = 'demo-session-001';
SELECT 'Gmail messages: ' || COUNT(*) as gmail_messages FROM gmail_messages WHERE session_id = 'demo-session-001';
