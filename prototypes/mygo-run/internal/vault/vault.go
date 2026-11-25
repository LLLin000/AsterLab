// Package vault implements AsterLab's Obsidian-style single-library transparent
// filesystem vault: one root directory containing `.asterlab/` system state,
// `assets/` media/attachments, `templates/` date-free `.asterml` templates,
// and arbitrary user project folders holding `.asterml` experiments.
package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const (
	SystemDirName    = ".asterlab"
	AssetsDirName    = "assets"
	TemplatesDirName = "templates"
	TrashDirName     = "trash"
	ConfigFileName   = "config.json"
	LocalFileName    = "local.json"
	WorkspaceFile    = "workspace.json"
	TrashMetaFile    = "trash_meta.json"
)

// LocalMachineState keeps machine-specific paths and UI layout isolated from
// syncable `.asterlab/config.json` so cloud-syncing a Vault never overwrites another device's path.
type LocalMachineState struct {
	VaultPath      string `json:"vaultPath,omitempty"`
	LastProjectDir string `json:"lastProjectDir,omitempty"`
}

// Config holds syncable vault & workspace preferences inside `.asterlab/config.json`.
type Config struct {
	VaultPath           string            `json:"vaultPath,omitempty"` // mirrored from local.json at runtime only
	LastProjectDir      string            `json:"lastProjectDir,omitempty"`
	DefaultProjectDir   string            `json:"defaultProjectDir,omitempty"`
	HideAssetsInTree    bool              `json:"hideAssetsInTree,omitempty"`
	AssetsCollapsed     bool              `json:"assetsCollapsed,omitempty"`
	ThemeSource         string            `json:"themeSource,omitempty"` // system | light | dark
	ThemePreset         string            `json:"themePreset,omitempty"` // aurora | zinc | warm
	LiquidGlass         *bool             `json:"liquidGlass,omitempty"`
	TextScale           float32           `json:"textScale,omitempty"`   // 0 (system) | 0.9 | 1.0 | 1.15 | 1.25
	Density             string            `json:"density,omitempty"`     // comfortable | compact
	ReduceMotion        bool              `json:"reduceMotion,omitempty"`
	DefaultOpenMode     string            `json:"defaultOpenMode,omitempty"` // edit | read
	DefaultPlateFormat  int               `json:"defaultPlateFormat,omitempty"`
	DefaultExcessPct    int               `json:"defaultExcessPct,omitempty"`
	AutoArchiveAssets   *bool             `json:"autoArchiveAssets,omitempty"`
	TemplatesDir        string            `json:"templatesDir,omitempty"`
	FolderTemplates     map[string]string `json:"folderTemplates,omitempty"`
	Hotkeys             map[string]string `json:"hotkeys,omitempty"`
	MobilePullAction    string            `json:"mobilePullAction,omitempty"` // palette | new_exp | calc | tree
	MobileAutoHideDock  *bool             `json:"mobileAutoHideDock,omitempty"`
	MobileEditToolbar   *bool             `json:"mobileEditToolbar,omitempty"`
	FeedbackMode        bool              `json:"feedbackMode,omitempty"`
}

// FileEntry summarizes one `.asterml` experiment or template discovered on disk.
type FileEntry struct {
	RelPath    string // slash-separated path relative to VaultRoot
	AbsPath    string
	Name       string // base filename without .asterml
	Folder     string // parent folder relpath ("" for root)
	IsTemplate bool
	IsAsset    bool
	Title      string
	Date       string  // YYYY-MM-DD (empty if unscheduled or template)
	StartTime  string  // HH:MM (empty if all-day / untimed)
	StartHour  float32 // fractional hour for Week Calendar hourly grid (0 if untimed)
	Status     string  // planned | in_progress | completed
	ModTime    time.Time
	Size       int64
}

// TreeNode represents a folder or file inside the Obsidian-like left file tree.
type TreeNode struct {
	RelPath  string
	AbsPath  string
	Name     string
	IsDir    bool
	IsSystem bool // templates or assets folder
	Entry    *FileEntry
	Children []*TreeNode
}

// TrashItem records a soft-deleted file or folder in `.asterlab/trash/`.
type TrashItem struct {
	TrashName string    `json:"trashName"`
	OrigRel   string    `json:"origRel"`
	DeletedAt time.Time `json:"deletedAt"`
}

// Vault owns the local root directory, event bus, and atomic file operations.
type Vault struct {
	Root   string
	Config Config
	Events *EventBus
}

// DefaultVaultDir returns the default single-vault path on the user's machine.
func DefaultVaultDir() string {
	if env := strings.TrimSpace(os.Getenv("ASTERLAB_VAULT_DIR")); env != "" {
		return filepath.Clean(env)
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "AsterLab", "vault")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "AsterLab")
	}
	return filepath.Join(".", "AsterLabVault")
}

// Open initializes (or opens) an AsterLab vault at rootDir.
func Open(rootDir string) (*Vault, error) {
	if strings.TrimSpace(rootDir) == "" {
		rootDir = DefaultVaultDir()
	}
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{
		abs,
		filepath.Join(abs, SystemDirName),
		filepath.Join(abs, SystemDirName, TrashDirName),
		filepath.Join(abs, SystemDirName, SnapshotsDirName),
		filepath.Join(abs, SystemDirName, "cache"),
		filepath.Join(abs, AssetsDirName),
		filepath.Join(abs, TemplatesDirName),
	} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return nil, fmt.Errorf("init vault dir %s: %w", sub, err)
		}
	}
	v := &Vault{
		Root:   abs,
		Events: &EventBus{},
		Config: Config{
			VaultPath:        abs,
			HideAssetsInTree: false,
			AssetsCollapsed:  true,
		},
	}
	_ = v.LoadConfig()
	return v, nil
}

func (v *Vault) systemPath(parts ...string) string {
	all := append([]string{v.Root, SystemDirName}, parts...)
	return filepath.Join(all...)
}

// AssetsDir returns `<Root>/assets`.
func (v *Vault) AssetsDir() string {
	return filepath.Join(v.Root, AssetsDirName)
}

// TemplatesDir returns `<Root>/templates`.
func (v *Vault) TemplatesDir() string {
	return filepath.Join(v.Root, TemplatesDirName)
}

// AbsPath converts a vault-relative slash path into an OS path inside Root.
func (v *Vault) AbsPath(rel string) string {
	clean := filepath.FromSlash(strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "/"))
	if clean == "" || clean == "." {
		return v.Root
	}
	return filepath.Join(v.Root, clean)
}

// RelPath converts an OS path inside Root into a slash-separated relative path.
func (v *Vault) RelPath(abs string) string {
	rel, err := filepath.Rel(v.Root, abs)
	if err != nil {
		return ""
	}
	slash := filepath.ToSlash(rel)
	if slash == "." {
		return ""
	}
	return slash
}

// LoadConfig reads syncable `.asterlab/config.json` and machine-local `.asterlab/local.json`.
func (v *Vault) LoadConfig() error {
	p := v.systemPath(ConfigFileName)
	if data, err := os.ReadFile(p); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		v.Config = cfg
	}
	if localData, err := os.ReadFile(v.systemPath(LocalFileName)); err == nil {
		var lm LocalMachineState
		if json.Unmarshal(localData, &lm) == nil && lm.LastProjectDir != "" {
			v.Config.LastProjectDir = lm.LastProjectDir
		}
	}
	v.Config.VaultPath = v.Root
	return nil
}

// SaveConfig persists syncable `.asterlab/config.json` (without machine-local absolute paths)
// and stores machine-local state inside `.asterlab/local.json`.
func (v *Vault) SaveConfig() error {
	lm := LocalMachineState{
		VaultPath:      v.Root,
		LastProjectDir: v.Config.LastProjectDir,
	}
	if lData, err := json.MarshalIndent(lm, "", "  "); err == nil {
		_ = asterdoc.WriteFileAtomic(v.systemPath(LocalFileName), lData)
	}
	syncCfg := v.Config
	syncCfg.VaultPath = ""
	syncCfg.LastProjectDir = ""
	data, err := json.MarshalIndent(syncCfg, "", "  ")
	if err != nil {
		return err
	}
	return asterdoc.WriteFileAtomic(v.systemPath(ConfigFileName), data)
}

// LoadWorkspaceJSON reads `.asterlab/workspace.json`.
func (v *Vault) LoadWorkspaceJSON(dst any) error {
	data, err := os.ReadFile(v.systemPath(WorkspaceFile))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// SaveWorkspaceJSON writes `.asterlab/workspace.json` atomically.
func (v *Vault) SaveWorkspaceJSON(src any) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	return asterdoc.WriteFileAtomic(v.systemPath(WorkspaceFile), data)
}

// SeedTemplates populates `templates/` once when empty with starter protocol templates.
func (v *Vault) SeedTemplates(seeds map[string]string) error {
	if len(seeds) == 0 {
		return nil
	}
	entries, err := os.ReadDir(v.TemplatesDir())
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".asterml") {
				return nil
			}
		}
	}
	for name, content := range seeds {
		fname := SanitizeFileName(name)
		if !strings.HasSuffix(strings.ToLower(fname), ".asterml") {
			fname += ".asterml"
		}
		target := filepath.Join(v.TemplatesDir(), fname)
		clean, err := asterdoc.ClearExecutionDataForTemplate(content, strings.TrimSuffix(fname, ".asterml"))
		if err != nil {
			clean = content
		}
		if err := asterdoc.WriteFileAtomic(target, []byte(clean)); err != nil {
			return err
		}
	}
	return nil
}

// ScanTree reads the vault folder hierarchy and all `.asterml` file entries.
// `.asterlab/` is always excluded; `assets/` is placed at the bottom (or omitted when HideAssetsInTree).
func (v *Vault) ScanTree() (*TreeNode, []FileEntry, error) {
	rootNode := &TreeNode{
		RelPath: "",
		AbsPath: v.Root,
		Name:    filepath.Base(v.Root),
		IsDir:   true,
	}
	var allFiles []FileEntry
	if err := v.walkDir(rootNode, &allFiles); err != nil {
		return nil, nil, err
	}
	return rootNode, allFiles, nil
}

func (v *Vault) walkDir(dirNode *TreeNode, allFiles *[]FileEntry) error {
	entries, err := os.ReadDir(dirNode.AbsPath)
	if err != nil {
		return err
	}
	var dirs, files []*TreeNode
	var assetsNode *TreeNode

	for _, de := range entries {
		name := de.Name()
		if name == SystemDirName || strings.HasPrefix(name, ".") {
			continue
		}
		abs := filepath.Join(dirNode.AbsPath, name)
		rel := v.RelPath(abs)
		if de.IsDir() {
			if dirNode.RelPath == "" && name == AssetsDirName {
				if v.Config.HideAssetsInTree {
					continue
				}
				an := &TreeNode{RelPath: rel, AbsPath: abs, Name: name, IsDir: true, IsSystem: true}
				_ = v.walkAssetsDir(an)
				assetsNode = an
				continue
			}
			child := &TreeNode{
				RelPath:  rel,
				AbsPath:  abs,
				Name:     name,
				IsDir:    true,
				IsSystem: dirNode.RelPath == "" && name == TemplatesDirName,
			}
			if err := v.walkDir(child, allFiles); err == nil {
				dirs = append(dirs, child)
			}
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".asterml") {
			continue
		}
		fe := v.inspectFileEntry(abs, rel, dirNode.RelPath)
		*allFiles = append(*allFiles, fe)
		files = append(files, &TreeNode{
			RelPath: rel,
			AbsPath: abs,
			Name:    fe.Name,
			IsDir:   false,
			Entry:   &fe,
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].IsSystem != dirs[j].IsSystem {
			return !dirs[i].IsSystem
		}
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	dirNode.Children = append(dirNode.Children, dirs...)
	dirNode.Children = append(dirNode.Children, files...)
	if assetsNode != nil {
		dirNode.Children = append(dirNode.Children, assetsNode)
	}
	return nil
}

func (v *Vault) walkAssetsDir(an *TreeNode) error {
	entries, err := os.ReadDir(an.AbsPath)
	if err != nil {
		return err
	}
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		abs := filepath.Join(an.AbsPath, de.Name())
		rel := v.RelPath(abs)
		info, _ := de.Info()
		var mt time.Time
		var sz int64
		if info != nil {
			mt = info.ModTime()
			sz = info.Size()
		}
		an.Children = append(an.Children, &TreeNode{
			RelPath: rel,
			AbsPath: abs,
			Name:    de.Name(),
			Entry: &FileEntry{
				RelPath: rel,
				AbsPath: abs,
				Name:    de.Name(),
				Folder:  AssetsDirName,
				IsAsset: true,
				Title:   de.Name(),
				ModTime: mt,
				Size:    sz,
			},
		})
	}
	return nil
}

func (v *Vault) inspectFileEntry(abs, rel, folder string) FileEntry {
	info, _ := os.Stat(abs)
	var mt time.Time
	var sz int64
	if info != nil {
		mt = info.ModTime()
		sz = info.Size()
	}
	baseName := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	isTmpl := folder == TemplatesDirName || strings.HasPrefix(rel, TemplatesDirName+"/")
	fe := FileEntry{
		RelPath:    rel,
		AbsPath:    abs,
		Name:       baseName,
		Folder:     folder,
		IsTemplate: isTmpl,
		Title:      baseName,
		Status:     "planned",
		ModTime:    mt,
		Size:       sz,
	}
	rawBytes, err := os.ReadFile(abs)
	if err != nil {
		return fe
	}
	raw := string(rawBytes)
	if fm, _, ok := asterdoc.ScanFrontmatter(raw); ok {
		if strings.TrimSpace(fm.Title) != "" {
			fe.Title = strings.TrimSpace(fm.Title)
		}
		if !isTmpl {
			fe.Date = asterdoc.NormalizeDateYMD(fm.ScheduledDate)
			fe.StartTime = asterdoc.NormalizeStartTimeHHMM(fm.StartTime)
			fe.StartHour = asterdoc.ParseStartHour(fe.StartTime)
		}
		if strings.TrimSpace(fm.Stage) != "" {
			fe.Status = normalizeStatus(fm.Stage)
		}
	}
	if fe.Date == "" && !isTmpl {
		meta := asterdoc.InspectExperiment(raw, nil)
		if meta.ScheduledDate != "" {
			fe.Date = meta.ScheduledDate
		}
		if fe.Title == baseName && strings.TrimSpace(meta.Title) != "" {
			fe.Title = meta.Title
		}
	}
	return fe
}

func normalizeStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "in_progress", "running":
		return "in_progress"
	case "completed", "done":
		return "completed"
	default:
		return "planned"
	}
}

// ProjectFolders returns all folder relative paths (excluding `.asterlab`, `assets`, `templates`).
func (v *Vault) ProjectFolders() []string {
	var folders []string
	_ = filepath.WalkDir(v.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path == v.Root {
			return nil
		}
		name := d.Name()
		if name == SystemDirName || strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		rel := v.RelPath(path)
		if rel == AssetsDirName || rel == TemplatesDirName ||
			strings.HasPrefix(rel, AssetsDirName+"/") || strings.HasPrefix(rel, TemplatesDirName+"/") {
			return filepath.SkipDir
		}
		folders = append(folders, rel)
		return nil
	})
	sort.Strings(folders)
	return folders
}

// CreateFolder creates a project directory under parentRel ("" for vault root).
func (v *Vault) CreateFolder(parentRel, name string) (string, error) {
	clean := SanitizeFileName(name)
	if clean == "" {
		return "", fmt.Errorf("文件夹名称不能为空")
	}
	parentAbs := v.AbsPath(parentRel)
	targetAbs := filepath.Join(parentAbs, clean)
	if _, err := os.Stat(targetAbs); err == nil {
		return "", fmt.Errorf("文件夹 %q 已存在", clean)
	}
	if err := os.MkdirAll(targetAbs, 0o755); err != nil {
		return "", err
	}
	return v.RelPath(targetAbs), nil
}

// CreateExperiment creates a new `.asterml` file in folderRel ("" for root)
// with optional template source and scheduled date, avoiding filename collisions.
func (v *Vault) CreateExperiment(folderRel, title, dateYMD, templateSource string) (FileEntry, error) {
	baseTitle := strings.TrimSpace(title)
	if baseTitle == "" {
		baseTitle = "未命名实验"
	}
	dirAbs := v.AbsPath(folderRel)
	if err := os.MkdirAll(dirAbs, 0o755); err != nil {
		return FileEntry{}, err
	}
	targetAbs := v.uniqueAsterMLPath(dirAbs, SanitizeFileName(baseTitle))
	if strings.TrimSpace(templateSource) == "" {
		return FileEntry{}, fmt.Errorf("缺少初始实验内容")
	}
	src := templateSource
	// Instantiate template via asterdoc.InstantiateTemplate so module IDs and internal
	// references are freshly remapped instead of duplicating template IDs.
	if instantiated, instErr := asterdoc.InstantiateTemplate(src, nil); instErr == nil && strings.TrimSpace(instantiated) != "" {
		src = instantiated
	}
	if norm := asterdoc.NormalizeDateYMD(dateYMD); norm != "" {
		src = asterdoc.SetFrontmatterMetadata(src, baseTitle, norm, "planned")
	} else {
		src = asterdoc.SetFrontmatterMetadata(src, baseTitle, "", "planned")
	}
	if err := asterdoc.WriteFileAtomic(targetAbs, []byte(src)); err != nil {
		return FileEntry{}, err
	}
	if folderRel != TemplatesDirName && !strings.HasPrefix(folderRel, TemplatesDirName+"/") {
		v.Config.LastProjectDir = folderRel
		_ = v.SaveConfig()
	}
	rel := v.RelPath(targetAbs)
	if v.Events != nil {
		v.Events.Emit(VaultEvent{Kind: EventCreate, RelPath: rel})
	}
	return v.inspectFileEntry(targetAbs, rel, folderRel), nil
}

// SetExperimentStartTime updates start_time (HH:MM) in an experiment's frontmatter.
func (v *Vault) SetExperimentStartTime(relPath, startTimeHHMM string) error {
	abs := v.AbsPath(relPath)
	raw, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	updated := asterdoc.SetFrontmatterStartTime(string(raw), startTimeHHMM)
	if updated == string(raw) {
		return nil
	}
	return asterdoc.WriteFileAtomic(abs, []byte(updated))
}

// DuplicateFile creates a non-colliding copy in the same folder (`<Name> 2.asterml`),
// optionally assigning a new target date (for biological repeats).
func (v *Vault) DuplicateFile(relPath, targetDateYMD string) (FileEntry, error) {
	abs := v.AbsPath(relPath)
	rawBytes, err := os.ReadFile(abs)
	if err != nil {
		return FileEntry{}, err
	}
	dirAbs := filepath.Dir(abs)
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	nextAbs := v.uniqueAsterMLPath(dirAbs, base)
	newTitle := strings.TrimSuffix(filepath.Base(nextAbs), filepath.Ext(nextAbs))
	src := string(rawBytes)
	if strings.TrimSpace(targetDateYMD) != "" {
		src = asterdoc.SetFrontmatterMetadata(src, newTitle, targetDateYMD, "planned")
	} else {
		src = asterdoc.SetFrontmatterMetadata(src, newTitle, "", "planned")
	}
	if err := asterdoc.WriteFileAtomic(nextAbs, []byte(src)); err != nil {
		return FileEntry{}, err
	}
	nextRel := v.RelPath(nextAbs)
	return v.inspectFileEntry(nextAbs, nextRel, v.RelPath(dirAbs)), nil
}

// SaveAsTemplate writes a date-free, execution-cleared copy into `templates/`.
func (v *Vault) SaveAsTemplate(relPath, templateName string) (FileEntry, error) {
	abs := v.AbsPath(relPath)
	rawBytes, err := os.ReadFile(abs)
	if err != nil {
		return FileEntry{}, err
	}
	base := strings.TrimSpace(templateName)
	if base == "" {
		base = strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs)) + " 模板"
	}
	clean, err := asterdoc.ClearExecutionDataForTemplate(string(rawBytes), base)
	if err != nil {
		return FileEntry{}, err
	}
	targetAbs := v.uniqueAsterMLPath(v.TemplatesDir(), SanitizeFileName(base))
	if err := asterdoc.WriteFileAtomic(targetAbs, []byte(clean)); err != nil {
		return FileEntry{}, err
	}
	rel := v.RelPath(targetAbs)
	return v.inspectFileEntry(targetAbs, rel, TemplatesDirName), nil
}

// Rename renames a file (updating its title metadata) or folder on disk.
func (v *Vault) Rename(relPath, newName string) (string, error) {
	clean := SanitizeFileName(newName)
	if clean == "" {
		return "", fmt.Errorf("名称不能为空")
	}
	oldAbs := v.AbsPath(relPath)
	info, err := os.Stat(oldAbs)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(oldAbs)
	targetName := clean
	if !info.IsDir() && strings.HasSuffix(strings.ToLower(oldAbs), ".asterml") && !strings.HasSuffix(strings.ToLower(clean), ".asterml") {
		targetName += ".asterml"
	}
	newAbs := filepath.Join(dir, targetName)
	if strings.EqualFold(oldAbs, newAbs) && oldAbs == newAbs {
		return relPath, nil
	}
	if _, err := os.Stat(newAbs); err == nil && !strings.EqualFold(oldAbs, newAbs) {
		return "", fmt.Errorf("%q 已存在", targetName)
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return "", err
	}
	if !info.IsDir() && strings.HasSuffix(strings.ToLower(newAbs), ".asterml") {
		if raw, rErr := os.ReadFile(newAbs); rErr == nil {
			titleOnly := strings.TrimSuffix(targetName, filepath.Ext(targetName))
			fm, _, _ := asterdoc.ScanFrontmatter(string(raw))
			updated := asterdoc.SetFrontmatterMetadata(string(raw), titleOnly, fm.ScheduledDate, fm.Stage)
			_ = asterdoc.WriteFileAtomic(newAbs, []byte(updated))
		}
	}
	newRel := v.RelPath(newAbs)
	if !info.IsDir() && strings.HasSuffix(strings.ToLower(newAbs), ".asterml") {
		_, _ = v.RewriteWikiAndRepeatLinks(relPath, newRel)
	}
	if v.Events != nil {
		v.Events.Emit(VaultEvent{Kind: EventRename, RelPath: newRel, OldPath: relPath})
	}
	return newRel, nil
}

// Move moves a file or folder into destFolderRel ("" for vault root).
func (v *Vault) Move(srcRel, destFolderRel string) (string, error) {
	srcAbs := v.AbsPath(srcRel)
	destDirAbs := v.AbsPath(destFolderRel)
	if err := os.MkdirAll(destDirAbs, 0o755); err != nil {
		return "", err
	}
	if strings.HasPrefix(filepath.Clean(destDirAbs)+string(filepath.Separator), filepath.Clean(srcAbs)+string(filepath.Separator)) {
		return "", fmt.Errorf("不能将文件夹移动到自身子目录")
	}
	base := filepath.Base(srcAbs)
	destAbs := filepath.Join(destDirAbs, base)
	if srcAbs == destAbs {
		return srcRel, nil
	}
	if info, err := os.Stat(srcAbs); err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(base), ".asterml") {
		if _, statErr := os.Stat(destAbs); statErr == nil {
			stem := strings.TrimSuffix(base, filepath.Ext(base))
			destAbs = v.uniqueAsterMLPath(destDirAbs, stem)
		}
	} else if _, statErr := os.Stat(destAbs); statErr == nil {
		return "", fmt.Errorf("目标目录已存在同名项 %q", base)
	}
	if err := os.Rename(srcAbs, destAbs); err != nil {
		return "", err
	}
	newRel := v.RelPath(destAbs)
	if info, stErr := os.Stat(destAbs); stErr == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(destAbs), ".asterml") {
		_, _ = v.RewriteWikiAndRepeatLinks(srcRel, newRel)
	}
	if v.Events != nil {
		v.Events.Emit(VaultEvent{Kind: EventRename, RelPath: newRel, OldPath: srcRel})
	}
	return newRel, nil
}

// Trash soft-deletes a file or folder into `.asterlab/trash/`.
func (v *Vault) Trash(relPath string) (TrashItem, error) {
	srcAbs := v.AbsPath(relPath)
	if _, err := os.Stat(srcAbs); err != nil {
		return TrashItem{}, err
	}
	stamp := time.Now().Format("20060102-150405")
	trashName := fmt.Sprintf("%s_%s", stamp, filepath.Base(srcAbs))
	destAbs := v.systemPath(TrashDirName, trashName)
	for n := 2; ; n++ {
		if _, err := os.Stat(destAbs); os.IsNotExist(err) {
			break
		}
		trashName = fmt.Sprintf("%s_%d_%s", stamp, n, filepath.Base(srcAbs))
		destAbs = v.systemPath(TrashDirName, trashName)
	}
	if err := os.Rename(srcAbs, destAbs); err != nil {
		return TrashItem{}, err
	}
	item := TrashItem{TrashName: trashName, OrigRel: relPath, DeletedAt: time.Now()}
	_ = v.appendTrashMeta(item)
	if v.Events != nil {
		v.Events.Emit(VaultEvent{Kind: EventDelete, RelPath: relPath})
	}
	return item, nil
}

// RestoreFromTrash moves a soft-deleted item back to its original location.
func (v *Vault) RestoreFromTrash(trashName string) (string, error) {
	items := v.listTrashMeta()
	var target *TrashItem
	var remaining []TrashItem
	for i := range items {
		if items[i].TrashName == trashName {
			cp := items[i]
			target = &cp
		} else {
			remaining = append(remaining, items[i])
		}
	}
	trashAbs := v.systemPath(TrashDirName, trashName)
	if _, err := os.Stat(trashAbs); err != nil {
		return "", err
	}
	origRel := strings.TrimPrefix(trashName, strings.SplitN(trashName, "_", 2)[0]+"_")
	if target != nil && target.OrigRel != "" {
		origRel = target.OrigRel
	}
	origAbs := v.AbsPath(origRel)
	_ = os.MkdirAll(filepath.Dir(origAbs), 0o755)
	if err := os.Rename(trashAbs, origAbs); err != nil {
		return "", err
	}
	_ = v.saveTrashMeta(remaining)
	return v.RelPath(origAbs), nil
}

// ListTrash returns all recorded soft-deleted items in `.asterlab/trash/`, newest first.
func (v *Vault) ListTrash() []TrashItem {
	items := v.listTrashMeta()
	out := make([]TrashItem, 0, len(items))
	for _, it := range items {
		if _, err := os.Stat(v.systemPath(TrashDirName, it.TrashName)); err == nil {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DeletedAt.After(out[j].DeletedAt)
	})
	return out
}

func (v *Vault) listTrashMeta() []TrashItem {
	data, err := os.ReadFile(v.systemPath(TrashMetaFile))
	if err != nil {
		return nil
	}
	var out []TrashItem
	_ = json.Unmarshal(data, &out)
	return out
}

func (v *Vault) saveTrashMeta(items []TrashItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return asterdoc.WriteFileAtomic(v.systemPath(TrashMetaFile), data)
}

func (v *Vault) appendTrashMeta(item TrashItem) error {
	items := v.listTrashMeta()
	items = append(items, item)
	return v.saveTrashMeta(items)
}

// ImportAssetBytes stores pasted/dropped image or document bytes into `<Vault>/assets/`
// using a short content hash to prevent collisions, returning a vault-relative `./assets/<file>` ref.
func (v *Vault) ImportAssetBytes(data []byte, origName string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("空资源内容")
	}
	if err := os.MkdirAll(v.AssetsDir(), 0o755); err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(origName)))
	if ext == "" {
		ext = ".png"
	}
	stem := SanitizeFileName(strings.TrimSuffix(filepath.Base(origName), filepath.Ext(origName)))
	if stem == "" {
		stem = "figure"
	}
	sum := sha256.Sum256(data)
	shortHash := hex.EncodeToString(sum[:4])
	fileName := fmt.Sprintf("%s_%s%s", stem, shortHash, ext)
	dest := filepath.Join(v.AssetsDir(), fileName)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		if err := asterdoc.WriteFileAtomic(dest, data); err != nil {
			return "", err
		}
	}
	return "./" + AssetsDirName + "/" + fileName, nil
}

// ImportAssetFile copies an external local file into `<Vault>/assets/`.
func (v *Vault) ImportAssetFile(srcPath string) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	return v.ImportAssetBytes(data, filepath.Base(srcPath))
}

// UpdateFileDate updates an `.asterml` file's date metadata on disk.
func (v *Vault) UpdateFileDate(relPath, dateYMD string) error {
	abs := v.AbsPath(relPath)
	raw, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	fe := v.inspectFileEntry(abs, relPath, v.RelPath(filepath.Dir(abs)))
	updated := asterdoc.SetFrontmatterMetadata(string(raw), fe.Title, dateYMD, fe.Status)
	return asterdoc.WriteFileAtomic(abs, []byte(updated))
}

func (v *Vault) uniqueAsterMLPath(dirAbs, baseStem string) string {
	stem := SanitizeFileName(baseStem)
	if stem == "" {
		stem = "未命名实验"
	}
	cand := filepath.Join(dirAbs, stem+".asterml")
	if _, err := os.Stat(cand); os.IsNotExist(err) {
		return cand
	}
	for n := 2; ; n++ {
		c := filepath.Join(dirAbs, stem+" "+strconv.Itoa(n)+".asterml")
		if _, err := os.Stat(c); os.IsNotExist(err) {
			return c
		}
	}
}

// SanitizeFileName strips OS-forbidden filename characters while keeping CJK and spaces.
func SanitizeFileName(name string) string {
	s := strings.TrimSpace(name)
	replacer := strings.NewReplacer(
		"<", "_", ">", "_", ":", "_", "\"", "_",
		"/", "_", "\\", "_", "|", "_", "?", "_", "*", "_",
		"\r", "", "\n", "", "\t", " ",
	)
	s = replacer.Replace(s)
	s = strings.Trim(s, ". ")
	return s
}

// NormalizeFolderPath validates and normalizes a vault-relative folder path using '/' separators.
// Empty string or "/" resolves to "" (vault root). Single names create root folders; nested "a/b"
// paths create nested subfolders. Rejects traversal ("..", "."), empty segments, and system roots.
func NormalizeFolderPath(raw string) (string, error) {
	norm := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	norm = strings.Trim(norm, "/")
	if norm == "" {
		return "", nil
	}
	parts := strings.Split(norm, "/")
	cleaned := make([]string, 0, len(parts))
	for i, part := range parts {
		seg := strings.TrimSpace(part)
		if seg == "" {
			return "", fmt.Errorf("文件夹路径包含空目录名")
		}
		if seg == "." || seg == ".." {
			return "", fmt.Errorf("文件夹路径不能包含 . 或 ..")
		}
		if i == 0 && (strings.EqualFold(seg, SystemDirName) || strings.EqualFold(seg, AssetsDirName) || strings.EqualFold(seg, TemplatesDirName)) {
			return "", fmt.Errorf("不能使用系统保留目录 %q", seg)
		}
		sanitized := SanitizeFileName(seg)
		if sanitized == "" {
			return "", fmt.Errorf("文件夹名称 %q 包含无效字符", seg)
		}
		if i == 0 && (strings.EqualFold(sanitized, SystemDirName) || strings.EqualFold(sanitized, AssetsDirName) || strings.EqualFold(sanitized, TemplatesDirName)) {
			return "", fmt.Errorf("不能使用系统保留目录 %q", sanitized)
		}
		cleaned = append(cleaned, sanitized)
	}
	return strings.Join(cleaned, "/"), nil
}

// MigrateLegacyExperiments safely imports any existing `legacyDataDir/experiments/*/experiment.asterml`
// into the vault once, flattening legacy exec/record sections into single-source `.asterml`.
func (v *Vault) MigrateLegacyExperiments(legacyDataDir string) (int, error) {
	if strings.TrimSpace(legacyDataDir) == "" {
		return 0, nil
	}
	marker := v.systemPath("migrated_legacy.flag")
	if _, err := os.Stat(marker); err == nil {
		return 0, nil
	}
	expRoot := filepath.Join(legacyDataDir, "experiments")
	entries, err := os.ReadDir(expRoot)
	if err != nil {
		return 0, nil
	}
	migrated := 0
	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		srcFile := filepath.Join(expRoot, de.Name(), "experiment.asterml")
		rawBytes, rErr := os.ReadFile(srcFile)
		if rErr != nil {
			continue
		}
		// Copy any local ./assets/* into vault/assets/
		legacyAssets := filepath.Join(expRoot, de.Name(), "assets")
		if assetEntries, aErr := os.ReadDir(legacyAssets); aErr == nil {
			for _, ae := range assetEntries {
				if !ae.IsDir() {
					_, _ = v.ImportAssetFile(filepath.Join(legacyAssets, ae.Name()))
				}
			}
		}
		flat, fErr := asterdoc.FlattenEnvelopeToSingleSource(string(rawBytes))
		if fErr != nil {
			flat = string(rawBytes)
		}
		meta := asterdoc.InspectExperiment(flat, nil)
		title := meta.Title
		if fm, _, ok := asterdoc.ScanFrontmatter(flat); ok && strings.TrimSpace(fm.Title) != "" {
			title = fm.Title
		}
		if strings.TrimSpace(title) == "" {
			title = de.Name()
		}
		target := v.uniqueAsterMLPath(v.Root, SanitizeFileName(title))
		if wErr := asterdoc.WriteFileAtomic(target, []byte(flat)); wErr == nil {
			migrated++
		}
	}
	_ = asterdoc.WriteFileAtomic(marker, []byte(time.Now().Format(time.RFC3339)))
	return migrated, nil
}
