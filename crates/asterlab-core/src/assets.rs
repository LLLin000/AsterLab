//! Local asset references for a Run.
//!
//! An asset link stores a logical root plus a relative path and the file's
//! observable metadata. It never copies the file and never persists an
//! absolute path: the workspace owns the root-id -> directory mapping, so a
//! moved data directory only changes one mapping instead of every link.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    fmt,
    fs::{self, File},
    io::Read,
    path::{Component, Path, PathBuf},
    time::{SystemTime, UNIX_EPOCH},
};

/// Coarse classification used for display and open behaviour.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum MediaType {
    Image,
    Table,
    Document,
    Binary,
    Unknown,
}

/// Whether the referenced file can be read and still matches its captured state.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Availability {
    Available,
    Missing,
    Modified,
    Unavailable,
}

/// Evidence used to identify a moved file without changing its logical identity.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AssetMatchMethod {
    Sha256,
    Metadata,
}

/// Durable reference plus the state captured when the link was created.
///
/// `size`, `mod_time` and `sha256` are evidence that the file was not silently
/// swapped. They are observations, not a second copy of the content.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AssetRef {
    pub id: String,
    pub root_id: String,
    pub relative_path: String,
    pub media_type: MediaType,
    pub size: u64,
    pub mod_time: String,
    #[serde(default)]
    pub sha256: String,
    pub availability: Availability,
}

/// A candidate relative path that matched the stored file evidence.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AssetCandidate {
    pub relative_path: String,
    pub matched_by: AssetMatchMethod,
}

/// Immutable before/after evidence for rebinding a reference.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AssetRebinding {
    pub previous: AssetRef,
    pub updated: AssetRef,
    pub matched_by: AssetMatchMethod,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AssetError(pub String);

impl fmt::Display for AssetError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for AssetError {}

impl From<std::io::Error> for AssetError {
    fn from(error: std::io::Error) -> Self {
        Self(error.to_string())
    }
}

fn invalid(message: impl Into<String>) -> AssetError {
    AssetError(message.into())
}

/// Maps an extension to a coarse media type. Unknown extensions stay unknown;
/// a guess must never turn a binary file into a "table".
pub fn classify(path: &Path) -> MediaType {
    let extension = path
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or_default()
        .to_ascii_lowercase();
    match extension.as_str() {
        "png" | "jpg" | "jpeg" | "tif" | "tiff" | "bmp" | "gif" | "webp" => MediaType::Image,
        "csv" | "tsv" | "xlsx" | "xls" | "txt" => MediaType::Table,
        "pdf" | "doc" | "docx" | "md" => MediaType::Document,
        "bin" | "dat" | "zip" | "raw" => MediaType::Binary,
        _ => MediaType::Unknown,
    }
}

/// True for a Windows drive prefix (`C:`, `C:/`, `C:\`) at the start of a
/// path-like value. Checked on every platform so a reference created on Linux
/// cannot become absolute when the data directory moves to Windows.
fn starts_with_drive_prefix(value: &str) -> bool {
    let bytes = value.as_bytes();
    bytes.len() >= 2
        && bytes[0].is_ascii_alphabetic()
        && bytes[1] == b':'
        && (bytes.len() == 2 || matches!(bytes[2], b'/' | b'\\'))
}

/// A logical root ID is persisted and exported, so it must not be a filesystem
/// path: an absolute path here would leak the local layout even though the
/// referenced file itself stays relative.
fn validate_root_id(value: &str) -> Result<(), AssetError> {
    if value.is_empty() {
        return Err(invalid("root ID is required"));
    }
    if value.len() > 200 || value.chars().any(char::is_control) {
        return Err(invalid("root ID is invalid"));
    }
    if value.starts_with('/')
        || value.starts_with('\\')
        || starts_with_drive_prefix(value)
        || Path::new(value).is_absolute()
    {
        return Err(invalid(
            "root ID must be a logical identifier, not an absolute path",
        ));
    }
    if value.split(['/', '\\']).any(|segment| segment == "..") {
        return Err(invalid("root ID must not contain parent-directory segments"));
    }
    Ok(())
}

/// Normalizes a relative path to forward slashes and rejects anything that
/// could escape the logical root.
pub fn normalize_relative_path(value: &str) -> Result<String, AssetError> {
    if value.is_empty() {
        return Err(invalid("relative path is required"));
    }
    if value.chars().any(char::is_control) {
        return Err(invalid("relative path contains a control character"));
    }
    let normalized = value.replace('\\', "/");
    let path = Path::new(&normalized);
    if path.is_absolute() || starts_with_drive_prefix(&normalized) {
        return Err(invalid("relative path must not be absolute"));
    }
    let mut parts = Vec::new();
    for component in path.components() {
        match component {
            Component::Normal(part) => {
                let part = part
                    .to_str()
                    .ok_or_else(|| invalid("relative path is not valid UTF-8"))?;
                if part.is_empty() {
                    return Err(invalid("relative path is invalid"));
                }
                parts.push(part.to_owned());
            }
            Component::CurDir => return Err(invalid("relative path must be normalized")),
            Component::ParentDir => {
                return Err(invalid("relative path must not escape its root"))
            }
            Component::RootDir | Component::Prefix(_) => {
                return Err(invalid("relative path must not be absolute"))
            }
        }
    }
    if parts.is_empty() {
        return Err(invalid("relative path is required"));
    }
    Ok(parts.join("/"))
}

/// Resolves a relative path inside an existing root, following symlinks and
/// requiring the final target to stay inside that root.
pub fn resolve(root_dir: &Path, relative_path: &str) -> Result<PathBuf, AssetError> {
    let relative_path = normalize_relative_path(relative_path)?;
    let root = fs::canonicalize(root_dir)
        .map_err(|error| invalid(format!("resolve asset root {}: {error}", root_dir.display())))?;
    if !fs::metadata(&root)
        .map(|metadata| metadata.is_dir())
        .unwrap_or(false)
    {
        return Err(invalid("asset root is not a directory"));
    }
    let target = fs::canonicalize(root.join(&relative_path)).map_err(|error| {
        invalid(format!(
            "resolve asset {relative_path}: {error}; the file may be unavailable"
        ))
    })?;
    if !target.starts_with(&root) {
        return Err(invalid("asset resolves outside its root"));
    }
    Ok(target)
}

/// Streams a reader through SHA-256. No buffer proportional to the content
/// size and no temporary copy is created.
fn hash_reader<R: Read>(mut reader: R) -> Result<String, AssetError> {
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 8192];
    loop {
        let read = reader.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    Ok(hasher
        .finalize()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect())
}

/// Streams a regular file through SHA-256.
pub fn hash_file(path: &Path) -> Result<String, AssetError> {
    let metadata = fs::metadata(path)?;
    if !metadata.is_file() {
        return Err(invalid("asset path must be a regular file"));
    }
    hash_reader(File::open(path)?)
}

/// Compares a stored hash against a freshly computed one.
///
/// `hash_reader` emits lowercase hex, but references can be imported from
/// external JSON whose hash is uppercase; hex case is not evidence of change.
fn hashes_equal(stored: &str, computed: &str) -> bool {
    stored.eq_ignore_ascii_case(computed)
}

/// Builds a validated reference from a logical root and either an absolute
/// file path or a path relative to the root.
///
/// `root_id` is a logical identifier, never a filesystem path; `root_dir` is
/// used for this call only and is not stored. The file must exist and be
/// readable: a link is created from observed metadata, never from
/// caller-supplied size or hash fields.
pub fn link(
    root_id: &str,
    root_dir: &Path,
    path: Option<&str>,
    relative_path: Option<&str>,
) -> Result<AssetRef, AssetError> {
    let root_id = root_id.trim();
    validate_root_id(root_id)?;
    let root = fs::canonicalize(root_dir)
        .map_err(|error| invalid(format!("resolve asset root {}: {error}", root_dir.display())))?;
    if !fs::metadata(&root)
        .map(|metadata| metadata.is_dir())
        .unwrap_or(false)
    {
        return Err(invalid("asset root is not a directory"));
    }

    let (relative_path, target) = match (path, relative_path) {
        (Some(path), declared) => {
            let target = fs::canonicalize(path)
                .map_err(|error| invalid(format!("resolve asset path {path}: {error}")))?;
            let derived = target
                .strip_prefix(&root)
                .map_err(|_| invalid("asset is outside its root"))?;
            let derived = normalize_relative_path(
                derived
                    .to_str()
                    .ok_or_else(|| invalid("asset path is not valid UTF-8"))?,
            )?;
            if let Some(declared) = declared {
                if normalize_relative_path(declared)? != derived {
                    return Err(invalid(
                        "path and relative_path do not identify the same file",
                    ));
                }
            }
            (derived, target)
        }
        (None, Some(relative_path)) => {
            let relative_path = normalize_relative_path(relative_path)?;
            let target = resolve(&root, &relative_path)?;
            (relative_path, target)
        }
        (None, None) => return Err(invalid("path or relative_path is required")),
    };

    let metadata = fs::metadata(&target)?;
    if !metadata.is_file() {
        return Err(invalid("asset path must be a regular file"));
    }
    let mut file = File::open(&target)?;
    // Re-read from the opened handle so size, mtime and hash describe the same
    // file generation even if the path was replaced between the checks.
    let metadata = file.metadata()?;
    if !metadata.is_file() {
        return Err(invalid("asset path must be a regular file"));
    }
    let mod_time = metadata
        .modified()
        .map(format_rfc3339)
        .unwrap_or_else(|_| String::new());
    let sha256 = hash_reader(&mut file)?;
    Ok(AssetRef {
        id: stable_id(root_id, &relative_path),
        root_id: root_id.to_owned(),
        media_type: classify(Path::new(&relative_path)),
        relative_path,
        size: metadata.len(),
        mod_time,
        sha256,
        availability: Availability::Available,
    })
}

/// Checks that the stored relative path still resolves to the captured file.
/// Missing paths and changed contents are distinct from inaccessible paths.
pub fn inspect(root_dir: &Path, asset: &AssetRef) -> Result<Availability, AssetError> {
    let relative_path = normalize_relative_path(&asset.relative_path)?;
    let root = match fs::canonicalize(root_dir) {
        Ok(root) => root,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(Availability::Missing);
        }
        Err(_) => return Ok(Availability::Unavailable),
    };
    let root_metadata = match fs::metadata(&root) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(Availability::Missing);
        }
        Err(_) => return Ok(Availability::Unavailable),
    };
    if !root_metadata.is_dir() {
        return Ok(Availability::Unavailable);
    }
    let target = match fs::canonicalize(root.join(&relative_path)) {
        Ok(target) => target,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(Availability::Missing);
        }
        Err(_) => return Ok(Availability::Unavailable),
    };
    if !target.starts_with(&root) {
        return Err(invalid("asset resolves outside its root"));
    }
    let mut file = match File::open(target) {
        Ok(file) => file,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(Availability::Missing);
        }
        Err(_) => return Ok(Availability::Unavailable),
    };
    let metadata = match file.metadata() {
        Ok(metadata) => metadata,
        Err(_) => return Ok(Availability::Unavailable),
    };
    if !metadata.is_file() {
        return Ok(Availability::Unavailable);
    }
    if !asset.sha256.is_empty() {
        let current_hash = match hash_reader(&mut file) {
            Ok(hash) => hash,
            Err(_) => return Ok(Availability::Unavailable),
        };
        return Ok(if hashes_equal(&asset.sha256, &current_hash) {
            Availability::Available
        } else {
            Availability::Modified
        });
    }
    let mod_time = metadata
        .modified()
        .map(format_rfc3339)
        .unwrap_or_else(|_| String::new());
    Ok(if !asset.mod_time.is_empty()
        && metadata.len() == asset.size
        && mod_time == asset.mod_time
    {
        Availability::Available
    } else {
        Availability::Modified
    })
}

fn candidate_metadata(root_dir: &Path, relative_path: &str) -> Result<(u64, String), AssetError> {
    let target = resolve(root_dir, relative_path)?;
    let metadata = fs::metadata(target)?;
    if !metadata.is_file() {
        return Err(invalid("asset path must be a regular file"));
    }
    let mod_time = metadata
        .modified()
        .map(format_rfc3339)
        .unwrap_or_else(|_| String::new());
    Ok((metadata.len(), mod_time))
}

/// Finds matching files from caller-supplied candidate paths under the same root.
/// A known hash is authoritative; size + modification time is only a legacy
/// fallback when no hash was captured. Ambiguous matches are returned together.
pub fn find_candidates(
    root_dir: &Path,
    asset: &AssetRef,
    candidate_paths: &[&str],
) -> Result<Vec<AssetCandidate>, AssetError> {
    let mut matches = Vec::new();
    for candidate_path in candidate_paths {
        let relative_path = normalize_relative_path(candidate_path)?;
        let matched_by = if asset.sha256.is_empty() {
            let (size, mod_time) = candidate_metadata(root_dir, &relative_path)?;
            (!asset.mod_time.is_empty() && size == asset.size && mod_time == asset.mod_time)
                .then_some(AssetMatchMethod::Metadata)
        } else {
            let candidate = link(&asset.root_id, root_dir, None, Some(&relative_path))?;
            hashes_equal(&asset.sha256, &candidate.sha256).then_some(AssetMatchMethod::Sha256)
        };
        if let Some(matched_by) = matched_by {
            matches.push(AssetCandidate {
                relative_path,
                matched_by,
            });
        }
    }
    Ok(matches)
}

/// Revalidates a selected candidate, preserving the asset ID and returning both
/// the old and current references so callers can append history without loss.
pub fn rebind(
    root_dir: &Path,
    asset: &AssetRef,
    candidate: &AssetCandidate,
) -> Result<AssetRebinding, AssetError> {
    let mut updated = link(
        &asset.root_id,
        root_dir,
        None,
        Some(&candidate.relative_path),
    )?;
    let matched_by = match_method(asset, &updated)
        .ok_or_else(|| invalid("candidate no longer matches the stored asset evidence"))?;
    if matched_by != candidate.matched_by {
        return Err(invalid("candidate match method changed; search again"));
    }
    updated.id = asset.id.clone();
    Ok(AssetRebinding {
        previous: asset.clone(),
        updated,
        matched_by,
    })
}

fn match_method(asset: &AssetRef, candidate: &AssetRef) -> Option<AssetMatchMethod> {
    if !asset.sha256.is_empty() {
        hashes_equal(&asset.sha256, &candidate.sha256).then_some(AssetMatchMethod::Sha256)
    } else if !asset.mod_time.is_empty()
        && asset.size == candidate.size
        && asset.mod_time == candidate.mod_time
    {
        Some(AssetMatchMethod::Metadata)
    } else {
        None
    }
}

/// Initial 128-bit identity of a logical root plus relative path.
/// Rebinding retains the identity allocated by the original link.
pub fn stable_id(root_id: &str, relative_path: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(root_id.as_bytes());
    hasher.update(b"\0");
    hasher.update(relative_path.as_bytes());
    let digest = hasher.finalize();
    let mut id = String::from("asset-");
    for byte in digest.iter().take(16) {
        id.push_str(&format!("{byte:02x}"));
    }
    id
}

fn format_rfc3339(time: SystemTime) -> String {
    let seconds = match time.duration_since(UNIX_EPOCH) {
        Ok(duration) => duration.as_secs() as i64,
        Err(error) => -(error.duration().as_secs() as i64),
    };
    format_unix_seconds(seconds)
}

fn format_unix_seconds(seconds: i64) -> String {
    let days = seconds.div_euclid(86_400);
    let second_of_day = seconds.rem_euclid(86_400);
    let (year, month, day) = civil_from_days(days);
    format!(
        "{year:04}-{month:02}-{day:02}T{:02}:{:02}:{:02}Z",
        second_of_day / 3600,
        (second_of_day % 3600) / 60,
        second_of_day % 60
    )
}

// Howard Hinnant's civil_from_days; valid for the full i64 day range.
fn civil_from_days(days: i64) -> (i64, u32, u32) {
    let z = days + 719_468;
    let era = if z >= 0 { z } else { z - 146_096 } / 146_097;
    let day_of_era = z - era * 146_097;
    let year_of_era =
        (day_of_era - day_of_era / 1_460 + day_of_era / 36_524 - day_of_era / 146_096) / 365;
    let year = year_of_era + era * 400;
    let day_of_year = day_of_era - (365 * year_of_era + year_of_era / 4 - year_of_era / 100);
    let month_prime = (5 * day_of_year + 2) / 153;
    let day = (day_of_year - (153 * month_prime + 2) / 5 + 1) as u32;
    let month = if month_prime < 10 {
        month_prime + 3
    } else {
        month_prime - 9
    } as u32;
    (if month <= 2 { year + 1 } else { year }, month, day)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicU64, Ordering};

    static COUNTER: AtomicU64 = AtomicU64::new(0);

    struct TempTree(PathBuf);

    impl TempTree {
        fn new() -> Self {
            let unique = COUNTER.fetch_add(1, Ordering::Relaxed);
            let path = std::env::temp_dir().join(format!(
                "asterlab-assets-{}-{unique}",
                std::process::id()
            ));
            fs::create_dir_all(&path).expect("temp dir");
            Self(path)
        }
    }

    impl Drop for TempTree {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    #[test]
    fn classify_uses_the_extension_without_guessing() {
        assert_eq!(classify(Path::new("a/b/c.TIF")), MediaType::Image);
        assert_eq!(classify(Path::new("readout.csv")), MediaType::Table);
        assert_eq!(classify(Path::new("notes.md")), MediaType::Document);
        assert_eq!(classify(Path::new("raw.dat")), MediaType::Binary);
        assert_eq!(classify(Path::new("slots")), MediaType::Unknown);
        assert_eq!(classify(Path::new("archive.xyz")), MediaType::Unknown);
    }

    #[test]
    fn normalize_rejects_escapes_and_unnormalized_paths() {
        assert_eq!(
            normalize_relative_path("huvec\\d1\\image.tif").unwrap(),
            "huvec/d1/image.tif"
        );
        assert!(normalize_relative_path("../secret.txt").is_err());
        assert!(normalize_relative_path("d1/../../secret.txt").is_err());
        assert!(normalize_relative_path("..").is_err());
        assert!(normalize_relative_path("d1/..").is_err());
        assert!(normalize_relative_path("/abs/file").is_err());
        assert!(normalize_relative_path("./file").is_err());
        assert!(normalize_relative_path("\\\\server\\share\\file").is_err());
        assert!(normalize_relative_path("C:/Windows/win.ini").is_err());
        // A colon in the first component is a legal Unix file name but a
        // drive-relative prefix on Windows; both paths stay inside the root.
        #[cfg(not(windows))]
        assert_eq!(normalize_relative_path("a:b.csv").unwrap(), "a:b.csv");
        #[cfg(windows)]
        assert!(normalize_relative_path("a:b.csv").is_err());
        assert_eq!(normalize_relative_path("  ").unwrap(), "  ");
    }

    #[test]
    fn link_preserves_spaces_in_legal_file_names() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        fs::write(root.join("a.txt"), b"plain").unwrap();
        fs::write(root.join(" a.txt"), b"spaced-content").unwrap();

        let spaced = link("root", &root, None, Some(" a.txt")).unwrap();
        let plain = link("root", &root, None, Some("a.txt")).unwrap();
        assert_eq!(spaced.relative_path, " a.txt");
        assert_eq!(spaced.size, b"spaced-content".len() as u64);
        assert_ne!(
            spaced.sha256, plain.sha256,
            "the spaced name must not resolve to the unspaced file"
        );
    }

    #[test]
    fn link_records_observed_metadata_and_hash_without_copying() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(root.join("d1")).unwrap();
        let file = root.join("d1").join("image.tif");
        fs::write(&file, b"abc").unwrap();

        let asset = link("project-data", &root, None, Some("d1/image.tif")).unwrap();
        assert_eq!(asset.root_id, "project-data");
        assert_eq!(asset.relative_path, "d1/image.tif");
        assert_eq!(asset.media_type, MediaType::Image);
        assert_eq!(asset.size, 3);
        assert_eq!(
            asset.sha256,
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
        );
        assert_eq!(asset.availability, Availability::Available);
        assert_eq!(asset.id, stable_id("project-data", "d1/image.tif"));
        assert_eq!(asset.id.len(), "asset-".len() + 32);
        assert!(asset.mod_time.ends_with('Z'));
        assert_eq!(fs::read_dir(&root).unwrap().count(), 1, "no copy was created");
        assert_eq!(fs::read(&file).unwrap(), b"abc", "original is untouched");
    }

    #[test]
    fn link_derives_the_relative_path_from_an_absolute_file() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(root.join("d1")).unwrap();
        let file = root.join("d1").join("readout.csv");
        fs::write(&file, b"a,b\n1,2\n").unwrap();

        let asset = link(
            "root",
            &root,
            Some(file.to_str().unwrap()),
            Some("d1/readout.csv"),
        )
        .unwrap();
        assert_eq!(asset.relative_path, "d1/readout.csv");
        assert_eq!(asset.media_type, MediaType::Table);

        let mismatch = link(
            "root",
            &root,
            Some(file.to_str().unwrap()),
            Some("d1/other.csv"),
        )
        .unwrap_err();
        assert!(mismatch.0.contains("do not identify the same file"));
    }

    #[test]
    fn link_rejects_files_outside_the_root() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        let outside = tree.0.join("outside.txt");
        fs::create_dir_all(&root).unwrap();
        fs::write(&outside, b"secret").unwrap();

        let error = link("root", &root, Some(outside.to_str().unwrap()), None).unwrap_err();
        assert!(error.0.contains("outside its root"), "{error}");

        let escape = link("root", &root, None, Some("../outside.txt")).unwrap_err();
        assert!(escape.0.contains("escape"), "{error}");
    }

    #[test]
    fn link_reports_a_missing_file_instead_of_inventing_metadata() {
        let tree = TempTree::new();
        let error = link("root", &tree.0, None, Some("missing.tif")).unwrap_err();
        assert!(error.0.contains("unavailable"), "{error}");
    }

    #[cfg(unix)]
    #[test]
    fn link_rejects_non_regular_files_without_opening_them() {
        use std::os::unix::net::UnixListener;

        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        let socket = root.join("pipe.sock");
        let _listener = UnixListener::bind(&socket).unwrap();

        let error = link("root", &root, None, Some("pipe.sock")).unwrap_err();
        assert!(error.0.contains("regular file"), "{error}");
    }

    #[test]
    fn inspection_reports_deleted_and_modified_references_explicitly() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        let path = root.join("sample.bin");
        fs::write(&path, b"original").unwrap();
        let asset = link("root", &root, None, Some("sample.bin")).unwrap();

        fs::write(&path, b"replaced").unwrap();
        assert_eq!(
            inspect(&root, &asset).unwrap(),
            Availability::Modified,
            "same-size content replacement must be detected by hash"
        );

        fs::remove_file(&path).unwrap();
        assert_eq!(inspect(&root, &asset).unwrap(), Availability::Missing);
    }

    #[test]
    fn moved_asset_matches_by_hash_and_rebind_preserves_the_original_reference() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(root.join("before")).unwrap();
        let old_path = root.join("before/image.tif");
        fs::write(&old_path, b"image bytes").unwrap();
        let asset = link("root", &root, None, Some("before/image.tif")).unwrap();
        fs::create_dir_all(root.join("after")).unwrap();
        fs::rename(&old_path, root.join("after/image.tif")).unwrap();
        assert_eq!(inspect(&root, &asset).unwrap(), Availability::Missing);

        let matches = find_candidates(&root, &asset, &["after/image.tif"]).unwrap();
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].relative_path, "after/image.tif");
        assert_eq!(matches[0].matched_by, AssetMatchMethod::Sha256);

        let rebinding = rebind(&root, &asset, &matches[0]).unwrap();
        assert_eq!(rebinding.previous, asset);
        assert_eq!(rebinding.updated.id, asset.id);
        assert_eq!(rebinding.updated.relative_path, "after/image.tif");
        assert_eq!(rebinding.updated.availability, Availability::Available);
        assert_eq!(
            inspect(&root, &rebinding.updated).unwrap(),
            Availability::Available
        );
        assert_eq!(asset.relative_path, "before/image.tif");
    }

    #[test]
    fn rebinding_rejects_a_candidate_changed_after_matching() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        fs::write(root.join("original.txt"), b"same bytes").unwrap();
        let asset = link("root", &root, None, Some("original.txt")).unwrap();
        fs::copy(root.join("original.txt"), root.join("candidate.txt")).unwrap();
        let matches = find_candidates(&root, &asset, &["candidate.txt"]).unwrap();
        fs::write(root.join("candidate.txt"), b"changed bytes").unwrap();

        assert!(rebind(&root, &asset, &matches[0]).is_err());
    }

    #[test]
    fn metadata_fallback_requires_an_unavailable_hash() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        let old_path = root.join("old.csv");
        fs::write(&old_path, b"x,y\n1,2\n").unwrap();
        let linked = link("root", &root, None, Some("old.csv")).unwrap();
        let mut serialized = serde_json::to_value(linked).unwrap();
        serialized.as_object_mut().unwrap().remove("sha256");
        let legacy: AssetRef = serde_json::from_value(serialized).unwrap();
        assert_eq!(inspect(&root, &legacy).unwrap(), Availability::Available);
        fs::rename(&old_path, root.join("new.csv")).unwrap();

        let matches = find_candidates(&root, &legacy, &["new.csv"]).unwrap();
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].matched_by, AssetMatchMethod::Metadata);
        let rebinding = rebind(&root, &legacy, &matches[0]).unwrap();
        assert_eq!(rebinding.matched_by, AssetMatchMethod::Metadata);
        assert_eq!(rebinding.updated.relative_path, "new.csv");

        let mut known_hash = legacy.clone();
        known_hash.sha256 = "different-content-hash".into();
        assert!(find_candidates(&root, &known_hash, &["new.csv"])
            .unwrap()
            .is_empty());
    }

    #[test]
    fn ambiguous_candidates_are_returned_without_choosing_one() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        fs::write(root.join("first.txt"), b"same").unwrap();
        let asset = link("root", &root, None, Some("first.txt")).unwrap();
        fs::copy(root.join("first.txt"), root.join("second.txt")).unwrap();

        let matches = find_candidates(&root, &asset, &["first.txt", "second.txt"]).unwrap();
        assert_eq!(matches.len(), 2);
        assert!(matches
            .iter()
            .all(|candidate| candidate.matched_by == AssetMatchMethod::Sha256));
    }

    #[test]
    fn uppercase_stored_hash_still_matches_the_file() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        fs::write(root.join("sample.csv"), b"a,b\n1,2\n").unwrap();
        let linked = link("root", &root, None, Some("sample.csv")).unwrap();
        let mut serialized = serde_json::to_value(&linked).unwrap();
        serialized["sha256"] = serde_json::Value::String(
            serialized["sha256"].as_str().unwrap().to_ascii_uppercase(),
        );
        let uppercase: AssetRef = serde_json::from_value(serialized).unwrap();
        assert_eq!(
            inspect(&root, &uppercase).unwrap(),
            Availability::Available,
            "hex case must not change hash evidence"
        );

        fs::rename(root.join("sample.csv"), root.join("moved.csv")).unwrap();
        let matches = find_candidates(&root, &uppercase, &["moved.csv"]).unwrap();
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].matched_by, AssetMatchMethod::Sha256);
        let rebinding = rebind(&root, &uppercase, &matches[0]).unwrap();
        assert_eq!(rebinding.matched_by, AssetMatchMethod::Sha256);
        assert_eq!(rebinding.updated.sha256, linked.sha256);
    }

    #[test]
    fn rfc3339_formatting_is_utc_and_stable() {
        assert_eq!(format_unix_seconds(0), "1970-01-01T00:00:00Z");
        assert_eq!(format_unix_seconds(1_700_000_000), "2023-11-14T22:13:20Z");
    }

    #[test]
    fn root_id_must_be_logical_not_an_absolute_path() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(&root).unwrap();
        fs::write(root.join("a.txt"), b"a").unwrap();

        let rejected = vec![
            root.to_str().unwrap().to_owned(),
            "/var/data".to_owned(),
            "C:\\data".to_owned(),
            "C:/data".to_owned(),
            "C:".to_owned(),
            "\\\\server\\share".to_owned(),
            "roots/../escape".to_owned(),
        ];
        for root_id in rejected {
            let error = link(&root_id, &root, None, Some("a.txt")).unwrap_err();
            assert!(
                error.0.contains("logical identifier")
                    || error.0.contains("parent-directory"),
                "{root_id}: {error}"
            );
        }
        assert!(link("root", &root, None, Some("a.txt")).is_ok());
        assert!(link("root", &root, None, Some("/etc/passwd")).is_err());
    }

    #[test]
    fn exported_reference_contains_no_absolute_path_and_survives_a_restart() {
        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(root.join("d1")).unwrap();
        let file = root.join("d1").join("readout.csv");
        fs::write(&file, b"a,b\n1,2\n").unwrap();

        let asset = link("project-data", &root, None, Some("d1/readout.csv")).unwrap();
        let exported = serde_json::to_string_pretty(&asset).unwrap();
        let root_text = root.to_str().unwrap();
        let file_text = file.to_str().unwrap();
        assert!(!exported.contains(root_text), "{exported}");
        assert!(!exported.contains(file_text), "{exported}");
        assert!(!exported.contains(tree.0.to_str().unwrap()), "{exported}");
        assert!(exported.contains("\"relativePath\": \"d1/readout.csv\""));

        // The exported bytes are the whole durable reference: a fresh process
        // restores it without the local root directory.
        let restored: AssetRef = serde_json::from_str(&exported).unwrap();
        assert_eq!(restored, asset);
        assert_eq!(inspect(&root, &restored).unwrap(), Availability::Available);

        // After a restart a move is still detected and rebinding keeps identity.
        fs::rename(&file, root.join("d1/moved.csv")).unwrap();
        assert_eq!(inspect(&root, &restored).unwrap(), Availability::Missing);
        let candidates = find_candidates(&root, &restored, &["d1/moved.csv"]).unwrap();
        assert_eq!(candidates.len(), 1);
        let rebinding = rebind(&root, &restored, &candidates[0]).unwrap();
        assert_eq!(rebinding.updated.id, asset.id);
        let exported_rebinding = serde_json::to_string(&rebinding).unwrap();
        assert!(!exported_rebinding.contains(root_text));
        assert!(!exported_rebinding.contains(file_text));
    }

    #[cfg(unix)]
    #[test]
    fn symlinks_that_leave_the_root_are_rejected() {
        use std::os::unix::fs::symlink;

        let tree = TempTree::new();
        let root = tree.0.join("data");
        let outside = tree.0.join("outside");
        fs::create_dir_all(&root).unwrap();
        fs::create_dir_all(&outside).unwrap();
        fs::write(outside.join("secret.csv"), b"a,b\n1,2\n").unwrap();

        // A file symlink whose target is outside the root.
        symlink(outside.join("secret.csv"), root.join("leak.csv")).unwrap();
        let error = link("root", &root, None, Some("leak.csv")).unwrap_err();
        assert!(error.0.contains("outside its root"), "{error}");
        let error = resolve(&root, "leak.csv").unwrap_err();
        assert!(error.0.contains("outside its root"), "{error}");

        // A directory symlink whose target is outside the root.
        symlink(&outside, root.join("linked-dir")).unwrap();
        let error = link("root", &root, None, Some("linked-dir/secret.csv")).unwrap_err();
        assert!(error.0.contains("outside its root"), "{error}");

        // Candidate search cannot be used to reach outside the root either.
        fs::write(root.join("inside.csv"), b"a,b\n1,2\n").unwrap();
        let asset = link("root", &root, None, Some("inside.csv")).unwrap();
        assert!(find_candidates(&root, &asset, &["../outside/secret.csv"]).is_err());
        assert!(find_candidates(&root, &asset, &["linked-dir/secret.csv"]).is_err());

        // Replacing a valid reference with an escaping symlink fails visibly
        // instead of reporting the outside file as available.
        fs::remove_file(root.join("inside.csv")).unwrap();
        symlink(outside.join("secret.csv"), root.join("inside.csv")).unwrap();
        let error = inspect(&root, &asset).unwrap_err();
        assert!(error.0.contains("outside its root"), "{error}");
    }

    #[cfg(unix)]
    #[test]
    fn symlinks_that_stay_inside_the_root_are_allowed() {
        use std::os::unix::fs::symlink;

        let tree = TempTree::new();
        let root = tree.0.join("data");
        fs::create_dir_all(root.join("real")).unwrap();
        fs::write(root.join("real").join("readout.csv"), b"a,b\n1,2\n").unwrap();
        symlink(root.join("real/readout.csv"), root.join("readout.csv")).unwrap();

        let asset = link("root", &root, None, Some("readout.csv")).unwrap();
        assert_eq!(asset.relative_path, "readout.csv");
        assert_eq!(inspect(&root, &asset).unwrap(), Availability::Available);
        let exported = serde_json::to_string(&asset).unwrap();
        assert!(!exported.contains(root.to_str().unwrap()), "{exported}");
        assert!(!exported.contains("real/readout.csv"), "{exported}");
    }

    #[test]
    fn concurrent_links_and_inspections_agree_without_writing() {
        use std::sync::Arc;
        use std::thread;

        let tree = TempTree::new();
        let root = Arc::new(tree.0.join("data"));
        fs::create_dir_all(root.join("d1")).unwrap();
        fs::write(root.join("d1/readout.csv"), b"a,b\n1,2\n").unwrap();

        let expected = link("root", root.as_path(), None, Some("d1/readout.csv")).unwrap();
        let mut handles = Vec::new();
        for _ in 0..8 {
            let root = Arc::clone(&root);
            let expected = expected.clone();
            handles.push(thread::spawn(move || {
                let asset = link("root", root.as_path(), None, Some("d1/readout.csv")).unwrap();
                assert_eq!(asset, expected);
                assert_eq!(
                    inspect(root.as_path(), &asset).unwrap(),
                    Availability::Available
                );
            }));
        }
        for handle in handles {
            handle.join().unwrap();
        }
        assert_eq!(fs::read_dir(root.join("d1")).unwrap().count(), 1);
        assert_eq!(fs::read(root.join("d1/readout.csv")).unwrap(), b"a,b\n1,2\n");
    }
}
