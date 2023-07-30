package server

import (
	"testing"
	"fmt"

	"github.com/Khan/genqlient/graphql"
    "google.golang.org/protobuf/types/known/wrapperspb"
	"github.com/stretchr/testify/assert"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
	"github.com/wandb/wandb/nexus/internal/gql"
	"github.com/wandb/wandb/nexus/internal/nexustest"
	"github.com/golang/mock/gomock"
)

func makeSender(client graphql.Client, resultChan chan *service.Result) Sender {
	logger := observability.NewNexusLogger(SetupDefaultLogger(), nil)
	sender := Sender{
		logger: logger,
		settings: &service.Settings{
			RunId: &wrapperspb.StringValue{Value: "run1"},
		},
		graphqlClient: client,
		resultChan: resultChan,
	}
	return sender
}

func TestSendRun(t *testing.T) {
	// Verify that project and entity are properly passed through to graphql
	to := nexustest.MakeTestObject(t)
	defer to.TeardownTest()

	sender := makeSender(to.MockClient, make(chan *service.Result, 1))

	run := &service.Record{
		RecordType: &service.Record_Run{
			Run: &service.RunRecord{
				Config: to.MakeConfig(),
				Project: "testProject",
				Entity: "testEntity",
			}}}

	respEncode := &graphql.Response{
		Data: &gql.UpsertBucketResponse{
			UpsertBucket: &gql.UpsertBucketUpsertBucketUpsertBucketPayload{
				Bucket: &gql.UpsertBucketUpsertBucketUpsertBucketPayloadBucketRun{
					DisplayName: nexustest.StrPtr("FakeName"),
					Project: &gql.UpsertBucketUpsertBucketUpsertBucketPayloadBucketRunProject{
						Name: "FakeProject",
						Entity: gql.UpsertBucketUpsertBucketUpsertBucketPayloadBucketRunProjectEntity{
							Name: "FakeEntity",
						},
					},
				},
			},
		}}
	to.MockClient.EXPECT().MakeRequest(
		gomock.Any(), // context.Context
		gomock.Any(), // *graphql.Request
		gomock.Any(), // *graphql.Request
	).Return(nil).Do(nexustest.InjectResponse(
		respEncode,
		func (vars nexustest.RequestVars) {
			fmt.Printf("got: %+v\n", vars)
			assert.Equal(t, "testEntity", vars["entity"])
			assert.Equal(t, "testProject", vars["project"])
		},
	))

	sender.sendRecord(run)
	result := <- sender.resultChan
}
