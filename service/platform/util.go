package platform

const (
	dataPlatformFile = "./conf/data_platform.yaml"
)

var (
	ForwardPathMethMap = map[string]string{
		"/utility-project/ysMemberConfig/queryById":            "GET",
		"/utility-project/ysCustomer/abtainDailyFreeUser":      "POST",
		"/utility-project/ysCustomer/accessToUseOrNo":          "POST",
		"/utility-project/ysCustomer/queryById":                "GET",
		"/utility-project/ysCustomer/queryByUsername":          "GET",
		"/utility-project/ysCustomer/queryDailyFreeUse":        "GET",
		"/utility-project/ysCustomer/queryUseTimeAndValidTime": "GET",
		"/utility-project/ysCustomer/save":                     "POST",
		"/utility-project/ysExperienceRecord/getByUserName":    "GET",
		"/utility-project/ysExperienceRecord/queryById":        "GET",
		"/utility-project/ysExperienceRecord/save":             "POST",
		"/utility-project/ysOrder/queryById":                   "GET",
		"/utility-project/ysOrder/queryByUsername":             "GET",
		"/utility-project/ysOrder/save":                        "POST",
		"/utility-project/ysPaper/queryById":                   "GET",
		"/utility-project/ysPaper/queryPaperList":              "POST",
		"/utility-project/ysPaper/queryExamByPaperType":        "GET",
		"/utility-project/ysExam/queryById":                    "GET",
		"/utility-project/ysExam/queryExamByPaperId":           "POST",
	}
)
