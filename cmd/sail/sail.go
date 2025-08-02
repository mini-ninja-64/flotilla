package sail

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/mini-ninja-64/flotilla/internal/kube"
	"github.com/mini-ninja-64/flotilla/internal/util"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
)

//TODO: Should resolve svc port -> pod port as required
//TODO: write tests
//TODO: Add JSON/YAML output options

type PodHttpResponse struct {
	Response *http.Response
	Error    error
	Pod      *v1.Pod
}

// TODO: Maybe a `Closable“ interface is more go-ish 🤔
type ClientCloser = func()
type ClientFactory = func(*PodRequest) (*http.Client, ClientCloser, error)

func requestWithClient(clientFactory ClientFactory, request *PodRequest) *PodHttpResponse {
	httpClient, closer, err := clientFactory(request)
	if err != nil {
		return &PodHttpResponse{Error: err, Pod: request.Pod}
	}
	if closer != nil {
		defer closer()
	}

	response, err := httpClient.Do(request.Request)
	if err != nil {
		return &PodHttpResponse{Error: err, Pod: request.Pod}
	}
	return &PodHttpResponse{Response: response, Pod: request.Pod}
}

func requestsWithClient(clientFactory ClientFactory, requests []PodRequest) []*PodHttpResponse {
	var wg sync.WaitGroup
	responses := make([]*PodHttpResponse, len(requests))
	for idx, req := range requests {
		wg.Add(1)
		go func() {
			responses[idx] = requestWithClient(clientFactory, &req)
			wg.Done()
		}()
	}
	wg.Wait()
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
			pods, _ := kube.GetPodsForService(cmd.Context(), kubeClient, &sailArgs.ServiceName)
			requests, err := httpRequests(
				pods,
				sailArgs.Method,
				sailArgs.Protocol,
				sailArgs.Port,
				sailArgs.Headers,
				sailArgs.Path,
			)
			if err != nil {
				return err
			}

			var responses []*PodHttpResponse
			if kubeClient.ClientType == kube.InCluster {
				inClusterHttpClientFactory := func(_ *PodRequest) (*http.Client, ClientCloser, error) {
					return http.DefaultClient, nil, nil
				}
				responses = requestsWithClient(inClusterHttpClientFactory, requests)
			} else {
				outOfClusterHttpClientFactory := func(podRequest *PodRequest) (*http.Client, ClientCloser, error) {
					portForward, err := kube.PortForward(kubeClient, podRequest.Pod, sailArgs.Port)
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
				responses = requestsWithClient(outOfClusterHttpClientFactory, requests)
			}
			// TODO: add proper ui instead of temporary printout
			for _, response := range responses {
				if response.Error != nil {
					println(response.Pod.Name + ": " + response.Error.Error())
				} else if response.Response.Body == nil {
					println(response.Pod.Name + ": " + string(response.Response.StatusCode))
				} else {
					bodyBytes, _ := io.ReadAll(response.Response.Body)
					println(response.Pod.Name + ": " + string(bodyBytes))
				}
			}
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
