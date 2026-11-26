//go:build linux

// Package sourcegenesis owns one offline Direct-Origin Source genesis cohort.
// It prepares one public declaration, fixed distinct State and Node keys,
// and the reviewed Record and Epoch with their original protected file custody.
// Platform Artifacts supplies the Linux durability floor. The infrastructure
// provision command is its caller. Network State independently accepts
// the produced bytes and owns every runtime root.
package sourcegenesis
