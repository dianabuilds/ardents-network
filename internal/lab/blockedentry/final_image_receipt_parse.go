package blockedentry

import (
	"errors"
	"strings"
)

func parseProductReceipt(output, source, archive, recipe, modules string) (finalProductReceipt, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	paths := []string{"/usr/local/bin/ardents-route", "/usr/local/bin/ardents-bridge",
		"/usr/local/bin/ardents-service", "/usr/local/bin/ardents-stream-app",
		"/usr/local/bin/ardents-publish-app", "/usr/local/bin/network-live.test",
		"/usr/local/bin/camouflage.test"}
	if len(lines) != len(paths)+4 || strings.TrimSpace(lines[0]) != source ||
		strings.TrimSpace(lines[1]) != archive || strings.TrimSpace(lines[2]) != recipe ||
		strings.TrimSpace(lines[3]) != modules {
		return finalProductReceipt{}, errors.New("product image returned an incomplete source/binary receipt")
	}
	hashes := make([]string, 0, len(paths))
	for index, path := range paths {
		hash, err := parseReceiptHash(lines[index+4], path)
		if err != nil {
			return finalProductReceipt{}, err
		}
		hashes = append(hashes, hash)
	}
	return finalProductReceipt{SourceSHA256: source, GoArchiveSHA256: archive, GoRecipeSHA256: recipe,
		GoModuleSHA256: modules,
		RouteSHA256:    hashes[0], BridgeSHA256: hashes[1],
		ServiceSHA256: hashes[2], StreamSHA256: hashes[3], PublishSHA256: hashes[4],
		NetworkSHA256: hashes[5], AdapterSHA256: hashes[6]}, nil
}

func parseToolReceipt(output, lock, source string) (finalToolReceipt, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 || strings.TrimSpace(lines[2]) != source {
		return finalToolReceipt{}, errors.New("tool image returned an incomplete lock/source/binary receipt")
	}
	embeddedLock, err := parseReceiptHash(lines[0], "/usr/share/ardents/carrier-lab-tools.lock")
	if err != nil || embeddedLock != lock {
		return finalToolReceipt{}, errors.Join(err, errors.New("tool image embeds the wrong tool lock"))
	}
	carrier, err := parseReceiptHash(lines[1], "/usr/local/bin/carrier-lab")
	if err != nil {
		return finalToolReceipt{}, err
	}
	return finalToolReceipt{BaseDigest: finalImageHash, ToolLockSHA256: lock,
		SourceSHA256: source, CarrierSHA256: carrier}, nil
}

func parseReceiptHash(line, path string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 2 || fields[1] != path || !hexDigest(fields[0], 32) {
		return "", errors.New("image returned a malformed binary receipt")
	}
	return fields[0], nil
}

func validProductReceipt(value finalProductReceipt, source string) bool {
	return value.SourceSHA256 == source && hexDigest(value.GoArchiveSHA256, 32) &&
		hexDigest(value.GoRecipeSHA256, 32) && hexDigest(value.GoModuleSHA256, 32) &&
		hexDigest(value.RouteSHA256, 32) &&
		hexDigest(value.BridgeSHA256, 32) && hexDigest(value.ServiceSHA256, 32) &&
		hexDigest(value.StreamSHA256, 32) && hexDigest(value.PublishSHA256, 32) &&
		hexDigest(value.NetworkSHA256, 32) && hexDigest(value.AdapterSHA256, 32)
}

func validToolReceipt(value finalToolReceipt) bool {
	return value.BaseDigest == finalImageHash && hexDigest(value.ToolLockSHA256, 32) &&
		hexDigest(value.SourceSHA256, 32) && hexDigest(value.CarrierSHA256, 32)
}
