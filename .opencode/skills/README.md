# OpenCode Skills

This directory contains custom skills for goplaying development. Skills are specialized workflows that guide you through complex, multi-step tasks.

## Available Skills

### `release.md` - Release Workflow
Complete guide for creating and publishing new releases, including:
- Pre-release verification and testing
- Git tagging and version numbering
- GitHub Actions monitoring
- Distribution updates (GitHub, Homebrew, AUR)
- Post-release verification

**Use when:** You're ready to publish a new version

**Invoke with:** Ask OpenCode to "use the release skill" or "help me create a release"

## How to Use Skills

Skills can be invoked by:

1. **Explicit request:**
   - "Use the release skill to create version 0.3.1"
   - "Help me create a release using the release skill"

2. **Context-based:**
   - "I want to release version 0.3.1" (AI may suggest using the skill)

3. **Direct reference:**
   - "Follow the release skill workflow"

## Creating New Skills

Skills should be created for:
- Multi-step workflows done repeatedly
- Complex processes with specific sequences
- Tasks requiring verification at multiple stages
- Workflows involving multiple tools/repositories

**Format:**
- Markdown file in `.opencode/skills/`
- Clear workflow steps with commands
- Troubleshooting section
- Checklist for verification

**Example use cases for new skills:**
- Adding new media controller methods
- Creating comprehensive test suites
- Platform-specific debugging workflows
- Performance profiling and optimization

## Skill vs Documentation

**Use a skill when:**
- Task is multi-step and procedural
- Order of operations matters
- Verification needed at each stage
- AI should guide you through interactively

**Use documentation (CLAUDE.md) when:**
- Explaining architecture or patterns
- Providing reference information
- Describing codebase structure
- General development guidelines

## Contributing Skills

When adding new skills:
1. Create skill file in `.opencode/skills/`
2. Follow existing skill structure
3. Include clear commands and examples
4. Add troubleshooting section
5. Update this README with skill description
6. Test the skill workflow before committing

Skills should be living documents - update them as processes evolve!
