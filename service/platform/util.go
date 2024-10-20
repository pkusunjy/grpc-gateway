package platform

import (
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/grpclog"
)

const (
	dataPlatformFile = "./conf/data_platform.yaml"
)

var (
	ForwardPathMethMap = map[string]string{
		"/utility-project/ysBsSetting/queryAppBooleanValue":    "GET",
		"/utility-project/ysCustomer/abtainDailyFreeUser":      "POST",
		"/utility-project/ysCustomer/accessToUseOrNo":          "POST",
		"/utility-project/ysCustomer/queryById":                "GET",
		"/utility-project/ysCustomer/queryByUsername":          "GET",
		"/utility-project/ysCustomer/queryDailyFreeUse":        "GET",
		"/utility-project/ysCustomer/queryUseTimeAndValidTime": "GET",
		"/utility-project/ysCustomer/save":                     "POST",
		"/utility-project/ysExam/queryById":                    "GET",
		"/utility-project/ysExam/queryExamByPaperId":           "POST",
		"/utility-project/ysExamAnswer/queryExamAnswerList":    "POST",
		"/utility-project/ysExamAnswer/saveExamAnswer":         "POST",
		"/utility-project/ysExperienceRecord/getByUserName":    "GET",
		"/utility-project/ysExperienceRecord/queryById":        "GET",
		"/utility-project/ysExperienceRecord/save":             "POST",
		"/utility-project/ysMemberConfig/queryById":            "GET",
		"/utility-project/ysOrder/editOrderStatus":             "POST",
		"/utility-project/ysOrder/queryById":                   "GET",
		"/utility-project/ysOrder/queryByUsername":             "GET",
		"/utility-project/ysOrder/save":                        "POST",
		"/utility-project/ysPaper/queryById":                   "GET",
		"/utility-project/ysPaper/queryPaperList":              "POST",
		"/utility-project/ysPaper/queryExamByPaperType":        "GET",
	}
)

func DoHttpPost(url string, reqBody []byte) ([]byte, error) {
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(reqBody)))
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		grpclog.Errorf("Error sending request:%v", err)
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		grpclog.Errorf("Error read resp:%v", err)
		return nil, err
	}
	return respBody, nil
}
