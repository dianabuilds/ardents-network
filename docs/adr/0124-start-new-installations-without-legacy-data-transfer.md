---
status: accepted
date: 2026-11-14
---

# ADR-0124 — Start new installations without legacy data transfer

## Context

The Product Owner confirmed that no predecessor installation, issued right,
private key or retained history must survive the transition to the new owners.
The [recorded decision](https://github.com/dianabuilds/ardents-network/issues/558#issuecomment-6879828081)
removes the deployment obligation that motivated a cross-owner data transfer.
This is a product disposition, not a claim that formats are interchangeable.

## Decision

1. The transition provisions independent new installations. Do not implement
   predecessor history conversion, key import, ledger replay or cross-owner
   root adoption solely to preserve the retired deployment. No old installation
   or right is continued by an empty new root.
2. Provision purpose-specific keys and independently admitted trust inputs for
   the new installation. Do not reuse predecessor issuing/Service authority,
   private material or signed rights as a compatibility shortcut. Artifact
   verification, initial-pin delivery and genuine signed Network acceptance
   remain required. A new path or matching schema alone supplies none of them.
3. Initial creation uses each owner's actual fresh-root contract. Existing,
   partial or foreign destinations never authorize overwrite or reset. Retained
   opening, replacement and recovery of a new installation keep its own spend,
   exact results, conflict/replay/time/Release floors and original key lifetimes.
   Failure to reopen must not fall back to initialization.
4. Remove an old reader with its final production, tool and recovery consumers
   after their maintained scenario has a verified new owner or is explicitly
   retired. No reader must remain solely to migrate the predecessor deployment.
   Reused public grammar, active new owners and their recovery are not legacy
   merely because their bytes or filenames have historical names.
5. This disposition authorizes no automatic filesystem purge, protocol/version
   renumbering, old/new runtime bridge, weaker accepting path or qualification
   waiver. Git evidence and the remote old branch remain. Physical deletion of
   user files is separate from retiring their code consumers.

## Consequences

Cross-owner transfer is not a prerequisite of legacy code removal. New-installation
recovery and complete installed qualification remain prerequisites of their own
supported scenarios. Existing old-owner migration tests describe their retained
implementation until removal; they do not establish a future support obligation.
ADR-0117's exact incompatible Service editions and all current safety/lifecycle
rules remain unchanged. Shared wire grammars need not change: separation rests
on genuine new authority and independent roots, not a guessed version label.
