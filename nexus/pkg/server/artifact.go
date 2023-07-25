package server

type ArtifactSaver struct {
}

type ArtifactSaverResult struct {
	ArtifactId string
}

// do sending of messages to the server
func (as *ArtifactSaver) save() ArtifactSaverResult {
	result := ArtifactSaverResult{
		ArtifactId: "artid",
	}
	return result
}
