use serde::{Deserialize, Serialize};
use std::time::Duration;

const BASE_URL: &str = "https://easychuan.cn";
const MAX_FILES: usize = 10;
const MAX_TEXT_LENGTH: usize = 50_000;

#[derive(Deserialize, Serialize)]
pub struct FileDetails {
    filename: String,
    filesize: u64,
}

#[derive(Deserialize, Serialize)]
pub struct RegisteredFile {
    file_index: usize,
    filekey: String,
}

#[derive(Deserialize, Serialize)]
pub struct Registration {
    fetch_code: String,
    files: Vec<RegisteredFile>,
}

#[derive(Deserialize, Serialize)]
pub struct TextRegistration {
    fetch_code: String,
    date_expire: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum MediaKind {
    Screen,
    Video,
}

#[derive(Deserialize)]
pub struct MediaDetails {
    stream_id: String,
    has_audio: bool,
}

#[derive(Deserialize)]
struct RegisteredStream {
    #[serde(alias = "screen_index", alias = "video_index")]
    index: usize,
    #[serde(alias = "screenkey", alias = "videokey")]
    key: String,
}

#[derive(Deserialize)]
struct MediaResponse {
    fetch_code: String,
    #[serde(alias = "screens", alias = "videos")]
    streams: Vec<RegisteredStream>,
}

#[derive(Serialize)]
pub struct MediaRegistration {
    fetch_code: String,
    stream_key: String,
}

#[tauri::command]
pub async fn register_easychuan_files(files: Vec<FileDetails>) -> Result<Registration, String> {
    if files.is_empty()
        || files.len() > MAX_FILES
        || files.iter().any(|file| {
            file.filename.is_empty()
                || file.filename.contains(['/', '\\', '\0'])
                || file.filesize > 65_528 * u64::from(u32::MAX)
        })
    {
        return Err("轻松传文件列表无效".into());
    }

    post("/files/direct", serde_json::json!({ "files": files })).await
}

#[tauri::command]
pub async fn register_easychuan_text(content: String) -> Result<TextRegistration, String> {
    if content.is_empty() || content.encode_utf16().count() > MAX_TEXT_LENGTH {
        return Err("文本长度必须在 1 到 50,000 个字符之间".into());
    }
    post("/texts", serde_json::json!({ "content": content })).await
}

#[tauri::command]
pub async fn register_easychuan_media(
    kind: MediaKind,
    stream: MediaDetails,
) -> Result<MediaRegistration, String> {
    if stream.stream_id.is_empty() || stream.stream_id.len() > 128 {
        return Err("轻松传媒体信息无效".into());
    }
    let video_label = match kind {
        MediaKind::Screen => "Screen",
        MediaKind::Video => "Camera",
    };
    let details = serde_json::json!({
        "stream_id": stream.stream_id,
        "video_track_label": video_label,
        "audio_track_label": stream.has_audio.then_some("Audio"),
    });
    let (endpoint, request) = match kind {
        MediaKind::Screen => (
            "/screens",
            serde_json::json!({ "screens": [details], "access_key": "" }),
        ),
        MediaKind::Video => (
            "/videos",
            serde_json::json!({ "videos": [details], "access_key": "" }),
        ),
    };
    let response: MediaResponse = post(endpoint, request).await?;
    let stream_key = response
        .streams
        .into_iter()
        .find(|stream| stream.index == 0 && !stream.key.is_empty())
        .ok_or("轻松传返回了无效的取件信息")?
        .key;
    Ok(MediaRegistration {
        fetch_code: response.fetch_code,
        stream_key,
    })
}

async fn post<T: serde::de::DeserializeOwned>(
    endpoint: &str,
    request: serde_json::Value,
) -> Result<T, String> {
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(12))
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(|_| "无法连接轻松传")?;
    let response = client
        .post(format!("{BASE_URL}{endpoint}"))
        .header(reqwest::header::CONTENT_TYPE, "application/json")
        .body(request.to_string())
        .send()
        .await
        .map_err(|_| "无法连接轻松传")?;
    if !response.status().is_success() {
        return Err("轻松传未能创建取件码，请稍后重试".into());
    }

    let data = response.bytes().await.map_err(|_| "无法连接轻松传")?;
    serde_json::from_slice(&data).map_err(|_| "轻松传返回了无效的取件信息".into())
}
