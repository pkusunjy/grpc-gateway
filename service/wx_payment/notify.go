package wx_payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pkusunjy/openai-server-proto/wx_payment"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type NotifyServiceImpl struct {
	WxAppID              string `yaml:"wx_appid"`
	WxMchID              string `yaml:"wx_mchid"`
	WxMchAPIv3Key        string `yaml:"wx_mch_apiv3"`
	WxSecret             string `yaml:"wx_secret"`
	WxSerialNo           string `yaml:"wx_serial_no"`
	DataPlatformEndpoint string `yaml:"endpoint"`
	NotifyHandler        *notify.Handler
	wx_payment.UnimplementedNotifyServiceServer
}

func NotifyServiceInitialize(ctx *context.Context) (*NotifyServiceImpl, error) {
	server := NotifyServiceImpl{}
	// load wx payment file
	content, err := os.ReadFile(*authFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	// load data_platform file
	content, err = os.ReadFile(*dataPlatformFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
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

func (server NotifyServiceImpl) NotifyWxPayment(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	grpclog.Infof("notify wx_payment recv")
	defer func() {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
	}()
	// parse notify request
	content := payments.Transaction{}
	notifyReq, err := server.NotifyHandler.ParseNotifyRequest(*ctx, r, &content)
	if err != nil {
		grpclog.Errorf("ParseNotifyRequest failed error: %v", err)
		return
	}
	grpclog.Infof("notify summary: %v, content: %v", notifyReq.Summary, content)
	// write to backend
	// save customer
	jsonStr, _ := json.Marshal(CustomerParam{
		MemberType: "0",
		UserName:   *content.Payer.Openid,
	})
	saveCustomerUrl := fmt.Sprintf("http://%s/utility-project/ysCustomer/save", server.DataPlatformEndpoint)
	req, _ := http.NewRequest("POST", saveCustomerUrl, strings.NewReader(string(jsonStr)))
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		grpclog.Errorf("Error sending request:%v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		grpclog.Errorf("Error read resp body:%v", err)
		return
	}
	grpclog.Infof("save customer received response:%v", string(body))

	// save order
	jsonStr, _ = json.Marshal(OrderParam{
		OrderCode: *content.OutTradeNo,
		UserName:  *content.Payer.Openid,
	})
	saveOrderUrl := fmt.Sprintf("http://%s/utility-project/ysOrder/save", server.DataPlatformEndpoint)
	req, _ = http.NewRequest("POST", saveOrderUrl, strings.NewReader(string(jsonStr)))
	req.Header.Add("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		grpclog.Errorf("Error sending request:%v", err)
		return
	}
	defer resp2.Body.Close()

	body, err = io.ReadAll(resp2.Body)
	if err != nil {
		grpclog.Errorf("Error read resp body:%v", err)
		return
	}
	grpclog.Infof("save order received response:%v", string(body))
}
