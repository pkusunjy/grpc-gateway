package doubao

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkusunjy/openai-server-proto/chat_completion"
	"google.golang.org/grpc/grpclog"
)

var (
	flagVoiceType = flag.String("voice_type", "zh_female_shuangkuaisisi_moon_bigtts", "voice_type")
	flagEncoding  = flag.String("encoding", "mp3", "encoding")
	flagEndpoint  = flag.String("endpoint", "wss://openspeech.bytedance.com/api/v3/tts/bidirection", "endpoint")
)

type TTSService struct {
	loc       *time.Location
	ossClient *oss.Client
}

func TTSServiceInitialize(ctx *context.Context) (*TTSService, error) {
	timeZoneName := "Asia/Shanghai"
	loc, _ := time.LoadLocation(timeZoneName)
	region := "cn-hangzhou"
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	return &TTSService{loc: loc, ossClient: oss.NewClient(cfg)}, nil
}

func (s *TTSService) TTS(ctx context.Context, req *chat_completion.ChatMessage) (*chat_completion.ChatMessage, error) {
	uid := req.GetUserid()
	text := req.GetContent()
	// 1. call api & save local audio file
	uniqId := fmt.Sprintf("%s_%d", uid, time.Now().UnixMilli())
	fileName, err := s.TTSImpl(uniqId, text)
	if err != nil {
		return nil, err
	}
	// 2. upload oss
	cur := time.Now().In(s.loc)
	pattern := cur.Format("2006/1/2")
	remoteFileName := fmt.Sprintf("%s/%s", pattern, fileName)
	_, err = s.ossClient.PutObjectFromFile(ctx, &oss.PutObjectRequest{
		Bucket: oss.Ptr("mikiai"),
		Key:    oss.Ptr(remoteFileName),
	}, fileName)
	if err != nil {
		grpclog.Warningf("failed to put object %v err: %v", fileName, err)
		return nil, err
	}
	// 3. generate presigned url
	getObjRequest := &oss.GetObjectRequest{
		Bucket: oss.Ptr("mikiai"),
		Key:    oss.Ptr(remoteFileName),
	}
	getObjResult, err := s.ossClient.Presign(ctx, getObjRequest)
	if err != nil {
		grpclog.Warningf("failed to get object presign %v", err)
		return nil, err
	}
	grpclog.Infof("presigned url for object %s, url: %s", remoteFileName, getObjResult.URL)
	// 4. delete local audio file
	err = os.Remove(fileName)
	if err != nil {
		grpclog.Warningf("failed to remove local file %v err: %v", fileName, err)
	}
	return &chat_completion.ChatMessage{Content: getObjResult.URL}, nil
}

func (s *TTSService) TTSImpl(uniqId string, text string) (string, error) {
	sessionId := uuid.New().String()
	header := NewAuthHeader(sessionId, "volc.seedasr.sauc.duration")

	conn, r, err := websocket.DefaultDialer.DialContext(context.Background(), *flagEndpoint, header)
	if err != nil {
		glog.Exit(r, err)
	}
	defer func() {
		err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			glog.Exit(err)
		}
		conn.Close()
	}()
	glog.Info("Connection established, Logid: ", r.Header.Get("x-tt-logid"))
	if err := StartConnection(conn); err != nil {
		glog.Exit(err)
	}
	msg, err := WaitForEvent(conn, MsgTypeFullServerResponse, EventType_ConnectionStarted)
	if err != nil {
		glog.Exit(msg, err)
	}
	defer func() {
		// ----------------finish connection----------------
		if err := FinishConnection(conn); err != nil {
			glog.Exit(err)
		}
		// ----------------wait connection finished----------------
		msg, err := WaitForEvent(conn, MsgTypeFullServerResponse, EventType_ConnectionFinished)
		if err != nil {
			glog.Exit(msg, err)
		}
	}()

	request := map[string]any{
		"user": map[string]any{
			"uid": sessionId,
		},
		"namespace": "BidirectionalTTS",
		"req_params": map[string]any{
			"speaker": *flagVoiceType,
			"audio_params": map[string]any{
				"format":           *flagEncoding,
				"sample_rate":      24000,
				"enable_timestamp": true,
			},
			"additions": func() string {
				str, _ := json.Marshal(map[string]any{
					"disable_markdown_filter": false,
				})
				return string(str)
			}(),
		},
	}

	startReq := map[string]any{
		"user":       request["user"],
		"event":      int(EventType_StartSession),
		"namespace":  request["namespace"],
		"req_params": request["req_params"],
	}
	payload, err := json.Marshal(&startReq)
	if err != nil {
		glog.Exit(err)
	}
	// ----------------start session----------------
	if err := StartSession(conn, payload, sessionId); err != nil {
		glog.Exit(err)
	}
	msg, err = WaitForEvent(conn, MsgTypeFullServerResponse, EventType_SessionStarted)
	if err != nil {
		glog.Exit(msg, err)
	}
	go func() {
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		for _, char := range text {
			request["req_params"].(map[string]any)["text"] = string(char)
			ttsReq := map[string]any{
				"user":       request["user"],
				"event":      int(EventType_TaskRequest),
				"namespace":  request["namespace"],
				"req_params": request["req_params"],
			}
			payload, err := json.Marshal(&ttsReq)
			if err != nil {
				glog.Exit(err)
			}
			// ----------------send task request----------------
			if err := TaskRequest(conn, payload, sessionId); err != nil {
				glog.Exit(err)
			}
			<-t.C
		}
		if err := FinishSession(conn, sessionId); err != nil {
			glog.Exit(err)
		}
	}()

	var audio []byte
	var fileName string
	for {
		msg, err := ReceiveMessage(conn)
		if err != nil {
			glog.Exit(msg, err)
		}
		switch msg.MsgType {
		case MsgTypeFullServerResponse:
		case MsgTypeAudioOnlyServer:
			audio = append(audio, msg.Payload...)
		default:
			glog.Exit(msg)
		}
		if msg.EventType == EventType_SessionFinished {
			break
		}
		if len(audio) == 0 {
			continue
		}
		fileName = "text_to_speech_" + uniqId + "." + string(*flagEncoding)
		if err := os.WriteFile(fileName, audio, 0644); err != nil {
			glog.Exit(err)
		}
		glog.Infof("audio received: %d, saved to %s", len(audio), fileName)
	}

	if len(fileName) == 0 {
		return "", errors.New("no audio received")
	}
	return fileName, nil
}
