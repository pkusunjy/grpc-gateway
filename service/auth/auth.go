package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func (server AuthServiceImpl) Jscode2Session(ctx context.Context, req *auth.AuthRequest) (*auth.AuthResponse, error) {
	url := fmt.Sprintf(code2SessionUrl, server.WxAppID, server.WxSecret, req.Code)
	grpclog.Infof("code2session gen url: %s", url)
	code2SessionResp, err := http.DefaultClient.Get(url)
	if err != nil {
		grpclog.Errorf("code2session get fail, err:%v", err)
		return nil, err
	}
	defer code2SessionResp.Body.Close()
	var wxMap map[string]string
	err = json.NewDecoder(code2SessionResp.Body).Decode(&wxMap)
	if err != nil {
		grpclog.Errorf("code2session json decode fail, err:%v", err)
		return nil, err
	}
	grpclog.Infof("code2session wxMap:%+v", wxMap)
	resp := auth.AuthResponse{
		Openid:     wxMap["openid"],
		SessionKey: wxMap["session_key"],
		Unionid:    wxMap["unionid"],
	}
	return &resp, nil
}
