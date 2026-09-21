package masque

import (
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

const DefaultPath = "/.well-known/masque/ip/{target}/{ipproto}/"

type expression struct {
	operator  byte
	variables []string
}

type Template struct {
	literals       []string
	expressions    []expression
	pathExpression *regexp.Regexp
	pathVariables  []string
	queryVariables map[string]string
}

type Scope struct {
	Domain   string
	Prefix   netip.Prefix
	Protocol uint8
}

func ParseTemplate(path string) (*Template, error) {
	if path == "" {
		path = DefaultPath
	}
	if !strings.HasPrefix(path, "/") {
		return nil, E.New("path must start with a slash: ", path)
	}
	for _, character := range []byte(path) {
		if character < 0x21 || character > 0x7E {
			return nil, E.New("path contains invalid characters: ", path)
		}
	}
	template := &Template{queryVariables: make(map[string]string)}
	var pathPattern strings.Builder
	pathPattern.WriteString("^")
	inQuery := false
	remaining := path
	for {
		open := strings.IndexByte(remaining, '{')
		if open == -1 {
			break
		}
		closing := strings.IndexByte(remaining[open:], '}')
		if closing == -1 {
			return nil, E.New("unterminated expression in path: ", path)
		}
		literal := remaining[:open]
		body := remaining[open+1 : open+closing]
		remaining = remaining[open+closing+1:]
		if body == "" {
			return nil, E.New("empty expression in path: ", path)
		}
		var item expression
		switch body[0] {
		case '?', '&':
			item.operator = body[0]
			body = body[1:]
		case '+', '#', '.', '/', ';':
			return nil, E.New("unsupported expression operator in path: ", path)
		}
		item.variables = strings.Split(body, ",")
		for _, variable := range item.variables {
			if variable == "" || strings.ContainsAny(variable, ":*") {
				return nil, E.New("unsupported expression in path: ", path)
			}
		}
		if !inQuery {
			literalPath, _, hasQuery := strings.Cut(literal, "?")
			if hasQuery {
				inQuery = true
				pathPattern.WriteString(regexp.QuoteMeta(literalPath))
			} else if item.operator != 0 {
				inQuery = true
				pathPattern.WriteString(regexp.QuoteMeta(literal))
			} else {
				if len(item.variables) != 1 {
					return nil, E.New("unsupported expression in path: ", path)
				}
				pathPattern.WriteString(regexp.QuoteMeta(literal))
				pathPattern.WriteString("([^/]*)")
				template.pathVariables = append(template.pathVariables, item.variables[0])
			}
		}
		if inQuery && item.operator == 0 {
			queryKey := literal[strings.LastIndexAny(literal, "?&")+1:]
			if len(item.variables) != 1 || len(queryKey) < 2 || !strings.HasSuffix(queryKey, "=") {
				return nil, E.New("unsupported expression in path: ", path)
			}
			template.queryVariables[strings.TrimSuffix(queryKey, "=")] = item.variables[0]
		}
		template.literals = append(template.literals, literal)
		template.expressions = append(template.expressions, item)
	}
	if !inQuery {
		remainingPath, _, _ := strings.Cut(remaining, "?")
		pathPattern.WriteString(regexp.QuoteMeta(remainingPath))
	}
	pathPattern.WriteString("$")
	template.literals = append(template.literals, remaining)
	pathExpression, err := regexp.Compile(pathPattern.String())
	if err != nil {
		return nil, E.Cause(err, "compile path: ", path)
	}
	template.pathExpression = pathExpression
	return template, nil
}

func (t *Template) Expand() string {
	var result strings.Builder
	for i, item := range t.expressions {
		result.WriteString(t.literals[i])
		separator := item.operator
		for _, variable := range item.variables {
			if variable != "target" && variable != "ipproto" {
				continue
			}
			if separator != 0 {
				result.WriteByte(separator)
				result.WriteString(variable)
				result.WriteByte('=')
				separator = '&'
			}
			result.WriteByte('*')
		}
	}
	result.WriteString(t.literals[len(t.literals)-1])
	return result.String()
}

func (t *Template) Match(requestURL *url.URL) (Scope, bool, error) {
	match := t.pathExpression.FindStringSubmatch(requestURL.EscapedPath())
	if match == nil {
		return Scope{}, false, nil
	}
	target := "*"
	protocol := "*"
	query := requestURL.Query()
	for _, item := range t.expressions {
		if item.operator == 0 {
			continue
		}
		for _, variable := range item.variables {
			if !query.Has(variable) {
				continue
			}
			switch variable {
			case "target":
				target = query.Get(variable)
			case "ipproto":
				protocol = query.Get(variable)
			}
		}
	}
	for queryKey, variable := range t.queryVariables {
		if !query.Has(queryKey) {
			continue
		}
		switch variable {
		case "target":
			target = query.Get(queryKey)
		case "ipproto":
			protocol = query.Get(queryKey)
		}
	}
	for i, variable := range t.pathVariables {
		value, err := url.PathUnescape(match[i+1])
		if err != nil {
			return Scope{}, true, E.Cause(err, "decode ", variable)
		}
		switch variable {
		case "target":
			target = value
		case "ipproto":
			protocol = value
		}
	}
	var scope Scope
	switch target {
	case "":
		return Scope{}, true, E.New("empty target")
	case "*":
	default:
		prefix, err := parseTarget(target)
		if err == nil {
			scope.Prefix = prefix
		} else if strings.ContainsAny(target, ":/") {
			return Scope{}, true, E.Cause(err, "parse target")
		} else {
			scope.Domain = target
		}
	}
	if protocol != "*" {
		protocolNumber, err := strconv.ParseUint(protocol, 10, 8)
		if err != nil {
			return Scope{}, true, E.New("invalid ipproto: ", protocol)
		}
		scope.Protocol = uint8(protocolNumber)
	}
	return scope, true, nil
}

func parseTarget(target string) (netip.Prefix, error) {
	var prefix netip.Prefix
	if strings.Contains(target, "/") {
		var err error
		prefix, err = netip.ParsePrefix(target)
		if err != nil {
			return netip.Prefix{}, err
		}
	} else {
		address, err := netip.ParseAddr(target)
		if err != nil {
			return netip.Prefix{}, err
		}
		prefix = netip.PrefixFrom(address, address.BitLen())
	}
	if prefix.Addr().Zone() != "" {
		return netip.Prefix{}, E.New("zone identifiers are not supported: ", target)
	}
	if prefix.Masked() != prefix {
		return netip.Prefix{}, E.New("prefix has host bits set: ", target)
	}
	return prefix, nil
}
