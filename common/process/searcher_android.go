package process

import (
	"context"
	"encoding/xml"
	"io"
	"net/netip"
	"os"
	"strconv"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/fsnotify/fsnotify"
)

var _ Searcher = (*androidSearcher)(nil)

type androidSearcher struct {
	logger        log.ContextLogger
	watcher       *fsnotify.Watcher
	userMap       map[string]int32
	packageMap    map[int32]string
	sharedUserMap map[int32]string
}

func NewSearcher(logger log.ContextLogger) (Searcher, error) {
	return &androidSearcher{logger: logger}, nil
}

func (s *androidSearcher) Start() error {
	err := s.updatePackages()
	if err != nil {
		return E.Cause(err, "read packages list")
	}
	err = s.startWatcher()
	if err != nil {
		s.logger.Warn("create fsnotify watcher: ", err)
	}
	return nil
}

func (s *androidSearcher) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add("/data/system/packages.xml")
	if err != nil {
		return err
	}
	s.watcher = watcher
	go s.loopUpdate()
	return nil
}

func (s *androidSearcher) loopUpdate() {
	for {
		select {
		case _, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			err := s.updatePackages()
			if err != nil {
				s.logger.Error(E.Cause(err, "update packages list"))
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error(E.Cause(err, "fsnotify error"))
		}
	}
}

func (s *androidSearcher) Close() error {
	return common.Close(common.PtrOrNil(s.watcher))
}

func (s *androidSearcher) FindProcessInfo(ctx context.Context, network string, srcIP netip.Addr, srcPort int) (*Info, error) {
	_, uid, err := resolveSocketByNetlink(network, srcIP, srcPort)
	if err != nil {
		return nil, err
	}
	if sharedUser, loaded := s.sharedUserMap[uid]; loaded {
		return &Info{
			UserId:      uid,
			PackageName: sharedUser,
		}, nil
	}
	if packageName, loaded := s.packageMap[uid]; loaded {
		return &Info{
			UserId:      uid,
			PackageName: packageName,
		}, nil
	}
	return &Info{UserId: uid}, nil
}

func (s *androidSearcher) updatePackages() error {
	userMap := make(map[string]int32)
	packageMap := make(map[int32]string)
	sharedUserMap := make(map[int32]string)
	packagesData, err := os.Open("/data/system/packages.xml")
	if err != nil {
		return err
	}
	decoder := xml.NewDecoder(packagesData)
	var token xml.Token
	for {
		token, err = decoder.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		element, isStart := token.(xml.StartElement)
		if !isStart {
			continue
		}

		switch element.Name.Local {
		case "package":
			var name string
			var userID int64
			for _, attr := range element.Attr {
				switch attr.Name.Local {
				case "name":
					name = attr.Value
				case "userId", "sharedUserId":
					userID, err = strconv.ParseInt(attr.Value, 10, 32)
					if err != nil {
						return err
					}
				}
			}
			if userID == 0 && name == "" {
				continue
			}
			userMap[name] = int32(userID)
			packageMap[int32(userID)] = name
		case "shared-user":
			var name string
			var userID int64
			for _, attr := range element.Attr {
				switch attr.Name.Local {
				case "name":
					name = attr.Value
				case "userId":
					userID, err = strconv.ParseInt(attr.Value, 10, 32)
					if err != nil {
						return err
					}
					packageMap[int32(userID)] = name
				}
			}
			if userID == 0 && name == "" {
				continue
			}
			sharedUserMap[int32(userID)] = name
		}
	}
	s.logger.Info("updated packages list: ", len(packageMap), " packages, ", len(sharedUserMap), " shared users")
	s.userMap = userMap
	s.packageMap = packageMap
	s.sharedUserMap = sharedUserMap
	return nil
}
