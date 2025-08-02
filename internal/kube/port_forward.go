package kube

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func createDialer(kubeClient *KubeClient, podName *string) (httpstream.Dialer, error) {
	portforwardRequest := kubeClient.Client.
		CoreV1().
		RESTClient().
		Post().
		Resource("pods").
		Namespace(kubeClient.Namespace).
		Name(*podName).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(kubeClient.Config)
	if err != nil {
		return nil, err
	}

	spdyDialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", portforwardRequest.URL())
	tunnelingDialer, err := portforward.NewSPDYOverWebsocketDialer(portforwardRequest.URL(), kubeClient.Config)
	if err != nil {
		return nil, err
	}
	// First attempt tunneling (websocket) dialer, then fallback to spdy dialer.
	dialer := portforward.NewFallbackDialer(tunnelingDialer, spdyDialer, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	return dialer, nil
}

type PortTunnel struct {
	ErrorStream httpstream.Stream
	DataStream  httpstream.Stream
	StreamConn  httpstream.Connection
}

func PortForward(kubeClient *KubeClient, pod *v1.Pod, port uint16) (*PortTunnel, error) {
	requestId := uuid.New().String()
	dialer, err := createDialer(kubeClient, &pod.Name)
	if err != nil {
		return nil, err
	}

	streamConn, protocol, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		return nil, err
	}

	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, fmt.Sprintf("%d", port))
	headers.Set(v1.PortForwardRequestIDHeader, requestId)
	errorStream, err := streamConn.CreateStream(headers)
	if protocol != portforward.PortForwardProtocolV1Name {
		return nil, fmt.Errorf("Server responded with incorrect protocol: %q", protocol)
	}
	// We can close as only used for reading
	errorStream.Close()
	// TODO: Should probs handle error channel in a goroutine or smthng

	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := streamConn.CreateStream(headers)
	if err != nil {
		errorStream.Close()
		streamConn.RemoveStreams(errorStream)
		return nil, err
	}

	return &PortTunnel{
		ErrorStream: errorStream,
		DataStream:  dataStream,
		StreamConn:  streamConn,
	}, nil
}

func (openPort *PortTunnel) Close() {
	openPort.DataStream.Close()
	openPort.StreamConn.RemoveStreams(openPort.DataStream)

	openPort.ErrorStream.Close()
	openPort.StreamConn.RemoveStreams(openPort.ErrorStream)

	openPort.StreamConn.Close()
}
