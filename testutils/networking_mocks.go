package testutils

import (
	"time"
	"net"
	"bytes"
	"errors"
	"sync"
	"bufio"
	"net/http"
)

type mockAddr struct {
	network string
	value   string
}

func (addr *mockAddr) Network() string {
	return addr.network
}

func (addr *mockAddr) String() string {
	return addr.value
}

func NewAddr(network string, value string) net.Addr {
	return &mockAddr{
		network: network,
		value:   value,
	}
}

type MockConnection struct {
	ToRead        *bytes.Buffer
	Written       *bytes.Buffer
	LocalAddress  net.Addr
	RemoteAddress net.Addr
	Closed        bool
	Deadline      time.Time
	ReadDeadline  time.Time
	WriteDeadline time.Time

	mutex *sync.Mutex
}

func NewMockConnection() *MockConnection {
	return &MockConnection{
		ToRead:        new(bytes.Buffer),
		Written:       new(bytes.Buffer),
		LocalAddress:  NewAddr("tcp", "127.0.0.1"),
		RemoteAddress: NewAddr("tcp", "127.0.0.2"),
		mutex:         new(sync.Mutex),
	}
}

func (con *MockConnection) Read(b []byte) (n int, err error) {
	con.mutex.Lock()
	defer con.mutex.Unlock()
	if con.Closed {
		return 0, errors.New("Connection already closed.")
	}
	return con.ToRead.Read(b)
}

func (con *MockConnection) Write(b []byte) (n int, err error) {
	con.mutex.Lock()
	defer con.mutex.Unlock()
	if con.Closed {
		return 0, errors.New("Connection already closed.")
	}
	return con.Written.Write(b)
}

func (con *MockConnection) Close() error {
	con.mutex.Lock()
	defer con.mutex.Unlock()
	if con.Closed {
		return errors.New("Connection already closed.")
	}
	con.Closed = true
	return nil
}

func (con *MockConnection) LocalAddr() net.Addr {
	return con.LocalAddress
}

func (con *MockConnection) RemoteAddr() net.Addr {
	return con.RemoteAddress
}

func (con *MockConnection) SetDeadline(t time.Time) error {
	con.Deadline = t
	return nil
}

func (con *MockConnection) SetReadDeadline(t time.Time) error {
	con.ReadDeadline = t
	return nil
}

func (con *MockConnection) SetWriteDeadline(t time.Time) error {
	con.WriteDeadline = t
	return nil
}

type MockResponseWriter struct {
	Written     *bytes.Buffer
	Code        int
	header      http.Header
	Connections []*MockConnection
}

func (mrw *MockResponseWriter) Header() http.Header {
	return mrw.header
}

func (mrw *MockResponseWriter) Write(b []byte) (int, error) {
	return mrw.Written.Write(b)
}

func (mrw *MockResponseWriter) WriteHeader(code int) {
	mrw.Code = code
}

func (mrw *MockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	con := NewMockConnection()
	readWriter := bufio.NewReadWriter(bufio.NewReader(con), bufio.NewWriter(con))
	mrw.Connections = append(mrw.Connections, con)
	return con, readWriter, nil
}

func NewMockResponseWriter() *MockResponseWriter {
	return &MockResponseWriter{
		Written: new(bytes.Buffer),
		Code:    -1,
		header:  http.Header{},
	}
}
