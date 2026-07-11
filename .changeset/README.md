# Changesets

This directory contains [Changesets](https://github.com/changesets/changesets) configuration and pending changeset files.

## Usage

Create a changeset for your PR:

```bash
pnpm changeset
```

Select the packages/apps affected and describe the change. The changeset file will be committed and used by the `changeset` GitHub workflow to produce a Version Packages PR.
