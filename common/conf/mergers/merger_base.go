package mergers

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/sagernet/sing-box/common/conf/merge"
)

// Merger is a configurable format merger for V2Ray config files.
type Merger struct {
	Name       Format
	Extensions []string
	Merge      mergeFunc
}

// mergeFunc is a utility to merge the input into map[string]interface{}
type mergeFunc func(interface{}, map[string]interface{}) error

// mapFunc converts the input bytes of a config content to map[string]interface{}
type mapFunc func([]byte) (map[string]interface{}, error)

// makeMerger makes a merger who merge the format by converting it to JSON
func makeMerger(name Format, extensions []string, converter mapFunc) *Merger {
	return &Merger{
		Name:       name,
		Extensions: extensions,
		Merge:      makeMergeFunc(converter),
	}
}

// makeMergeFunc makes a merge func who merge the input to
func makeMergeFunc(converter mapFunc) mergeFunc {
	return func(input interface{}, target map[string]interface{}) error {
		if target == nil {
			panic("merge target is nil")
		}
		switch v := input.(type) {
		case string:
			err := loadFile(v, target, converter)
			if err != nil {
				return err
			}
		case []string:
			err := loadFiles(v, target, converter)
			if err != nil {
				return err
			}
		case []byte:
			err := loadBytes(v, target, converter)
			if err != nil {
				return err
			}
		case io.Reader:
			err := loadReader(v, target, converter)
			if err != nil {
				return err
			}
		default:
			return errors.New("unknow merge input type")
		}
		return nil
	}
}

func loadFiles(files []string, target map[string]interface{}, converter mapFunc) error {
	for _, file := range files {
		err := loadFile(file, target, converter)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadFile(file string, target map[string]interface{}, converter mapFunc) error {
	bs, err := loadToBytes(file)
	if err != nil {
		return fmt.Errorf("fail to load %s: %s", file, err)
	}
	return loadBytes(bs, target, converter)
}

func loadReader(reader io.Reader, target map[string]interface{}, converter mapFunc) error {
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	return loadBytes(bs, target, converter)
}

func loadBytes(bs []byte, target map[string]interface{}, converter mapFunc) error {
	m, err := converter(bs)
	if err != nil {
		return err
	}
	return merge.Maps(target, m)
}
