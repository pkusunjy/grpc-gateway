package doubao

import "net/http"

func NewAuthHeader(reqID string, resourceID string) http.Header {
	header := http.Header{}
	header.Add("X-Api-Resource-Id", resourceID)
	header.Add("X-Api-Request-Id", reqID)
	header.Add("X-Api-Access-Key", AccessToken)
	header.Add("X-Api-App-Key", AppID)
	return header
}
