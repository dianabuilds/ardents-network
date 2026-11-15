package deployment

// SourceTemplate is the exact release resource. Future rendering is restricted
// to one executable/binding and the two independently admitted writable roots.
// It deliberately has no Endpoint policy switch or arbitrary service selector.
const SourceTemplate = `[Unit]
Description=Ardents Direct-Origin Source
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-source
Group=ardents-source
SupplementaryGroups=
ExecStart=:@ARDENTS_SOURCE_PROGRAM@ source start-installed @ARDENTS_SOURCE_BINDING@
` + serviceBeforeWritePaths + `ReadWritePaths=@ARDENTS_SOURCE_WRITE_PATHS@
` + serviceAfterWritePaths
