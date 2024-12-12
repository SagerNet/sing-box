package main

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	_ "unsafe"

	"github.com/cidertool/asc-go/asc"
	"github.com/google/go-querystring/query"
)

func (c *Client) newRequest(ctx context.Context, method string, path string, body *requestBody, options ...requestOption) (*http.Request, error) {
	return clientNewRequest(c.Client, ctx, method, path, body, options...)
}

//go:linkname clientNewRequest github.com/cidertool/asc-go/asc.(*Client).newRequest
func clientNewRequest(c *asc.Client, ctx context.Context, method string, path string, body *requestBody, options ...requestOption) (*http.Request, error)

func (c *Client) do(ctx context.Context, req *http.Request, v interface{}) (*asc.Response, error) {
	return clientDo(c.Client, ctx, req, v)
}

//go:linkname clientDo github.com/cidertool/asc-go/asc.(*Client).do
func clientDo(c *asc.Client, ctx context.Context, req *http.Request, v interface{}) (*asc.Response, error)

// get sends a GET request to the API as configured.
func (c *Client) get(ctx context.Context, url string, query interface{}, v interface{}, options ...requestOption) (*asc.Response, error) {
	var err error
	if query != nil {
		url, err = appendingQueryOptions(url, query)
		if err != nil {
			return nil, err
		}
	}

	req, err := c.newRequest(ctx, "GET", url, nil, options...)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req, v)
	if err != nil {
		return resp, err
	}

	return resp, err
}

// post sends a POST request to the API as configured.
func (c *Client) post(ctx context.Context, url string, body *requestBody, v interface{}) (*asc.Response, error) {
	req, err := c.newRequest(ctx, "POST", url, body, withContentType("application/json"))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req, v)
	if err != nil {
		return resp, err
	}

	return resp, err
}

// patch sends a PATCH request to the API as configured.
func (c *Client) patch(ctx context.Context, url string, body *requestBody, v interface{}) (*asc.Response, error) {
	req, err := c.newRequest(ctx, "PATCH", url, body, withContentType("application/json"))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req, v)
	if err != nil {
		return resp, err
	}

	return resp, err
}

// delete sends a DELETE request to the API as configured.
func (c *Client) delete(ctx context.Context, url string, body *requestBody) (*asc.Response, error) {
	req, err := c.newRequest(ctx, "DELETE", url, body, withContentType("application/json"))
	if err != nil {
		return nil, err
	}

	return c.do(ctx, req, nil)
}

// request is a common structure for a request body sent to the API.
type requestBody struct {
	Data     interface{} `json:"data"`
	Included interface{} `json:"included,omitempty"`
}

func newRequestBody(data interface{}) *requestBody {
	return newRequestBodyWithIncluded(data, nil)
}

func newRequestBodyWithIncluded(data interface{}, included interface{}) *requestBody {
	return &requestBody{Data: data, Included: included}
}

type requestOption func(*http.Request)

func withAccept(typ string) requestOption {
	return func(req *http.Request) {
		req.Header.Set("Accept", typ)
	}
}

func withContentType(typ string) requestOption {
	return func(req *http.Request) {
		req.Header.Set("Content-Type", typ)
	}
}

// AddOptions adds the parameters in opt as URL query parameters to s.  opt
// must be a struct whose fields may contain "url" tags.
func appendingQueryOptions(s string, opt interface{}) (string, error) {
	v := reflect.ValueOf(opt)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opt)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()

	return u.String(), nil
}
