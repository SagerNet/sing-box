package surge

import (
	"encoding/base64"
	"strings"

	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/boxctx"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	"github.com/dop251/goja"
)

type Notification struct {
	class             jsc.Class[*Module, *Notification]
	logger            logger.ContextLogger
	tag               string
	platformInterface platform.Interface
}

func createNotification(module *Module) jsc.Class[*Module, *Notification] {
	class := jsc.NewClass[*Module, *Notification](module)
	class.DefineMethod("post", (*Notification).post)
	class.DefineMethod("toString", (*Notification).toString)
	return class
}

func newNotification(class jsc.Class[*Module, *Notification]) goja.Value {
	context := boxctx.MustFromRuntime(class.Runtime())
	return class.New(&Notification{
		class:             class,
		logger:            context.Logger,
		tag:               context.Tag,
		platformInterface: service.FromContext[platform.Interface](context.Context),
	})
}

func (s *Notification) post(call goja.FunctionCall) any {
	var (
		title       string
		subtitle    string
		body        string
		openURL     string
		clipboard   string
		mediaURL    string
		mediaData   []byte
		mediaType   string
		autoDismiss int
	)
	title = jsc.AssertString(s.class.Runtime(), call.Argument(0), "title", true)
	subtitle = jsc.AssertString(s.class.Runtime(), call.Argument(1), "subtitle", true)
	body = jsc.AssertString(s.class.Runtime(), call.Argument(2), "body", true)
	options := jsc.AssertObject(s.class.Runtime(), call.Argument(3), "options", true)
	if options != nil {
		action := jsc.AssertString(s.class.Runtime(), options.Get("action"), "options.action", true)
		switch action {
		case "open-url":
			openURL = jsc.AssertString(s.class.Runtime(), options.Get("url"), "options.url", false)
		case "clipboard":
			clipboard = jsc.AssertString(s.class.Runtime(), options.Get("clipboard"), "options.clipboard", false)
		}
		mediaURL = jsc.AssertString(s.class.Runtime(), options.Get("media-url"), "options.media-url", true)
		mediaBase64 := jsc.AssertString(s.class.Runtime(), options.Get("media-base64"), "options.media-base64", true)
		if mediaBase64 != "" {
			mediaBinary, err := base64.StdEncoding.DecodeString(mediaBase64)
			if err != nil {
				panic(s.class.Runtime().NewGoError(E.Cause(err, "decode media-base64")))
			}
			mediaData = mediaBinary
			mediaType = jsc.AssertString(s.class.Runtime(), options.Get("media-base64-mime"), "options.media-base64-mime", false)
		}
		autoDismiss = int(jsc.AssertInt(s.class.Runtime(), options.Get("auto-dismiss"), "options.auto-dismiss", true))
	}
	if title != "" && subtitle == "" && body == "" {
		body = title
		title = ""
	} else if title != "" && subtitle != "" && body == "" {
		body = subtitle
		subtitle = ""
	}
	var builder strings.Builder
	if title != "" {
		builder.WriteString("[")
		builder.WriteString(title)
		if subtitle != "" {
			builder.WriteString(" - ")
			builder.WriteString(subtitle)
		}
		builder.WriteString("]: ")
	}
	builder.WriteString(body)
	s.logger.Info("notification: " + builder.String())
	if s.platformInterface != nil {
		err := s.platformInterface.SendNotification(&platform.Notification{
			Identifier: "surge-script-notification-" + s.tag,
			TypeName:   "Surge Script Notification (" + s.tag + ")",
			TypeID:     11,
			Title:      title,
			Subtitle:   subtitle,
			Body:       body,
			OpenURL:    openURL,
			Clipboard:  clipboard,
			MediaURL:   mediaURL,
			MediaData:  mediaData,
			MediaType:  mediaType,
			Timeout:    autoDismiss,
		})
		if err != nil {
			s.logger.Error(E.Cause(err, "send notification"))
		}
	}
	return nil
}

func (s *Notification) toString(call goja.FunctionCall) any {
	return "[sing-box Surge notification]"
}
