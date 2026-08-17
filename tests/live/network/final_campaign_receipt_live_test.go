//go:build live

package network_test

import (
	"errors"
	"fmt"
	"strings"
)

func verifyFinalEmbeddedReceipts(value finalRunnerSchedule, projectToken string) error {
	product, err := readFinalImageReceipt(value.ProductImageID, "product", projectToken,
		"cat /usr/share/ardents/stage5-source.sha256 /usr/share/ardents/go-archive.sha256 /usr/share/ardents/go-builder-recipe.sha256 /usr/share/ardents/go-module-cache.sha256; sha256sum /usr/local/bin/ardents-route /usr/local/bin/ardents-bridge /usr/local/bin/ardents-service /usr/local/bin/ardents-stream-app /usr/local/bin/ardents-publish-app /usr/local/bin/network-live.test /usr/local/bin/camouflage.test")
	if err != nil {
		return err
	}
	productLines := strings.Split(strings.TrimSpace(string(product)), "\n")
	wantProduct := []string{value.ProductReceipt.RouteSHA256, value.ProductReceipt.BridgeSHA256,
		value.ProductReceipt.ServiceSHA256, value.ProductReceipt.StreamSHA256, value.ProductReceipt.PublishSHA256,
		value.ProductReceipt.NetworkSHA256, value.ProductReceipt.AdapterSHA256}
	paths := []string{"/usr/local/bin/ardents-route", "/usr/local/bin/ardents-bridge",
		"/usr/local/bin/ardents-service", "/usr/local/bin/ardents-stream-app",
		"/usr/local/bin/ardents-publish-app", "/usr/local/bin/network-live.test",
		"/usr/local/bin/camouflage.test"}
	if len(productLines) != len(paths)+4 || strings.TrimSpace(productLines[0]) != value.SourceSHA256 ||
		strings.TrimSpace(productLines[1]) != value.ProductReceipt.GoArchiveSHA256 ||
		strings.TrimSpace(productLines[2]) != value.ProductReceipt.GoRecipeSHA256 ||
		strings.TrimSpace(productLines[3]) != value.ProductReceipt.GoModuleSHA256 {
		return errors.New("product image source/binary receipt is incomplete")
	}
	for index := range paths {
		if !matchesFinalReceiptLine(productLines[index+4], paths[index], wantProduct[index]) {
			return errors.New("product image binary differs from the frozen receipt")
		}
	}
	tool, err := readFinalImageReceipt(value.ToolImageID, "tool", projectToken,
		"sha256sum /usr/share/ardents/carrier-lab-tools.lock /usr/local/bin/carrier-lab; cat /usr/share/ardents/carrier-lab-source.sha256")
	if err != nil {
		return err
	}
	toolLines := strings.Split(strings.TrimSpace(string(tool)), "\n")
	if len(toolLines) != 3 || !matchesFinalReceiptLine(toolLines[0],
		"/usr/share/ardents/carrier-lab-tools.lock", value.ToolReceipt.ToolLockSHA256) ||
		!matchesFinalReceiptLine(toolLines[1], "/usr/local/bin/carrier-lab", value.ToolReceipt.CarrierSHA256) ||
		strings.TrimSpace(toolLines[2]) != value.ToolReceipt.SourceSHA256 {
		return errors.New("tool image lock/source/binary receipt differs from the frozen receipt")
	}
	return nil
}

func readFinalImageReceipt(image, role, projectToken, script string) ([]byte, error) {
	name := fmt.Sprintf("ardents-s5-runtime-receipt-%s-%s", role, projectToken)
	project := "ardents-final-" + projectToken
	return finalSupplyOutput("docker", "run", "--name", name,
		"--label", "com.docker.compose.project="+project, "--pull", "never", "--network", "none",
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--entrypoint", "/bin/sh", image, "-ceu", script)
}

func matchesFinalReceiptLine(line, path, hash string) bool {
	fields := strings.Fields(strings.TrimSpace(line))
	return len(fields) == 2 && fields[0] == hash && fields[1] == path
}
