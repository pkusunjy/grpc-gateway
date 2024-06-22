package wx_payment

import (
	"context"
	"fmt"
	"os"

	"github.com/pkusunjy/openai-server-proto/wx_payment"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type NotifyServiceImpl struct {
	WxAppID       string `yaml:"wx_appid"`
	WxMchID       string `yaml:"wx_mchid"`
	WxMchAPIv3Key string `yaml:"wx_mch_apiv3"`
	WxSecret      string `yaml:"wx_secret"`
	WxSerialNo    string `yaml:"wx_serial_no"`
	NotifyHandler *notify.Handler
	wx_payment.UnimplementedNotifyServiceServer
}

func NotifyServiceInitialize(ctx *context.Context) (*NotifyServiceImpl, error) {
	// load keys
	content, err := os.ReadFile(*authFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	fmt.Println(string(content))
	server := NotifyServiceImpl{}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	certificateVisitor := downloader.MgrInstance().GetCertificateVisitor(server.WxMchID)
	server.NotifyHandler = notify.NewNotifyHandler(
		server.WxMchAPIv3Key,
		verifiers.NewSHA256WithRSAVerifier(certificateVisitor),
	)
	return nil, nil
}
