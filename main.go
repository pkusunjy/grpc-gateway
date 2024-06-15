package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"

	auth "github.com/pkusunjy/openai-server-proto/auth"
	chat "github.com/pkusunjy/openai-server-proto/chat_completion"
	pool "github.com/pkusunjy/openai-server-proto/exercise_pool"
	user "github.com/pkusunjy/openai-server-proto/user"
	wx_payment "github.com/pkusunjy/openai-server-proto/wx_payment"
)

var (
	// command-line options:
	grpcServerEndpoint = flag.String("grpc-server-endpoint", "localhost:8123", "gRPC server endpoint")
	certChain          = flag.String("cert-chain", "/home/work/cert/cert_chain.pem", "cert chain file")
	privKey            = flag.String("privkey", "/home/work/cert/privkey.key", "privkey")
	apiClientKeyPath   = flag.String("api_client_key_path", "/home/work/certs/apiclient_key.pem", "api_client_key_path")
	offlineModeLocal   = flag.Bool("is_offline_local", false, "whether enable ssl certification on gateway side")
	offlineModeGrpc    = flag.Bool("is_offline_grpc", false, "whether enable ssl certification between gateway and grpc")
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
	content, err := os.ReadFile("./conf/auth.yaml")
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
		grpclog.Fatal("call PrepayWithRequestPayment failed error:", err)
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

func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register gRPC server endpoint
	// Note: Make sure the gRPC server is running properly and accessible
	mux := runtime.NewServeMux()
	var opts []grpc.DialOption
	if *offlineModeGrpc {
		opts = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	} else {
		creds, err := credentials.NewClientTLSFromFile(*certChain, "")
		if err != nil {
			return err
		}
		opts = []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	}

	err := auth.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	err = chat.RegisterChatServiceHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	err = pool.RegisterExercisePoolServiceHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	err = user.RegisterUserServiceHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	err = wx_payment.RegisterNotifyServiceHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	// custom routes
	wxPaymentServer, err := WxPaymentServiceInitialize(&ctx)
	if err != nil {
		grpclog.Fatal("WxPaymentServiceInitialize failed error:", err)
		return err
	}
	err = wx_payment.RegisterWxPaymentServiceHandlerServer(ctx, mux, wxPaymentServer)
	if err != nil {
		return err
	}

	// Start HTTP server (and proxy calls to gRPC server endpoint)
	if *offlineModeLocal {
		return http.ListenAndServe(":8124", mux)
	} else {
		return http.ListenAndServeTLS(":8124", *certChain, *privKey, mux)
	}
}

func main() {
	flag.Parse()

	if err := run(); err != nil {
		grpclog.Fatal(err)
	}
}
