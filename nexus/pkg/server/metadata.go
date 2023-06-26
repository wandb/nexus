package server

import (
	"encoding/json"
	"os"
	"time"

	"golang.org/x/exp/slog"
)

const MetaFilename string = "wandb-metadata.json"

type metaGitInfo struct {
	Remote string `json:"remote"`
	Commit string `json:"commit"`
}

type diskInfo struct {
	Total float64 `json:"total"`
	Used  float64 `json:"used"`
}

type gpuAppleInfo struct {
	GpuType string `json:"type"`
	Vendor  string `json:"vendor"`
}

type memInfo struct {
	Total float64 `json:"total"`
}

type Metadata struct {
	Os                string       `json:"os"`
	Python            string       `json:"python"`
	HeartbeatAt       time.Time    `json:"heartbeatAt"`
	StartedAt         time.Time    `json:"startedAt"`
	Docker            string       `json:"docker"`
	Cuda              string       `json:"cuda"`
	Args              []string     `json:"args"`
	State             string       `json:"state"`
	Program           string       `json:"program"`
	CodePath          string       `json:"codePath"`
	Git               metaGitInfo  `json:"git"`
	Email             string       `json:"email"`
	Root              string       `json:"root"`
	Host              string       `json:"host"`
	Username          string       `json:"username"`
	Executable        string       `json:"executable"`
	Cpu_count         int          `json:"cpu_count"`
	Cpu_count_logical int          `json:"cpu_count_logical"`
	Disk              diskInfo     `json:"disk"`
	Gpuapple          gpuAppleInfo `json:"gpuapple"`
	Memory            memInfo      `json:"memory"`
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

	_ = os.WriteFile(fileName, file, 0644)
}
