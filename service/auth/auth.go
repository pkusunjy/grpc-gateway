package auth

import (
	"context"
	"os"

	"github.com/pkusunjy/openai-server-proto/auth"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type AuthServiceImpl struct {
	AliyunOssEndpoint        string `yaml:"oss_endpoint"`
	AliyunOssAccessKeyID     string `yaml:"oss_access_key_id"`
	AliyunOssAccessKeySecret string `yaml:"oss_access_key_secret"`
	WxAppID                  string `yaml:"wx_appid"`
	WxSecret                 string `yaml:"wx_secret"`
	auth.UnimplementedAuthServiceServer
}

func AuthServiceInitialize(ctx *context.Context) (*AuthServiceImpl, error) {
	server := AuthServiceImpl{}
	// load aliyun conf
	content, err := os.ReadFile(aliyunOssFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	// load wx_payment conf
	content, err = os.ReadFile(wxPaymentFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	grpclog.Infof("initialized auth: %v", server)
	return &server, nil
}

func (server AuthServiceImpl) GetWxMiniprogramToken(ctx context.Context, req *auth.AuthRequest) (*auth.AuthResponse, error) {
	resp := auth.AuthResponse{
		Appid:  server.WxAppID,
		Secret: server.WxSecret,
	}
	return &resp, nil
}

func (server AuthServiceImpl) GetOssToken(ctx context.Context, req *auth.AuthRequest) (*auth.AuthResponse, error) {
	resp := auth.AuthResponse{
		OssEndpoint:        server.AliyunOssEndpoint,
		OssAccessKeyId:     server.AliyunOssAccessKeyID,
		OssAccessKeySecret: server.AliyunOssAccessKeySecret,
	}
	return &resp, nil
}
