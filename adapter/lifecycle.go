package adapter

import (
	"reflect"
	"strings"
	"time"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

type SimpleLifecycle interface {
	Start() error
	Close() error
}

type StartStage uint8

const (
	StartStateInitialize StartStage = iota
	StartStateStart
	StartStatePostStart
	StartStateStarted
)

var ListStartStages = []StartStage{
	StartStateInitialize,
	StartStateStart,
	StartStatePostStart,
	StartStateStarted,
}

func (s StartStage) String() string {
	switch s {
	case StartStateInitialize:
		return "initialize"
	case StartStateStart:
		return "start"
	case StartStatePostStart:
		return "post-start"
	case StartStateStarted:
		return "finish-start"
	default:
		panic("unknown stage")
	}
}

type Lifecycle interface {
	Start(stage StartStage) error
	Close() error
}

type LifecycleService interface {
	Name() string
	Lifecycle
}

func getServiceName(service any) string {
	if named, ok := service.(interface {
		Type() string
		Tag() string
	}); ok {
		tag := named.Tag()
		if tag != "" {
			return named.Type() + "[" + tag + "]"
		}
		return named.Type()
	}
	t := reflect.TypeOf(service)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return strings.ToLower(t.Name())
}

func Start(logger log.ContextLogger, stage StartStage, services ...Lifecycle) error {
	for _, service := range services {
		name := getServiceName(service)
		logger.Trace(stage, " ", name)
		startTime := time.Now()
		err := service.Start(stage)
		if err != nil {
			return err
		}
		logger.Trace(stage, " ", name, " completed (", F.Seconds(time.Since(startTime).Seconds()), "s)")
	}
	return nil
}

func StartNamed(logger log.ContextLogger, stage StartStage, services []LifecycleService) error {
	for _, service := range services {
		logger.Trace(stage, " ", service.Name())
		startTime := time.Now()
		err := service.Start(stage)
		if err != nil {
			return E.Cause(err, stage.String(), " ", service.Name())
		}
		logger.Trace(stage, " ", service.Name(), " completed (", F.Seconds(time.Since(startTime).Seconds()), "s)")
	}
	return nil
}
