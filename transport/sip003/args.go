package sip003

import (
	"bytes"
	"fmt"
)

// mod from https://github.com/shadowsocks/v2ray-plugin/blob/master/args.go

// Args maps a string key to a list of values. It is similar to url.Values.
type Args map[string][]string

// Get the first value associated with the given key. If there are any values
// associated with the key, the value return has the value and ok is set to
// true. If there are no values for the given key, value is "" and ok is false.
// If you need access to multiple values, use the map directly.
func (args Args) Get(key string) (value string, ok bool) {
	if args == nil {
		return "", false
	}
	vals, ok := args[key]
	if !ok || len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}

// Add Append value to the list of values for key.
func (args Args) Add(key, value string) {
	args[key] = append(args[key], value)
}

// Return the index of the next unescaped byte in s that is in the term set, or
// else the length of the string if no terminators appear. Additionally return
// the unescaped string up to the returned index.
func indexUnescaped(s string, term []byte) (int, string, error) {
	var i int
	unesc := make([]byte, 0)
	for i = 0; i < len(s); i++ {
		b := s[i]
		// A terminator byte?
		if bytes.IndexByte(term, b) != -1 {
			break
		}
		if b == '\\' {
			i++
			if i >= len(s) {
				return 0, "", fmt.Errorf("nothing following final escape in %q", s)
			}
			b = s[i]
		}
		unesc = append(unesc, b)
	}
	return i, string(unesc), nil
}

// ParsePluginOptions Parse a name–value mapping as from SS_PLUGIN_OPTIONS.
//
// "<value> is a k=v string value with options that are to be passed to the
// transport. semicolons, equal signs and backslashes must be escaped
// with a backslash."
// Example: secret=nou;cache=/tmp/cache;secret=yes
func ParsePluginOptions(s string) (opts Args, err error) {
	opts = make(Args)
	if len(s) == 0 {
		return
	}
	i := 0
	for {
		var key, value string
		var offset, begin int

		if i >= len(s) {
			break
		}
		begin = i
		// Read the key.
		offset, key, err = indexUnescaped(s[i:], []byte{'=', ';'})
		if err != nil {
			return
		}
		if len(key) == 0 {
			err = fmt.Errorf("empty key in %q", s[begin:i])
			return
		}
		i += offset
		// End of string or no equals sign?
		if i >= len(s) || s[i] != '=' {
			opts.Add(key, "1")
			// Skip the semicolon.
			i++
			continue
		}
		// Skip the equals sign.
		i++
		// Read the value.
		offset, value, err = indexUnescaped(s[i:], []byte{';'})
		if err != nil {
			return
		}
		i += offset
		opts.Add(key, value)
		// Skip the semicolon.
		i++
	}
	return opts, nil
}

// Escape backslashes and all the bytes that are in set.
func backslashEscape(s string, set []byte) string {
	var buf bytes.Buffer
	for _, b := range []byte(s) {
		if b == '\\' || bytes.IndexByte(set, b) != -1 {
			buf.WriteByte('\\')
		}
		buf.WriteByte(b)
	}
	return buf.String()
}
