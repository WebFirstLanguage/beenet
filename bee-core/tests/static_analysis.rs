//! Static analysis tests to ensure code compliance
//! Run with: cargo test --test static_analysis

use std::fs;
use std::path::{Path, PathBuf};

/// List of banned strings that indicate mDNS usage
const BANNED_MDNS_PATTERNS: &[&str] = &[
    "224.0.0.251",
    "5353",
    "_services._dns-sd._udp",
    "mdns",
    "mDNS",
    "MDNS",
    "multicast_dns",
    "MulticastDNS",
    ".local",
    "avahi",
    "bonjour",
    "zeroconf",
];

/// List of file extensions to check
// const CHECK_EXTENSIONS: &[&str] = &["rs", "toml", "md", "txt", "yaml", "yml", "json"];

#[test]
fn no_mdns_calls_present() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR")).parent().unwrap();
    let mut violations = Vec::new();

    check_directory(workspace_root, &mut violations);

    if !violations.is_empty() {
        eprintln!("Found mDNS-related patterns in the following locations:");
        for (file, line_num, line, pattern) in &violations {
            eprintln!(
                "  {}:{} - Found '{}' in: {}",
                file.display(),
                line_num,
                pattern,
                line.trim()
            );
        }
        panic!(
            "Static analysis failed: Found {} mDNS-related patterns. \
             Beenet must use HELLO beacons, not mDNS!",
            violations.len()
        );
    }
}

fn check_directory(dir: &Path, violations: &mut Vec<(PathBuf, usize, String, String)>) {
    let entries = fs::read_dir(dir).expect("Failed to read directory");

    for entry in entries {
        let entry = entry.expect("Failed to read directory entry");
        let path = entry.path();

        // Skip hidden directories and target directory
        if let Some(name) = path.file_name() {
            let name_str = name.to_string_lossy();
            if name_str.starts_with('.') || name_str == "target" {
                continue;
            }
        }

        if path.is_dir() {
            check_directory(&path, violations);
        } else if should_check_file(&path) {
            check_file(&path, violations);
        }
    }
}

fn should_check_file(path: &Path) -> bool {
    // Skip this test file itself
    if path.file_name() == Some(std::ffi::OsStr::new("static_analysis.rs")) {
        return false;
    }

    // Skip documentation files - they legitimately mention mDNS to explain we don't use it
    if let Some(parent) = path.parent() {
        if parent.file_name() == Some(std::ffi::OsStr::new("Docs")) {
            return false;
        }
    }

    // Skip CLAUDE.md - it's documentation about not using mDNS
    if path.file_name() == Some(std::ffi::OsStr::new("CLAUDE.md")) {
        return false;
    }

    if let Some(ext) = path.extension() {
        // Only check source code files
        ext == "rs" || ext == "toml"
    } else {
        false
    }
}

fn check_file(path: &Path, violations: &mut Vec<(PathBuf, usize, String, String)>) {
    let content = match fs::read_to_string(path) {
        Ok(content) => content,
        Err(_) => return, // Skip files we can't read
    };

    for (line_num, line) in content.lines().enumerate() {
        // Skip comments and doc comments
        let trimmed = line.trim();
        if trimmed.starts_with("//") || trimmed.starts_with("///") || trimmed.starts_with("#") {
            continue;
        }

        for pattern in BANNED_MDNS_PATTERNS {
            if line.to_lowercase().contains(&pattern.to_lowercase()) {
                if *pattern == ".local" && (line.contains("local_addr()") || line.contains(".local_addr()")) {
                    continue;
                }
                
                violations.push((
                    path.to_path_buf(),
                    line_num + 1,
                    line.to_string(),
                    pattern.to_string(),
                ));
            }
        }
    }
}

#[test]
fn no_unsafe_in_core_crates() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR")).parent().unwrap();
    let core_crates = [
        "bee-core",
        "bee-sim",
        "bee-dht",
        "bee-route",
        "bee-transport",
    ];
    let mut unsafe_uses = Vec::new();

    for crate_name in &core_crates {
        let crate_path = workspace_root.join(crate_name).join("src");
        if crate_path.exists() {
            check_unsafe_in_directory(&crate_path, &mut unsafe_uses);
        }
    }

    if !unsafe_uses.is_empty() {
        eprintln!("Found 'unsafe' keyword in core crates:");
        for (file, line_num, line) in &unsafe_uses {
            eprintln!("  {}:{} - {}", file.display(), line_num, line.trim());
        }
        panic!(
            "Static analysis failed: Found {} uses of 'unsafe' in core crates. \
             Core crates must not use unsafe code!",
            unsafe_uses.len()
        );
    }
}

fn check_unsafe_in_directory(dir: &Path, unsafe_uses: &mut Vec<(PathBuf, usize, String)>) {
    let entries = fs::read_dir(dir).expect("Failed to read directory");

    for entry in entries {
        let entry = entry.expect("Failed to read directory entry");
        let path = entry.path();

        if path.is_dir() {
            check_unsafe_in_directory(&path, unsafe_uses);
        } else if path.extension() == Some(std::ffi::OsStr::new("rs")) {
            check_unsafe_in_file(&path, unsafe_uses);
        }
    }
}

fn check_unsafe_in_file(path: &Path, unsafe_uses: &mut Vec<(PathBuf, usize, String)>) {
    let content = match fs::read_to_string(path) {
        Ok(content) => content,
        Err(_) => return,
    };

    for (line_num, line) in content.lines().enumerate() {
        if line.contains("unsafe") && !line.trim().starts_with("//") {
            unsafe_uses.push((path.to_path_buf(), line_num + 1, line.to_string()));
        }
    }
}
