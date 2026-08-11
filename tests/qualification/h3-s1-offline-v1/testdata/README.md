# H3 Stage 1 offline golden vector v1

The lowercase hexadecimal files freeze the Stage 1 laboratory record, Epoch,
ordered input-log, and Candidate Materialization bytes. The JSON/JSONL files
freeze the manifest, assignment transition, accepted-generation event, and
terminal verdict bytes. They are not a public wire format.
`internal/networkstate` accepts them through its product Interface and
`internal/qualification` parses and recomputes them independently.

- verification Unix time: `1800000100`;
- network identity: `488a631a444652b50d760a739c338d5f7e54bc14e92a3c3d6002eaeead4f2d3d`;
- authority Ed25519 public key: `c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631`;
- generation: `243fba444fe71948f6cd4a253552301192857a156c7eb6359eed604c2d2cda4b`;
- expected result: two accepted records; capacity, malformed, duplicate,
  source-identity collision, and publication-cutoff rejections; `pass`.

Merkle leaf tags are `0x00` for canonical records and `0x02` for deterministic
rejections. Branches use `0x01`; empty input, View, and rejection roots use
`0x10`, `0x11`, and `0x12` respectively.
