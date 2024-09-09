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
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

const (
	mysqlConf = "./conf/mysql.yaml"
	yyyymmdd  = "2006-01-02"
)

type PlatformService struct {
	DatabaseIP       string `yaml:"ip"`
	DatabasePort     string `yaml:"port"`
	DatabaseUser     string `yaml:"user"`
	DatabasePassword string `yaml:"password"`
	db               *sql.DB
	redisClient      *redis.Client
}

type WhitelistUserData struct {
	OpenID         *string `json:"openid,omitempty"`
	Name           *string `json:"name,omitempty"`
	AddedTime      *uint64 `json:"added_time,omitempty"`
	ExpirationTime *uint64 `json:"expiration_date,omitempty"`
	AddedBy        *string `json:"added_by,omitempty"`
	Status         *int8   `json:"status,omitempty"`
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
		server.DatabaseUser,
		server.DatabasePassword,
		server.DatabaseIP,
		server.DatabasePort,
		server.DatabaseUser,
	)
	grpclog.Infof("dataSourceName:%v", dataSourceName)
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
	// init redis client
	server.redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	return &server, nil
}

func (server PlatformService) Destroy() error {
	return server.db.Close()
}

func (server PlatformService) WhitelistMySqlInsert(ctx *context.Context, data *WhitelistUserData) (int64, error) {
	fields := "(openid"
	values := fmt.Sprintf("('%s'", *data.OpenID)
	if data.Name != nil {
		fields = fields + ", name"
		values = values + fmt.Sprintf(", '%s'", *data.Name)
	}
	if data.AddedTime != nil {
		fields = fields + ", added_time"
		values = values + fmt.Sprintf(", '%d'", *data.AddedTime)
	}
	if data.ExpirationTime != nil {
		fields = fields + ", expiration_date"
		values = values + fmt.Sprintf(", '%d'", *data.ExpirationTime)
	}
	if data.AddedBy != nil {
		fields = fields + ", added_by"
		values = values + fmt.Sprintf(", '%s'", *data.AddedBy)
	}
	if data.Status != nil {
		fields = fields + ", status"
		values = values + fmt.Sprintf(", '%d'", *data.Status)
	}
	fields = fields + ")"
	values = values + ")"
	execCmd := fmt.Sprintf("INSERT INTO whitelist_user %s VALUES %s;", fields, values)
	rs, err := server.db.ExecContext(*ctx, execCmd)
	if err != nil {
		grpclog.Errorf("exec insert failed error: %v", err)
		return 0, err
	}
	rowsAffected, err := rs.RowsAffected()
	if err != nil {
		grpclog.Errorf("get RowsAffected failed error: %v", err)
		return 0, err
	}
	grpclog.Infof("cmd: %v rows affected: %v", execCmd, rowsAffected)

	return rowsAffected, nil
}

func (server PlatformService) WhitelistMySqlUpdate(ctx *context.Context, data *WhitelistUserData) (int64, error) {
	execCmd := "UPDATE whitelist_user SET "
	if data.Name != nil {
		execCmd = execCmd + fmt.Sprintf("name = '%s' ", *data.Name)
	}
	if data.AddedTime != nil {
		execCmd = execCmd + fmt.Sprintf(", added_time = '%d' ", *data.AddedTime)
	}
	if data.ExpirationTime != nil {
		execCmd = execCmd + fmt.Sprintf(", expiration_date = '%d' ", *data.ExpirationTime)
	}
	if data.AddedBy != nil {
		execCmd = execCmd + fmt.Sprintf(", added_by = '%s' ", *data.AddedBy)
	}
	if data.Status != nil {
		execCmd = execCmd + fmt.Sprintf(", status = '%d' ", *data.Status)
	}
	execCmd = execCmd + fmt.Sprintf("WHERE openid = '%s';", *data.OpenID)
	grpclog.Infof("WhitelistMySqlUpdate cmd:%s", execCmd)

	rs, err := server.db.ExecContext(*ctx, execCmd)
	if err != nil {
		grpclog.Errorf("exec update failed error: %v", err)
		return 0, err
	}
	rowsAffected, err := rs.RowsAffected()
	if err != nil {
		grpclog.Errorf("get RowsAffected failed error: %v", err)
		return 0, err
	}
	grpclog.Infof("cmd: %v rows affected: %v", execCmd, rowsAffected)

	return rowsAffected, nil
}

func (server PlatformService) WhitelistMySqlQuery(ctx *context.Context, data *WhitelistUserData) ([]WhitelistUserData, error) {
	var queryCmd string
	if data.OpenID == nil {
		queryCmd = "SELECT * FROM whitelist_user;"
	} else {
		queryCmd = fmt.Sprintf("SELECT * FROM whitelist_user WHERE openid='%s';", *data.OpenID)
	}
	rows, err := server.db.QueryContext(*ctx, queryCmd)
	if err != nil {
		grpclog.Errorf("query context failed openid: %v error: %v", data.OpenID, err)
		return nil, err
	}
	defer rows.Close()

	var res []WhitelistUserData
	for rows.Next() {
		var tmp WhitelistUserData
		if err := rows.Scan(&tmp.OpenID, &tmp.Name, &tmp.AddedTime, &tmp.ExpirationTime, &tmp.AddedBy, &tmp.Status); err != nil {
			grpclog.Errorf("rows scan failed error: %v", err)
			return nil, err
		}
		res = append(res, tmp)
	}
	return res, nil
}

func (server PlatformService) WhitelistMySqlDelete(ctx *context.Context, data *WhitelistUserData) (int64, error) {
	execCmd := fmt.Sprintf("DELETE FROM whitelist_user WHERE openid='%s';", *data.OpenID)
	rs, err := server.db.ExecContext(*ctx, execCmd)
	if err != nil {
		grpclog.Errorf("exec delete failed error: %v", err)
		return 0, err
	}
	rowsAffected, err := rs.RowsAffected()
	if err != nil {
		grpclog.Errorf("get RowsAffected failed error: %v", err)
		return 0, err
	}
	grpclog.Infof("cmd: %v rows affected: %v", execCmd, rowsAffected)
	return rowsAffected, nil
}

func (server PlatformService) RedisSAdd(ctx *context.Context, w http.ResponseWriter, r *http.Request) {

	type RedisData struct {
		Key    string   `json:"key,omitempty"`
		Values []string `json:"values,omitempty"`
	}

	var data RedisData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	grpclog.Infof("Received request:%+v", data)
	key := "mikiai_whitelist_user"
	if len(data.Key) != 0 {
		key = data.Key
	}
	ret := server.redisClient.SAdd(*ctx, key, data.Values)
	if ret == nil {
		grpclog.Errorf("redis sadd failed")
		http.Error(w, "redis sadd failed", http.StatusBadRequest)
		return
	}
	resp := fmt.Sprintf("{\"res\":%v}", ret.Val())
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(resp))
}

func (server PlatformService) RedisSAddGet(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	openid := query.Get("openid")

	grpclog.Infof("Received openid:%+v", openid)
	key := "mikiai_whitelist_user"
	ret := server.redisClient.SAdd(*ctx, key, []string{openid})
	if ret == nil {
		grpclog.Errorf("redis sadd failed")
		http.Error(w, "redis sadd failed", http.StatusBadRequest)
		return
	}
	resp := fmt.Sprintf("{\"res\":%v}", ret.Val())
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(resp))
}

func (server PlatformService) RedisSMembers(ctx *context.Context, w http.ResponseWriter, r *http.Request) {

	type RedisData struct {
		Key    string   `json:"key,omitempty"`
		Values []string `json:"values,omitempty"`
	}

	var data RedisData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	grpclog.Infof("Received request:%+v", data)
	key := "mikiai_whitelist_user"
	if len(data.Key) != 0 {
		key = data.Key
	}
	whitelist := server.redisClient.SMembers(*ctx, key)
	if whitelist == nil {
		grpclog.Errorf("error exec smembers cmd")
		return
	}

	whitelistJsonObj, _ := json.Marshal(whitelist.Val())
	resp := fmt.Sprintf("{\"res\":%v}", string(whitelistJsonObj))
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(resp))
}

func (server PlatformService) RedisSRem(ctx *context.Context, w http.ResponseWriter, r *http.Request) {

	type RedisData struct {
		Key    string   `json:"key,omitempty"`
		Values []string `json:"values,omitempty"`
	}

	var data RedisData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	grpclog.Infof("Received request:%+v", data)
	key := "mikiai_whitelist_user"
	if len(data.Key) != 0 {
		key = data.Key
	}
	ret := server.redisClient.SRem(*ctx, key, data.Values)
	if ret == nil {
		grpclog.Errorf("error exec srem cmd")
		return
	}

	resp := fmt.Sprintf("{\"res\":%v}", ret.Val())
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(resp))
}
