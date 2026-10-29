package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
)

const transitionPacketLimit = 256 << 10

// This private root-peer packet is physical coordination, not a Release,
// domain-completion or recovery proof. Exact from-birth scope, operation,
// nonce/sequence and original journal checks belong to the controller.
type transitionMessage struct {
	Schema        string `json:"schema"`
	Nonce         string `json:"nonce"`
	Sequence      uint32 `json:"sequence"`
	Operation     string `json:"operation"`
	Deadline      int64  `json:"deadline"`
	Journal       string `json:"journal"`
	Device        uint64 `json:"device"`
	Inode         uint64 `json:"inode"`
	PrefixSHA256  string `json:"prefix_sha256"`
	Records       uint32 `json:"records"`
	Generation    string `json:"generation"`
	BindingSHA256 string `json:"binding_sha256"`
	Manager       string `json:"manager"`
	// Byte strings preserve the canonical trailing LF of the original snapshots;
	// embedded RawMessage JSON would silently compact that committed preimage.
	OriginalContext []byte `json:"original_context"`
	NativeContext   []byte `json:"native_context"`
	Binding         []byte `json:"binding"`
	StartIntent     []byte `json:"start_intent,omitempty"`
}

func encodeTransitionMessage(message transitionMessage) ([]byte, error) {
	if len(message.OriginalContext) > 64<<10 || len(message.NativeContext) > 64<<10 || len(message.Binding) > 32<<10 || len(message.StartIntent) > 8<<10 {
		return nil, ErrInput
	}
	raw, err := json.Marshal(message)
	if err != nil || len(raw)+1 > transitionPacketLimit {
		return nil, errors.Join(ErrInput, err)
	}
	return append(raw, '\n'), nil
}

func decodeTransitionMessage(raw []byte) (transitionMessage, error) {
	var message transitionMessage
	if len(raw) == 0 || len(raw) > transitionPacketLimit || json.Unmarshal(raw, &message) != nil {
		return message, ErrBinding
	}
	canonical, err := encodeTransitionMessage(message)
	if err != nil || !bytes.Equal(raw, canonical) {
		return transitionMessage{}, errors.Join(ErrBinding, err)
	}
	return message, nil
}

func writeTransitionMessage(channel *net.UnixConn, message transitionMessage) error {
	if channel == nil {
		return ErrInput
	}
	raw, err := encodeTransitionMessage(message)
	if err != nil {
		return err
	}
	n, ancillary, err := channel.WriteMsgUnix(raw, nil, nil)
	if err != nil || n != len(raw) || ancillary != 0 {
		return errors.Join(io.ErrShortWrite, err)
	}
	return nil
}

func readTransitionMessage(channel *net.UnixConn, peer int32) (transitionMessage, error) {
	if channel == nil || peer <= 0 {
		return transitionMessage{}, ErrInput
	}
	raw := make([]byte, transitionPacketLimit+1)
	control := make([]byte, syscall.CmsgSpace(syscall.SizeofUcred)+syscall.CmsgSpace(16*4))
	n, cn, flags, _, readErr := channel.ReadMsgUnix(raw, control)
	messages, controlErr := syscall.ParseSocketControlMessage(control[:cn])
	// Even a refused/truncated recvmsg can install descriptors. Close every
	// delivered right before interpreting framing or credentials. Linux closes
	// rights that did not fit this bounded ancillary buffer.
	rights := false
	var closeErr error
	for _, message := range messages {
		if message.Header.Level == syscall.SOL_SOCKET && message.Header.Type == syscall.SCM_RIGHTS {
			rights = true
			fds, err := syscall.ParseUnixRights(&message)
			closeErr = errors.Join(closeErr, err)
			for _, fd := range fds {
				closeErr = errors.Join(closeErr, syscall.Close(fd))
			}
		}
	}
	if transitionOrderlyEOF(n, cn, flags, readErr, controlErr) {
		return transitionMessage{}, io.EOF
	}
	if readErr != nil || controlErr != nil || rights || closeErr != nil || n <= 0 || n > transitionPacketLimit || flags&(syscall.MSG_TRUNC|syscall.MSG_CTRUNC) != 0 || len(messages) != 1 {
		return transitionMessage{}, errors.Join(ErrBinding, readErr, controlErr, closeErr)
	}
	credentials, err := syscall.ParseUnixCredentials(&messages[0])
	if err != nil || credentials.Pid != peer || credentials.Uid != 0 || credentials.Gid != 0 {
		return transitionMessage{}, errors.Join(ErrBinding, err)
	}
	return decodeTransitionMessage(raw[:n])
}

// net.UnixConn uses MSG_CMSG_CLOEXEC on Linux; recvmsg may retain that requested
// descriptor-safety bit even on actual orderly closure. It carries no payload,
// truncation or independent failure. All other flags remain disqualifying.
// Recognize only that exact leaf (or the syscall's empty successful return),
// never a wrapper/group that may also carry a physical or cancellation failure.
func transitionOrderlyEOF(n, control, flags int, readErr, controlErr error) bool {
	return n == 0 && control == 0 && flags & ^syscall.MSG_CMSG_CLOEXEC == 0 && controlErr == nil && (readErr == nil || readErr == io.EOF)
}

func transitionSocketPair() (channel *net.UnixConn, endpoint *os.File, returned error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	parent, child := os.NewFile(uintptr(fds[0]), "original Forwarding transition controller"), os.NewFile(uintptr(fds[1]), "original Forwarding transition coordinator")
	defer func() {
		returned = errors.Join(returned, parent.Close())
		if returned != nil {
			if channel != nil {
				returned = errors.Join(returned, channel.Close())
			}
			returned = errors.Join(returned, child.Close())
			channel, endpoint = nil, nil
		}
	}()
	for _, fd := range fds {
		// The two raw bounded snapshots plus binding expand under base64. Retain
		// a finite native capacity for one complete largest private packet; a
		// kernel that cannot supply it is an invalid environment, never truncation.
		for _, option := range []int{syscall.SO_SNDBUF, syscall.SO_RCVBUF} {
			if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, option, 256<<10); err != nil {
				return nil, nil, err
			}
			capacity, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, option)
			if err != nil || capacity < transitionPacketLimit+32 {
				return nil, nil, errors.Join(ErrInput, err)
			}
		}
		if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1); err != nil {
			return nil, nil, err
		}
	}
	connection, err := net.FileConn(parent)
	if err != nil {
		return nil, nil, err
	}
	var ok bool
	channel, ok = connection.(*net.UnixConn)
	if !ok {
		return nil, nil, errors.Join(ErrBinding, connection.Close())
	}
	return channel, child, nil
}
