package option

import (
	"context"
	"reflect"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

type nestedRuleDepthContextKey struct{}

const (
	RouteRuleActionNestedUnsupportedMessage = "rule action is not supported in nested rules"
	DNSRuleActionNestedUnsupportedMessage   = "DNS rule action is not supported in nested rules"
)

var (
	routeRuleActionKeys = jsonFieldNames(reflect.TypeFor[_RuleAction](), reflect.TypeFor[RouteActionOptions]())
	dnsRuleActionKeys   = jsonFieldNames(reflect.TypeFor[_DNSRuleAction](), reflect.TypeFor[DNSRouteActionOptions]())
)

func nestedRuleChildContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, nestedRuleDepthContextKey{}, nestedRuleDepth(ctx)+1)
}

func rejectNestedRouteRuleAction(ctx context.Context, content []byte) error {
	return rejectNestedRuleAction(ctx, content, routeRuleActionKeys, RouteRuleActionNestedUnsupportedMessage)
}

func rejectNestedDNSRuleAction(ctx context.Context, content []byte) error {
	return rejectNestedRuleAction(ctx, content, dnsRuleActionKeys, DNSRuleActionNestedUnsupportedMessage)
}

func nestedRuleDepth(ctx context.Context) int {
	depth, _ := ctx.Value(nestedRuleDepthContextKey{}).(int)
	return depth
}

func rejectNestedRuleAction(ctx context.Context, content []byte, keys []string, message string) error {
	if nestedRuleDepth(ctx) == 0 {
		return nil
	}
	hasActionKey, err := hasAnyJSONKey(ctx, content, keys...)
	if err != nil {
		return err
	}
	if hasActionKey {
		return E.New(message)
	}
	return nil
}

func hasAnyJSONKey(ctx context.Context, content []byte, keys ...string) (bool, error) {
	var object badjson.JSONObject
	err := object.UnmarshalJSONContext(ctx, content)
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		if object.ContainsKey(key) {
			return true, nil
		}
	}
	return false, nil
}

func inspectRouteRuleAction(ctx context.Context, content []byte) (string, RouteActionOptions, error) {
	var rawAction _RuleAction
	err := json.UnmarshalContext(ctx, content, &rawAction)
	if err != nil {
		return "", RouteActionOptions{}, err
	}
	var routeOptions RouteActionOptions
	err = json.UnmarshalContext(ctx, content, &routeOptions)
	if err != nil {
		return "", RouteActionOptions{}, err
	}
	return rawAction.Action, routeOptions, nil
}

func inspectDNSRuleAction(ctx context.Context, content []byte) (string, DNSRouteActionOptions, error) {
	var rawAction _DNSRuleAction
	err := json.UnmarshalContext(ctx, content, &rawAction)
	if err != nil {
		return "", DNSRouteActionOptions{}, err
	}
	var routeOptions DNSRouteActionOptions
	err = json.UnmarshalContext(ctx, content, &routeOptions)
	if err != nil {
		return "", DNSRouteActionOptions{}, err
	}
	return rawAction.Action, routeOptions, nil
}

func jsonFieldNames(types ...reflect.Type) []string {
	fieldMap := make(map[string]struct{})
	for _, fieldType := range types {
		appendJSONFieldNames(fieldMap, fieldType)
	}
	fieldNames := make([]string, 0, len(fieldMap))
	for fieldName := range fieldMap {
		fieldNames = append(fieldNames, fieldName)
	}
	return fieldNames
}

func appendJSONFieldNames(fieldMap map[string]struct{}, fieldType reflect.Type) {
	for fieldType.Kind() == reflect.Pointer {
		fieldType = fieldType.Elem()
	}
	if fieldType.Kind() != reflect.Struct {
		return
	}
	for i := range fieldType.NumField() {
		field := fieldType.Field(i)
		tagValue := field.Tag.Get("json")
		tagName, _, _ := strings.Cut(tagValue, ",")
		if tagName == "-" {
			continue
		}
		if field.Anonymous && tagName == "" {
			appendJSONFieldNames(fieldMap, field.Type)
			continue
		}
		if tagName == "" {
			tagName = field.Name
		}
		fieldMap[tagName] = struct{}{}
	}
}
