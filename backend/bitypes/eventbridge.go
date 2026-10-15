package bitypes

import "time"

type EventBridgeEvent[Detail any] struct {
	Version    string    `json:"version"`
	Id         string    `json:"id"`
	DetailType string    `json:"detail-type"`
	Source     string    `json:"source"`
	Account    string    `json:"account"`
	Time       time.Time `json:"time"`
	Region     string    `json:"region"`
	Resources  []string  `json:"resources"`
	Detail     Detail    `json:"detail"`
}

type StepFunctionEvent struct {
	ExecutionArn    string   `json:"executionArn"`
	StateMachineArn string   `json:"stateMachineArn"`
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	StartDate       int64    `json:"startDate"`
	StopDate        int64    `json:"stopDate"`
	Input           string   `json:"input"`
	InputDetails    Included `json:"inputDetails"`
	Output          string   `json:"output"`
	OutputDetails   Included `json:"outputDetails"`
}

type Included struct {
	Included bool `json:"included"`
}
