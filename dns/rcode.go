package dns

import F "github.com/sagernet/sing/common/format"

const (
	RCodeSuccess        RCodeError = 0 // NoError
	RCodeFormatError    RCodeError = 1 // FormErr
	RCodeServerFailure  RCodeError = 2 // ServFail
	RCodeNameError      RCodeError = 3 // NXDomain
	RCodeNotImplemented RCodeError = 4 // NotImp
	RCodeRefused        RCodeError = 5 // Refused
)

type RCodeError uint16

func (e RCodeError) Error() string {
	switch e {
	case RCodeSuccess:
		return "success"
	case RCodeFormatError:
		return "format error"
	case RCodeServerFailure:
		return "server failure"
	case RCodeNameError:
		return "name error"
	case RCodeNotImplemented:
		return "not implemented"
	case RCodeRefused:
		return "refused"
	default:
		return F.ToString("unknown error: ", uint16(e))
	}
}
