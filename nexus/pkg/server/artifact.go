package server

import (
	"github.com/wandb/wandb/nexus/pkg/service"
)

type ArtifactSaver struct {
	Artifact *service.ArtifactRecord
}

type ArtifactSaverResult struct {
	ArtifactId string
}

func (as *ArtifactSaver) save() ArtifactSaverResult {
	result := ArtifactSaverResult{
		ArtifactId: "artid",
	}
	return result
}
