package statistic

import (
	"os"
	"strings"
)

//Environ : get the process' environ.
type Environ struct {
	EnvItems map[string]string
	EnvRaw   string `json:"raw_string"`
}

//EnvParse : parse the env item from raw string.
func envParse(strs []string) *map[string]string {
	res := make(map[string]string)
	for _, item := range strs {
		tmp := strings.Split(item, "=")
		res[tmp[0]] = tmp[1]
	}
	return &res
}

//GetEnv : get the env via os.envrion.
func GetEnv() *Environ {
	return &Environ{
		EnvItems: *envParse(os.Environ()),
		EnvRaw:   strings.Join(os.Environ(), " "),
	}
}
