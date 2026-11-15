package deployment

// JoinTemplate is the fixed join service release resource.
const JoinTemplate = `[Unit]
Description=Ardents JOIN Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:@ARDENTS_JOIN_PROGRAM@ join start-installed @ARDENTS_JOIN_BINDING@
` + serviceBeforeWritePaths + `ReadWritePaths=@ARDENTS_JOIN_WRITE_PATHS@
` + serviceAfterWritePaths
