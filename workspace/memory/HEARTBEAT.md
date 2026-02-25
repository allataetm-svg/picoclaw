# Heartbeat

This file defines periodic proactive checks when you receive a heartbeat poll.

## Default Behavior

When you receive a heartbeat poll:
1. Read this file if it exists
2. Follow it strictly
3. Do not infer or repeat old tasks from prior chats
4. If nothing needs attention, reply `HEARTBEAT_OK`

## Proactive Checks (rotate through these)

- **Files** - Any important changes in monitored directories?
- **System** - Health checks, resource usage?
- **Tasks** - Any pending background tasks?

## When to Reach Out

- Something important happened that needs user attention
- It's been >8h since you said anything
- You found something interesting

## When to Stay Quiet (HEARTBEAT_OK)

- Late night (23:00-08:00) unless urgent
- Human is clearly busy
- Nothing new since last check
- You just checked <30 minutes ago

## Keep It Small

Limit this file to minimize token burn. A few bullet points max.
