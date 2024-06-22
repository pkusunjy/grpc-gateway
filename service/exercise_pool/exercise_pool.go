package exercise_pool

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkusunjy/openai-server-proto/exercise_pool"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v3"
)

type ExercisePoolServiceImpl struct {
	DatabaseIP       string `yaml:"ip"`
	DatabasePort     string `yaml:"port"`
	DatabaseUser     string `yaml:"user"`
	DatabasePassword string `yaml:"password"`
	db               *sql.DB
	exercise_pool.UnimplementedExercisePoolServiceServer
}

func ExercisePoolServiceInitialize(ctx *context.Context) (*ExercisePoolServiceImpl, error) {
	// load keys
	content, err := os.ReadFile(mysqlConf)
	if err != nil {
		grpclog.Fatal(err)
		return nil, err
	}
	server := ExercisePoolServiceImpl{}
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

func (server ExercisePoolServiceImpl) Destroy() error {
	return server.db.Close()
}

func (server ExercisePoolServiceImpl) Set(ctx context.Context, req *exercise_pool.ExercisePoolRequest) (*exercise_pool.ExercisePoolResponse, error) {
	scene := req.GetScene()
	items := req.GetItems()
	var resp exercise_pool.ExercisePoolResponse
	if items == nil {
		grpclog.Errorf("received request %v, input items empty", req)
		resp.ErrNo = 1
		resp.ErrMsg = "input items empty"
		return &resp, nil
	}

	tx, err := server.db.BeginTx(ctx, nil)
	if err != nil {
		grpclog.Errorf("sql db begin transaction failed")
		resp.ErrNo = 1
		resp.ErrMsg = "db BeginTx failed"
		return &resp, nil
	}
	defer tx.Rollback()

	for _, item := range items {
		title := item.GetTitle()
		// escape
		title = strings.ReplaceAll(title, "'", "\\'")
		title = strings.ReplaceAll(title, "\"", "\\\"")
		author := item.GetAuthor()
		// escape
		author = strings.ReplaceAll(author, "'", "\\'")
		author = strings.ReplaceAll(author, "\"", "\\\"")
		createTime := item.GetCreateTime()
		if createTime == 0 {
			createTime = uint64(time.Now().Unix())
		}
		expireTime := item.GetExpireTime()
		if expireTime == 0 {
			expireTime = uint64(time.Now().Unix() + 3600*24*365) // 1 year
		}
		for _, content := range item.Content {
			// escape
			content = strings.ReplaceAll(content, "'", "\\'")
			content = strings.ReplaceAll(content, "\"", "\\\"")
			execCmd := fmt.Sprintf("INSERT INTO exercise_pool VALUES (%d, '%s', '%s', '%s', '%s', '%s');",
				scene,
				title,
				content,
				author,
				time.Unix(int64(createTime), 0).Format(yyyymmdd),
				time.Unix(int64(expireTime), 0).Format(yyyymmdd),
			)
			rs, err := tx.ExecContext(ctx, execCmd)
			if err != nil {
				grpclog.Errorf("exec insert failed error: %v", err)
				continue
			}
			rowsAffected, err := rs.RowsAffected()
			if err != nil {
				grpclog.Errorf("get RowsAffected failed error: %v", err)
				continue
			}
			grpclog.Infof("cmd: %v rows affected: %v", execCmd, rowsAffected)
		}
	}
	if err := tx.Commit(); err != nil {
		grpclog.Errorf("tx Commit failed error: %v", err)
		resp.ErrNo = 1
		resp.ErrMsg = "tx commit failed " + err.Error()
		return &resp, nil
	}
	resp.ErrNo = 0
	resp.ErrMsg = "success"
	return &resp, nil
}

func (server ExercisePoolServiceImpl) Get(ctx context.Context, req *exercise_pool.ExercisePoolRequest) (*exercise_pool.ExercisePoolResponse, error) {
	var resp exercise_pool.ExercisePoolResponse
	scene := req.GetScene()
	if scene == exercise_pool.Scene_ILLEGAL {
		resp.ErrNo = 1
		resp.ErrMsg = "illegal scene"
		return &resp, nil
	}
	rows, err := server.db.QueryContext(ctx, "SELECT title, content, author FROM exercise_pool WHERE scene=?;", scene)
	if err != nil {
		grpclog.Errorf("query context failed scene: %v", scene)
		resp.ErrNo = 2
		resp.ErrMsg = "query context failed error: " + err.Error()
		return &resp, nil
	}
	defer rows.Close()

	tmp := make(map[string][]string)
	for rows.Next() {
		var title, content, author string
		if err := rows.Scan(&title, &content, &author); err != nil {
			grpclog.Errorf("rows scan failed error: %v", err)
			continue
		}
		// unescape
		title = strings.ReplaceAll(title, "\\'", "'")
		title = strings.ReplaceAll(title, "\\\"", "\"")
		content = strings.ReplaceAll(content, "\\'", "'")
		content = strings.ReplaceAll(content, "\\\"", "\"")
		author = strings.ReplaceAll(author, "\\'", "'")
		author = strings.ReplaceAll(author, "\\\"", "\"")
		if _, ok := tmp[title]; !ok {
			tmp[title] = []string{content}
		} else {
			tmp[title] = append(tmp[title], content)
		}
	}
	resp.ErrNo = 0
	resp.ErrMsg = "success"
	for k, v := range tmp {
		resp.Items = append(resp.Items, &exercise_pool.ExerciseItem{
			Title:   k,
			Content: v,
		})
	}
	return &resp, nil
}
