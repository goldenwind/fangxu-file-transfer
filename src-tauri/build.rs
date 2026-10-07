use std::{env, path::PathBuf, process::Command};

fn main() {
    let target = env::var("TARGET").expect("Cargo TARGET");
    let goos = if target.contains("apple-darwin") {
        "darwin"
    } else if target.contains("windows") {
        "windows"
    } else if target.contains("linux") {
        "linux"
    } else {
        panic!("unsupported desktop target: {target}")
    };
    let goarch = if target.starts_with("aarch64") {
        "arm64"
    } else if target.starts_with("x86_64") {
        "amd64"
    } else {
        panic!("unsupported desktop architecture: {target}")
    };
    let manifest = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let extension = if goos == "windows" { ".exe" } else { "" };
    let filename = format!("fangxu-transfer-service-{target}{extension}");
    let binaries = manifest.join("binaries");
    std::fs::create_dir_all(&binaries).expect("create sidecar directory");
    let binary = binaries.join(filename);
    for source in [
        "main.go",
        "desktop.go",
        "go.mod",
        "go.sum",
        "assets/fangxu-file-transfer-logo.png",
    ] {
        println!("cargo:rerun-if-changed=../{source}");
    }
    let status = Command::new("go")
        .current_dir(manifest.parent().unwrap())
        .env("CGO_ENABLED", "0")
        .env("GOOS", goos)
        .env("GOARCH", goarch)
        .args(["build", "-trimpath", "-ldflags=-s -w", "-o"])
        .arg(&binary)
        .arg(".")
        .status()
        .expect("Go 1.22+ is required to build the bundled transfer service");
    assert!(status.success(), "failed to build Go transfer service");
    println!("cargo:rustc-env=TRANSFER_DEV_BINARY={}", binary.display());
    tauri_build::build();
}
