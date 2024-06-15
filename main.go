package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
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
	offlineModeLocal   = flag.Bool("is_offline_local", false, "whether enable ssl certification on gateway side")
	offlineModeGrpc    = flag.Bool("is_offline_grpc", false, "whether enable ssl certification between gateway and grpc")
)

type AuthConf struct {
	WxAppID  string `yaml:"wx_appid"`
	WxMchID  string `yaml:"wx_mchid"`
	WxSecret string `yaml:"wx_secret"`
}

func loadYaml() AuthConf {
	content, err := os.ReadFile("./conf/auth.yaml")
	if err != nil {
		grpclog.Fatal(err)
	}
	fmt.Println(string(content))
	authConf := AuthConf{}
	err = yaml.Unmarshal(content, &authConf)
	if err != nil {
		grpclog.Fatal(err)
	}
	return authConf
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

	err = wx_payment.RegisterWxPaymentServiceHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
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

	authconf := loadYaml()
	fmt.Println(authconf)

	if err := run(); err != nil {
		grpclog.Fatal(err)
	}
}
