package server

import (
	"context"
	"encoding/json"
	b64 "encoding/base64"
    "crypto/md5"
	"fmt"
	"os"

	"github.com/Khan/genqlient/graphql"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type ArtifactSaver struct {
	ctx           context.Context
	logger        *observability.NexusLogger
	artifact      *service.ArtifactRecord
	graphqlClient graphql.Client
	uploader      *Uploader
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
	for _, alias := range as.artifact.Aliases {
		aliases = append(aliases,
			ArtifactAliasInput{
				ArtifactCollectionName: as.artifact.Name,
				Alias:                  alias,
			},
		)
	}
	// fmt.Printf("SSSS: %+v\n", as.artifact)
	data, err := CreateArtifact(
		as.ctx,
		as.graphqlClient,
		as.artifact.Type,
		[]string{as.artifact.Name},
		as.artifact.Entity,
		as.artifact.Project,
		&as.artifact.RunId,
		&as.artifact.Description,
		as.artifact.Digest,
		nil, // Labels
		aliases,
		nil, // metadata
		// 0,   // historyStep
		// &as.artifact.DistributedId,
		as.artifact.ClientId,
		as.artifact.SequenceClientId,
		&enableDedup, // enableDigestDeduplication
	)
	if err != nil {
		err = fmt.Errorf("createartifact: %s, error: %+v data: %+v", as.artifact.Name, err, data)
		as.logger.CaptureFatalAndPanic("Artifact saver error", err)
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

func (as *ArtifactSaver) createManifest(artifactId string, baseArtifactId *string, manifestDigest string, includeUpload bool) (string, *string, []string) {
	manifestType := ArtifactManifestTypeFull
	manifestFilename := "wandb_manifest.json"

	// ---
	got, err := CreateArtifactManifest(
		as.ctx,
		as.graphqlClient,
		manifestFilename,
		manifestDigest,
		artifactId,
		baseArtifactId,
		as.artifact.Entity,
		as.artifact.Project,
		as.artifact.RunId,
		includeUpload, // includeUpload
		&manifestType,
	)
	if err != nil {
		err = fmt.Errorf("artifact manifest: %s, error: %+v data: %+v", as.artifact.Name, err, got)
		as.logger.CaptureFatalAndPanic("Artifact saver error", err)
	}
	createManifest := got.GetCreateArtifactManifest()
	manifest := createManifest.ArtifactManifest
	as.logger.Info("createamanifest", "manifest", manifest)
	// fmt.Printf("GOT ART MAN RESP: %+v %+v %+v\n", manifest, manifest.Id, manifest.File)

	var upload *string
	var headers []string
	if includeUpload {
		upload = manifest.File.GetUploadUrl()
		headers = manifest.File.GetUploadHeaders()
	}

	return manifest.Id, upload, headers
}

func (as *ArtifactSaver) sendFiles(artifactID string, manifestID string) {
	// TODO iterate over all entries...
	artifactFiles := []CreateArtifactFileSpecInput{}
	man := as.artifact.Manifest
	for _, entry := range man.Contents {
		as.logger.Info("sendfiles", "entry", entry)
		// fmt.Printf("Got %+v\n", entry)
		md5Checksum := ""
		artifactFiles = append(artifactFiles,
			CreateArtifactFileSpecInput{
				ArtifactID:         artifactID,
				Name:               entry.Path,
				Md5:                md5Checksum,
				ArtifactManifestID: &manifestID,
			})
	}
	got, err := CreateArtifactFiles(
		as.ctx,
		as.graphqlClient,
		ArtifactStorageLayoutV2,
		artifactFiles,
	)
	if err != nil {
		err = fmt.Errorf("artifact files: %s, error: %+v data: %+v", as.artifact.Name, err, got)
		as.logger.CaptureFatalAndPanic("Artifact files error", err)
	}
	for n, edge := range got.GetCreateArtifactFiles().GetFiles().Edges {
		as.logger.Info("Create artifact files", "artifact", artifactID, "filespec", edge.Node)
		// fmt.Printf("FILES:::::: %+v\n", edge.Node)
		upload := UploadTask{
			url:  *edge.Node.GetUploadUrl(),
			path: man.Contents[n].LocalPath,
		}
		as.uploader.AddTask(&upload)
	}
	// use uploader to send files
	// wait for upload responses
	// update manifest checksums/artifact info
}

func (as *ArtifactSaver) writeManifest() string {
	man := as.artifact.Manifest

	m := &ManifestV1{
		Version:       man.Version,
		StoragePolicy: man.StoragePolicy,
		StoragePolicyConfig: ManifestStoragePolicyConfig{
			StorageLayout: "V2",
		},
		Contents: make(map[string]ManifestEntry),
	}

	for _, entry := range man.Contents {
		// digest := entry.Digest
		// digest := "junk"
		m.Contents[entry.Path] = ManifestEntry{
			Digest: entry.Digest,
			Size:   entry.Size,
			// BirthArtifactID: entry.Digest,
		}
	}

	jsonBytes, _ := json.MarshalIndent(m, "", "    ")

	f, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		panic(err)
	}

	defer f.Close()
	// TODO: remove this later
	// defer os.Remove(f.Name())

	if _, err := f.Write(jsonBytes); err != nil {
		panic(err)
	}

	return f.Name()
}

func (as *ArtifactSaver) sendManifest(manifestFile string, uploadUrl *string, uploadHeaders []string) {
	response := make(chan bool)
	upload := UploadTask{
		url:         *uploadUrl,
		path:        manifestFile,
		headers:     uploadHeaders,
		respondChan: response,
	}
	as.uploader.AddTask(&upload)
	worked := <-response
	if !worked {
		panic("manifest not saved")
	}
}

func (as *ArtifactSaver) commitArtifact(artifactId string) {
	// ---
	com, err := CommitArtifact(
		as.ctx,
		as.graphqlClient,
		artifactId,
	)
	if err != nil {
		err = fmt.Errorf("artifact commit: %s, error: %+v data: %+v", as.artifact.Name, err, com)
		as.logger.CaptureFatalAndPanic("Artifact commit error", err)
	}
	// commitArtifact := com.GetCommitArtifact()
	// fmt.Printf("GOT ART COM RESP: %+v\n", commitArtifact)
}

func computeB64MD5(manifestFile string) string {
	file, err := os.ReadFile(manifestFile)
	if err != nil {
		panic("bad manifestfile")
	}
	hasher := md5.New()
	hasher.Write(file)
	encodedString := b64.StdEncoding.EncodeToString(hasher.Sum(nil))
	return encodedString
}

func (as *ArtifactSaver) save() ArtifactSaverResult {
	artifactId, baseArtifactId := as.createArtifact()
	// create manifest to get manifest id for file uploads
	manifestId, _, _ := as.createManifest(artifactId, baseArtifactId, "", false)
	// fmt.Printf("manid %+v\n", manifestId)
	// TODO file uploads
	// create manifest to get manifest for commit
	as.sendFiles(artifactId, manifestId)
	manifestFile := as.writeManifest()
	manifestDigest := computeB64MD5(manifestFile)
	_, uploadUrl, uploadHeaders := as.createManifest(artifactId, baseArtifactId, manifestDigest, true)
	as.sendManifest(manifestFile, uploadUrl, uploadHeaders)
	as.commitArtifact(artifactId)

	result := ArtifactSaverResult{
		ArtifactId: "artid",
	}
	return result
}
