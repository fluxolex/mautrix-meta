package msgconv

// OpenCommunication fork: Messenger content that upstream bridges as
// "Unsupported message" — call log entries ("org.opencommunication.call"),
// unsent messages in the encrypted history ("org.opencommunication.unsent")
// and replies to notes ("org.opencommunication.note_reply").

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/proto/waArmadilloApplication"
	"go.mau.fi/whatsmeow/proto/waArmadilloXMA"
	"go.mau.fi/whatsmeow/proto/waConsumerApplication"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"
)

// OCCall is the structured call info attached to call log events.
type OCCall struct {
	// Media is "audio" or "video".
	Media string `json:"media"`
	// State is "ended", "missed" or "ongoing".
	State       string `json:"state"`
	Group       bool   `json:"group,omitempty"`
	DurationSec int64  `json:"duration_sec,omitempty"`
	// ActorID is the Facebook user ID of whoever started the call.
	ActorID string `json:"actor_id,omitempty"`
}

type ocCallingTemplate struct {
	Content struct {
		CustomTemplateData struct {
			CallDurationSec int64  `json:"call_duration_sec"`
			CallState       string `json:"call_state"`
			CallType        string `json:"call_type"`
			EventActorID    string `json:"event_actor_id"`
		} `json:"custom_template_data"`
	} `json:"content"`
}

func ocCallFromXMA(content *waArmadilloXMA.ExtendedContentMessage) *OCCall {
	call := &OCCall{Media: "audio", State: "ended"}
	switch content.GetTargetType() {
	case waArmadilloXMA.ExtendedContentMessage_RTC_AUDIO_CALL:
	case waArmadilloXMA.ExtendedContentMessage_RTC_VIDEO_CALL:
		call.Media = "video"
	case waArmadilloXMA.ExtendedContentMessage_RTC_MISSED_AUDIO_CALL:
		call.State = "missed"
	case waArmadilloXMA.ExtendedContentMessage_RTC_MISSED_VIDEO_CALL:
		call.Media, call.State = "video", "missed"
	case waArmadilloXMA.ExtendedContentMessage_RTC_GROUP_AUDIO_CALL:
		call.Group = true
	case waArmadilloXMA.ExtendedContentMessage_RTC_GROUP_VIDEO_CALL:
		call.Media, call.Group = "video", true
	case waArmadilloXMA.ExtendedContentMessage_RTC_MISSED_GROUP_AUDIO_CALL:
		call.State, call.Group = "missed", true
	case waArmadilloXMA.ExtendedContentMessage_RTC_MISSED_GROUP_VIDEO_CALL:
		call.Media, call.State, call.Group = "video", "missed", true
	case waArmadilloXMA.ExtendedContentMessage_RTC_ONGOING_AUDIO_CALL:
		call.State = "ongoing"
	case waArmadilloXMA.ExtendedContentMessage_RTC_ONGOING_VIDEO_CALL:
		call.Media, call.State = "video", "ongoing"
	default:
		return nil
	}
	var tpl ocCallingTemplate
	if json.Unmarshal([]byte(content.GetXmaDataclass()), &tpl) == nil {
		data := tpl.Content.CustomTemplateData
		switch strings.ToUpper(data.CallType) {
		case "AUDIO":
			call.Media = "audio"
		case "VIDEO":
			call.Media = "video"
		}
		switch strings.ToUpper(data.CallState) {
		case "MISSED":
			call.State = "missed"
		case "ENDED":
			call.State = "ended"
		case "ONGOING", "IN_PROGRESS":
			call.State = "ongoing"
		}
		call.ActorID = data.EventActorID
		// For missed calls the "duration" is how long it rang.
		if call.State == "ended" {
			call.DurationSec = data.CallDurationSec
		}
	}
	return call
}

func ocFormatDuration(sec int64) string {
	if sec >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", sec/3600, sec/60%60, sec%60)
	}
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

func (c *OCCall) body() string {
	kind := "Audio call"
	if c.Media == "video" {
		kind = "Video call"
	}
	if c.Group {
		kind = "Group " + strings.ToLower(kind)
	}
	switch c.State {
	case "missed":
		return "Missed " + strings.ToLower(kind)
	case "ongoing":
		return kind + " in progress"
	}
	if c.DurationSec > 0 {
		return fmt.Sprintf("%s (%s)", kind, ocFormatDuration(c.DurationSec))
	}
	return kind
}

// ocCallPart converts a call log XMA, or returns nil for anything else.
func ocCallPart(_ context.Context, content *waArmadilloXMA.ExtendedContentMessage) *bridgev2.ConvertedMessagePart {
	call := ocCallFromXMA(content)
	if call == nil {
		return nil
	}
	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &event.MessageEventContent{MsgType: event.MsgNotice, Body: call.body()},
		Extra:   map[string]any{"org.opencommunication.call": call},
	}
}

// ocConsumerParts handles consumer messages without content (e.g. an unsent
// message in the encrypted history), or returns nil.
func ocConsumerParts(ctx context.Context, msg *waConsumerApplication.ConsumerApplication) []*bridgev2.ConvertedMessagePart {
	if msg.GetPayload().GetContent().GetContent() != nil {
		return nil
	}
	if msg.GetPayload().GetApplicationData().GetRevoke() != nil {
		return []*bridgev2.ConvertedMessagePart{{
			Type:    event.EventMessage,
			Content: &event.MessageEventContent{MsgType: event.MsgNotice, Body: "Message unsent"},
			Extra:   map[string]any{"org.opencommunication.unsent": true},
		}}
	}
	ocDumpProto(ctx, "consumer application", msg)
	return nil
}

// ocArmadilloParts converts Armadillo content upstream doesn't support, or
// returns nil.
func (mc *MessageConverter) ocArmadilloParts(ctx context.Context, content *waArmadilloApplication.Armadillo_Content) []*bridgev2.ConvertedMessagePart {
	reply := content.GetNoteReplyMessage()
	if reply == nil {
		return nil
	}
	var part *bridgev2.ConvertedMessagePart
	if text := reply.GetTextContent(); text != nil {
		part = mc.WhatsAppTextToMatrix(ctx, text)
	} else {
		kind := "a sticker"
		if reply.GetVideoContent() != nil {
			kind = "a video"
		}
		part = &bridgev2.ConvertedMessagePart{
			Type:    event.EventMessage,
			Content: &event.MessageEventContent{MsgType: event.MsgNotice, Body: "Replied to a note with " + kind},
		}
	}
	if part.Extra == nil {
		part.Extra = map[string]any{}
	}
	part.Extra["org.opencommunication.note_reply"] = map[string]any{
		"note_text": reply.GetNoteText().GetText(),
		"note_ts":   reply.GetNoteTimestampMS(),
	}
	return []*bridgev2.ConvertedMessagePart{part}
}
