package platform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type ForwardService struct {
	DataPlatformEndpoint string `yaml:"endpoint"`
}

func ForwardServiceInitialize(ctx *context.Context) (*ForwardService, error) {
	// load data_platform file
	content, err := os.ReadFile(dataPlatformFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}

	server := ForwardService{}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}

	return &server, nil
}

func (s ForwardService) Forward(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	grpclog.Infof("Forward recv raw request:%+v", r)

	parsedURL, err := url.Parse(r.URL.String())
	if err != nil {
		errmsg := fmt.Sprintf("Forward failed to parse url err:%v", err)
		grpclog.Error(errmsg)
		http.Error(w, errmsg, http.StatusInternalServerError)
	}

	forwardURL := fmt.Sprintf("http://%s%s", s.DataPlatformEndpoint, parsedURL.Path)
	if parsedURL.RawQuery != "" {
		forwardURL = fmt.Sprintf("%s?%s", forwardURL, parsedURL.RawQuery)
	}
	grpclog.Infof("Forward generate url:%v", forwardURL)

	forwardRequest, _ := http.NewRequestWithContext(*ctx, r.Method, forwardURL, r.Body)
	forwardResponse, err := http.DefaultClient.Do(forwardRequest)
	if err != nil {
		errmsg := fmt.Sprintf("Forward failed to request backend err:%+v", err)
		grpclog.Error(errmsg)
		http.Error(w, errmsg, http.StatusInternalServerError)
	}
	defer forwardResponse.Body.Close()

	grpclog.Infof("Forward response:%+v", forwardResponse)

	// deep copy response
	for key, values := range forwardResponse.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(forwardResponse.StatusCode)
	_, err = io.Copy(w, forwardResponse.Body)
	if err != nil {
		errmsg := fmt.Sprintf("Forward copy body faile rr:%+v", err)
		grpclog.Error(errmsg)
		http.Error(w, errmsg, http.StatusInternalServerError)
	}
}
