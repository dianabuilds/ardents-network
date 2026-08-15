//go:build windows

package campaign

func publishManifest(pending, final, root string) error {
	return publishReceipt(pending, final, root)
}
