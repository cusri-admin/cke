package executor

type TaskResultState int

const (
	TASKRESULT_STARTING TaskResultState = 0
	TASKRESULT_RUNNING  TaskResultState = 1
	TASKRESULT_FINISHED TaskResultState = 2
	TASKRESULT_FAILED   TaskResultState = 3
)

func (s TaskResultState) String() string {
	switch s {
	case TASKRESULT_STARTING:
		return "TASKRESULT_STARTING"
	case TASKRESULT_RUNNING:
		return "TASKRESULT_RUNNING"
	case TASKRESULT_FINISHED:
		return "TASKRESULT_FINISHED"
	case TASKRESULT_FAILED:
		return "TASKRESULT_FAILED"
	default:
		return "UNKNOWN"
	}
}
