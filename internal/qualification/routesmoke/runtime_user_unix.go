//go:build !windows

package routesmoke

import (
	"fmt"
	"os"
)

func runtimeUser() string { return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()) }
