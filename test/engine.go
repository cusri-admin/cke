package test

import (
	"fmt"
	"strings"
	"sync"
)

//UseCase 测试用例
type UseCase func(ctx *Context) error

type caseNode struct {
	index   int
	name    string
	useCase UseCase
	ctx     *Context
}

type engine struct {
	sync.Mutex
	useNodes []*caseNode
}

var eng *engine = &engine{
	useNodes: make([]*caseNode, 0),
}

//AddCase 添加测试用例
func AddCase(index int, name string, uc UseCase) error {
	eng.Lock()
	defer eng.Unlock()
	for _, cn := range eng.useNodes {
		if cn.name == name {
			return fmt.Errorf("Add testing case error: case [%s] has existed", name)
		}
	}

	cns := make([]*caseNode, 0)
	for i, cn := range eng.useNodes {
		if cn.index < index {
			cns = append(cns, cn)
		} else {
			cns = append(cns, &caseNode{
				index:   index,
				name:    name,
				useCase: uc,
			})
			eng.useNodes = append(cns, eng.useNodes[i:]...)
			return nil
		}
	}
	eng.useNodes = append(eng.useNodes, &caseNode{
		index:   index,
		name:    name,
		useCase: uc,
	})
	return nil
}

//RunTestCase 运行所有测试用例
func RunTestCase(filePath string, cases string) int {
	return eng.runCases(filePath, cases)
}

//Stop 停止测试
func Stop() {
	eng.stop()
}

func (e *engine) runCases(filePath string, cases string) int {
	okCount := 0
	failCount := 0
	cs := strings.Split(cases, ",")
	for _, un := range eng.useNodes {
		flag := true
		//如果指定了用例名同时有这个用例，则执行
		if len(cs) > 0 {
			flag = false
			for _, cn := range cs {
				if cn == un.name {
					flag = true
					break
				}
			}
		}
		if flag {
			fmt.Println("========================================")
			fmt.Printf("== CASE: %s\n", un.name)
			fmt.Println("========================================")
			cfg, err := LoadConfig(filePath, un.name)
			if err != nil {
				fmt.Printf("Read config file error: %s\n", err.Error())
			} else {
				fmt.Printf("scheduler number: %d\n", len(cfg.SchConf))
				var err error
				un.ctx, err = newContext(cfg)
				if err != nil {
					fmt.Printf("Init case %s error: %s\n", un.name, err.Error())
				}
				err = un.useCase(un.ctx)
				if err != nil {
					fmt.Printf("Test case [%s] FAILED: %s\n", un.name, err.Error())
					failCount++
				} else {
					fmt.Printf("Test case [%s] PASSED\n", un.name)
					okCount++
				}
			}
		}
	}
	fmt.Printf("Test completed: PASSED %d,\tFAILED: %d\n", okCount, failCount)
	return failCount
}

func (e *engine) stop() {
	for _, un := range eng.useNodes {
		if un.ctx != nil {
			un.ctx.Stop()
		}
	}
}
