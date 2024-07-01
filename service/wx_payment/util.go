package wx_payment

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"google.golang.org/grpc/grpclog"
)

type CustomerParam struct {
	MemberType  string `json:"memberType,omitempty"`
	Nickname    string `json:"nickName,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	UserName    string `json:"username,omitempty"`
}

type OrderParam struct {
	OrderCode string `json:"orderCode,omitempty"`
	OrderType string `json:"orderType,omitempty"`
	UserName  string `json:"username,omitempty"`
}

func GenRandomStr() (*string, error) {
	file, err := os.Open("/dev/random")
	if err != nil {
		grpclog.Fatal("/dev/random not found")
		return nil, err
	}
	defer file.Close()
	buf := make([]byte, 16)
	_, err = file.Read(buf)
	if err != nil {
		grpclog.Fatalf("failed to read from /dev/random: %v", err)
		return nil, err
	}
	var ss bytes.Buffer
	for i := 0; i < 4; i++ {
		value := binary.BigEndian.Uint32(buf[i*4 : (i+1)*4])
		ss.WriteString(fmt.Sprintf("%08X", value))
	}
	res := ss.String()
	return &res, nil
}
