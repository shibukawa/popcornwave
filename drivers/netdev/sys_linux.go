//go:build linux

package netdev

/*
#include <stdint.h>
#include <string.h>

// Prefer libc symbols from musl/glibc. TinyGo's Linux builds link a full libc.

int socket(int domain, int type, int protocol);
int bind(int fd, void *addr, unsigned addrlen);
int listen(int fd, int backlog);
int accept(int fd, void *addr, unsigned *addrlen);
int connect(int fd, void *addr, unsigned addrlen);
int setsockopt(int fd, int level, int optname, void *optval, unsigned optlen);
int getsockname(int fd, void *addr, unsigned *addrlen);
int getpeername(int fd, void *addr, unsigned *addrlen);
long read(int fd, void *buf, unsigned long n);
long write(int fd, void *buf, unsigned long n);
int close(int fd);
int fcntl(int fd, int cmd, int arg);
int select(int nfds, void *rfds, void *wfds, void *efds, void *timeout);

// errno location (glibc / musl)
extern int *__errno_location(void);
static int h_errno(void) { return *__errno_location(); }

static int h_select(int nfds, void *rfds, void *wfds, void *efds, void *timeout) {
	return select(nfds, rfds, wfds, efds, timeout);
}
*/
import "C"
import (
	"errors"
	"net/netip"
	"time"
	"unsafe"
)

const (
	osAF_INET      = 2
	osSOCK_STREAM  = 1
	osSOCK_DGRAM   = 2
	osIPPROTO_TCP  = 6
	osIPPROTO_UDP  = 17
	osSOL_SOCKET   = 1
	osSO_REUSEADDR = 2
	osSO_KEEPALIVE = 9
	osSO_LINGER    = 13
	osSOL_TCP      = 6
	osTCP_KEEPINTVL = 5
)

// Linux sockaddr_in: sin_family is 16-bit, no length field.
type sockaddrInet4 struct {
	Family uint16
	Port   uint16
	Addr   [4]byte
	Zero   [8]byte
}

func htons(v uint16) uint16 {
	return v<<8 | v>>8
}

func ntohs(v uint16) uint16 { return htons(v) }

func toSockaddr(ip netip.AddrPort) (sockaddrInet4, error) {
	var sa sockaddrInet4
	sa.Family = osAF_INET
	sa.Port = htons(ip.Port())
	if !ip.Addr().IsValid() || ip.Addr().IsUnspecified() {
		return sa, nil
	}
	if !ip.Addr().Is4() {
		if ip.Addr().Is4In6() {
			sa.Addr = ip.Addr().As4()
			return sa, nil
		}
		return sa, ErrFamilyNotSupported
	}
	sa.Addr = ip.Addr().As4()
	return sa, nil
}

func fromSockaddr(sa sockaddrInet4) netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom4(sa.Addr), ntohs(sa.Port))
}

func lastErrno() error {
	e := int(C.h_errno())
	if e == 0 {
		return errors.New("syscall error")
	}
	return errors.New(errnoName(e))
}

func errnoName(e int) string {
	switch e {
	case 11: // EAGAIN
		return "resource temporarily unavailable"
	case 110: // ETIMEDOUT
		return "connection timed out"
	case 111: // ECONNREFUSED
		return "connection refused"
	case 104: // ECONNRESET
		return "connection reset by peer"
	case 98: // EADDRINUSE
		return "address already in use"
	default:
		return "syscall error"
	}
}

func sysSocket(domain, stype, proto int) (int, error) {
	var ostype, oproto int
	switch stype {
	case SOCK_STREAM:
		ostype = osSOCK_STREAM
	case SOCK_DGRAM:
		ostype = osSOCK_DGRAM
	default:
		return -1, ErrProtocolNotSupported
	}
	switch proto {
	case IPPROTO_TCP:
		oproto = osIPPROTO_TCP
	case IPPROTO_UDP:
		oproto = osIPPROTO_UDP
	case 0:
		oproto = 0
	default:
		return -1, ErrProtocolNotSupported
	}
	fd := int(C.socket(C.int(osAF_INET), C.int(ostype), C.int(oproto)))
	if fd < 0 {
		return -1, lastErrno()
	}
	return fd, nil
}

func sysBind(fd int, ip netip.AddrPort) error {
	sa, err := toSockaddr(ip)
	if err != nil {
		return err
	}
	if C.bind(C.int(fd), unsafe.Pointer(&sa), 16) != 0 {
		return lastErrno()
	}
	return nil
}

func sysListen(fd, backlog int) error {
	if C.listen(C.int(fd), C.int(backlog)) != 0 {
		return lastErrno()
	}
	return nil
}

func sysAccept(fd int) (int, netip.AddrPort, error) {
	var sa sockaddrInet4
	n := C.uint(16)
	nfd := int(C.accept(C.int(fd), unsafe.Pointer(&sa), &n))
	if nfd < 0 {
		return -1, netip.AddrPort{}, lastErrno()
	}
	return nfd, fromSockaddr(sa), nil
}

func sysConnect(fd int, ip netip.AddrPort) error {
	sa, err := toSockaddr(ip)
	if err != nil {
		return err
	}
	if C.connect(C.int(fd), unsafe.Pointer(&sa), 16) != 0 {
		return lastErrno()
	}
	return nil
}

func sysClose(fd int) error {
	if C.close(C.int(fd)) != 0 {
		return lastErrno()
	}
	return nil
}

func sysSend(fd int, buf []byte, flags int) (int, error) {
	n := int(C.write(C.int(fd), unsafe.Pointer(&buf[0]), C.ulong(len(buf))))
	if n < 0 {
		return -1, lastErrno()
	}
	return n, nil
}

func sysRecv(fd int, buf []byte, flags int) (int, error) {
	n := int(C.read(C.int(fd), unsafe.Pointer(&buf[0]), C.ulong(len(buf))))
	if n < 0 {
		return -1, lastErrno()
	}
	return n, nil
}

func sysSetReuseAddr(fd int) error {
	one := C.int(1)
	if C.setsockopt(C.int(fd), osSOL_SOCKET, osSO_REUSEADDR, unsafe.Pointer(&one), 4) != 0 {
		return lastErrno()
	}
	return nil
}

func sysSetSockOpt(fd int, level, opt int, value interface{}) error {
	osLevel, osOpt, ok := mapSockOpt(level, opt)
	if !ok {
		return ErrNotSupported
	}
	switch v := value.(type) {
	case bool:
		iv := C.int(0)
		if v {
			iv = 1
		}
		if C.setsockopt(C.int(fd), C.int(osLevel), C.int(osOpt), unsafe.Pointer(&iv), 4) != 0 {
			return lastErrno()
		}
		return nil
	case int:
		iv := C.int(v)
		if C.setsockopt(C.int(fd), C.int(osLevel), C.int(osOpt), unsafe.Pointer(&iv), 4) != 0 {
			return lastErrno()
		}
		return nil
	case float64:
		iv := C.int(v)
		if C.setsockopt(C.int(fd), C.int(osLevel), C.int(osOpt), unsafe.Pointer(&iv), 4) != 0 {
			return lastErrno()
		}
		return nil
	default:
		return ErrNotSupported
	}
}

func mapSockOpt(level, opt int) (int, int, bool) {
	switch level {
	case SOL_SOCKET:
		switch opt {
		case SO_KEEPALIVE:
			return osSOL_SOCKET, osSO_KEEPALIVE, true
		case SO_LINGER:
			return osSOL_SOCKET, osSO_LINGER, true
		}
	case SOL_TCP:
		switch opt {
		case TCP_KEEPINTVL:
			return osSOL_TCP, osTCP_KEEPINTVL, true
		}
	}
	return 0, 0, false
}

const fdSetSize = 1024
const fdBits = 64 // Linux __NFDBITS is typically 64 on 64-bit

type fdSet struct {
	bits [fdSetSize / fdBits]uint64
}

func (s *fdSet) set(fd int) {
	if fd < 0 || fd >= fdSetSize {
		return
	}
	s.bits[fd/fdBits] |= 1 << uint(fd%fdBits)
}

type timeval struct {
	Sec  int64
	Usec int64
}

func waitRead(fd int, deadline time.Time) error {
	return waitFD(fd, true, deadline)
}

func waitWrite(fd int, deadline time.Time) error {
	return waitFD(fd, false, deadline)
}

func waitFD(fd int, read bool, deadline time.Time) error {
	if deadline.IsZero() {
		return nil
	}
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrTimeout
		}
		var rfds, wfds fdSet
		var rptr, wptr unsafe.Pointer
		if read {
			rfds.set(fd)
			rptr = unsafe.Pointer(&rfds)
		} else {
			wfds.set(fd)
			wptr = unsafe.Pointer(&wfds)
		}
		tv := timeval{
			Sec:  int64(remaining / time.Second),
			Usec: int64((remaining % time.Second) / time.Microsecond),
		}
		n := C.h_select(C.int(fd+1), rptr, wptr, nil, unsafe.Pointer(&tv))
		if n > 0 {
			return nil
		}
		if n == 0 {
			return ErrTimeout
		}
		if int(C.h_errno()) == 4 { // EINTR
			continue
		}
		return lastErrno()
	}
}

func localIPv4() (netip.Addr, error) {
	fd, err := sysSocket(AF_INET, SOCK_DGRAM, IPPROTO_UDP)
	if err != nil {
		return netip.AddrFrom4([4]byte{127, 0, 0, 1}), nil
	}
	defer sysClose(fd)
	_ = sysConnect(fd, netip.AddrPortFrom(netip.AddrFrom4([4]byte{8, 8, 8, 8}), 53))
	var sa sockaddrInet4
	n := C.uint(16)
	if C.getsockname(C.int(fd), unsafe.Pointer(&sa), &n) != 0 {
		return netip.AddrFrom4([4]byte{127, 0, 0, 1}), nil
	}
	return fromSockaddr(sa).Addr(), nil
}
