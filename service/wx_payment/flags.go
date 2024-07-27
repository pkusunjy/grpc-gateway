package wx_payment

import "flag"

var (
	apiClientKeyPath = flag.String("api_client_key_path", "/home/work/cert/apiclient_key.pem", "api_client_key_path")
	authFile         = flag.String("auth_file", "./conf/wx_payment.yaml", "auth_file")
	dataPlatformFile = flag.String("data_platform_file", "./conf/data_platform.yaml", "data_platform_file")
	notifyUrl        = flag.String("notify_url", "https://mikiai.tuyaedu.com:8124/wx_payment_notify/jsapi_notify_url", "notify_url")
)
