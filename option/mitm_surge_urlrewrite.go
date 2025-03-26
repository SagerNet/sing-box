package option

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
)

type SurgeURLRewriteLine struct {
	Pattern     *regexp.Regexp
	Destination *url.URL
	Redirect    bool
	Reject      bool
}

func (l SurgeURLRewriteLine) String() string {
	var fields []string
	fields = append(fields, l.Pattern.String())
	if l.Reject {
		fields = append(fields, "_")
	} else {
		fields = append(fields, l.Destination.String())
	}
	switch {
	case l.Redirect:
		fields = append(fields, "302")
	case l.Reject:
		fields = append(fields, "reject")
	default:
		fields = append(fields, "header")
	}
	return encodeSurgeKeys(fields)
}

func (l SurgeURLRewriteLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l *SurgeURLRewriteLine) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err != nil {
		return err
	}
	fields, err := surgeFields(stringValue)
	if err != nil {
		return E.Cause(err, "invalid surge_url_rewrite line: ", stringValue)
	} else if len(fields) < 2 || len(fields) > 3 {
		return E.New("invalid surge_url_rewrite line: ", stringValue)
	}
	pattern, err := regexp.Compile(fields[0].Key)
	if err != nil {
		return E.Cause(err, "invalid surge_url_rewrite line: invalid pattern: ", stringValue)
	}
	l.Pattern = pattern
	l.Destination, err = url.Parse(fields[1].Key)
	if err != nil {
		return E.Cause(err, "invalid surge_url_rewrite line: invalid destination: ", stringValue)
	}
	if len(fields) == 3 {
		switch fields[2].Key {
		case "header":
		case "302":
			l.Redirect = true
		case "reject":
			l.Reject = true
		default:
			return E.New("invalid surge_url_rewrite line: invalid action: ", stringValue)
		}
	}
	return nil
}

type SurgeHeaderRewriteLine struct {
	Response     bool
	Pattern      *regexp.Regexp
	Add          bool
	Delete       bool
	Replace      bool
	ReplaceRegex bool
	Key          string
	Match        *regexp.Regexp
	Value        string
}

func (l SurgeHeaderRewriteLine) String() string {
	var fields []string
	if !l.Response {
		fields = append(fields, "http-request")
	} else {
		fields = append(fields, "http-response")
	}
	fields = append(fields, l.Pattern.String())
	if l.Add {
		fields = append(fields, "header-add")
	} else if l.Delete {
		fields = append(fields, "header-del")
	} else if l.Replace {
		fields = append(fields, "header-replace")
	} else if l.ReplaceRegex {
		fields = append(fields, "header-replace-regex")
	}
	fields = append(fields, l.Key)
	if l.Add || l.Replace {
		fields = append(fields, l.Value)
	} else if l.ReplaceRegex {
		fields = append(fields, l.Match.String(), l.Value)
	}
	return encodeSurgeKeys(fields)
}

func (l SurgeHeaderRewriteLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l *SurgeHeaderRewriteLine) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err != nil {
		return err
	}
	fields, err := surgeFields(stringValue)
	if err != nil {
		return E.Cause(err, "invalid surge_header_rewrite line: ", stringValue)
	} else if len(fields) < 4 {
		return E.New("invalid surge_header_rewrite line: ", stringValue)
	}
	switch fields[0].Key {
	case "http-request":
	case "http-response":
		l.Response = true
	default:
		return E.New("invalid surge_header_rewrite line: invalid type: ", stringValue)
	}
	l.Pattern, err = regexp.Compile(fields[1].Key)
	if err != nil {
		return E.Cause(err, "invalid surge_header_rewrite line: invalid pattern: ", stringValue)
	}
	switch fields[2].Key {
	case "header-add":
		l.Add = true
		if len(fields) != 5 {
			return E.New("invalid surge_header_rewrite line: " + stringValue)
		}
		l.Key = fields[3].Key
		l.Value = fields[4].Key
	case "header-del":
		l.Delete = true
		l.Key = fields[3].Key
	case "header-replace":
		l.Replace = true
		if len(fields) != 5 {
			return E.New("invalid surge_header_rewrite line: " + stringValue)
		}
		l.Key = fields[3].Key
		l.Value = fields[4].Key
	case "header-replace-regex":
		l.ReplaceRegex = true
		if len(fields) != 6 {
			return E.New("invalid surge_header_rewrite line: " + stringValue)
		}
		l.Key = fields[3].Key
		l.Match, err = regexp.Compile(fields[4].Key)
		if err != nil {
			return E.Cause(err, "invalid surge_header_rewrite line: invalid match: ", stringValue)
		}
		l.Value = fields[5].Key
	default:
		return E.New("invalid surge_header_rewrite line: invalid action: ", stringValue)
	}
	return nil
}

type SurgeBodyRewriteLine struct {
	Response bool
	Pattern  *regexp.Regexp
	Match    []*regexp.Regexp
	Replace  []string
}

func (l SurgeBodyRewriteLine) String() string {
	var fields []string
	if !l.Response {
		fields = append(fields, "http-request")
	} else {
		fields = append(fields, "http-response")
	}
	for i := 0; i < len(l.Match); i += 2 {
		fields = append(fields, l.Match[i].String(), l.Replace[i])
	}
	return strings.Join(fields, " ")
}

func (l SurgeBodyRewriteLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l *SurgeBodyRewriteLine) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err != nil {
		return err
	}
	fields, err := surgeFields(stringValue)
	if err != nil {
		return E.Cause(err, "invalid surge_body_rewrite line: ", stringValue)
	} else if len(fields) < 4 {
		return E.New("invalid surge_body_rewrite line: ", stringValue)
	} else if len(fields)%2 != 0 {
		return E.New("invalid surge_body_rewrite line: ", stringValue)
	}
	switch fields[0].Key {
	case "http-request":
	case "http-response":
		l.Response = true
	default:
		return E.New("invalid surge_body_rewrite line: invalid type: ", stringValue)
	}
	l.Pattern, err = regexp.Compile(fields[1].Key)
	for i := 2; i < len(fields); i += 2 {
		var match *regexp.Regexp
		match, err = regexp.Compile(fields[i].Key)
		if err != nil {
			return E.Cause(err, "invalid surge_body_rewrite line: invalid match: ", stringValue)
		}
		l.Match = append(l.Match, match)
		l.Replace = append(l.Replace, fields[i+1].Key)
	}
	return nil
}

type SurgeMapLocalLine struct {
	Pattern    *regexp.Regexp
	StatusCode int
	File       bool
	Text       bool
	TinyGif    bool
	Base64     bool
	Data       string
	Base64Data []byte
	Headers    http.Header
}

func (l SurgeMapLocalLine) String() string {
	var fields []surgeField
	fields = append(fields, surgeField{Key: l.Pattern.String()})
	if l.File {
		fields = append(fields, surgeField{Key: "data-type", Value: "file"})
		fields = append(fields, surgeField{Key: "data", Value: l.Data})
	} else if l.Text {
		fields = append(fields, surgeField{Key: "data-type", Value: "text"})
		fields = append(fields, surgeField{Key: "data", Value: l.Data})
	} else if l.TinyGif {
		fields = append(fields, surgeField{Key: "data-type", Value: "tiny-gif"})
	} else if l.Base64 {
		fields = append(fields, surgeField{Key: "data-type", Value: "base64"})
		fields = append(fields, surgeField{Key: "data-type", Value: base64.StdEncoding.EncodeToString(l.Base64Data)})
	}
	if l.StatusCode != 0 {
		fields = append(fields, surgeField{Key: "status-code", Value: F.ToString(l.StatusCode), ValueSet: true})
	}
	if len(l.Headers) > 0 {
		var headers []string
		for key, values := range l.Headers {
			for _, value := range values {
				headers = append(headers, key+":"+value)
			}
		}
		fields = append(fields, surgeField{Key: "headers", Value: strings.Join(headers, "|")})
	}
	return encodeSurgeFields(fields)
}

func (l SurgeMapLocalLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l *SurgeMapLocalLine) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err != nil {
		return err
	}
	fields, err := surgeFields(stringValue)
	if err != nil {
		return E.Cause(err, "invalid surge_map_local line: ", stringValue)
	} else if len(fields) < 1 {
		return E.New("invalid surge_map_local line: ", stringValue)
	}
	l.Pattern, err = regexp.Compile(fields[0].Key)
	if err != nil {
		return E.Cause(err, "invalid surge_map_local line: invalid pattern: ", stringValue)
	}
	dataTypeField := common.Find(fields, func(it surgeField) bool {
		return it.Key == "data-type"
	})
	if !dataTypeField.ValueSet {
		return E.New("invalid surge_map_local line: missing data-type: ", stringValue)
	}
	switch dataTypeField.Value {
	case "file":
		l.File = true
	case "text":
		l.Text = true
	case "tiny-gif":
		l.TinyGif = true
	case "base64":
		l.Base64 = true
	default:
		return E.New("unsupported data-type ", dataTypeField.Value)
	}
	for i := 1; i < len(fields); i++ {
		switch fields[i].Key {
		case "data-type":
			continue
		case "data":
			if l.File {
				l.Data = fields[i].Value
			} else if l.Text {
				l.Data = fields[i].Value
			} else if l.Base64 {
				l.Base64Data, err = base64.StdEncoding.DecodeString(fields[i].Value)
				if err != nil {
					return E.New("invalid surge_map_local line: invalid base64 data: ", stringValue)
				}
			}
		case "status-code":
			statusCode, err := strconv.ParseInt(fields[i].Value, 10, 16)
			if err != nil {
				return E.New("invalid surge_map_local line: invalid status code: ", stringValue)
			}
			l.StatusCode = int(statusCode)
		case "header":
			headers := make(http.Header)
			for _, headerLine := range strings.Split(fields[i].Value, "|") {
				if !strings.Contains(headerLine, ":") {
					return E.New("invalid surge_map_local line: headers: missing `:` in item: ", stringValue, ": ", headerLine)
				}
				headers.Add(common.SubstringBefore(headerLine, ":"), common.SubstringAfter(headerLine, ":"))
			}
			l.Headers = headers
		default:
			return E.New("invalid surge_map_local line: unknown options: ", fields[i].Key)
		}
	}
	return nil
}

type surgeField struct {
	Key      string
	Value    string
	ValueSet bool
}

func encodeSurgeKeys(keys []string) string {
	keys = common.Map(keys, func(it string) string {
		if strings.ContainsFunc(it, unicode.IsSpace) {
			return "\"" + it + "\""
		} else {
			return it
		}
	})
	return strings.Join(keys, " ")
}

func encodeSurgeFields(fields []surgeField) string {
	return strings.Join(common.Map(fields, func(it surgeField) string {
		if !it.ValueSet {
			if strings.ContainsFunc(it.Key, unicode.IsSpace) {
				return "\"" + it.Key + "\""
			} else {
				return it.Key
			}
		} else {
			if strings.ContainsFunc(it.Value, unicode.IsSpace) {
				return it.Key + "=\"" + it.Value + "\""
			} else {
				return it.Key + "=" + it.Value
			}
		}
	}), " ")
}

func surgeFields(s string) ([]surgeField, error) {
	var (
		fields       []surgeField
		currentField *surgeField
	)
	for _, field := range strings.Fields(s) {
		if currentField != nil {
			field = " " + field
			if strings.HasSuffix(field, "\"") {
				field = field[:len(field)-1]
				if !currentField.ValueSet {
					currentField.Key += field
				} else {
					currentField.Value += field
				}
				fields = append(fields, *currentField)
				currentField = nil
			} else {
				if !currentField.ValueSet {
					currentField.Key += field
				} else {
					currentField.Value += field
				}
			}
			continue
		}
		if !strings.Contains(field, "=") {
			if strings.HasPrefix(field, "\"") {
				field = field[1:]
				if strings.HasSuffix(field, "\"") {
					field = field[:len(field)-1]
				} else {
					currentField = &surgeField{Key: field}
					continue
				}
			}
			fields = append(fields, surgeField{Key: field})
		} else {
			key := common.SubstringBefore(field, "=")
			value := common.SubstringAfter(field, "=")
			if strings.HasPrefix(value, "\"") {
				value = value[1:]
				if strings.HasSuffix(field, "\"") {
					value = value[:len(value)-1]
				} else {
					currentField = &surgeField{Key: key, Value: value, ValueSet: true}
					continue
				}
			}
			fields = append(fields, surgeField{Key: key, Value: value, ValueSet: true})
		}
	}
	if currentField != nil {
		return nil, E.New("invalid surge fields line: ", s)
	}
	return fields, nil
}
