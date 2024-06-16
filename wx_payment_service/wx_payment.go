package wxpaymentservice

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/pkusunjy/openai-server-proto/wx_payment"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type WxPaymentServiceImpl struct {
	WxAppID       string `yaml:"wx_appid"`
	WxMchID       string `yaml:"wx_mchid"`
	WxMchAPIv3Key string `yaml:"wx_mch_apiv3"`
	WxSecret      string `yaml:"wx_secret"`
	WxSerialNo    string `yaml:"wx_serial_no"`
	WxClient      *core.Client
	wx_payment.UnimplementedWxPaymentServiceServer
}

func WxPaymentServiceInitialize(ctx *context.Context) (*WxPaymentServiceImpl, error) {
	// load keys
	content, err := os.ReadFile(*authFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	fmt.Println(string(content))
	server := WxPaymentServiceImpl{}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	// init wx client
	mchPrivateKey, err := utils.LoadPrivateKeyWithPath(*apiClientKeyPath)
	if err != nil {
		grpclog.Fatal("load merchant private key error: ", err)
		return nil, err
	}
	wxClient, err := core.NewClient(
		*ctx,
		option.WithWechatPayAutoAuthCipher(server.WxMchID, server.WxSerialNo, mchPrivateKey, server.WxMchAPIv3Key),
	)
	if err != nil {
		grpclog.Fatal("new wechat pay client error: ", err)
		return nil, err
	}

	server.WxClient = wxClient
	return &server, nil
}

func (server WxPaymentServiceImpl) Jsapi(ctx context.Context, req *wx_payment.JsApiRequest) (*wx_payment.JsApiResponse, error) {
	svc := jsapi.JsapiApiService{Client: server.WxClient}
	prepayResp, _, err := svc.PrepayWithRequestPayment(ctx,
		jsapi.PrepayRequest{
			Appid:       &server.WxAppID,
			Mchid:       &server.WxMchID,
			Description: core.String("MikiAi会员购买"),
			OutTradeNo:  core.String(req.GetOpenid() + strconv.FormatInt(time.Now().Unix(), 10)),
			Attach:      core.String("MikiAi会员购买"),
			NotifyUrl:   core.String("https://mikiai.tuyaedu.com:8124/wx_payment.NotifyService/jsapi_notify_url"),
			Amount: &jsapi.Amount{
				Total: core.Int64(1),
			},
			Payer: &jsapi.Payer{
				Openid: core.String(req.GetOpenid()),
			},
		},
	)
	if err != nil {
		grpclog.Error("call PrepayWithRequestPayment failed error:", err)
		return nil, err
	}
	resp := wx_payment.JsApiResponse{
		Timestamp: *prepayResp.TimeStamp,
		NonceStr:  *prepayResp.NonceStr,
		Package:   "prepay_id=" + *prepayResp.PrepayId,
		SignType:  *prepayResp.SignType,
		PaySign:   *prepayResp.PaySign,
	}
	return &resp, nil
}
