package executor

type TaskInfoWithResource struct {
	Task interface{}
	Cpus float64
	Mem  float64
}
