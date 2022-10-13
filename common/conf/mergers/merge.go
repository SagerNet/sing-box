package mergers

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// MergeAs load input and merge as specified format into m
func MergeAs(formatName Format, input interface{}, m map[string]interface{}) error {
	f, found := mergersByName[formatName]
	if !found {
		return fmt.Errorf("format merger not found for: %s", formatName)
	}
	return f.Merge(input, m)
}

// Merge loads inputs and merges them into target
// it detects extension for merger selecting, or try all mergers
// if no extension found
func Merge(input interface{}, target map[string]interface{}) error {
	switch v := input.(type) {
	case string:
		err := mergeSingleFile(v, target)
		if err != nil {
			return err
		}
	case []string:
		for _, file := range v {
			err := mergeSingleFile(file, target)
			if err != nil {
				return err
			}
		}
	case []byte:
		err := mergeSingleFile(v, target)
		if err != nil {
			return err
		}
	case io.Reader:
		// read to []byte incase it tries different mergers
		bs, err := ioutil.ReadAll(v)
		if err != nil {
			return err
		}
		err = mergeSingleFile(bs, target)
		if err != nil {
			return err
		}
	default:
		return errors.New("unknow merge input type")
	}
	return nil
}

func mergeSingleFile(input interface{}, m map[string]interface{}) error {
	if file, ok := input.(string); ok {
		ext := getExtension(file)
		if ext != "" {
			lext := strings.ToLower(ext)
			f, found := mergersByExt[lext]
			if !found {
				return fmt.Errorf("unmergeable format extension: %s", ext)
			}
			return f.Merge(file, m)
		}
	}
	var errs []string
	// no extension, try all mergers
	for _, f := range mergersByName {
		if f.Name == FormatAuto {
			continue
		}
		err := f.Merge(input, m)
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Sprintf("[%s] %s", f.Name, err))
	}
	return fmt.Errorf("tried all mergers but failed for: \n\n%s\n\nreason:\n\n  %s", input, strings.Join(errs, "\n  "))
}

func getExtension(filename string) string {
	ext := filepath.Ext(filename)
	return strings.ToLower(ext)
}
