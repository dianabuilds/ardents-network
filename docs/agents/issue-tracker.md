# Issue tracker: Local Markdown

Issues and PRDs for this repository live as Markdown files in `.scratch/`.

## Conventions

- One feature or work program per directory: `.scratch/<feature-slug>/`.
- Its optional PRD is `.scratch/<feature-slug>/PRD.md`.
- Implementation issues are
  `.scratch/<feature-slug>/issues/<NN>-<slug>.md`, numbered from `01`.
- Triage state is recorded as a `Status:` line near the top of each issue.
- Comments and conversation history are appended under `## Comments`.

## Skill operations

When a skill says to publish an issue, create the corresponding Markdown file
under `.scratch/<feature-slug>/`.

When a skill says to fetch an issue, read the referenced file or resolve the
number within the named feature directory.

Local Markdown is the authoritative issue tracker for this repository. GitHub
Issues and external pull requests are not triage request surfaces.
