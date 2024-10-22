package wx_payment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pkusunjy/grpc-gateway/service/platform"
	"github.com/pkusunjy/openai-server-proto/wx_payment"
	"github.com/redis/go-redis/v9"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type WxPaymentServiceImpl struct {
	DataPlatformEndpoint string `yaml:"endpoint"`
	WxAppID              string `yaml:"wx_appid"`
	WxMchID              string `yaml:"wx_mchid"`
	WxMchAPIv3Key        string `yaml:"wx_mch_apiv3"`
	WxSecret             string `yaml:"wx_secret"`
	WxSerialNo           string `yaml:"wx_serial_no"`
	RedisClient          *redis.Client
	WxClient             *core.Client
	Platform             *platform.PlatformService
	wx_payment.UnimplementedWxPaymentServiceServer
}

func WxPaymentServiceInitialize(ctx *context.Context, platform *platform.PlatformService) (*WxPaymentServiceImpl, error) {
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
	// init redis client
	server.RedisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	server.WxClient = wxClient
	server.Platform = platform
	return &server, nil
}

func (server WxPaymentServiceImpl) Jsapi(ctx context.Context, req *wx_payment.JsApiRequest) (*wx_payment.JsApiResponse, error) {
	reqJson, _ := json.Marshal(req)
	grpclog.Infof("jsapi received request: %v", string(reqJson))

	openid := req.GetOpenid()

	// Add user to db
	// From integration test results, it seems that no additional check is needed
	// So just send a "save" request, and print response for logging and debugging
	saveCustomerReqBody, _ := json.Marshal(CustomerParam{
		MemberType: "0",
		UserName:   openid,
	})
	saveCustomerUrl := fmt.Sprintf("http://%s/utility-project/ysCustomer/save", server.DataPlatformEndpoint)
	saveCustomerRespBody, err := platform.DoHttpPost(saveCustomerUrl, saveCustomerReqBody)
	if err != nil {
		grpclog.Errorf("Error HttpPost, url:%v, reqBody:%v, error:%v", saveCustomerUrl, string(saveCustomerReqBody), err)
	}
	grpclog.Infof("save customer received response:%v", string(saveCustomerRespBody))

	// Generate out_trade_no, for order storange and wechat prepay request
	outTradeNo, err := GenRandomStr()
	if err != nil || outTradeNo == nil {
		grpclog.Errorf("generate out_trade_no failed out_trade_no:%+v, error:%v", outTradeNo, err)
		return nil, err
	}

	// Create an order to db
	ysOrderSaveReqBody, _ := json.Marshal(OrderParam{
		OrderCode: *outTradeNo,
		OrderType: req.DataPlatformOrderType,
		UserName:  openid,
	})
	ysOrderSaveUrl := fmt.Sprintf("http://%s/utility-project/ysOrder/save", server.DataPlatformEndpoint)
	ysOrderSaveRespBody, err := platform.DoHttpPost(ysOrderSaveUrl, ysOrderSaveReqBody)
	if err != nil {
		grpclog.Errorf("Error HttpPost, url:%v, reqBody:%v, error:%v", ysOrderSaveUrl, string(ysOrderSaveReqBody), err)
	}
	grpclog.Infof("save order received response:%v", string(ysOrderSaveRespBody))

	resp := wx_payment.JsApiResponse{}
	// If openid is in whitelist, he/she doesn't need to pay, so no notify will be called.
	// But he/she has access to functions that need payment.
	// This is a new attribute for users, therefore a new column should be added into the MySQL user-related table.
	// All user-related getter/setter interfaces should be added with extra logic to deal with these "special" users, if needed.
	// It sucks.
	if req.DataPlatformOrderType == 3 {
		// isMember := server.RedisClient.SIsMember(ctx, "mikiai_whitelist_user", openid)
		// if isMember == nil {
		// 	grpclog.Errorf("error exec smembers cmd")
		// 	return &resp, nil
		// }
		dbQueryData := platform.WhitelistUserData{OpenID: &openid}
		dbQueryRes, err := server.Platform.WhitelistMySqlQuery(&ctx, &dbQueryData)
		if err != nil {
			grpclog.Errorf("whitelist query openid: %v fail err:%v", dbQueryData.OpenID, err)
			return &resp, nil
		}
		grpclog.Infof("WhitelistMySqlQuery resp: %+v", dbQueryRes)
		if len(dbQueryRes) > 0 && dbQueryRes[0].Status != nil && *dbQueryRes[0].Status == 1 {
			now_unix := time.Now().Unix()
			is_free_user := false
			if dbQueryRes[0].Status != nil {
				is_free_user = is_free_user && *dbQueryRes[0].Status == 1
			}
			if dbQueryRes[0].AddedTime != nil && dbQueryRes[0].ExpirationTime != nil {
				is_free_user = is_free_user && (*dbQueryRes[0].AddedTime < uint64(now_unix) && uint64(now_unix) < *dbQueryRes[0].ExpirationTime)
			}
			if is_free_user {
				grpclog.Infof("openid:%v is in whitelist, order_type=3", openid)
				// Edit order db
				editOrderReqBody, _ := json.Marshal(OrderParam{
					OrderCode: *outTradeNo,
				})
				editOrderUrl := fmt.Sprintf("http://%s/utility-project/ysOrder/editOrderStatus", server.DataPlatformEndpoint)
				editOrderRespBody, err := platform.DoHttpPost(editOrderUrl, editOrderReqBody)
				if err != nil {
					grpclog.Errorf("Error HttpPost, url:%v, reqBody:%v, error:%v", editOrderUrl, string(editOrderReqBody), err)
				}
				grpclog.Infof("edit order received response:%v", string(editOrderRespBody))
				// Whitelist users don't need to create payment, so return an empty JsApiResponse
				return &resp, nil
			}
		}
	}

	// Create prepay_id
	amount := req.GetAmount()
	if len(openid) == 0 || amount == 0 {
		grpclog.Errorf("request params invalid, received openid:%v amount:%v", openid, amount)
		return nil, fmt.Errorf("openid: %s amount:%d", openid, amount)
	}
	svc := jsapi.JsapiApiService{Client: server.WxClient}
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
	resp = wx_payment.JsApiResponse{
		Timestamp: *prepayResp.TimeStamp,
		NonceStr:  *prepayResp.NonceStr,
		Package:   "prepay_id=" + *prepayResp.PrepayId,
		SignType:  *prepayResp.SignType,
		PaySign:   *prepayResp.PaySign,
	}
	respJson, _ := json.Marshal(&resp)
	grpclog.Infof("PrepayWithRequestPayment return response: %v", string(respJson))

	return &resp, nil
}
