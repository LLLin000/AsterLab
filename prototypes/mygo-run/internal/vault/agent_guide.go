package vault

import (
	_ "embed"
	"os"
	"path/filepath"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const AgentGuideFileName = "AGENTS.md"

// DefaultVaultAgentGuide is seeded once into new Vault roots (never overwriting
// existing user edits) so any zero-context AI Agent (Claude Code, Cursor, Codex,
// Antigravity, etc.) immediately knows how to read, author, and validate `.asterml`
// experiments and inspect Vault-local Pantry inventory.
//
//go:embed agent_guide.md
var DefaultVaultAgentGuide string

// SeedAgentGuide writes AGENTS.md into the vault root if absent so external
// AI Agents have self-contained instructions without prior conversation context.
func (v *Vault) SeedAgentGuide() error {
	if v == nil || v.Root == "" {
		return nil
	}
	target := filepath.Join(v.Root, AgentGuideFileName)
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	return asterdoc.WriteFileAtomic(target, []byte(DefaultVaultAgentGuide))
}
