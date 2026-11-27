use serde::{Deserialize, Serialize};
use std::{
    collections::BTreeMap,
    io::{BufReader, Write},
    path::PathBuf,
    process::{ChildStdin, Stdio},
    sync::{Arc, Mutex},
};
use tauri::{ipc::Channel, Manager, State, WebviewUrl, WebviewWindowBuilder};
use tauri_plugin_opener::OpenerExt;
use url::Url;

const WECHAT_URL: &str = "https://filehelper.weixin.qq.com/";
const WECHAT_OPEN_ERROR: &str = "无法打开微信网页传输";

fn is_wechat_navigation(url: &Url) -> bool {
    url.scheme() == "https"
        && url.host_str() == Some("filehelper.weixin.qq.com")
        && url.port().is_none()
}

const FILE_ERROR: &str = "文件无法读取或已修改，请重新选择";
const SELECTION_ERROR: &str = "无法读取所选文件";
const CONNECTION_ERROR: &str = "无法连接中转服务，请重试";
const RESPONSE_ERROR: &str = "中转服务返回了无效响应";
const SIDECAR_ERROR: &str = "无法启动传输组件";
const UPLOAD_CANCELED: &str = "上传已取消";
const UPLOAD_BUSY: &str = "请等当前上传结束";

#[derive(Default)]
pub(super) struct RelayState(Arc<Mutex<RelayData>>);

#[derive(Default)]
struct RelayData {
    files: Vec<SelectedFile>,
    active: Option<ActiveUpload>,
}

struct SelectedFile {
    path: PathBuf,
    identity: PathBuf,
    info: FileInfo,
}

#[derive(Clone, Serialize)]
pub(super) struct FileInfo {
    id: String,
    name: String,
    size: u64,
    links: BTreeMap<String, String>,
}

struct ActiveUpload {
    id: String,
    file_id: String,
    canceled: bool,
    input: Option<ChildStdin>,
}

#[tauri::command]
pub(super) async fn choose_relay_files(
    state: State<'_, RelayState>,
    language: String,
) -> Result<Option<Vec<FileInfo>>, String> {
    let Some(paths) = rfd::AsyncFileDialog::new()
        .set_title(if language == "en" {
            "Choose files"
        } else {
            "选择文件"
        })
        .pick_files()
        .await
    else {
        return Ok(None);
    };
    let mut selected = Vec::with_capacity(paths.len());
    for file in paths {
        let path = file.path();
        let metadata = path.metadata().map_err(|_| FILE_ERROR)?;
        if !metadata.is_file() || path.to_str().is_none() {
            return Err(FILE_ERROR.to_owned());
        }
        selected.push(SelectedFile {
            path: path.to_owned(),
            identity: path.canonicalize().map_err(|_| FILE_ERROR)?,
            info: FileInfo {
                id: uuid::Uuid::new_v4().to_string(),
                name: file.file_name(),
                size: metadata.len(),
                links: BTreeMap::new(),
            },
        });
    }

    let mut data = state.0.lock().map_err(|_| SELECTION_ERROR)?;
    for file in selected {
        if let Some(existing) = data
            .files
            .iter_mut()
            .find(|item| item.identity == file.identity)
        {
            existing.path = file.path;
            existing.info.name = file.info.name;
            existing.info.size = file.info.size;
        } else {
            data.files.push(file);
        }
    }
    Ok(Some(
        data.files.iter().map(|file| file.info.clone()).collect(),
    ))
}

#[tauri::command]
pub(super) fn remove_relay_file(
    state: State<'_, RelayState>,
    file_id: String,
) -> Result<(), String> {
    let mut data = state.0.lock().map_err(|_| SELECTION_ERROR)?;
    if data
        .active
        .as_ref()
        .is_some_and(|upload| upload.file_id == file_id)
    {
        return Err(UPLOAD_BUSY.to_owned());
    }
    data.files.retain(|file| file.info.id != file_id);
    Ok(())
}

#[derive(Deserialize, Serialize)]
struct UploadProgress {
    uploaded: u64,
    speed: f64,
}

#[derive(Default, Deserialize, Serialize)]
pub(super) struct UploadResponse {
    progress: Option<UploadProgress>,
    url: Option<String>,
    error: Option<String>,
}

#[tauri::command]
pub(super) fn start_relay_upload(
    state: State<'_, RelayState>,
    provider: String,
    file_id: String,
    events: Channel<UploadResponse>,
) -> Result<String, String> {
    if !matches!(
        provider.as_str(),
        "kiwi" | "gofile" | "tempfile" | "tmp0" | "filebin"
    ) {
        return Err("中转服务拒绝了上传".to_owned());
    }
    let mut data = state.0.lock().map_err(|_| SELECTION_ERROR)?;
    let file = data
        .files
        .iter()
        .find(|file| file.info.id == file_id)
        .ok_or(FILE_ERROR)?;
    let id = uuid::Uuid::new_v4().to_string();
    if let Some(link) = file.info.links.get(&provider) {
        let _ = events.send(UploadResponse {
            url: Some(link.clone()),
            ..Default::default()
        });
        return Ok(id);
    }
    if data.active.is_some() {
        return Err(UPLOAD_BUSY.to_owned());
    }
    let (path, size) = (file.path.clone(), file.info.size);
    data.active = Some(ActiveUpload {
        id: id.clone(),
        file_id: file_id.clone(),
        canceled: false,
        input: None,
    });
    drop(data);

    let state = Arc::clone(&state.0);
    tauri::async_runtime::spawn_blocking(move || {
        let result = upload(&provider, path, size, &events, &state);
        let response = match state.lock() {
            Ok(mut data) => {
                let canceled = data.active.take().is_some_and(|upload| upload.canceled);
                match result {
                    Ok(url) => {
                        if let Some(file) =
                            data.files.iter_mut().find(|file| file.info.id == file_id)
                        {
                            file.info.links.insert(provider, url.clone());
                        }
                        UploadResponse {
                            url: Some(url),
                            ..Default::default()
                        }
                    }
                    Err(error) => UploadResponse {
                        error: Some(if canceled {
                            UPLOAD_CANCELED.to_owned()
                        } else {
                            error
                        }),
                        ..Default::default()
                    },
                }
            }
            Err(_) => UploadResponse {
                error: Some(CONNECTION_ERROR.to_owned()),
                ..Default::default()
            },
        };
        let _ = events.send(response);
    });
    Ok(id)
}

#[tauri::command]
pub(super) fn cancel_relay_upload(
    state: State<'_, RelayState>,
    upload_id: String,
) -> Result<(), String> {
    let mut data = state.0.lock().map_err(|_| CONNECTION_ERROR)?;
    if let Some(upload) = data.active.as_mut().filter(|upload| upload.id == upload_id) {
        upload.canceled = true;
        // EOF cancels the sidecar's HTTP context, including requests awaiting a response.
        upload.input.take();
    }
    Ok(())
}

fn upload(
    provider: &str,
    path: PathBuf,
    size: u64,
    events: &Channel<UploadResponse>,
    state: &Mutex<RelayData>,
) -> Result<String, String> {
    if state
        .lock()
        .map_err(|_| CONNECTION_ERROR)?
        .active
        .as_ref()
        .is_some_and(|upload| upload.canceled)
    {
        return Err(UPLOAD_CANCELED.to_owned());
    }
    let mut child = super::sidecar_command()?
        .arg("--relay-upload")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .map_err(|_| SIDECAR_ERROR)?;
    let response = (|| {
        let mut input = child.stdin.take().ok_or(SIDECAR_ERROR)?;
        let request = serde_json::json!({ "provider": provider, "path": path, "size": size });
        {
            let mut data = state.lock().map_err(|_| CONNECTION_ERROR)?;
            let upload = data.active.as_mut().ok_or(CONNECTION_ERROR)?;
            if upload.canceled {
                return Err(UPLOAD_CANCELED);
            }
            writeln!(input, "{request}").map_err(|_| SIDECAR_ERROR)?;
            upload.input = Some(input);
        }
        let output = child.stdout.take().ok_or(SIDECAR_ERROR)?;
        for message in serde_json::Deserializer::from_reader(BufReader::new(output))
            .into_iter::<UploadResponse>()
        {
            let message = message.map_err(|_| RESPONSE_ERROR)?;
            if message.progress.is_some() {
                let _ = events.send(message);
            } else {
                return Ok(message);
            }
        }
        Err(RESPONSE_ERROR)
    })();
    if let Ok(mut data) = state.lock() {
        if let Some(upload) = data.active.as_mut() {
            upload.input.take();
        }
    }
    if response.is_err() {
        let _ = child.kill();
    }
    let _ = child.wait();
    let response = response?;
    response
        .url
        .ok_or_else(|| response.error.unwrap_or_else(|| RESPONSE_ERROR.to_owned()))
}

#[tauri::command]
pub(super) fn open_relay_link(app: tauri::AppHandle, link: String) -> Result<(), String> {
    let url = Url::parse(&link).map_err(|_| "中转服务返回了无效响应")?;
    if url.scheme() != "https"
        || url.port().is_some()
        || !url.username().is_empty()
        || url.password().is_some()
        || !matches!(
            url.host_str(),
            Some("file.kiwi" | "gofile.io" | "tempfile.org" | "tmp0.cc" | "filebin.net")
        )
        || url.path().len() < 2
    {
        return Err("中转服务返回了无效响应".to_owned());
    }
    app.opener()
        .open_url(link, None::<&str>)
        .map_err(|_| "无法打开取件链接".to_owned())
}

#[tauri::command]
pub(super) async fn open_wechat_transfer(
    app: tauri::AppHandle,
    language: String,
) -> Result<(), String> {
    const LABEL: &str = "wechat-transfer";
    let error = WECHAT_OPEN_ERROR;
    if let Some(window) = app.get_webview_window(LABEL) {
        window.unminimize().map_err(|_| error)?;
        window.show().map_err(|_| error)?;
        return window.set_focus().map_err(|_| error.to_owned());
    }
    let url = Url::parse(WECHAT_URL).map_err(|_| error)?;
    WebviewWindowBuilder::new(&app, LABEL, WebviewUrl::External(url))
        .title(if language == "en" {
            "WeChat File Transfer"
        } else {
            "微信网页传输"
        })
        .inner_size(960.0, 700.0)
        .min_inner_size(520.0, 480.0)
        .center()
        .disable_drag_drop_handler()
        .on_navigation(is_wechat_navigation)
        .build()
        .map_err(|_| error)?;
    Ok(())
}

#[cfg(target_os = "macos")]
#[tauri::command]
pub(super) async fn set_wechat_view(
    webview: tauri::Webview,
    bounds: Option<super::WebviewBounds>,
) -> Result<(), String> {
    use tauri::{
        webview::{NewWindowResponse, WebviewBuilder},
        Rect,
    };

    const LABEL: &str = "wechat-pane";
    let error = || WECHAT_OPEN_ERROR.to_owned();
    let window = webview.window();
    if webview.label() != "main" || window.label() != "main" {
        return Err(error());
    }
    let pane = webview.app_handle().get_webview(LABEL);
    let Some(bounds) = bounds else {
        if let Some(pane) = pane {
            pane.hide().map_err(|_| error())?;
        }
        return Ok(());
    };
    let (position, size) = bounds.logical_parts();
    if let Some(pane) = pane {
        pane.set_bounds(Rect {
            position: position.into(),
            size: size.into(),
        })
        .map_err(|_| error())?;
        return pane.show().map_err(|_| error());
    }
    let url = Url::parse(WECHAT_URL).map_err(|_| error())?;
    let builder = WebviewBuilder::new(LABEL, WebviewUrl::External(url))
        .focused(false)
        .disable_drag_drop_handler()
        .on_navigation(is_wechat_navigation)
        .on_new_window(|_, _| NewWindowResponse::Deny);
    window
        .add_child(builder, position, size)
        .map_err(|_| error())?;
    Ok(())
}
