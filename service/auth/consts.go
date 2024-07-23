package auth

const (
	aliyunOssFile   = "./conf/aliyun_oss.yaml"
	wxPaymentFile   = "./conf/wx_payment.yaml"
	code2SessionUrl = "https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code"
)
