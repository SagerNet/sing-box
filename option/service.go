package option

import (
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

type _Service struct {
	Type                string                     `json:"type"`
	Tag                 string                     `json:"tag,omitempty"`
	SubscriptionOptions SubscriptionServiceOptions `json:"-"`
}

type Service _Service

func (h Service) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case C.ServiceSubscription:
		v = h.SubscriptionOptions
	default:
		return nil, E.New("unknown service type: ", h.Type)
	}
	return MarshallObjects((_Service)(h), v)
}

func (h *Service) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Service)(h))
	if err != nil {
		return err
	}
	var v any
	switch h.Type {
	case C.ServiceSubscription:
		v = &h.SubscriptionOptions
	default:
		return E.New("unknown service type: ", h.Type)
	}
	err = UnmarshallExcluded(bytes, (*_Service)(h), v)
	if err != nil {
		return E.Cause(err, "service options")
	}
	return nil
}
