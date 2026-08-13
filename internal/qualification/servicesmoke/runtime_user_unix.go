//go:build !windows

package servicesmoke

import (
	"fmt"
	"os"
)

func runtimeUser() string { return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()) }
