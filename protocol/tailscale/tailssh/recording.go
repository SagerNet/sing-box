//go:build with_tailscale

package tailssh

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	gliderssh "github.com/sagernet/gliderssh"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/tailscale/sessionrecording"
	"github.com/sagernet/tailscale/tailcfg"
	"github.com/sagernet/tailscale/types/key"
)

type recordingRejectedError struct {
	message string
	cause   error
}

func (e *recordingRejectedError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.message
}

func (e *recordingRejectedError) Unwrap() error {
	return e.cause
}

func newConnID() string {
	random := make([]byte, 5)
	rand.Read(random)
	return fmt.Sprintf("ssh-conn-%s-%02x", time.Now().UTC().Format("20060102T150405"), random)
}

func (s *Server) startNewRecording(sessionCtx context.Context, cancel context.CancelCauseFunc, session gliderssh.Session, conn *sshConn, recorderList []netip.AddrPort, onFailure *tailcfg.SSHRecorderFailureAction) (*recording, error) {
	localBackend := s.tsnetServer.ExportLocalBackend()
	// Capture before any blocking call, in case the user switches mid-setup.
	nodeKey := localBackend.NodeKey()
	if nodeKey.IsZero() {
		return nil, E.New("ssh server is unavailable: no node key")
	}

	var window gliderssh.Window
	ptyReq, _, isPty := session.Pty()
	if isPty {
		window = ptyReq.Window
	}
	term := ptyReq.Term
	if term == "" {
		term = "xterm-256color"
	}

	now := time.Now()
	rec := &recording{
		start:        now,
		failOpen:     onFailure == nil || onFailure.TerminateSessionWithMessage == "",
		metadataOnly: session.Subsystem() != "",
	}

	// Tied to the server lifetime rather than the session, so the upload survives a
	// normal session close but the bounded recorder dial is still aborted on
	// Server.Close() instead of stalling shutdown for up to its 30s timeout. Finished
	// by closing rec.out.
	uploadCtx := s.serverCtx
	out, attempts, errChan, err := sessionrecording.ConnectToRecorder(uploadCtx, recorderList, localBackend.Dialer().UserDial)
	if err != nil {
		if onFailure != nil && onFailure.NotifyURL != "" && len(attempts) > 0 {
			eventType := tailcfg.SSHSessionRecordingFailed
			if onFailure.RejectSessionWithMessage != "" {
				eventType = tailcfg.SSHSessionRecordingRejected
			}
			s.notifyControl(uploadCtx, nodeKey, eventType, attempts, onFailure.NotifyURL, conn)
		}
		if onFailure != nil && onFailure.RejectSessionWithMessage != "" {
			s.logger.Error("recording: error starting recording (rejecting session): ", err)
			return nil, &recordingRejectedError{message: onFailure.RejectSessionWithMessage, cause: err}
		}
		s.logger.Warn("recording: error starting recording (failing open): ", err)
		return nil, nil
	}
	rec.out = out

	go func() {
		uploadErr := <-errChan
		if uploadErr == nil {
			select {
			case <-sessionCtx.Done():
				s.logger.Debug("recording: finished uploading recording")
				return
			default:
				uploadErr = E.New("recording upload ended before the SSH session")
			}
		}
		if onFailure != nil && onFailure.NotifyURL != "" && len(attempts) > 0 {
			lastAttempt := attempts[len(attempts)-1]
			lastAttempt.FailureMessage = uploadErr.Error()
			eventType := tailcfg.SSHSessionRecordingFailed
			if onFailure.TerminateSessionWithMessage != "" {
				eventType = tailcfg.SSHSessionRecordingTerminated
			}
			s.notifyControl(uploadCtx, nodeKey, eventType, attempts, onFailure.NotifyURL, conn)
		}
		if onFailure != nil && onFailure.TerminateSessionWithMessage != "" {
			s.logger.Error("recording: error uploading recording (closing session): ", uploadErr)
			cancel(&terminationError{message: onFailure.TerminateSessionWithMessage})
			return
		}
		s.logger.Warn("recording: error uploading recording (failing open): ", uploadErr)
	}()

	castHeader := sessionrecording.CastHeader{
		Version:      2,
		Width:        window.Width,
		Height:       window.Height,
		Timestamp:    now.Unix(),
		Command:      cmp.Or(session.Subsystem(), session.RawCommand()),
		Env:          map[string]string{"TERM": term},
		SSHUser:      conn.sshUser,
		LocalUser:    conn.localUser.Username,
		SrcNode:      strings.TrimSuffix(conn.node.Name(), "."),
		SrcNodeID:    conn.node.StableID(),
		ConnectionID: conn.connID,
	}
	if !conn.node.IsTagged() {
		castHeader.SrcNodeUser = conn.userProfile.LoginName
		castHeader.SrcNodeUserID = conn.node.User()
	} else {
		castHeader.SrcNodeTags = conn.node.Tags().AsSlice()
	}
	headerLine, err := json.Marshal(castHeader)
	if err != nil {
		return nil, err
	}
	headerLine = append(headerLine, '\n')
	_, err = rec.out.Write(headerLine)
	if err != nil {
		// Recorder closed the pipe from the watcher goroutine; surface that cause.
		if errors.Is(err, io.ErrClosedPipe) && sessionCtx.Err() != nil {
			return nil, context.Cause(sessionCtx)
		}
		return nil, err
	}
	return rec, nil
}

func (s *Server) notifyControl(ctx context.Context, nodeKey key.NodePublic, eventType tailcfg.SSHEventType, attempts []*tailcfg.SSHRecordingAttempt, notifyURL string, conn *sshConn) {
	request := tailcfg.SSHEventNotifyRequest{
		EventType:         eventType,
		ConnectionID:      conn.connID,
		CapVersion:        tailcfg.CurrentCapabilityVersion,
		NodeKey:           nodeKey,
		SrcNode:           conn.node.ID(),
		SSHUser:           conn.sshUser,
		LocalUser:         conn.localUser.Username,
		RecordingAttempts: attempts,
	}
	body, err := json.Marshal(request)
	if err != nil {
		s.logger.Warn("notifyControl: marshal request: ", err)
		return
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, notifyURL, bytes.NewReader(body))
	if err != nil {
		s.logger.Warn("notifyControl: create request: ", err)
		return
	}
	response, err := s.tsnetServer.ExportLocalBackend().DoNoiseRequest(httpRequest)
	if err != nil {
		s.logger.Warn("notifyControl: send noise request: ", err)
		return
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		s.logger.Warn("notifyControl: noise request returned status ", response.Status)
	}
}

type recording struct {
	start        time.Time
	failOpen     bool
	metadataOnly bool

	access sync.Mutex // guards out
	out    io.WriteCloser
}

func (r *recording) Close() error {
	r.access.Lock()
	defer r.access.Unlock()
	if r.out == nil {
		return nil
	}
	err := r.out.Close()
	r.out = nil
	return err
}

// Only output is wrapped; input is never recorded since it may contain passwords.
func (r *recording) writer(w io.Writer) io.Writer {
	if r == nil || r.metadataOnly {
		return w
	}
	return &loggingWriter{rec: r, target: w}
}

type loggingWriter struct {
	rec        *recording
	target     io.Writer
	failedOpen bool
}

func (l *loggingWriter) Write(p []byte) (int, error) {
	if !l.failedOpen {
		castLine, err := json.Marshal([]any{
			time.Since(l.rec.start).Seconds(),
			"o",
			string(p),
		})
		if err != nil {
			return 0, err
		}
		castLine = append(castLine, '\n')
		writeErr := l.writeCastLine(castLine)
		if writeErr != nil {
			if !l.rec.failOpen {
				return 0, writeErr
			}
			l.failedOpen = true
		}
	}
	return l.target.Write(p)
}

func (l *loggingWriter) writeCastLine(castLine []byte) error {
	l.rec.access.Lock()
	defer l.rec.access.Unlock()
	if l.rec.out == nil {
		return E.New("recording closed")
	}
	_, err := l.rec.out.Write(castLine)
	if err != nil {
		return E.Cause(err, "write recording")
	}
	return nil
}
