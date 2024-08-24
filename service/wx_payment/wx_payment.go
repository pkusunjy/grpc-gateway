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
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type WxPaymentServiceImpl struct {
	DataPlatformEndpoint string   `yaml:"endpoint"`
	WxAppID              string   `yaml:"wx_appid"`
	WxMchID              string   `yaml:"wx_mchid"`
	WxMchAPIv3Key        string   `yaml:"wx_mch_apiv3"`
	WxSecret             string   `yaml:"wx_secret"`
	WxSerialNo           string   `yaml:"wx_serial_no"`
	WhitelistOpenIDs     []string `yaml:"whitelist_openids"`
	WhitelistOpenIDMap   map[string]struct{}
	WxClient             *core.Client
	wx_payment.UnimplementedWxPaymentServiceServer
}

func WxPaymentServiceInitialize(ctx *context.Context) (*WxPaymentServiceImpl, error) {
	// load keys
	content, err := os.ReadFile(*authFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
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
	// load whitelist
	content, err = os.ReadFile(*whitelistUserFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	server.WhitelistOpenIDMap = make(map[string]struct{})
	for _, openid := range server.WhitelistOpenIDs {
		server.WhitelistOpenIDMap[openid] = struct{}{}
	}
	grpclog.Infof("WhitelistOpenIDMap: %+v", server.WhitelistOpenIDMap)

	server.WxClient = wxClient
	return &server, nil
}

func (server WxPaymentServiceImpl) Jsapi(ctx context.Context, req *wx_payment.JsApiRequest) (*wx_payment.JsApiResponse, error) {
	openid := req.GetOpenid()
	amount := req.GetAmount()
	if len(openid) == 0 || amount == 0 {
		grpclog.Errorf("request params invalid, received openid:%v amount:%v", openid, amount)
		return nil, fmt.Errorf("openid: %s amount:%d", openid, amount)
	}
	reqJson, _ := json.Marshal(req)
	grpclog.Infof("received request: %v", string(reqJson))
	svc := jsapi.JsapiApiService{Client: server.WxClient}
	outTradeNo, err := GenRandomStr()
	if err != nil {
		grpclog.Error("generate out_trade_no failed error:", err)
		return nil, err
	}
	grpclog.Infof("received request: %v", string(reqJson))
	prepayResp, _, err := svc.PrepayWithRequestPayment(ctx,
		jsapi.PrepayRequest{
			Appid:       &server.WxAppID,
			Mchid:       &server.WxMchID,
			Description: core.String(jsapiDescription),
			OutTradeNo:  core.String(*outTradeNo),
			Attach:      core.String(jsapiAttach),
			NotifyUrl:   core.String(*notifyUrl),
			Amount: &jsapi.Amount{
				Total: core.Int64(int64(amount)),
			},
			Payer: &jsapi.Payer{
				Openid: core.String(openid),
			},
		},
	)
	if err != nil {
		grpclog.Error("call PrepayWithRequestPayment failed error:", err)
		return nil, err
	} else {
		grpclog.Info("call PrepayWithRequestPayment success")
	}
	resp := wx_payment.JsApiResponse{
		Timestamp: *prepayResp.TimeStamp,
		NonceStr:  *prepayResp.NonceStr,
		Package:   "prepay_id=" + *prepayResp.PrepayId,
		SignType:  *prepayResp.SignType,
		PaySign:   *prepayResp.PaySign,
	}
	respJson, _ := json.Marshal(&resp)
	grpclog.Infof("return response: %v", string(respJson))

	// add user
	jsonStr, _ := json.Marshal(CustomerParam{
		MemberType: "0",
		UserName:   openid,
	})
	saveCustomerUrl := fmt.Sprintf("http://%s/utility-project/ysCustomer/save", server.DataPlatformEndpoint)
	saveCustomerReq, _ := http.NewRequest("POST", saveCustomerUrl, strings.NewReader(string(jsonStr)))
	saveCustomerReq.Header.Add("Content-Type", "application/json")
	saveCustomerResp, err := http.DefaultClient.Do(saveCustomerReq)
	if err != nil {
		grpclog.Errorf("Error sending request:%v", err)
		return nil, err
	}
	defer saveCustomerResp.Body.Close()
	saveCustomerBody, err := io.ReadAll(saveCustomerResp.Body)
	if err != nil {
		grpclog.Errorf("Error read resp body:%v", err)
		return nil, err
	}
	grpclog.Infof("save customer received response:%v", string(saveCustomerBody))

	// save order
	jsonStr, _ = json.Marshal(OrderParam{
		OrderCode: *outTradeNo,
		OrderType: req.DataPlatformOrderType,
		UserName:  openid,
	})
	saveOrderUrl := fmt.Sprintf("http://%s/utility-project/ysOrder/save", server.DataPlatformEndpoint)
	saveOrderReq, _ := http.NewRequest("POST", saveOrderUrl, strings.NewReader(string(jsonStr)))
	saveOrderReq.Header.Add("Content-Type", "application/json")
	saveOrderResp, err := http.DefaultClient.Do(saveOrderReq)
	if err != nil {
		grpclog.Errorf("Error sending request:%v", err)
		return nil, err
	}
	defer saveOrderResp.Body.Close()

	saveOrderBody, err := io.ReadAll(saveOrderResp.Body)
	if err != nil {
		grpclog.Errorf("Error read resp body:%v", err)
		return nil, err
	}
	grpclog.Infof("save order received response:%v", string(saveOrderBody))

	// If openid is in whitelist, he/she doesn't need to pay, so no notify will be called.
	// But he/she has access to functions that need payment.
	// This is a new attribute for users, therefore a new column should be added into the MySQL user-related table.
	// All user-related getter/setter interfaces should be added with extra logic to deal with these "special" users, if needed.
	// It sucks.
	if _, exists := server.WhitelistOpenIDMap[openid]; exists {
		// edit backend order table
		jsonStr, _ := json.Marshal(OrderParam{
			OrderCode: *outTradeNo,
		})
		editOrderUrl := fmt.Sprintf("http://%s/utility-project/ysOrder/editOrderStatus", server.DataPlatformEndpoint)
		req, _ := http.NewRequest("POST", editOrderUrl, strings.NewReader(string(jsonStr)))
		req.Header.Add("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			grpclog.Errorf("Error sending request:%v", err)
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			grpclog.Errorf("Error read resp body:%v", err)
			return nil, err
		}
		grpclog.Infof("edit order received response:%v", string(body))
	}
	return &resp, nil
}
