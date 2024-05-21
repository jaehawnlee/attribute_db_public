//go:build daemon_attr
// +build daemon_attr

package main

import (
	"attribute-db/logging"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaehawnlee/go-daemon"
	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
)

func main() {
	if _, err := os.Stat("log/"); os.IsNotExist(err) {
		if err := os.Mkdir("log/", os.ModePerm); err != nil {
			fmt.Println(err)
		}
	}

	cntxt := &daemon.Context{
		PidFileName: "attribute_db-daemon.pid",
		LogFilePerm: 0644,
		PidFilePerm: 0644,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"daemon"},
	}

	rl, err := rotatelogs.New("log/attribute_db-daemon.log-%Y-%m-%d", rotatelogs.WithRotationTime(time.Second*1), rotatelogs.WithMaxAge(time.Hour*24*30))
	if err != nil {
		fmt.Println(err)
		return
	}

	d, err := cntxt.Reborn()
	if err != nil {
		fmt.Println(err)
		logging.PrintERROR("-", "-", logging.DAEMON, "데몬 실행 실패 ", err.Error())
		return
	}

	if d != nil {
		return
	}

	defer cntxt.Release()

	log.SetOutput(rl)

	log.Print("- - - - - - - - - - - - - - -")
	logging.PrintINFO("-", "-", "DAEMON", "데몬 시작")

	appName := "attribute-db"
	defer logging.PrintINFO("-", "-", "DAEMON", appName+" 종료")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.PrintERROR("-", "-", logging.LOG, appName+" => "+err.Error())
			}
		}()
		for {
			<-time.After(time.Second * 1)
			if err := os.Chmod(rl.CurrentFileName(), 0644); err != nil {
				logging.PrintERROR("-", "-", logging.LOG, appName, " => ", err.Error())
			}
		}
	}()

	wg := make(chan bool)
	sigchnl := make(chan os.Signal, 1)
	signal.Notify(sigchnl, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	var cmd *exec.Cmd

	go func() {
		for {
			select {
			case sig := <-sigchnl:
				logging.PrintINFO("-", "-", "DAEMON", "Get Signal -> ", sig.String())
				if sig == syscall.SIGTERM {
					logging.PrintINFO("-", "-", "DAEMON", "자식 프로세스 상태", fmt.Sprintf("%+v", cmd))
					if cmd != nil {
						logging.PrintINFO("-", "-", "DAEMON", "자식 프로세스 종료")
						cmd.Process.Kill()
					}
					logging.PrintINFO("-", "-", "DAEMON", "데몬 종료")
					os.Exit(0)
				} else if sig == syscall.SIGINT {
					if cmd != nil {
						cmd.Process.Kill()
					}
					os.Exit(0)
				} else if sig == syscall.SIGKILL {
					if cmd != nil {

						cmd.Process.Kill()
					}
					os.Exit(0)
				} else {
					fmt.Println("Ignoring signal: ", sig)
				}
			}
		}
	}()

	for {
		go func() {
			defer func() {
				wg <- true
			}()

			logging.PrintINFO("-", "-", "DAEMON", appName+" 시작")
			cmd = exec.Command("./attribute-db", "") // replace with your command and arguments

			defer func() {
				if cmd != nil {
					cmd.Process.Kill()
				}
			}()

			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			err = cmd.Start()
			if err != nil {
				logging.PrintINFO("-", "-", "DAEMON", appName+" ", err.Error())
				return
			}

			logging.PrintINFO("-", "-", "DAEMON", appName, fmt.Sprint(cmd.Process.Pid), " 대기")
			err = cmd.Wait() // 커맨드가 종료될 때까지 대기
			if err != nil {
				logging.PrintINFO("-", "-", "DAEMON", appName+" ", err.Error())
				return
			}
		}()
		<-wg
		logging.PrintINFO("-", "-", "DAEMON", "매크로 재시작")
		<-time.After(time.Second * 3)
	}
}
