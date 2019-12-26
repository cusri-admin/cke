package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cke/test"
	_ "cke/test/cases"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Printf("Test Error: can't find config yaml file\n")
		os.Exit(1)
		return
	}

	filePath := os.Args[1]

	cases := ""
	if len(os.Args) >= 3 {
		cases = os.Args[2]
	}

	//创建监听退出chan
	exitChan := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(exitChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT, syscall.SIGKILL)

	go func() {
		for s := range exitChan {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL:
				test.Stop()
				os.Exit(1)
			}
		}
	}()

	exitCode := 0
	if test.RunTestCase(filePath, cases) != 0 {
		exitCode = 1
	}
	fmt.Printf("Process exit: %d\n", exitCode)

	os.Exit(exitCode)
}
