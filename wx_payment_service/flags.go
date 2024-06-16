package wxpaymentservice

import "flag"

var (
	apiClientKeyPath = flag.String("api_client_key_path", "/home/work/cert/apiclient_key.pem", "api_client_key_path")
	authFile         = flag.String("auth_file", "./conf/auth.yaml", "auth_file")
)
