package identity

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"ardents/internal/cli/client"
	"ardents/internal/cli/output"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

type Command struct {
	Renderer     output.Renderer
	entropy      io.Reader
	now          func() time.Time
	sessions     sessionClient
	timeout      time.Duration
	input        io.Reader
	removeTicket func(string) error
}

type sessionClient interface {
	Login(context.Context) (client.SessionKey, error)
	SessionStatus() client.SessionKey
	Logout()
	PublicIdentityService() (ardentsv1connect.IdentityServiceClient, error)
	ProtectedIdentityService() (ardentsv1connect.IdentityServiceClient, error)
	TargetNodePrincipal() string
}

func New(renderer output.Renderer) Command {
	return Command{Renderer: renderer, entropy: rand.Reader, now: time.Now}
}

func NewOnline(renderer output.Renderer, sessions sessionClient, timeout time.Duration, input io.Reader) Command {
	return Command{Renderer: renderer, entropy: rand.Reader, now: time.Now, sessions: sessions, timeout: timeout, input: input}
}

func (c Command) Run(ctx context.Context, args []string) int {
	if err := ctx.Err(); err != nil {
		return c.Renderer.Failure(err)
	}
	if len(args) == 0 || args[0] == "help" {
		usage(c.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "principal":
		return c.runPrincipal(ctx, args[1:])
	case "device":
		return c.runDevice(ctx, args[1:])
	case "enroll":
		return c.runEnroll(ctx, args[1:])
	case "grant":
		return c.runGrant(ctx, args[1:])
	case "application-ticket":
		return c.runApplicationTicket(ctx, args[1:])
	case "login":
		return c.runLogin(ctx, args[1:])
	case "status":
		return c.runStatus(args[1:])
	case "logout":
		return c.runLogout(args[1:])
	default:
		return c.usageError(fmt.Sprintf("unknown subcommand %q", args[0]))
	}
}

func (c Command) runLogin(ctx context.Context, args []string) int {
	if len(args) != 0 {
		return c.usageError("identity login accepts no positional arguments")
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errors.New("Principal session mode is not configured"))
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	key, err := c.sessions.Login(callCtx)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	return c.renderSession("authenticated", key)
}

func (c Command) runStatus(args []string) int {
	if len(args) != 0 {
		return c.usageError("identity status accepts no positional arguments")
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errors.New("Principal session mode is not configured"))
	}
	key := c.sessions.SessionStatus()
	if key == (client.SessionKey{}) {
		return c.renderSession("not_authenticated", key)
	}
	return c.renderSession("authenticated", key)
}

func (c Command) runLogout(args []string) int {
	if len(args) != 0 {
		return c.usageError("identity logout accepts no positional arguments")
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errors.New("Principal session mode is not configured"))
	}
	c.sessions.Logout()
	return c.renderSession("not_authenticated", client.SessionKey{})
}

func (c Command) renderSession(status string, key client.SessionKey) int {
	value := struct {
		Status          string `json:"status"`
		NodePrincipal   string `json:"node_principal,omitempty"`
		SignerPrincipal string `json:"signer_principal,omitempty"`
		Interface       string `json:"interface,omitempty"`
		ProtocolMajor   uint32 `json:"protocol_major,omitempty"`
	}{Status: status, NodePrincipal: key.NodePrincipal, SignerPrincipal: key.SignerPrincipal, ProtocolMajor: key.ProtocolMajor}
	if key != (client.SessionKey{}) {
		value.Interface = "operator"
	}
	if c.Renderer.JSON {
		return c.renderJSON(value)
	}
	c.Renderer.Header("Principal session")
	c.Renderer.KV("status", value.Status)
	if value.NodePrincipal != "" {
		c.Renderer.KV("node_principal", value.NodePrincipal)
		c.Renderer.KV("signer_principal", value.SignerPrincipal)
		c.Renderer.KV("interface", value.Interface)
		c.Renderer.KV("protocol_major", fmt.Sprint(value.ProtocolMajor))
	}
	return 0
}

func (c Command) runPrincipal(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		principalUsage(c.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "create":
		path, ok := c.signerFileFlag("identity principal create", args[1:], false)
		if !ok {
			return 2
		}
		if err := ctx.Err(); err != nil {
			return c.Renderer.Failure(err)
		}
		info, err := CreatePrincipal(path, c.entropy)
		if err != nil {
			return c.Renderer.Failure(err)
		}
		return c.renderPrincipal(info)
	case "import":
		flags := c.flagSet("identity principal import")
		var source, destination string
		flags.StringVar(&source, "from-file", "", "protected Principal signer bundle to import")
		flags.StringVar(&destination, "signer-file", "", "destination protected Principal signer bundle")
		if !c.parseFlags(flags, args[1:]) {
			return 2
		}
		if source == "" {
			return c.usageError("--from-file is required")
		}
		if destination == "" {
			var err error
			destination, err = DefaultPrincipalSignerPath()
			if err != nil {
				return c.Renderer.Failure(err)
			}
		}
		info, err := ImportPrincipal(source, destination)
		if err != nil {
			return c.Renderer.Failure(err)
		}
		return c.renderPrincipal(info)
	case "show":
		path, ok := c.signerFileFlag("identity principal show", args[1:], false)
		if !ok {
			return 2
		}
		info, err := ShowPrincipal(path)
		if err != nil {
			return c.Renderer.Failure(err)
		}
		return c.renderPrincipal(info)
	default:
		return c.usageError(fmt.Sprintf("unknown principal subcommand %q", args[0]))
	}
}

func (c Command) runDevice(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		deviceUsage(c.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "create":
		flags := c.flagSet("identity device create")
		var rootPath, devicePath string
		validity := defaultCredentialTTL
		flags.StringVar(&rootPath, "root-signer-file", "", "protected Principal root signer bundle")
		flags.StringVar(&devicePath, "signer-file", "", "destination protected device signer bundle")
		flags.DurationVar(&validity, "valid-for", defaultCredentialTTL, "finite device Credential validity")
		if !c.parseFlags(flags, args[1:]) {
			return 2
		}
		var err error
		if rootPath == "" {
			rootPath, err = DefaultPrincipalSignerPath()
			if err != nil {
				return c.Renderer.Failure(err)
			}
		}
		if devicePath == "" {
			devicePath, err = DefaultDeviceSignerPath()
			if err != nil {
				return c.Renderer.Failure(err)
			}
		}
		if err := ctx.Err(); err != nil {
			return c.Renderer.Failure(err)
		}
		info, err := CreateDevice(rootPath, devicePath, validity, c.now(), c.entropy)
		if err != nil {
			return c.Renderer.Failure(err)
		}
		return c.renderDevice(info)
	case "revoke":
		return c.runDeviceRevoke(ctx, args[1:])
	case "show":
		path, ok := c.signerFileFlag("identity device show", args[1:], true)
		if !ok {
			return 2
		}
		info, err := ShowDevice(path)
		if err != nil {
			return c.Renderer.Failure(err)
		}
		return c.renderDevice(info)
	default:
		return c.usageError(fmt.Sprintf("unknown device subcommand %q", args[0]))
	}
}

func (c Command) signerFileFlag(name string, args []string, device bool) (string, bool) {
	flags := c.flagSet(name)
	var path string
	flags.StringVar(&path, "signer-file", "", "protected signer bundle")
	if !c.parseFlags(flags, args) {
		return "", false
	}
	if path != "" {
		return path, true
	}
	var err error
	if device {
		path, err = DefaultDeviceSignerPath()
	} else {
		path, err = DefaultPrincipalSignerPath()
	}
	if err != nil {
		c.Renderer.Failure(err)
		return "", false
	}
	return path, true
}

func (c Command) flagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet("ardentsctl "+name, flag.ContinueOnError)
	flags.SetOutput(c.Renderer.Err)
	return flags
}

func (c Command) parseFlags(flags *flag.FlagSet, args []string) bool {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return false
		}
		return false
	}
	if flags.NArg() != 0 {
		output.Writef(c.Renderer.Err, "ardentsctl identity: unexpected positional argument\n")
		return false
	}
	return true
}

func (c Command) renderPrincipal(info PrincipalInfo) int {
	if c.Renderer.JSON {
		return c.renderJSON(info)
	}
	c.Renderer.Header("Principal signer")
	c.Renderer.KV("principal", info.Principal)
	c.Renderer.KV("algorithm", info.Algorithm)
	c.Renderer.KV("root_public_key", info.RootPublicKey)
	return 0
}

func (c Command) renderDevice(info DeviceInfo) int {
	if c.Renderer.JSON {
		return c.renderJSON(info)
	}
	c.Renderer.Header("device signer")
	c.Renderer.KV("principal", info.Principal)
	c.Renderer.KV("device_id", info.DeviceID)
	c.Renderer.KV("algorithm", info.Algorithm)
	c.Renderer.KV("device_public_key", info.DevicePublicKey)
	c.Renderer.KV("credential_id", info.CredentialID)
	c.Renderer.KV("credential_not_before", info.CredentialNotBefore.Format(time.RFC3339))
	c.Renderer.KV("credential_not_after", info.CredentialNotAfter.Format(time.RFC3339))
	return 0
}

func (c Command) renderJSON(value any) int {
	raw, err := json.Marshal(value)
	if err != nil {
		return c.Renderer.Failure(fmt.Errorf("marshal public identity metadata: %w", err))
	}
	output.Writeln(c.Renderer.Out, string(raw))
	return 0
}

func (c Command) usageError(message string) int {
	output.Writef(c.Renderer.Err, "ardentsctl identity: %s\n", message)
	usage(c.Renderer.Err)
	return 2
}

func usage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] identity <principal|device|enroll|grant|application-ticket|login|status|logout> [subcommand]")
	output.Writeln(writer, "Principal/device custody is offline; session commands use the selected protected Operator transport.")
}

func principalUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [--output human|json] identity principal <create|import|show> [flags]")
}

func deviceUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [--output human|json] identity device <create|show|revoke> [flags]")
}
