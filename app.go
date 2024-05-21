//go:build !daemon_attr
// +build !daemon_attr

package main

import (
	"attribute-db/config"
	db "attribute-db/db/attr"
	"attribute-db/db/levelDB"
	"attribute-db/logging"
	"attribute-db/s3"
	"fmt"
	"log"
	"time"

	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
)

func main() {
	rl, err := rotatelogs.New("log/attribute_DB.log-%Y-%m-%d", rotatelogs.WithRotationTime(time.Second*1), rotatelogs.WithMaxAge(time.Hour*24*30))
	if err != nil {
		fmt.Println(err)
		return
	}

	log.SetOutput(rl)

	log.Print("- - - - - - 속성 디비 서버 시작 - - - - - - -")

	if conf, err := config.OpenConfig(); err == nil {
		defer levelDB.CloseAll()
		fmt.Println("Init attr DB server...")
		storage := s3.NewS3(conf.DB.Bucket, conf.DB.Region, conf.NCloud.AccessKey, conf.NCloud.SecretKey, true)
		fmt.Println("db init")
		db.Init("http", 9004, *storage)
		fmt.Println("db run")
		for {
			time.Sleep(time.Second * 1)
		}
	} else {
		logging.PrintERROR("-", "-", logging.CONFIG, err.Error())
	}
}
