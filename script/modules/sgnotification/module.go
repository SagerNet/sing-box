package sgnotification

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/script/jsc"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	"github.com/dop251/goja"
)

type SurgeNotification struct {
	vm                *goja.Runtime
	logger            logger.Logger
	platformInterface platform.Interface
	scriptTag         string
}

func Enable(vm *goja.Runtime, ctx context.Context, logger logger.Logger) {
	platformInterface := service.FromContext[platform.Interface](ctx)
	notification := &SurgeNotification{
		vm:                vm,
		logger:            logger,
		platformInterface: platformInterface,
	}
	notificationObject := vm.NewObject()
	notificationObject.Set("post", notification.js_post)
	vm.Set("$notification", notificationObject)
}

func (s *SurgeNotification) js_post(call goja.FunctionCall) goja.Value {
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
	title = jsc.AssertString(s.vm, call.Argument(0), "title", true)
	subtitle = jsc.AssertString(s.vm, call.Argument(1), "subtitle", true)
	body = jsc.AssertString(s.vm, call.Argument(2), "body", true)
	options := jsc.AssertObject(s.vm, call.Argument(3), "options", true)
	if options != nil {
		action := jsc.AssertString(s.vm, options.Get("action"), "options.action", true)
		switch action {
		case "open-url":
			openURL = jsc.AssertString(s.vm, options.Get("url"), "options.url", false)
		case "clipboard":
			clipboard = jsc.AssertString(s.vm, options.Get("clipboard"), "options.clipboard", false)
		}
		mediaURL = jsc.AssertString(s.vm, options.Get("media-url"), "options.media-url", true)
		mediaBase64 := jsc.AssertString(s.vm, options.Get("media-base64"), "options.media-base64", true)
		if mediaBase64 != "" {
			mediaBinary, err := base64.StdEncoding.DecodeString(mediaBase64)
			if err != nil {
				panic(s.vm.NewGoError(E.Cause(err, "decode media-base64")))
			}
			mediaData = mediaBinary
			mediaType = jsc.AssertString(s.vm, options.Get("media-base64-mime"), "options.media-base64-mime", false)
		}
		autoDismiss = int(jsc.AssertInt(s.vm, options.Get("auto-dismiss"), "options.auto-dismiss", true))
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
			Identifier: "surge-script-notification-" + s.scriptTag,
			TypeName:   "Surge Script Notification (" + s.scriptTag + ")",
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
	return goja.Undefined()
}
