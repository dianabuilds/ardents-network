---
name: ardents-legacy-extraction
description: Legacy extraction workflow for Ardents Network. Use when inspecting aim-core to selectively extract logic, signatures, heuristics, persistence patterns, or vocabulary into the new root implementation without importing aim-core architecture or package topology.
---

# Ardents Legacy Extraction

Use this skill when `aim-core` contains useful technical logic but its structure must not be copied.

## Read First

- `docs/legacy-decomposition.md`
- `docs/repo-strategy.md`
- the active domain document for the target slice

Read only the relevant `aim-core` package after the target boundary in the new code is clear.

## Extraction Workflow

1. Identify the target package in the root implementation.
2. Inspect the matching logic in `aim-core`.
3. Separate transferable logic from non-transferable structure.
4. Copy only the logic that survives the new domain boundary.
5. Rewrite naming, DTOs, and dependencies to fit the root package.
6. Add or update tests in the root implementation.

## What Is Usually Safe To Extract

- canonicalization logic;
- signature and verification logic;
- scoring heuristics;
- persistence encoding details;
- diagnostics vocabulary;
- transport scheme handling rules.

## What Must Usually Be Rewritten

- runtime assembly;
- service host layers;
- package topology;
- contract conversion stacks;
- multi-surface API patterns;
- adapter ownership that conflicts with the root domain map.

## Acceptance Check

Before finishing, verify:

- the new code does not import `aim-core`;
- the new code matches the root domain owner;
- tests cover the extracted behavior;
- the resulting package still looks like root Ardents code, not transplanted legacy.
