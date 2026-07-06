# Protobuf scaffold

This directory is scaffolding for a possible future client SDK generated from protobuf/gRPC definitions. There are no `.proto` files yet — nothing in this repo currently uses protobuf or gRPC; all APIs today are the FHIR REST API served by `interop/` (see `core/fhir/r5/` for the generated FHIR types used over HTTP/JSON).

When a concrete SDK or gRPC service is scoped:

1. Add `.proto` files under a new subdirectory here (e.g. `tools/proto/tpt/v1/`).
2. Run `buf generate` from this directory (requires [buf](https://buf.build/docs/installation), `protoc-gen-go`, and `protoc-gen-go-grpc` on `PATH`) — output lands in `gen/` per `buf.gen.yaml`.
3. Move the generated code to its real destination (e.g. a new `packages/*-sdk` for a TS client, or a `core/pb/` package for Go) once the SDK's actual location is decided — `gen/` here is a placeholder, not the final home for generated code.

Until then, this scaffold is unused and safe to ignore.
