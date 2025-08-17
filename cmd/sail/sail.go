package sail

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/mini-ninja-64/flotilla/internal/kube"
	"github.com/mini-ninja-64/flotilla/internal/ui"
	"github.com/mini-ninja-64/flotilla/internal/util"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
)

//TODO: write tests
//TODO: Add JSON/YAML output options

type PodHttpResponse struct {
	Response *http.Response
	Error    error
	Pod      *v1.Pod
	Body     []byte
}

// TODO: Maybe a `Closable“ interface is more go-ish 🤔
type ClientCloser = func()
type ClientFactory = func(*PodRequest) (*http.Client, ClientCloser, error)

type LengthWriter struct {
	currentLength uint64
	writeCallback func(increase uint64, currentLength uint64)
}

func (progressBuffer *LengthWriter) Write(bytes []byte) (int, error) {
	bytesLength := len(bytes)
	bytesLengthUnsigned := uint64(len(bytes))
	progressBuffer.currentLength += bytesLengthUnsigned

	progressBuffer.writeCallback(bytesLengthUnsigned, progressBuffer.currentLength)

	return bytesLength, nil
}

func NewLengthWriter(writeCallback func(increase uint64, currentLength uint64)) *LengthWriter {
	return &LengthWriter{
		writeCallback: writeCallback,
	}
}

func requestWithClient(clientFactory ClientFactory, request *PodRequest) (*http.Response, error) {
	httpClient, closer, err := clientFactory(request)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer()
	}

	return httpClient.Do(request.Request)
}

func requestsWithClient(clientFactory ClientFactory, requests []PodRequest) []*PodHttpResponse {
	var wgReq sync.WaitGroup
	requestCount := len(requests)
	responses := make([]*PodHttpResponse, requestCount)

	progressTrackers := ui.NewProgressTrackers()
	progressBars := make([]*ui.ProgressBar, requestCount)
	for i, request := range requests {
		url := request.Request.URL.String()
		subtitle := "(" + request.Request.Method + " " + url + ")"
		progressBars[i] = progressTrackers.AddProgressBar(request.Pod.Name, subtitle)
	}
	for idx, req := range requests {
		wgReq.Add(1)
		go func() {
			response, err := requestWithClient(clientFactory, &req)
			if err != nil {
				println(err.Error())
				wgReq.Done()
				return
			}
			defer response.Body.Close()
			index := uint64(idx)

			progressBars[index].SetText(response.Status)
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				progressBars[index].SetProgressState(ui.Success)
			} else if response.StatusCode >= 400 {
				progressBars[index].SetProgressState(ui.Failure)
			}

			contentLength := float64(response.ContentLength)
			if contentLength < 0 {
				// println("unknown content length")
				// TODO: Handle with spinner
			}

			bodyBuffer := NewLengthWriter(func(uint64, currentLength uint64) {
				progressBars[index].SetPercentage(float64(currentLength) / contentLength)
			})

			teeReader := io.TeeReader(response.Body, bodyBuffer)
			// TODO: use body
			body, _ := io.ReadAll(teeReader)
			responses[idx] = &PodHttpResponse{
				Body: body,
			}
			progressBars[index].SetContent(string(body))
			wgReq.Done()
		}()
	}

	progressTrackers.RunAsync()
	wgReq.Wait()

	progressTrackers.Finish()
	progressTrackers.Wait()

	return responses
}

type PodRequest struct {
	Pod     *v1.Pod
	Request *http.Request
}

func httpRequests(pods *v1.PodList, method, protocol string, port uint16, headers map[string]string, path string) ([]PodRequest, error) {
	requests := []PodRequest{}
	for _, pod := range pods.Items {
		podIP := pod.Status.PodIP
		url := fmt.Sprintf("%s://%s:%d%s", protocol, podIP, port, path)
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			return nil, err
		}

		for headerName, headerValue := range headers {
			req.Header.Add(headerName, headerValue)
		}
		requests = append(requests, PodRequest{
			Pod:     &pod,
			Request: req,
		})
	}
	return requests, nil
}

type SailArgs struct {
	Protocol    string
	ServiceName string
	Port        uint16
	Path        string
	Method      string
	Headers     map[string]string
}

func parseSailArgs(cmd *cobra.Command, args []string) (*SailArgs, error) {
	protocol, err := cmd.Flags().GetString("protocol")
	if err != nil {
		return nil, err
	}
	var port uint16
	if !cmd.Flags().Changed("port") {
		port, err = util.GetDefaultPortForProtocol(&protocol)
		if err != nil {
			return nil, err
		}
	} else {
		port, err = cmd.Flags().GetUint16("port")
		if err != nil {
			return nil, err
		}
	}
	method, err := cmd.Flags().GetString("method")
	if err != nil {
		return nil, err
	}
	headers, err := cmd.Flags().GetStringToString("header")
	if err != nil {
		return nil, err
	}

	return &SailArgs{
		Protocol:    protocol,
		ServiceName: args[0],
		Port:        port,
		Path:        args[1],
		Method:      method,
		Headers:     headers,
	}, nil
}

func Cmd() *cobra.Command {
	var sailCommand = &cobra.Command{
		Use:   "sail [service] [path]",
		Short: "Send a HTTP request to every pod in a service",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sailArgs, err := parseSailArgs(cmd, args)
			if err != nil {
				return err
			}
			kubeClient, err := kube.GetClientUsingFlags(cmd)
			if err != nil {
				return err
			}
			// TODO: Should have way to override this behaviour
			pods, service, _ := kube.GetPodsForService(cmd.Context(), kubeClient, &sailArgs.ServiceName)
			actualPort := sailArgs.Port
			for _, port := range service.Spec.Ports {
				if port.Port == int32(sailArgs.Port) {
					actualPort = uint16(port.TargetPort.IntVal)
					break
				}
			}
			requests, err := httpRequests(
				pods,
				sailArgs.Method,
				sailArgs.Protocol,
				actualPort,
				sailArgs.Headers,
				sailArgs.Path,
			)
			if err != nil {
				return err
			}

			// var responses []*PodHttpResponse
			if kubeClient.ClientType == kube.InCluster {
				inClusterHttpClientFactory := func(_ *PodRequest) (*http.Client, ClientCloser, error) {
					return http.DefaultClient, nil, nil
				}
				/*responses = */ requestsWithClient(inClusterHttpClientFactory, requests)
			} else {
				outOfClusterHttpClientFactory := func(podRequest *PodRequest) (*http.Client, ClientCloser, error) {
					portForward, err := kube.PortForward(kubeClient, podRequest.Pod, actualPort)
					if err != nil {
						return nil, nil, err
					}

					transport := &http.Transport{
						DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
							return kube.NewPodConn(podRequest.Pod, portForward.DataStream), nil
						},
					}
					client := http.Client{
						Transport: transport,
					}
					return &client, func() { portForward.Close() }, nil
				}
				/*responses = */ requestsWithClient(outOfClusterHttpClientFactory, requests)
			}
			// println(responses)
			// TODO: add proper ui instead of temporary printout
			// for _, response := range responses {
			// println(response == nil)
			// if response.Error != nil {
			// 	// println(response.Pod.Name + ": " + response.Error.Error())
			// } else {
			// 	// println(response.Pod.Name + ": " + strconv.Itoa(response.Response.StatusCode))
			// }
			// }
			return nil
		},
	}

	sailCommand.Flags().StringP("method", "m", "GET", "The HTTP Method to use")
	// TODO: Should probs support duplicate headers, we currently do not oooops
	sailCommand.Flags().StringToStringP("header", "H", map[string]string{}, "The HTTP header to add in the form name=value")
	sailCommand.Flags().Uint16P("port", "p", 0, "The port to use for the reques (by default this is inferred from protocol)")
	sailCommand.Flags().StringP("protocol", "P", "http", "The protocol to use (http/https)")

	return sailCommand
}
