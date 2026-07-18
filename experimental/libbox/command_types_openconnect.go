package libbox

import (
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
)

type OpenConnectStatusUpdate struct {
	endpoints []*OpenConnectEndpointStatus
}

func (u *OpenConnectStatusUpdate) Endpoints() OpenConnectEndpointStatusIterator {
	return newIterator(u.endpoints)
}

type OpenConnectEndpointStatusIterator interface {
	Next() *OpenConnectEndpointStatus
	HasNext() bool
}

type OpenConnectEndpointStatus struct {
	EndpointTag   string
	State         string
	StateText     string
	AuthChallenge *OpenConnectAuthChallenge
	Error         string
	TunnelInfo    *OpenConnectTunnelInfo
}

type OpenConnectTunnelInfo struct {
	Server         string
	Flavor         string
	Transport      string
	ipv4           []string
	ipv6           []string
	dns            []string
	MTU            int32
	ConnectedSince int64
}

func (i *OpenConnectTunnelInfo) IPv4() StringIterator {
	return newIterator(i.ipv4)
}

func (i *OpenConnectTunnelInfo) IPv6() StringIterator {
	return newIterator(i.ipv6)
}

func (i *OpenConnectTunnelInfo) DNS() StringIterator {
	return newIterator(i.dns)
}

type OpenConnectAuthChallenge struct {
	ID      string
	Banner  string
	Message string
	Error   string
	Form    *OpenConnectAuthForm
	Browser *OpenConnectBrowserRequest
}

type OpenConnectAuthForm struct {
	fields []*OpenConnectAuthFormField
}

func (f *OpenConnectAuthForm) Fields() OpenConnectAuthFormFieldIterator {
	return newIterator(f.fields)
}

type OpenConnectBrowserRequest struct {
	URL         string
	FinalURL    string
	cookieNames []string
	headerNames []string
}

func (r *OpenConnectBrowserRequest) CookieNames() StringIterator {
	return newIterator(r.cookieNames)
}

func (r *OpenConnectBrowserRequest) HeaderNames() StringIterator {
	return newIterator(r.headerNames)
}

type OpenConnectAuthFormFieldIterator interface {
	Next() *OpenConnectAuthFormField
	HasNext() bool
}

type OpenConnectAuthFormField struct {
	SubmissionKey string
	Name          string
	Label         string
	Kind          string
	Value         string
	options       []*OpenConnectAuthFormChoice
}

func (f *OpenConnectAuthFormField) Options() OpenConnectAuthFormChoiceIterator {
	return newIterator(f.options)
}

type OpenConnectAuthFormChoiceIterator interface {
	Next() *OpenConnectAuthFormChoice
	HasNext() bool
}

type OpenConnectAuthFormChoice struct {
	Value string
	Label string
}

type OpenConnectFormValues struct {
	values map[string]string
}

func NewOpenConnectFormValues() *OpenConnectFormValues {
	return &OpenConnectFormValues{values: make(map[string]string)}
}

func (v *OpenConnectFormValues) Add(key string, value string) {
	v.values[key] = value
}

type OpenConnectBrowserResult struct {
	FinalURL string
	cookies  []openConnectBrowserCookie
	headers  []openConnectBrowserHeader
}

func NewOpenConnectBrowserResult(finalURL string) *OpenConnectBrowserResult {
	return &OpenConnectBrowserResult{FinalURL: finalURL}
}

func (r *OpenConnectBrowserResult) AddCookie(name string, value string) {
	r.cookies = append(r.cookies, openConnectBrowserCookie{Name: name, Value: value})
}

func (r *OpenConnectBrowserResult) AddHeader(name string, value string) {
	for _, header := range r.headers {
		if header.Name == name {
			header.Values = append(header.Values, value)
			return
		}
	}
	r.headers = append(r.headers, openConnectBrowserHeader{Name: name, Values: []string{value}})
}

type openConnectBrowserCookie struct {
	Name  string
	Value string
}

type openConnectBrowserHeader struct {
	Name   string
	Values []string
}

type OpenConnectAuthResponse struct {
	formValues    *OpenConnectFormValues
	browserResult *OpenConnectBrowserResult
}

func NewOpenConnectAuthFormResponse(values *OpenConnectFormValues) *OpenConnectAuthResponse {
	return &OpenConnectAuthResponse{formValues: values}
}

func NewOpenConnectBrowserAuthResponse(result *OpenConnectBrowserResult) *OpenConnectAuthResponse {
	return &OpenConnectAuthResponse{browserResult: result}
}

type OpenConnectStatusHandler interface {
	OnStatusUpdate(status *OpenConnectStatusUpdate)
	OnError(message string)
}

type OpenConnectStatusSubscription struct {
	streamSession
}

func openConnectStatusUpdateFromGRPC(update *daemon.OpenConnectStatusUpdate) *OpenConnectStatusUpdate {
	return &OpenConnectStatusUpdate{
		endpoints: common.Map(update.Endpoints, openConnectEndpointStatusFromGRPC),
	}
}

func openConnectEndpointStatusFromGRPC(status *daemon.OpenConnectEndpointStatus) *OpenConnectEndpointStatus {
	result := &OpenConnectEndpointStatus{
		EndpointTag: status.EndpointTag,
		State:       status.State,
		StateText:   status.StateText,
		Error:       status.Error,
	}
	if status.AuthChallenge != nil {
		challenge := &OpenConnectAuthChallenge{
			ID:      status.AuthChallenge.Id,
			Banner:  status.AuthChallenge.Banner,
			Message: status.AuthChallenge.Message,
			Error:   status.AuthChallenge.Error,
		}
		form := status.AuthChallenge.GetForm()
		if form != nil {
			challenge.Form = &OpenConnectAuthForm{
				fields: common.Map(form.Fields, func(field *daemon.OpenConnectAuthFormField) *OpenConnectAuthFormField {
					return &OpenConnectAuthFormField{
						SubmissionKey: field.SubmissionKey,
						Name:          field.Name,
						Label:         field.Label,
						Kind:          field.Kind,
						Value:         field.Value,
						options: common.Map(field.Options, func(option *daemon.OpenConnectAuthFormChoice) *OpenConnectAuthFormChoice {
							return &OpenConnectAuthFormChoice{
								Value: option.Value,
								Label: option.Label,
							}
						}),
					}
				}),
			}
		}
		browser := status.AuthChallenge.GetBrowser()
		if browser != nil {
			challenge.Browser = &OpenConnectBrowserRequest{
				URL:         browser.Url,
				FinalURL:    browser.FinalURL,
				cookieNames: browser.CookieNames,
				headerNames: browser.HeaderNames,
			}
		}
		result.AuthChallenge = challenge
	}
	if status.TunnelInfo != nil {
		result.TunnelInfo = &OpenConnectTunnelInfo{
			Server:         status.TunnelInfo.Server,
			Flavor:         status.TunnelInfo.Flavor,
			Transport:      status.TunnelInfo.Transport,
			ipv4:           status.TunnelInfo.Ipv4,
			ipv6:           status.TunnelInfo.Ipv6,
			dns:            status.TunnelInfo.Dns,
			MTU:            int32(status.TunnelInfo.Mtu),
			ConnectedSince: status.TunnelInfo.ConnectedSince,
		}
	}
	return result
}
