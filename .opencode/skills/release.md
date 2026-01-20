# Release Workflow Skill

This skill guides you through creating a new release for goplaying, including automated checks, tagging, CI monitoring, and distribution updates.

## When to Use This Skill

Use this skill when you want to:
- Create a new version release (MINOR or PATCH bump)
- Tag and publish a release with proper versioning
- Update all distribution channels (GitHub, Homebrew, AUR)
- Ensure all release steps are completed correctly

## Prerequisites

Before starting a release, ensure:
- You're on the `main` branch
- All changes are merged and tested
- Tests pass locally (`go test ./...`)
- Code is formatted (`make fmt`)

## Workflow Steps

### 1. Pre-Release Verification

**Check status and recent changes:**
```bash
# Verify clean working tree
git status

# Ensure main is up to date
git pull origin main

# Review recent commits
git log --oneline -10
```

**Run quality checks:**
```bash
# Run tests
go test ./... -race

# Format code
make fmt

# Verify build
go build
```

**Determine version number:**
- Check current version: `git describe --tags --abbrev=0`
- Decide bump type:
  - **MINOR** (0.X.0): New features, significant changes
  - **PATCH** (0.X.Y): Bug fixes, small improvements
- Next version format: `v0.X.Y`

### 2. Create Release Tag

**Prepare release notes:**
Categorize recent commits since last tag into:
- New features
- Bug fixes
- UX improvements
- Performance improvements
- Breaking changes (if any)

**Create annotated tag:**
```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z - Brief description

New features:
- Feature 1
- Feature 2

Bug fixes:
- Fix 1
- Fix 2

UX improvements:
- Improvement 1"
```

**Push tag to trigger CI:**
```bash
git push origin vX.Y.Z
```

### 3. Monitor GitHub Actions

**Watch the release workflow:**
```bash
# Get latest run ID and watch it
gh run watch $(gh run list --limit 1 --json databaseId -q '.[0].databaseId')
```

**Expected results:**
- Build completes in ~1-2 minutes
- All platform binaries created:
  - Linux (x86_64, arm64)
  - macOS (x86_64, arm64)
- GitHub Release published with assets
- Homebrew formula auto-updated

**If workflow fails:**
- Check logs: `gh run view <run-id>`
- Common issues:
  - GoReleaser config syntax errors
  - Missing secrets/tokens
  - Build failures on specific platforms
- Fix issues, delete tag, recreate

### 4. Verify GitHub Release

**Check release was created:**
```bash
gh release view vX.Y.Z
```

**Verify assets:**
- [ ] `goplaying_X.Y.Z_darwin_amd64.tar.gz`
- [ ] `goplaying_X.Y.Z_darwin_arm64.tar.gz`
- [ ] `goplaying_X.Y.Z_linux_amd64.tar.gz`
- [ ] `goplaying_X.Y.Z_linux_arm64.tar.gz`
- [ ] `checksums.txt`

### 5. Verify Homebrew Formula Update

**Check tap repository:**
```bash
gh api repos/justinmdickey/homebrew-tap/contents/Formula/goplaying.rb | jq -r '.content' | base64 -d | head -20
```

**Verify:**
- [ ] Version number updated
- [ ] SHA256 checksums updated
- [ ] Formula syntax valid

**Test installation (optional):**
```bash
brew update
brew upgrade goplaying
```

### 6. Update AUR Package

**Get version information:**
```bash
# Get commit count
git rev-list --count HEAD

# Get short hash
git rev-parse --short HEAD
```

**Update goplaying-git package:**
```bash
cd ~/Dev/aur/goplaying-git

# Edit PKGBUILD - update pkgver line
# Format: pkgver=rCOUNT.HASH
# Example: pkgver=r89.abc1234

# Regenerate .SRCINFO
makepkg --printsrcinfo > .SRCINFO

# Verify changes
cat .SRCINFO | head -10
git diff

# Commit and push to AUR
git add PKGBUILD .SRCINFO
git commit -m "Update to rCOUNT.HASH - vX.Y.Z release"
git push origin master
```

**Note:** AUR updates appear within 1-2 minutes

### 7. Post-Release Verification

**Test installations:**
- [ ] GitHub Release: Download and test binary
- [ ] Homebrew: `brew upgrade goplaying` (if available)
- [ ] AUR: `yay -Si goplaying-git` shows new version

**Update documentation (if needed):**
- [ ] README.md updated with new features
- [ ] CHANGELOG.md updated (if exists)
- [ ] TODO.md moved completed items to "Recently Completed"

**Announce release (optional):**
- Post on relevant channels (Reddit, Discord, etc.)
- Share notable features/fixes

## Quick Reference Commands

```bash
# Pre-release checks
git status && git pull origin main
go test ./... -race
make fmt && go build

# Create and push tag
git tag -a vX.Y.Z -m "Release notes"
git push origin vX.Y.Z

# Monitor and verify
gh run watch $(gh run list --limit 1 --json databaseId -q '.[0].databaseId')
gh release view vX.Y.Z

# Update AUR
cd ~/Dev/aur/goplaying-git
# Edit PKGBUILD
makepkg --printsrcinfo > .SRCINFO
git add PKGBUILD .SRCINFO && git commit -m "Update to rXX.XXXXXXX" && git push
```

## Troubleshooting

### GitHub Actions Fails
- View logs: `gh run view <run-id> --log`
- Check GoReleaser config: `.goreleaser.yml`
- Verify secrets are set in repository settings
- Delete bad tag: `git tag -d vX.Y.Z && git push origin :refs/tags/vX.Y.Z`

### Homebrew Not Updating
- Check GoReleaser brew configuration
- Verify tap repository has write access
- Manually update formula if needed

### AUR Update Not Showing
- Wait 2-5 minutes for AUR cache refresh
- Check push succeeded: `cd ~/Dev/aur/goplaying-git && git log -1`
- Verify at: https://aur.archlinux.org/packages/goplaying-git

### Binary Won't Run
- Check architecture matches system
- Verify checksums: `shasum -a 256 -c checksums.txt`
- Test in clean environment

## Release Checklist

Use this checklist for each release:

**Pre-Release:**
- [ ] All changes committed to main
- [ ] Tests passing (`go test ./... -race`)
- [ ] Code formatted (`make fmt`)
- [ ] Build succeeds (`go build`)
- [ ] Version number decided
- [ ] Release notes drafted

**Release:**
- [ ] Tag created with proper format
- [ ] Tag pushed to GitHub
- [ ] CI workflow completed successfully
- [ ] GitHub Release created with all assets

**Post-Release:**
- [ ] Homebrew formula updated
- [ ] AUR package updated
- [ ] Installation tested from ≥1 channel
- [ ] Documentation updated (if needed)
- [ ] Release announced (optional)

## Notes

- **Version format:** `v0.MINOR.PATCH` (0-based during development)
- **Tag format:** Annotated tags only (not lightweight)
- **Release timing:** No specific schedule, release when ready
- **Hotfixes:** Create PATCH version for critical bugs
- **Breaking changes:** Document clearly in release notes

For detailed information, see the Release Process section in [CLAUDE.md](../../CLAUDE.md).
