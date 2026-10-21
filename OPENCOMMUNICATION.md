# OpenCommunication fork of mautrix-meta

Branch `opencommunication` = upstream [mautrix/meta](https://github.com/mautrix/meta) `main` + the commits below,
kept on top by rebasing (`npm run update:messenger-bridge` in the OpenCommunication repository does it, builds the
`opencommunication/mautrix-meta` image and restarts the bridge).

Changes live in `pkg/msgconv/oc_*.go`; upstream files only get one-line hooks, so rebases rarely conflict.

| What | Upstream | Fork |
| --- | --- | --- |
| Call log entries of encrypted chats (XMA `RTC_*`) | "Unsupported message" | notice like `Video call (10:28)` / `Missed audio call` + `org.opencommunication.call`: `{media: audio\|video, state: ended\|missed\|ongoing, group, duration_sec, actor_id}` |
| Unsent message in the encrypted history (consumer `applicationData.revoke`) | "Unsupported message (<nil>)" | notice `Message unsent` + `org.opencommunication.unsent: true` |
| Reply to a note (Armadillo `NoteReplyMessage`) | "Unsupported message (…NoteReplyMessage_)" | the reply text + `org.opencommunication.note_reply`: `{note_text, note_ts}` |
| Other consumer messages without content | "Unsupported message (<nil>)" | unchanged, payload dumped at debug level (`OC debug: unconverted payload`) |
