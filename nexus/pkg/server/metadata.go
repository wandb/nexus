package server

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"golang.org/x/exp/slog"
)

const MetaFilename string = "wandb-metadata.json"

type metaGitInfo struct {
	remote string `json:"remote"`
	commit string `json:"commit"`
}

type diskInfo struct {
	total float64 `json:"total"`
	used  float64 `json:"used"`
}

type gpuAppleInfo struct {
	gpuType string `json:"type"`
	vendor  string `json:"vendor"`
}

type memInfo struct {
	total float64 `json:"total"`
}

type Metadata struct {
	Os                string       `json:"os"`
	python            string       `json:"python"`
	heartbeatAt       time.Time    `json:"heartbeatAt"`
	startedAt         time.Time    `json:"startedAt"`
	docker            string       `json:"docker"`
	cuda              string       `json:"cuda"`
	args              []string     `json:"args"`
	State             string       `json:"state"`
	program           string       `json:"program"`
	codePath          string       `json:"codePath"`
	git               metaGitInfo  `json:"git"`
	email             string       `json:"email"`
	root              string       `json:"root"`
	host              string       `json:"host"`
	username          string       `json:"username"`
	executable        string       `json:"executable"`
	cpu_count         int          `json:"cpu_count"`
	cpu_count_logical int          `json:"cpu_count_logical"`
	disk              diskInfo     `json:"disk"`
	gpuapple          gpuAppleInfo `json:"gpuapple"`
	memory            memInfo      `json:"memory"`
}

func NewMetadata(meta Metadata) *Metadata {
	newMeta := meta
	newMeta.Os = "darwin"
	newMeta.State = "running"
	slog.Debug("Metadata: new")
	return &newMeta
}

func (m *Metadata) WriteFile(fileName string) {
	file, _ := json.MarshalIndent(m, "", "  ")

	_ = ioutil.WriteFile(fileName, file, 0644)
}
