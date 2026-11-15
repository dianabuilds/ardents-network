package deployment

// ForwardingTemplate is the fixed forwarding service release resource.
const ForwardingTemplate = `[Unit]
Description=Ardents Forwarding Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:@ARDENTS_FORWARDING_PROGRAM@ forwarding start-installed @ARDENTS_FORWARDING_BINDING@
` + serviceBeforeWritePaths + `ReadWritePaths=@ARDENTS_FORWARDING_WRITE_PATHS@
` + serviceAfterWritePaths
