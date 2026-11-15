package deployment

// DescriptorTemplate is the fixed descriptor service release resource.
const DescriptorTemplate = `[Unit]
Description=Ardents Descriptor Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:@ARDENTS_DESCRIPTOR_PROGRAM@ descriptor start-installed @ARDENTS_DESCRIPTOR_BINDING@
` + serviceBeforeWritePaths + `ReadWritePaths=@ARDENTS_DESCRIPTOR_WRITE_PATHS@
` + serviceAfterWritePaths
