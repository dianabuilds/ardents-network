package wire

//go:generate protoc --proto_path=../../../.. --go_out=../../../.. --go_opt=paths=source_relative ../../../../internal/network/privacy/wire/private.proto
