package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

const (
	mysqlConf = "./conf/mysql.yaml"
	yyyymmdd  = "2006-01-02"
)

type PlatformService struct {
	databaseIP       string `yaml:"ip"`
	databasePort     string `yaml:"port"`
	databaseUser     string `yaml:"user"`
	databasePassword string `yaml:"password"`
	db               *sql.DB
}

func PlatformServiceInitialize(ctx *context.Context) (*PlatformService, error) {
	// load keys
	content, err := os.ReadFile(mysqlConf)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	server := PlatformService{}
	err = yaml.Unmarshal(content, &server)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	driverName := "mysql"
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		server.databaseUser,
		server.databasePassword,
		server.databaseIP,
		server.databasePort,
		server.databaseUser,
	)
	server.db, err = sql.Open(driverName, dataSourceName)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	if server.db == nil {
		grpclog.Fatal("db nullptr")
		return nil, err
	}
	server.db.SetConnMaxLifetime(time.Minute * 3)
	server.db.SetMaxOpenConns(10)
	server.db.SetMaxIdleConns(10)
	return &server, nil
}

func (server PlatformService) Destroy() error {
	return server.db.Close()
}

func (server PlatformService) GetExercisePool(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	grpclog.Infof("GetExercisePool triggered")
	// defer func() {
	// 	w.WriteHeader(http.StatusOK)
	// 	w.Header().Set("Access-Control-Allow-Origin", "*")
	// 	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	// 	w.Header().Set("Content-Type", "application/json")
	// }()
	resp := make(map[string]interface{})
	resp["status"] = 0
	resp["msg"] = "success"
	resp["data"] = map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"id":  1,
				"xxx": "3",
			},
			map[string]interface{}{
				"id":  2,
				"xxx": "4",
			},
			map[string]interface{}{
				"id":  3,
				"xxx": "5",
			},
		},
	}
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		grpclog.Fatalf("Error happened in JSON marshal, err: %s", err)
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
}
