use serde::{Deserialize, Serialize};
use std::{
    fs::{self, OpenOptions},
    io::{Read, Write},
    path::Path,
    sync::Mutex,
    time::Duration,
};
use tauri::{Manager, Url};
use tauri_plugin_opener::OpenerExt;
use uuid::Uuid;

const UPDATE_URL: &str = "https://api.ip21.cn/api/products/10/releases/latest";
const MAX_RESPONSE_BYTES: usize = 256 * 1024;

#[derive(Default)]
pub struct UpdateState {
    installation_lock: Mutex<()>,
    // Only open the destination returned by the most recent successful check.
    destination: Mutex<Option<String>>,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct UpdateResult {
    update_available: bool,
    force_update: bool,
    current_version: String,
    #[serde(default)]
    latest_version: String,
    #[serde(default)]
    title: String,
    #[serde(default)]
    release_notes: String,
    #[serde(default)]
    download_url: String,
    #[serde(default)]
    page_url: String,
}

#[derive(Deserialize)]
struct Envelope {
    status: u16,
    #[serde(default)]
    message: String,
    data: Option<UpdateResult>,
}

fn installation_id(directory: &Path) -> Result<String, String> {
    fs::create_dir_all(directory).map_err(|e| format!("无法保存更新标识: {e}"))?;
    let path = directory.join("installation-id");
    match OpenOptions::new().write(true).create_new(true).open(&path) {
        Ok(mut file) => {
            let id = Uuid::new_v4().to_string();
            file.write_all(id.as_bytes())
                .map_err(|e| format!("无法保存更新标识: {e}"))?;
            Ok(id)
        }
        Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => {
            let mut value = String::new();
            fs::File::open(path)
                .and_then(|file| file.take(128).read_to_string(&mut value))
                .map_err(|e| format!("无法读取更新标识: {e}"))?;
            Uuid::parse_str(value.trim())
                .map(|id| id.to_string())
                .map_err(|_| "更新标识无效".to_string())
        }
        Err(e) => Err(format!("无法保存更新标识: {e}")),
    }
}

fn update_url(version: &str, platform: &str, arch: &str, id: &str) -> Url {
    let mut url = Url::parse(UPDATE_URL).expect("valid product update URL");
    url.query_pairs_mut().extend_pairs([
        ("current_version", version),
        ("channel", "stable"),
        ("platform", platform),
        ("arch", arch),
        ("installation_id", id),
    ]);
    url
}

fn destination(result: &UpdateResult) -> Result<Option<String>, String> {
    if !result.update_available {
        return Ok(None);
    }
    let current =
        semver::Version::parse(&result.current_version).map_err(|_| "更新响应无效".to_string())?;
    let latest =
        semver::Version::parse(result.latest_version.trim().trim_start_matches(['v', 'V']))
            .map_err(|_| "更新响应无效".to_string())?;
    if !latest.cmp_precedence(&current).is_gt() {
        return Err("更新响应无效".into());
    }
    // Prefer the installation package, with the release page as a fallback.
    for value in [&result.download_url, &result.page_url] {
        if let Ok(url) = Url::parse(value) {
            if url.scheme() == "https"
                && url.host_str().is_some()
                && url.username().is_empty()
                && url.password().is_none()
            {
                return Ok(Some(url.to_string()));
            }
        }
    }
    Err("更新下载地址无效".into())
}

fn parse_response(body: &[u8], version: &str) -> Result<UpdateResult, String> {
    let envelope: Envelope =
        serde_json::from_slice(body).map_err(|_| "更新响应无效".to_string())?;
    if envelope.status != 200 {
        return Err(format!("检查更新失败: {}", envelope.message));
    }
    let result = envelope.data.ok_or("更新响应无效")?;
    if result.current_version != version || (result.force_update && !result.update_available) {
        return Err("更新响应无效".into());
    }
    destination(&result)?;
    Ok(result)
}

async fn fetch_update(url: Url, version: &str) -> Result<UpdateResult, String> {
    let client = reqwest::Client::builder()
        .connect_timeout(Duration::from_secs(5))
        .timeout(Duration::from_secs(15))
        .redirect(reqwest::redirect::Policy::none())
        .user_agent(format!("Fangxu-File-Transfer/{version}"))
        .build()
        .map_err(|e| format!("检查更新失败: {e}"))?;
    let mut response = client
        .get(url)
        .header(reqwest::header::ACCEPT, "application/json")
        .send()
        .await
        .map_err(|e| format!("检查更新失败: {e}"))?;
    if !response.status().is_success() {
        return Err(format!("检查更新失败: HTTP {}", response.status()));
    }
    let mut body = Vec::new();
    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(|e| format!("检查更新失败: {e}"))?
    {
        if body.len() + chunk.len() > MAX_RESPONSE_BYTES {
            return Err("更新响应无效".into());
        }
        body.extend_from_slice(&chunk);
    }
    parse_response(&body, version)
}

#[tauri::command]
pub async fn check_product_update(
    app: tauri::AppHandle,
    state: tauri::State<'_, UpdateState>,
) -> Result<UpdateResult, String> {
    *state.destination.lock().map_err(|e| e.to_string())? = None;
    let directory = app.path().app_config_dir().map_err(|e| e.to_string())?;
    let app_for_id = app.clone();
    let id = tauri::async_runtime::spawn_blocking(move || {
        let state = app_for_id.state::<UpdateState>();
        let _lock = state.installation_lock.lock().map_err(|e| e.to_string())?;
        installation_id(&directory)
    })
    .await
    .map_err(|e| e.to_string())??;
    let version = app.package_info().version.to_string();
    let url = update_url(&version, std::env::consts::OS, std::env::consts::ARCH, &id);
    let result = fetch_update(url, &version).await?;
    *state.destination.lock().map_err(|e| e.to_string())? = destination(&result)?;
    Ok(result)
}

#[tauri::command]
pub async fn open_product_update(
    app: tauri::AppHandle,
    state: tauri::State<'_, UpdateState>,
) -> Result<(), String> {
    let url = state
        .destination
        .lock()
        .map_err(|e| e.to_string())?
        .clone()
        .ok_or("请先检查更新")?;
    app.opener()
        .open_url(url, None::<&str>)
        .map_err(|e| format!("无法打开更新下载地址: {e}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    use std::collections::HashMap;

    fn response(update: bool) -> serde_json::Value {
        json!({"status": 200, "data": {
            "update_available": update, "force_update": false,
            "current_version": "1.1.0", "latest_version": "v1.2.0",
            "download_url": "https://example.com/app.dmg", "page_url": "https://example.com/release"
        }})
    }

    #[test]
    fn request_uses_product_and_native_target_with_encoded_version() {
        for (platform, arch) in [
            ("macos", "aarch64"),
            ("windows", "x86_64"),
            ("linux", "aarch64"),
        ] {
            let url = update_url("1.1.0+build.1", platform, arch, "test-id");
            assert_eq!(url.path(), "/api/products/10/releases/latest");
            let params: HashMap<_, _> = url.query_pairs().into_owned().collect();
            assert_eq!(params["current_version"], "1.1.0+build.1");
            assert_eq!(params["platform"], platform);
            assert_eq!(params["arch"], arch);
            assert_eq!(params["channel"], "stable");
            assert_eq!(params["installation_id"], "test-id");
        }
    }

    #[test]
    fn installation_identifier_survives_restarts() {
        let directory =
            std::env::temp_dir().join(format!("file-transfer-update-{}", Uuid::new_v4()));
        let first = installation_id(&directory).unwrap();
        assert!(Uuid::parse_str(&first).is_ok());
        assert_eq!(installation_id(&directory).unwrap(), first);
        fs::remove_dir_all(directory).unwrap();
    }

    #[test]
    fn handles_available_forced_and_up_to_date_responses() {
        for available in [false, true] {
            let body = serde_json::to_vec(&response(available)).unwrap();
            let result = parse_response(&body, "1.1.0").unwrap();
            assert_eq!(result.update_available, available);
            assert_eq!(destination(&result).unwrap().is_some(), available);
        }
        let mut forced = response(true);
        forced["data"]["force_update"] = json!(true);
        assert!(
            parse_response(&serde_json::to_vec(&forced).unwrap(), "1.1.0")
                .unwrap()
                .force_update
        );
        let minimal = br#"{"status":200,"data":{"update_available":false,"force_update":false,"current_version":"1.1.0"}}"#;
        assert!(!parse_response(minimal, "1.1.0").unwrap().update_available);
    }

    #[test]
    fn rejects_errors_invalid_versions_and_unsafe_destinations() {
        for body in [
            b"<html>error</html>".as_slice(),
            br#"{"status":200}"#,
            br#"{"status":400,"message":"bad request"}"#,
        ] {
            assert!(parse_response(body, "1.1.0").is_err());
        }
        for (key, value) in [
            ("latest_version", "1.0.0"),
            ("latest_version", "invalid"),
            ("latest_version", "1.1.0+newbuild"),
            ("current_version", "9.0.0"),
        ] {
            let mut bad = response(true);
            bad["data"][key] = json!(value);
            assert!(parse_response(&serde_json::to_vec(&bad).unwrap(), "1.1.0").is_err());
        }
        let mut result =
            parse_response(&serde_json::to_vec(&response(true)).unwrap(), "1.1.0").unwrap();
        for unsafe_url in [
            "javascript:alert(1)",
            "file:///tmp/app",
            "http://example.com/app",
            "https://user:password@example.com/app",
        ] {
            result.download_url = unsafe_url.into();
            assert_eq!(
                destination(&result).unwrap().unwrap(),
                "https://example.com/release"
            );
            let page = std::mem::take(&mut result.page_url);
            assert!(destination(&result).is_err());
            result.page_url = page;
        }
    }

    #[test]
    fn http_client_handles_success_http_errors_and_malformed_bodies() {
        use std::{net::TcpListener, thread};

        for (status, body, success) in [
            (
                "200 OK",
                serde_json::to_string(&response(true)).unwrap(),
                true,
            ),
            ("503 Service Unavailable", "maintenance".into(), false),
            ("200 OK", "<html>not JSON</html>".into(), false),
            ("200 OK", "x".repeat(MAX_RESPONSE_BYTES + 1), false),
        ] {
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let url =
                Url::parse(&format!("http://{}/latest", listener.local_addr().unwrap())).unwrap();
            let server = thread::spawn(move || {
                let (mut socket, _) = listener.accept().unwrap();
                socket
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut request = [0; 4096];
                let read = socket.read(&mut request).unwrap();
                assert!(String::from_utf8_lossy(&request[..read]).contains("GET /latest HTTP/1.1"));
                let _ = write!(
                    socket,
                    "HTTP/1.1 {status}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
            });
            let result = tauri::async_runtime::block_on(fetch_update(url, "1.1.0"));
            server.join().unwrap();
            assert_eq!(
                result.is_ok(),
                success,
                "unexpected HTTP check result: {result:?}"
            );
        }
    }
}
