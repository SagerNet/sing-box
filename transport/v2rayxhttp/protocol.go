package v2rayxhttp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/http2/hpack"
)

func (c *config) applyMetadata(request *http.Request, sessionID, sequence string) {
	applyMeta(request, c.sessionPlacement, c.sessionKey, sessionID)
	applyMeta(request, c.seqPlacement, c.seqKey, sequence)
}

func applyMeta(request *http.Request, placement, key, value string) {
	if value == "" {
		return
	}
	switch placement {
	case placementPath:
		if !strings.HasSuffix(request.URL.Path, "/") {
			request.URL.Path += "/"
		}
		request.URL.Path += value
	case placementQuery:
		query := request.URL.Query()
		query.Set(key, value)
		request.URL.RawQuery = query.Encode()
	case placementHeader:
		request.Header.Set(key, value)
	case placementCookie:
		request.AddCookie(&http.Cookie{Name: key, Value: value})
	}
}

func (c *config) extractMetadata(request *http.Request) (string, string) {
	path := strings.TrimPrefix(request.URL.Path, c.path)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	nextPath := 0
	extract := func(placement, key string) string {
		switch placement {
		case placementPath:
			if nextPath < len(parts) && parts[nextPath] != "" {
				value := parts[nextPath]
				nextPath++
				return value
			}
		case placementQuery:
			return request.URL.Query().Get(key)
		case placementHeader:
			return request.Header.Get(key)
		case placementCookie:
			cookie, _ := request.Cookie(key)
			if cookie != nil {
				return cookie.Value
			}
		}
		return ""
	}
	return extract(c.sessionPlacement, c.sessionKey), extract(c.seqPlacement, c.seqKey)
}

func (c *config) applyPadding(request *http.Request) {
	length := c.padding.random()
	method := c.paddingMethod
	placement, key, header := c.paddingPlacement, c.paddingKey, c.paddingHeader
	if !c.paddingObfs {
		placement, key, header, method = placementQueryInHeader, "x_padding", "Referer", "repeat-x"
	}
	value := paddingValue(method, length)
	switch placement {
	case placementHeader:
		request.Header.Set(header, value)
	case placementCookie:
		request.AddCookie(&http.Cookie{Name: key, Value: value})
	case placementQuery:
		query := request.URL.Query()
		query.Set(key, value)
		request.URL.RawQuery = query.Encode()
	case placementQueryInHeader:
		copyURL := *request.URL
		query := copyURL.Query()
		query.Set(key, value)
		copyURL.RawQuery = query.Encode()
		request.Header.Set(header, copyURL.String())
	}
}

func (c *config) validPadding(request *http.Request) bool {
	placement, key, header, method := c.paddingPlacement, c.paddingKey, c.paddingHeader, c.paddingMethod
	if !c.paddingObfs {
		placement, key, header, method = placementQueryInHeader, "x_padding", "Referer", "repeat-x"
	}
	var value string
	switch placement {
	case placementHeader:
		value = request.Header.Get(header)
	case placementCookie:
		cookie, _ := request.Cookie(key)
		if cookie != nil {
			value = cookie.Value
		}
	case placementQuery:
		value = request.URL.Query().Get(key)
	case placementQueryInHeader:
		if reference, err := url.Parse(request.Header.Get(header)); err == nil {
			value = reference.Query().Get(key)
		}
	}
	if method == "tokenish" {
		encodedLength := int(hpack.HuffmanEncodeLength(value))
		return encodedLength >= int(c.padding.from)-2 && encodedLength <= int(c.padding.to)+2
	}
	return len(value) >= int(c.padding.from) && len(value) <= int(c.padding.to)
}

func paddingValue(method string, length int) string {
	if method != "tokenish" {
		return strings.Repeat("X", length)
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Base62 has a close-to-browser Huffman profile. Select the nearest length
	// so the value falls into the configured compressed-byte range.
	value := make([]byte, length+length/2+8)
	for index := range value {
		value[index] = alphabet[(index*37+length*17)%len(alphabet)]
	}
	for index := 1; index <= len(value); index++ {
		encodedLength := int(hpack.HuffmanEncodeLength(string(value[:index])))
		if encodedLength >= length {
			return string(value[:index])
		}
	}
	return string(value)
}

func (c *config) fillStreamRequest(request *http.Request, sessionID string) {
	request.Header = c.headers.Clone()
	c.applyPadding(request)
	c.applyMetadata(request, sessionID, "")
	if request.Body != nil && !c.noGRPCHeader {
		request.Header.Set("Content-Type", "application/grpc")
	}
}

func (c *config) fillPacketRequest(request *http.Request, sessionID string, sequence uint64, payload []byte) {
	request.Header = c.headers.Clone()
	switch c.dataPlacement {
	case placementHeader:
		c.fillHeaderPayload(request.Header, c.dataKey, payload)
	case placementCookie:
		for index, chunk := range c.splitEncodedPayload(payload) {
			request.AddCookie(&http.Cookie{Name: fmt.Sprintf("%s_%d", c.dataKey, index), Value: chunk})
		}
	default:
		request.Body = io.NopCloser(bytes.NewReader(payload))
		request.ContentLength = int64(len(payload))
	}
	c.applyPadding(request)
	c.applyMetadata(request, sessionID, strconv.FormatUint(sequence, 10))
}

func (c *config) fillHeaderPayload(header http.Header, key string, payload []byte) {
	for index, chunk := range c.splitEncodedPayload(payload) {
		header.Set(fmt.Sprintf("%s-%d", key, index), chunk)
	}
}

func (c *config) splitEncodedPayload(payload []byte) []string {
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	if encoded == "" {
		return nil
	}
	var chunks []string
	for len(encoded) > 0 {
		size := c.uplinkChunk.random()
		if size > len(encoded) {
			size = len(encoded)
		}
		chunks, encoded = append(chunks, encoded[:size]), encoded[size:]
	}
	return chunks
}

func (c *config) applyResponsePadding(writer http.ResponseWriter) {
	length := c.padding.random()
	if !c.paddingObfs {
		writer.Header().Set("X-Padding", paddingValue("repeat-x", length))
		return
	}
	value := paddingValue(c.paddingMethod, length)
	switch c.paddingPlacement {
	case placementHeader:
		writer.Header().Set(c.paddingHeader, value)
	case placementCookie:
		http.SetCookie(writer, &http.Cookie{Name: c.paddingKey, Value: value, Path: "/"})
	}
}

func (c *config) extractPacketPayload(request *http.Request) ([]byte, error) {
	maxSize := c.scMaxPost.random()
	headerPayload, err := decodeHeaderPayload(request, c.dataKey)
	if err != nil {
		return nil, err
	}
	cookiePayload, err := decodeCookiePayload(request, c.dataKey)
	if err != nil {
		return nil, err
	}
	bodyPayload, err := readRequestBody(request, maxSize)
	if err != nil {
		return nil, err
	}
	switch c.dataPlacement {
	case placementHeader:
		return headerPayload, nil
	case placementCookie:
		return cookiePayload, nil
	case placementAuto:
		return append(append(headerPayload, cookiePayload...), bodyPayload...), nil
	default:
		return bodyPayload, nil
	}
}

func decodeHeaderPayload(request *http.Request, key string) ([]byte, error) {
	var encoded string
	for index := 0; ; index++ {
		value := request.Header.Get(fmt.Sprintf("%s-%d", key, index))
		if value == "" {
			break
		}
		encoded += value
	}
	if encoded == "" {
		return nil, nil
	}
	return base64.RawURLEncoding.DecodeString(encoded)
}
func decodeCookiePayload(request *http.Request, key string) ([]byte, error) {
	var encoded string
	for index := 0; ; index++ {
		cookie, _ := request.Cookie(fmt.Sprintf("%s_%d", key, index))
		if cookie == nil {
			break
		}
		encoded += cookie.Value
	}
	if encoded == "" {
		return nil, nil
	}
	return base64.RawURLEncoding.DecodeString(encoded)
}

func readRequestBody(request *http.Request, maxSize int) ([]byte, error) {
	if request.ContentLength > int64(maxSize) {
		return nil, fmt.Errorf("request body exceeds xhttp limit")
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, int64(maxSize)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSize {
		return nil, fmt.Errorf("request body exceeds xhttp limit")
	}
	return data, nil
}
