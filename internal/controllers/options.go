package controllers

const (
	LabelWorkerName = "worker-name"
)

type ControllerRunOptions struct {
	WorkerCount    int
	WorkerName     string
	NodeUtilsImage string
}
