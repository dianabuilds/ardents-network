function Read-CanonicalJSONInstant {
    param(
        [Parameter(Mandatory = $true)][string]$JSON,
        [Parameter(Mandatory = $true)][string]$Property
    )

    $document = [Text.Json.JsonDocument]::Parse($JSON)
    try {
        $element = $document.RootElement.GetProperty($Property)
        if ($element.ValueKind -ne [Text.Json.JsonValueKind]::String) {
            throw "JSON property $Property must be a canonical UTC string."
        }
        return [DateTimeOffset]::ParseExact(
            $element.GetString(),
            'yyyy-MM-ddTHH:mm:ssZ',
            [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::AssumeUniversal -bor [Globalization.DateTimeStyles]::AdjustToUniversal
        )
    } finally {
        $document.Dispose()
    }
}
