package configdiff

import (
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
)

// InboundChange represents a change in inbound configuration
type InboundChange struct {
	Tag    string
	Type   string
	Action ChangeAction
	Old    option.Inbound
	New    option.Inbound
}

// OutboundChange represents a change in outbound configuration
type OutboundChange struct {
	Tag    string
	Type   string
	Action ChangeAction
	Old    option.Outbound
	New    option.Outbound
}

// EndpointChange represents a change in endpoint configuration
type EndpointChange struct {
	Tag    string
	Type   string
	Action ChangeAction
	Old    option.Endpoint
	New    option.Endpoint
}

type ChangeAction int

const (
	ActionAdd ChangeAction = iota
	ActionModify
	ActionRemove
	ActionUnchanged
)

// DiffInbounds compares old and new inbound configurations and returns the changes
func DiffInbounds(old, new []option.Inbound) []InboundChange {
	oldMap := make(map[string]option.Inbound)
	for _, inbound := range old {
		oldMap[inbound.Tag] = inbound
	}

	newMap := make(map[string]option.Inbound)
	for _, inbound := range new {
		newMap[inbound.Tag] = inbound
	}

	var changes []InboundChange

	// Check for removed inbounds
	for tag, oldInbound := range oldMap {
		if _, exists := newMap[tag]; !exists {
			changes = append(changes, InboundChange{
				Tag:    tag,
				Type:   oldInbound.Type,
				Action: ActionRemove,
				Old:    oldInbound,
			})
		}
	}

	// Check for added or modified inbounds
	for tag, newInbound := range newMap {
		oldInbound, exists := oldMap[tag]
		if !exists {
			changes = append(changes, InboundChange{
				Tag:    tag,
				Type:   newInbound.Type,
				Action: ActionAdd,
				New:    newInbound,
			})
		} else if !inboundsEqual(oldInbound, newInbound) {
			changes = append(changes, InboundChange{
				Tag:    tag,
				Type:   newInbound.Type,
				Action: ActionModify,
				Old:    oldInbound,
				New:    newInbound,
			})
		} else {
			changes = append(changes, InboundChange{
				Tag:    tag,
				Type:   newInbound.Type,
				Action: ActionUnchanged,
				Old:    oldInbound,
				New:    newInbound,
			})
		}
	}

	return changes
}

// DiffOutbounds compares old and new outbound configurations and returns the changes
func DiffOutbounds(old, new []option.Outbound) []OutboundChange {
	oldMap := make(map[string]option.Outbound)
	for _, outbound := range old {
		oldMap[outbound.Tag] = outbound
	}

	newMap := make(map[string]option.Outbound)
	for _, outbound := range new {
		newMap[outbound.Tag] = outbound
	}

	var changes []OutboundChange

	// Check for removed outbounds
	for tag, oldOutbound := range oldMap {
		if _, exists := newMap[tag]; !exists {
			changes = append(changes, OutboundChange{
				Tag:    tag,
				Type:   oldOutbound.Type,
				Action: ActionRemove,
				Old:    oldOutbound,
			})
		}
	}

	// Check for added or modified outbounds
	for tag, newOutbound := range newMap {
		oldOutbound, exists := oldMap[tag]
		if !exists {
			changes = append(changes, OutboundChange{
				Tag:    tag,
				Type:   newOutbound.Type,
				Action: ActionAdd,
				New:    newOutbound,
			})
		} else if !outboundsEqual(oldOutbound, newOutbound) {
			changes = append(changes, OutboundChange{
				Tag:    tag,
				Type:   newOutbound.Type,
				Action: ActionModify,
				Old:    oldOutbound,
				New:    newOutbound,
			})
		} else {
			changes = append(changes, OutboundChange{
				Tag:    tag,
				Type:   newOutbound.Type,
				Action: ActionUnchanged,
				Old:    oldOutbound,
				New:    newOutbound,
			})
		}
	}

	return changes
}

// DiffEndpoints compares old and new endpoint configurations and returns the changes
func DiffEndpoints(old, new []option.Endpoint) []EndpointChange {
	oldMap := make(map[string]option.Endpoint)
	for _, endpoint := range old {
		oldMap[endpoint.Tag] = endpoint
	}

	newMap := make(map[string]option.Endpoint)
	for _, endpoint := range new {
		newMap[endpoint.Tag] = endpoint
	}

	var changes []EndpointChange

	// Check for removed endpoints
	for tag, oldEndpoint := range oldMap {
		if _, exists := newMap[tag]; !exists {
			changes = append(changes, EndpointChange{
				Tag:    tag,
				Type:   oldEndpoint.Type,
				Action: ActionRemove,
				Old:    oldEndpoint,
			})
		}
	}

	// Check for added or modified endpoints
	for tag, newEndpoint := range newMap {
		oldEndpoint, exists := oldMap[tag]
		if !exists {
			changes = append(changes, EndpointChange{
				Tag:    tag,
				Type:   newEndpoint.Type,
				Action: ActionAdd,
				New:    newEndpoint,
			})
		} else if !endpointsEqual(oldEndpoint, newEndpoint) {
			changes = append(changes, EndpointChange{
				Tag:    tag,
				Type:   newEndpoint.Type,
				Action: ActionModify,
				Old:    oldEndpoint,
				New:    newEndpoint,
			})
		} else {
			changes = append(changes, EndpointChange{
				Tag:    tag,
				Type:   newEndpoint.Type,
				Action: ActionUnchanged,
				Old:    oldEndpoint,
				New:    newEndpoint,
			})
		}
	}

	return changes
}

// inboundsEqual checks if two inbounds are equal by comparing their JSON representation
func inboundsEqual(a, b option.Inbound) bool {
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// outboundsEqual checks if two outbounds are equal by comparing their JSON representation
func outboundsEqual(a, b option.Outbound) bool {
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// endpointsEqual checks if two endpoints are equal by comparing their JSON representation
func endpointsEqual(a, b option.Endpoint) bool {
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}
