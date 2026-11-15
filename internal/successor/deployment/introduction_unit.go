package deployment

// IntroductionTemplate is the fixed introduction service release resource.
const IntroductionTemplate = `[Unit]
Description=Ardents Introduction Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:@ARDENTS_INTRODUCTION_PROGRAM@ introduction start-installed @ARDENTS_INTRODUCTION_BINDING@
` + serviceBeforeWritePaths + `ReadWritePaths=@ARDENTS_INTRODUCTION_WRITE_PATHS@
` + serviceAfterWritePaths
