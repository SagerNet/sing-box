package json

import (
	"bufio"
	"io"
)

// kanged from v2ray

type commentFilterState = byte

const (
	commentFilterStateContent commentFilterState = iota
	commentFilterStateEscape
	commentFilterStateDoubleQuote
	commentFilterStateDoubleQuoteEscape
	commentFilterStateSingleQuote
	commentFilterStateSingleQuoteEscape
	commentFilterStateComment
	commentFilterStateSlash
	commentFilterStateMultilineComment
	commentFilterStateMultilineCommentStar
)

type CommentFilter struct {
	br    *bufio.Reader
	state commentFilterState
}

func NewCommentFilter(reader io.Reader) io.Reader {
	return &CommentFilter{br: bufio.NewReader(reader)}
}

func (v *CommentFilter) Read(b []byte) (int, error) {
	p := b[:0]
	for len(p) < len(b)-2 {
		x, err := v.br.ReadByte()
		if err != nil {
			if len(p) == 0 {
				return 0, err
			}
			return len(p), nil
		}
		switch v.state {
		case commentFilterStateContent:
			switch x {
			case '"':
				v.state = commentFilterStateDoubleQuote
				p = append(p, x)
			case '\'':
				v.state = commentFilterStateSingleQuote
				p = append(p, x)
			case '\\':
				v.state = commentFilterStateEscape
			case '#':
				v.state = commentFilterStateComment
			case '/':
				v.state = commentFilterStateSlash
			default:
				p = append(p, x)
			}
		case commentFilterStateEscape:
			p = append(p, '\\', x)
			v.state = commentFilterStateContent
		case commentFilterStateDoubleQuote:
			switch x {
			case '"':
				v.state = commentFilterStateContent
				p = append(p, x)
			case '\\':
				v.state = commentFilterStateDoubleQuoteEscape
			default:
				p = append(p, x)
			}
		case commentFilterStateDoubleQuoteEscape:
			p = append(p, '\\', x)
			v.state = commentFilterStateDoubleQuote
		case commentFilterStateSingleQuote:
			switch x {
			case '\'':
				v.state = commentFilterStateContent
				p = append(p, x)
			case '\\':
				v.state = commentFilterStateSingleQuoteEscape
			default:
				p = append(p, x)
			}
		case commentFilterStateSingleQuoteEscape:
			p = append(p, '\\', x)
			v.state = commentFilterStateSingleQuote
		case commentFilterStateComment:
			if x == '\n' {
				v.state = commentFilterStateContent
				p = append(p, '\n')
			}
		case commentFilterStateSlash:
			switch x {
			case '/':
				v.state = commentFilterStateComment
			case '*':
				v.state = commentFilterStateMultilineComment
			default:
				p = append(p, '/', x)
			}
		case commentFilterStateMultilineComment:
			switch x {
			case '*':
				v.state = commentFilterStateMultilineCommentStar
			case '\n':
				p = append(p, '\n')
			}
		case commentFilterStateMultilineCommentStar:
			switch x {
			case '/':
				v.state = commentFilterStateContent
			case '*':
				// Stay
			case '\n':
				p = append(p, '\n')
			default:
				v.state = commentFilterStateMultilineComment
			}
		default:
			panic("Unknown state.")
		}
	}
	return len(p), nil
}
