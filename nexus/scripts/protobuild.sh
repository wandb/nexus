#!/bin/bash
# Run this in the root directory like:
# ./scripts/protobuild.sh

# NOTE: this needs to be reworked.
# proto files should be unmodified from wandb/wandb repo
# and the following calls to protoc shouldn't need to be so hardcoded

# Other notes
# go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# mkdir -p proto
#cp ~/work/wb/wandb/wandb/proto/*.proto proto/

SRC_DIR=.
MOD=pkg/service/
INC=api/proto/

protoc -I=$INC \
    --go_opt=Mwandb_base.proto=$MOD \
    --go_opt=Mwandb_telemetry.proto=$MOD \
    --go_opt=Mwandb_internal.proto=$MOD \
    --go_out=. --proto_path=. wandb_internal.proto

protoc -I=$INC \
    --go_opt=Mwandb_base.proto=$MOD \
    --go_out=. --proto_path=. wandb_base.proto

protoc -I=$INC \
    --go_opt=Mwandb_base.proto=$MOD \
    --go_opt=Mwandb_telemetry.proto=$MOD \
    --go_out=. --proto_path=. wandb_telemetry.proto

protoc -I=$INC \
    --go_opt=Mwandb_base.proto=$MOD \
    --go_opt=Mwandb_telemetry.proto=$MOD \
    --go_opt=Mwandb_internal.proto=$MOD \
    --go_opt=Mwandb_server.proto=$MOD \
    --go_out=. --proto_path=. wandb_server.proto