//go:build !windows

package recoverysmoke

import (
	"fmt"
	"os"
)

func runtimeUser() string { return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()) }
