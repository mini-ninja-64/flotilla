package kube

import (
	"net"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
)

type PodConn struct {
	dataStream httpstream.Stream
	pod        *v1.Pod
}

func NewPodConn(pod *v1.Pod, stream httpstream.Stream) net.Conn {
	return PodConn{
		dataStream: stream,
		pod:        pod,
	}
}

func (p PodConn) Close() error {
	return p.dataStream.Close()
}

func (p PodConn) LocalAddr() net.Addr {
	return nil
}

func (p PodConn) RemoteAddr() net.Addr {
	return nil
}

func (p PodConn) Read(b []byte) (n int, err error) {
	return p.dataStream.Read(b)
}

func (p PodConn) Write(b []byte) (n int, err error) {
	return p.dataStream.Write(b)
}

func (p PodConn) SetDeadline(t time.Time) error {
	return p.SetDeadline(t)
}

func (p PodConn) SetReadDeadline(t time.Time) error {
	return p.SetReadDeadline(t)
}

func (p PodConn) SetWriteDeadline(t time.Time) error {
	return p.SetWriteDeadline(t)
}
