package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/natefinch/lumberjack.v2"

	auth_service "github.com/pkusunjy/grpc-gateway/service/auth"
	exercise_pool_service "github.com/pkusunjy/grpc-gateway/service/exercise_pool"
	wx_payment_service "github.com/pkusunjy/grpc-gateway/service/wx_payment"
	auth_pb "github.com/pkusunjy/openai-server-proto/auth"
	chat_pb "github.com/pkusunjy/openai-server-proto/chat_completion"
	exercise_pool_pb "github.com/pkusunjy/openai-server-proto/exercise_pool"
	wx_payment_pb "github.com/pkusunjy/openai-server-proto/wx_payment"
)

const (
	LOGINFO = "../logs/gateway.log"
	LOGWF   = "../logs/gateway.log.wf"
)

var (
	// command-line options:
	grpcServerEndpoint = flag.String("grpc-server-endpoint", "localhost:8123", "gRPC server endpoint")
	certChain          = flag.String("cert-chain", "/home/work/cert/cert_chain.pem", "cert chain file")
	privKey            = flag.String("privkey", "/home/work/cert/privkey.key", "privkey")
	offlineModeLocal   = flag.Bool("is_offline_local", false, "whether enable ssl certification on gateway side")
	offlineModeGrpc    = flag.Bool("is_offline_grpc", false, "whether enable ssl certification between gateway and grpc")
)

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

	err := chat_pb.RegisterChatServiceHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	// generated routes
	authService, err := auth_service.AuthServiceInitialize(&ctx)
	if err != nil {
		grpclog.Fatal("AuthServiceInitialize failed error:", err)
		return err
	}
	if err = auth_pb.RegisterAuthServiceHandlerServer(ctx, mux, authService); err != nil {
		return err
	}

	exercisePoolServer, err := exercise_pool_service.ExercisePoolServiceInitialize(&ctx)
	if err != nil {
		grpclog.Fatal("ExercisePoolService failed error:", err)
		return err
	}
	err = exercise_pool_pb.RegisterExercisePoolServiceHandlerServer(ctx, mux, exercisePoolServer)
	if err != nil {
		return err
	}

	wxPaymentServer, err := wx_payment_service.WxPaymentServiceInitialize(&ctx)
	if err != nil {
		grpclog.Fatal("WxPaymentServiceInitialize failed error:", err)
		return err
	}
	err = wx_payment_pb.RegisterWxPaymentServiceHandlerServer(ctx, mux, wxPaymentServer)
	if err != nil {
		return err
	}

	// custom routes
	notifyServer, err := wx_payment_service.NotifyServiceInitialize(&ctx)
	if err != nil {
		grpclog.Fatal("WxPaymentNotifyServiceInitialize failed error:", err)
		return err
	}
	err = mux.HandlePath("POST", "/wx_payment.NotifyService/jsapi_notify_url", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		notifyServer.NotifyWxPayment(&ctx, w, r)
	})
	if err != nil {
		grpclog.Fatal("WxPaymentNotifyService HandlePath failed error:", err)
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

	lumberjackInfoLogger := &lumberjack.Logger{
		Filename:   LOGINFO,
		MaxSize:    200,
		MaxBackups: 7,
		MaxAge:     28,
		Compress:   true,
	}

	lumberjackWfLogger := &lumberjack.Logger{
		Filename:   LOGWF,
		MaxSize:    200,
		MaxBackups: 7,
		MaxAge:     28,
		Compress:   true,
	}

	grpclog.SetLoggerV2(grpclog.NewLoggerV2(lumberjackInfoLogger, lumberjackWfLogger, lumberjackWfLogger))

	if err := run(); err != nil {
		grpclog.Fatal(err)
	}
}
