package nativecircuit

import (
	"os"
	"path/filepath"
)

const c3ComposeOverride = `services:
  user:
    profiles: !override ["c3"]
    networks: !override [c3-user-entry]
  user-entry:
    profiles: !override ["c3"]
    networks: !override [c3-user-entry, c3-entry-rendezvous, c3-user-introduction]
  rendezvous:
    profiles: !override ["c3"]
    networks: !override [c3-entry-rendezvous, c3-rendezvous-entry]
  data-service-entry:
    profiles: !override ["c3"]
    networks: !override [c3-rendezvous-entry, c3-entry-service, c3-service-introduction]
  service:
    profiles: !override ["c3"]
    networks: !override [c3-entry-service]
  introduction-node:
    profiles: !override ["c3"]
    networks: !override [c3-user-introduction, c3-service-introduction]
  shape-user: {profiles: !override ["c3"]}
  shape-user-entry: {profiles: !override ["c3"]}
  shape-rendezvous: {profiles: !override ["c3"]}
  shape-data-service-entry: {profiles: !override ["c3"]}
  shape-service: {profiles: !override ["c3"]}
  shape-introduction-node: {profiles: !override ["c3"]}
  capture-user: {profiles: !override ["c3"]}
  capture-user-entry: {profiles: !override ["c3"]}
  capture-rendezvous: {profiles: !override ["c3"]}
  capture-data-service-entry: {profiles: !override ["c3"]}
networks:
  c3-user-entry: {internal: true}
  c3-entry-rendezvous: {internal: true}
  c3-rendezvous-entry: {internal: true}
  c3-entry-service: {internal: true}
  c3-user-introduction: {internal: true}
  c3-service-introduction: {internal: true}
`

const directComposeOverride = `services:
  user:
    profiles: !override ["direct"]
    networks: !override [direct-link]
  service:
    profiles: !override ["direct"]
    networks: !override [direct-link]
  shape-user: {profiles: !override ["direct"]}
  shape-service: {profiles: !override ["direct"]}
  capture-user: {profiles: !override ["direct"]}
networks:
  direct-link: {internal: true}
`

func writeC3ComposeOverride(root string) error {
	return os.WriteFile(filepath.Join(root, "compose-c3.yaml"), []byte(c3ComposeOverride), 0o600)
}

func writeDirectComposeOverride(root string) error {
	return os.WriteFile(filepath.Join(root, "compose-direct.yaml"), []byte(directComposeOverride), 0o600)
}
