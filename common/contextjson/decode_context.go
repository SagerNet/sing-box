package json

import "strconv"

type decodeContext struct {
	parent *decodeContext
	index  int
	key    string
}

func (d *decodeState) formatContext() string {
	var description string
	context := d.context
	var appendDot bool
	for context != nil {
		if appendDot {
			description = "." + description
		}
		if context.key != "" {
			description = context.key + description
			appendDot = true
		} else {
			description = "[" + strconv.Itoa(context.index) + "]" + description
			appendDot = false
		}
		context = context.parent
	}
	return description
}

type contextError struct {
	parent  error
	context string
	index   bool
}

func (c *contextError) Unwrap() error {
	return c.parent
}

func (c *contextError) Error() string {
	//goland:noinspection GoTypeAssertionOnErrors
	switch c.parent.(type) {
	case *contextError:
		return c.context + "." + c.parent.Error()
	default:
		return c.context + ": " + c.parent.Error()
	}
}
