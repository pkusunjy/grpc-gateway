package report

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkusunjy/grpc-gateway/service/platform"
	"github.com/pkusunjy/openai-server-proto/chat_completion"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type ReportService struct {
	DataPlatformEndpoint string `yaml:"endpoint"`
	IeltsAiChatClient    chat_completion.ChatServiceClient
	chat_completion.UnimplementedReportServiceServer
}

func ReportServiceInitialize(ctx *context.Context) (*ReportService, error) {
	server := ReportService{}
	// load data_platform file
	content, err := os.ReadFile(dataPlatformFile)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	// 初始化IeltsAiChatClient
	conn, err := grpc.NewClient(grpcServerEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		grpclog.Fatalf("creating grpc new client failed, err:%+v", err)
		return nil, err
	}
	server.IeltsAiChatClient = chat_completion.NewChatServiceClient(conn)

	return &server, nil
}

func (server ReportService) IeltsTalkReport(ctx context.Context, req *chat_completion.QueryExamAnswerListRequest) (*chat_completion.TalkReport, error) {
	// 请求utility-project接口获取题目
	queryExamAnswerListReqBody, _ := json.Marshal(req)
	grpclog.Infof("IeltsTalkReport queryExamAnswerListReq:%+v", string(queryExamAnswerListReqBody))
	queryExamAnswerListUrl := fmt.Sprintf("http://%s/utility-project/ysExamAnswer/queryExamAnswerList", server.DataPlatformEndpoint)
	queryExamAnswerListRespBody, err := platform.DoHttpPost(queryExamAnswerListUrl, queryExamAnswerListReqBody)
	if err != nil {
		grpclog.Errorf("Error HttpPost, url:%v, reqBody:%v, error:%v", queryExamAnswerListUrl, string(queryExamAnswerListReqBody), err)
	}
	grpclog.Infof("queryExamAnswerList received response:%v", string(queryExamAnswerListRespBody))
	// 构造题目，请求grpc下游
	var responseBody chat_completion.QueryExamAnswerListResponse
	err = json.Unmarshal(queryExamAnswerListRespBody, &responseBody)
	if err != nil {
		grpclog.Errorf("json unmarshal failed error:%+v", err)
		return nil, err
	}
	grpclog.Infof("utility-project responseBody:%+v", &responseBody)
	// 判断utility-project返回接口是否异常
	if responseBody.GetData() == nil || len(responseBody.Data) == 0 {
		grpclog.Errorf("utility-project responseBody error, check the info log")
		return nil, fmt.Errorf("response body not valid")
	}
	// 精简utility-project接口，取真正需要的数据
	// utility-project可能返回m × n的Q&A列表，请求grpc服务的时候，拼接到一起
	examAnswerList := chat_completion.ExamAnswerList{}
	qaPairVec := make([]*chat_completion.QuestionAndAnswerPair, 0)
	for _, data := range responseBody.Data {
		qaPairVec = append(qaPairVec, data.AnswerList...)
	}
	examAnswerList.AnswerList = append(examAnswerList.AnswerList, qaPairVec...)
	// // 题库里没有答案，mock一下数据
	// examAnswerList.AnswerList[0].Answer = "No, I like to sleep."
	grpclog.Infof("grpc request:%+v", &examAnswerList)
	// 向grpc服务发起请求
	talkReport, err := server.IeltsAiChatClient.IeltsTalkReportImpl(ctx, &examAnswerList)
	if err != nil {
		grpclog.Errorf("gateway call grpc IeltsTalkReportImpl failed error:%+v", err)
		return nil, err
	}
	return talkReport, nil
}
