# CKE 测试

&ensp;&ensp;&ensp;&ensp;CKE test使用定制开发的测试框架进行测试

## 测试框架

&ensp;&ensp;&ensp;&ensp;测试框架是使用go开发的独立进程。源代码在cke/test/，入口为cke/test/main/main.go，目标程序名build/bin/cke-test。cke-test启动后，测试框架会执行cke/test/cases目录下的各个测试用例(*.go文件内描述的测试过程)。可以不断丰富cke/test/cases目录下的测试用例。  

## 用例

&ensp;&ensp;&ensp;&ensp;每一个测试用例为在cke/test/cases目录下的一个go文件，在每个go文件应该通过init()函数将自己注册到测试框架中。例如，如下的代码将一个叫做"Case_A"的测试用例注册到框架中。

```golang
import (
  "cke/test"
)

func init() {
  test.AddCase(10, "Case_A", run)
}

func run(ctx *test.Context) error {
  ......
}
```

&ensp;&ensp;&ensp;&ensp;其中第一个参数为该用例的运行优先级越大优先级越低；第二个为用例名；第三个为执行该用例的入口函数其定义为:  
**//UseCase 测试用例**  
**type UseCase func(ctx *Context) error**  
&ensp;&ensp;&ensp;&ensp;当每个用例启动时，框架会根据init()函数中注册的用例名在工作目录(cke-test进程启动时在args的第一个参数中指定)中，查找对应名称的yaml(如：用例为"base_case",则配置文件为"<工作目录>/base_case.yaml")并加载。  
&ensp;&ensp;&ensp;&ensp;当测试框架运行后，会在适当时间调用函数run(在cke/test/engine.go中定义)。run函数的参数为辅助测试的test.Context。用户可以在run中定义自己的测试业务，使用test.Context创建集群、删除节点等。  

&ensp;&ensp;&ensp;&ensp;框架提供一些通用的工具来开始测试，具体可以参考已有用例代码。  

## 进行测试

在项目目录输入如下内容，将编译框架并开始测试

```bash
make test
```

也可以直接调用测试程序：  

```bash
build/bin/cke-test test/cases base_case,ha_base
```

cke-test的参数有2个：  
1.第一个是工作目录，cke-test会在这个目录中扫描配置文件  
2.第二个为可选项，指定将要运行的用例名，多个用例名用逗号分割  

## 例子

参见test/cases/base_case.go  
