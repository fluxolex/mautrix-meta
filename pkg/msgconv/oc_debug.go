package msgconv

// OpenCommunication fork: diagnostic dumps of payloads the bridge can't convert.

import (
	"context"
	"encoding/hex"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func ocDumpProto(ctx context.Context, label string, msg proto.Message) {
	raw, _ := proto.Marshal(msg)
	js, _ := protojson.Marshal(msg)
	zerolog.Ctx(ctx).Debug().
		Str("oc_label", label).
		RawJSON("oc_json", append([]byte(nil), js...)).
		Str("oc_raw", hex.EncodeToString(raw)).
		Msg("OC debug: unconverted payload")
}
