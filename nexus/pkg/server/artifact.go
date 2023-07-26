package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Khan/genqlient/graphql"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type ArtifactSaver struct {
	Ctx           context.Context
	Logger        *observability.NexusLogger
	Artifact      *service.ArtifactRecord
	GraphqlClient graphql.Client
}

type ArtifactSaverResult struct {
	ArtifactId string
}

type ManifestStoragePolicyConfig struct {
	StorageLayout string `json:"storageLayout"`
}

type ManifestEntry struct {
	Digest          string `json:"digest"`
	BirthArtifactID string `json:"birthArifactID"`
	Size            int64  `json:"size"`
}

type ManifestV1 struct {
	Version             int32                       `json:"version"`
	StoragePolicy       string                      `json:"storagePolicy"`
	StoragePolicyConfig ManifestStoragePolicyConfig `json:"storagePolicyConfig"`
	Contents            map[string]ManifestEntry    `json:"contents"`
}

func (as *ArtifactSaver) createArtifact() (string, *string) {
	enableDedup := false
	aliases := []ArtifactAliasInput{}
	for _, alias := range as.Artifact.Aliases {
		aliases = append(aliases,
			ArtifactAliasInput{
				ArtifactCollectionName: as.Artifact.Name,
				Alias:                  alias,
			},
		)
	}
	// fmt.Printf("SSSS: %+v\n", as.Artifact)
	data, err := CreateArtifact(
		as.Ctx,
		as.GraphqlClient,
		as.Artifact.Type,
		[]string{as.Artifact.Name},
		as.Artifact.Entity,
		as.Artifact.Project,
		&as.Artifact.RunId,
		&as.Artifact.Description,
		as.Artifact.Digest,
		nil, // Labels
		aliases,
		nil, // metadata
		// 0,   // historyStep
		// &as.Artifact.DistributedId,
		as.Artifact.ClientId,
		as.Artifact.SequenceClientId,
		&enableDedup, // enableDigestDeduplication
	)
	if err != nil {
		err = fmt.Errorf("createartifact: %s, error: %+v data: %+v", as.Artifact.Name, err, data)
		as.Logger.CaptureFatalAndPanic("Artifact saver error", err)
	}
	artifact := data.GetCreateArtifact().GetArtifact()
	latest := artifact.ArtifactSequence.GetLatestArtifact()

	var baseId *string
	if latest != nil {
		baseId = &latest.Id
	}
	// fmt.Printf("GOT RESP: %+v latest:%+v\n", artifact, baseId)
	return artifact.Id, baseId
}

func (as *ArtifactSaver) createManifest(artifactId string, baseArtifactId *string, manifestDigest string, includeUpload bool) string {
	manifestType := ArtifactManifestTypeFull
	manifestFilename := "wandb_manifest.json"

	// ---
	got, err := CreateArtifactManifest(
		as.Ctx,
		as.GraphqlClient,
		manifestFilename,
		manifestDigest,
		artifactId,
		baseArtifactId,
		as.Artifact.Entity,
		as.Artifact.Project,
		as.Artifact.RunId,
		includeUpload, // includeUpload
		&manifestType,
	)
	if err != nil {
		err = fmt.Errorf("artifact manifest: %s, error: %+v data: %+v", as.Artifact.Name, err, got)
		as.Logger.CaptureFatalAndPanic("Artifact saver error", err)
	}
	createManifest := got.GetCreateArtifactManifest()
	manifest := createManifest.ArtifactManifest
	// fmt.Printf("GOT ART MAN RESP: %+v %+v %+v\n", manifest, manifest.Id, manifest.File)
	return manifest.Id
}

func (as *ArtifactSaver) sendFiles(manifestId string) {
}

func (as *ArtifactSaver) sendManifest() {
	man := as.Artifact.Manifest

	m := &ManifestV1{
		Version:       man.Version,
		StoragePolicy: man.StoragePolicy,
		StoragePolicyConfig: ManifestStoragePolicyConfig{
			StorageLayout: "V2",
		},
		Contents: make(map[string]ManifestEntry),
	}

	for _, entry := range man.Contents {
		m.Contents[entry.Path] = ManifestEntry{
			Digest: entry.Digest,
			Size:   entry.Size,
			// BirthArtifactID: entry.Digest,
		}
	}

	// fmt.Printf("Man: %+v\n", man)
	// fmt.Printf("MMM: %+v\n", m)
	jsonBytes, _ := json.MarshalIndent(m, "", "    ")
	// fmt.Printf("json %+v\n", string(jsonBytes))

	f, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		panic(err)
	}

	defer f.Close()
	// fmt.Printf("FFF: %+v\n", f.Name())
	defer os.Remove(f.Name())

	if _, err := f.Write(jsonBytes); err != nil {
		panic(err)
	}
}

func (as *ArtifactSaver) commitArtifact(artifactId string) {
	// ---
	com, err := CommitArtifact(
		as.Ctx,
		as.GraphqlClient,
		artifactId,
	)
	if err != nil {
		err = fmt.Errorf("artifact commit: %s, error: %+v data: %+v", as.Artifact.Name, err, com)
		as.Logger.CaptureFatalAndPanic("Artifact commit error", err)
	}
	// commitArtifact := com.GetCommitArtifact()
	// fmt.Printf("GOT ART COM RESP: %+v\n", commitArtifact)
}

func (as *ArtifactSaver) save() ArtifactSaverResult {
	artifactId, baseArtifactId := as.createArtifact()
	// create manifest to get manifest id for file uploads
	manifestId := as.createManifest(artifactId, baseArtifactId, "", false)
	fmt.Printf("manid %+v\n", manifestId)
	// TODO file uploads
	// create manifest to get manifest for commit
	manifestDigest := ""  // TODO compute
	as.createManifest(artifactId, baseArtifactId, manifestDigest, true)
	as.sendFiles(manifestDigest)
	as.sendManifest()
	as.commitArtifact(artifactId)

	result := ArtifactSaverResult{
		ArtifactId: "artid",
	}
	return result
}
