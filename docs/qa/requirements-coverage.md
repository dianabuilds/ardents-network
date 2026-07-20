# Requirements Coverage Contract

`docs/qa/requirements-coverage.json` is the curated traceability source between
mandatory Ardents properties and executable evidence. The test catalog remains
the generated projection of code/scenario bindings; the coverage manifest does
not duplicate test metadata.

Each requirement entry contains a stable ID, authoritative source and section,
`covered` or `blocked` status, and catalog scenario IDs and/or repository
static-evidence paths. A blocked entry also requires a concrete reason.

`go run ./tests/cmd/testcatalog -mode validate ./tests/...` fails when:

- a mandatory source is absent from the matrix;
- a requirement ID is duplicated;
- a source file or section marker does not exist;
- scenario evidence is unknown to the catalog;
- static evidence does not exist;
- a covered requirement has no evidence;
- a blocked requirement has no reason;
- any scenario lacks canonical metadata, a runnable binding, or non-empty
  false-positive and false-negative risk analysis.

Coverage status means the requirement is traceable to a concrete proof path.
It does not silently promote a historical report to a current pass. Phase and
release gates still retain versioned execution reports and rerun affected
scenarios after product-code changes. Documentation- or validator-only changes
use the targeted validator package instead of repeating unrelated network
suites.

