package nativecircuit

import (
	"os"
	"path/filepath"
)

const attachedComposeOverride = `services:
  user:
    volumes:
      - {type: bind, source: "${GATEC_USER_SOCKET_DIR:?}", target: /attached/user, bind: {create_host_path: false}}
  service:
    volumes:
      - {type: bind, source: "${GATEC_SERVICE_SOCKET_DIR:?}", target: /attached/service, bind: {create_host_path: false}}
`

func writeAttachedComposeOverride(root string) error {
	return os.WriteFile(filepath.Join(root, "compose-attached.yaml"), []byte(attachedComposeOverride), 0o600)
}
