package statistic

import (
	"os"
	"strings"
)

// Commands : command of one process.
type Commands struct {
	Nums         int               `json:"args_nums"`
	Args         map[string]string `json:"args"`
	StartCommand string            `json:"start_command"`
}

// ArgsParse : get args from the command line.
func (cmd *Commands) ArgsParse() {
	rawArgs := os.Args[:]
	argsMap := make(map[string]string)
	argName := ""
	for index, item := range rawArgs {
		if index == 0 {
			cmd.StartCommand = item
		} else if strings.HasPrefix(item, "-") {
			if strings.Contains(item, "=") {
				tmp := strings.Split(item, "=")
				argName = tmp[0]
				argsMap[argName] = tmp[1]
			} else {
				argName = item
				argsMap[argName] = ""
			}
		} else {
			argsMap[argName] = item
		}
	}
	cmd.Args = argsMap
}

// GetParams :get the start params of current process.
func GetParams() *Commands {
	cmd := Commands{}
	cmd.ArgsParse()
	cmd.Nums = len(cmd.Args)
	return &cmd
}
