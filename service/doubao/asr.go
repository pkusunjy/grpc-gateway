package doubao

import (
	"context"
	"encoding/json"
	"flag"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/pkusunjy/openai-server-proto/chat_completion"
	"google.golang.org/grpc/grpclog"
)

var (
	asrUrl = flag.String("url", "https://openspeech.bytedance.com/api/v3/auc/bigmodel", "url")
)

type AsrService struct {
	loc       *time.Location
	ossClient *oss.Client
}

func AsrServiceInitialize(ctx *context.Context) (*AsrService, error) {
	timeZoneName := "Asia/Shanghai"
	loc, _ := time.LoadLocation(timeZoneName)
	region := "cn-hangzhou"
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	return &AsrService{loc: loc, ossClient: oss.NewClient(cfg)}, nil
}

func (s *AsrService) Whisper(ctx context.Context, req *chat_completion.ChatMessage) (*chat_completion.ChatMessage, error) {
	fileName := req.GetContent()
	grpclog.Infof("received filename: %v", fileName)
	// 1. get presigned url for audio file
	getObjRequest := &oss.GetObjectRequest{
		Bucket: oss.Ptr("mikiai"),
		Key:    oss.Ptr(fileName),
	}
	getObjResult, err := s.ossClient.Presign(ctx, getObjRequest)
	if err != nil {
		grpclog.Warningf("failed to get object presign %v", err)
		return nil, err
	}
	audioUrl := getObjResult.URL
	grpclog.Infof("presigned url for object %s, url: %s", fileName, audioUrl)
	// 2. call asr api
	c := NewAsrHttpClient(*asrUrl)
	asrRes, err := c.Excute(ctx, audioUrl)
	if err != nil {
		grpclog.Warningf("failed to excute: %v", err)
		return nil, err
	}
	debugInfo, _ := json.Marshal(*asrRes)
	grpclog.Infof("received filename: %v text: %v raw: %v", fileName, asrRes.Result.Text, string(debugInfo))

	return &chat_completion.ChatMessage{Content: asrRes.Result.Text}, nil
}
