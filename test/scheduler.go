package test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

type scheduler struct {
	sync.Mutex
	index int
	cmd   *exec.Cmd
	cfg   *SchConf
	chs   []chan []byte
}

func newScheduler(index int, cfg *SchConf) (*scheduler, error) {
	return &scheduler{
		index: index,
		cfg:   cfg,
	}, nil
}

func (s *scheduler) start() error {
	s.cmd = exec.Command(s.cfg.Path, s.cfg.Args...)
	//cmd.Env = execInfo.Envs
	//cmd.Stdin = strings.NewReader("some input")
	s.cmd.Stdout = s
	s.cmd.Stderr = s
	err := s.cmd.Start()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode := int32(ws.ExitStatus())
			return fmt.Errorf("exit code %d", exitCode)
		}
		return err
	}
	return nil
}

func (s *scheduler) runWaitting() int {
	err := s.cmd.Wait()
	if err != nil {
		fmt.Printf("Test error: %s\n", err.Error())
	}

	return s.cmd.ProcessState.ExitCode()
}

func (s *scheduler) stop() {
	s.cmd.Process.Signal(os.Interrupt)
}

func (s *scheduler) removeChannel(ch chan []byte) {
	s.Lock()
	defer s.Unlock()
	for i, c := range s.chs {
		if c == ch {
			s.chs = append(s.chs[:i], s.chs[i+1:]...)
			return
		}
	}
}

func (s *scheduler) Write(p []byte) (n int, err error) {
	out := string(p)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if len(line) > 0 {
			fmt.Printf("%d>%s\n", s.index, line)
		}
	}
	s.Lock()
	defer s.Unlock()
	chs := s.chs //复制一份用来扫描
	for _, ch := range chs {
		ch <- p
	}
	return len(p), nil
}

//Waiting 等待事件发生
// exp 事件(日志)发生的匹配的exp
// timeOut 超时时间(毫秒)
func (s *scheduler) waiting(exp string, ch chan []byte) {
	reg := regexp.MustCompile(exp)
	s.Lock()
	s.chs = append(s.chs, ch)
	s.Unlock()

	for {
		b, ok := <-ch
		if !ok {
			s.removeChannel(ch)
			return
		}
		if reg.MatchString(string(b)) {
			//删除这个触发器
			s.removeChannel(ch)
			return
		}
	}
}
