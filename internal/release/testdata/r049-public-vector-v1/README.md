# R-049 public release-decision vector v1

This directory is a frozen, byte-exact public vector for S7.1. `root.json`,
the three online metadata objects, and `artifact.bin` are immutable inputs;
`expected.json` is the independently frozen classification and identity result.

The Ed25519 identities and both rebuild records are visibly project-controlled
test identities. No private signing material is retained in the repository and
the vector makes no independent-custody or independent-builder claim.
