package controllers

import "fmt"

const (
	LabelWorkerName = "worker-name"
)

type ControllerRunOptions struct {
	WorkerCount              int
	WorkerName               string
	NodeUtilsImage           string
	DisableWebhooks          bool
	CosmoGuardImage          string
	ReleaseName              string
	DisruptionCheckEnabled   bool
	DisruptionMaxUnavailable int
}

func (opts *ControllerRunOptions) GetDefaultPriorityClassName() string {
	return fmt.Sprintf("%s-default", opts.ReleaseName)
}

func (opts *ControllerRunOptions) GetNodesPriorityClassName() string {
	return fmt.Sprintf("%s-nodes", opts.ReleaseName)
}

func (opts *ControllerRunOptions) GetValidatorsPriorityClassName() string {
	return fmt.Sprintf("%s-validators", opts.ReleaseName)
}
