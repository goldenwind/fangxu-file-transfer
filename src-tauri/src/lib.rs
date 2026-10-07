use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    io::{BufRead, BufReader, Write},
    path::PathBuf,
    process::{Child, ChildStdin, Command, Stdio},
    sync::{mpsc, Mutex},
    time::Duration,
};
use tauri::Manager;
use tauri_plugin_opener::OpenerExt;

#[derive(Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct Settings {
    directory: String,
    port: u16,
    protected: bool,
    auto_start: bool,
}

struct Backend {
    child: Child,
    input: Option<ChildStdin>,
    responses: mpsc::Receiver<Result<Value, String>>,
}

impl Backend {
    fn spawn(app: &tauri::AppHandle) -> Result<Self, String> {
        let config = app
            .path()
            .app_config_dir()
            .map_err(|e| e.to_string())?
            .join("settings.json");
        let binary = if cfg!(debug_assertions) {
            PathBuf::from(env!("TRANSFER_DEV_BINARY"))
        } else {
            let executable = std::env::current_exe().map_err(|e| e.to_string())?;
            executable
                .parent()
                .ok_or("找不到客户端目录")?
                .join(if cfg!(windows) {
                    "fangxu-transfer-service.exe"
                } else {
                    "fangxu-transfer-service"
                })
        };
        let mut command = Command::new(binary);
        command
            .arg("--desktop")
            .arg("--config")
            .arg(config)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit());
        #[cfg(windows)]
        {
            use std::os::windows::process::CommandExt;
            command.creation_flags(0x08000000); // CREATE_NO_WINDOW
        }
        let mut child = command
            .spawn()
            .map_err(|e| format!("无法启动传输组件: {e}"))?;
        let input = child.stdin.take();
        let output = child.stdout.take().ok_or("无法读取传输组件输出")?;
        let (sender, responses) = mpsc::channel();
        std::thread::spawn(move || {
            for line in BufReader::new(output).lines() {
                let value = line
                    .map_err(|e| e.to_string())
                    .and_then(|line| serde_json::from_str(&line).map_err(|e| e.to_string()));
                if sender.send(value).is_err() {
                    break;
                }
            }
        });
        Ok(Self {
            child,
            input,
            responses,
        })
    }

    fn request(&mut self, request: Value) -> Result<Value, String> {
        let input = self.input.as_mut().ok_or("传输组件已关闭")?;
        writeln!(input, "{request}")
            .and_then(|_| input.flush())
            .map_err(|e| format!("传输组件已退出: {e}"))?;
        let response = self
            .responses
            .recv_timeout(Duration::from_secs(15))
            .map_err(|e| format!("传输组件没有响应: {e}"))??;
        if let Some(error) = response.get("error").and_then(Value::as_str) {
            return Err(error.to_owned());
        }
        response
            .get("status")
            .cloned()
            .ok_or_else(|| "传输组件返回无效状态".to_owned())
    }
}

impl Drop for Backend {
    fn drop(&mut self) {
        // EOF tells Go to close listeners, cancel uploads and release its lock.
        self.input.take();
        for _ in 0..100 {
            if !matches!(self.child.try_wait(), Ok(None)) {
                return;
            }
            std::thread::sleep(Duration::from_millis(20));
        }
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

#[derive(Default)]
struct Service(Mutex<Option<Backend>>);

fn send(app: &tauri::AppHandle, service: &Service, request: Value) -> Result<Value, String> {
    let mut backend = service.0.lock().map_err(|_| "传输组件状态不可用")?;
    if backend.is_none() {
        *backend = Some(Backend::spawn(app)?);
    }
    let result = backend.as_mut().unwrap().request(request);
    // A timed-out pipe cannot be reused: a delayed response could be mistaken
    // for the next command. Drop it and let a later command launch a clean worker.
    if result.is_err()
        && (backend
            .as_mut()
            .unwrap()
            .child
            .try_wait()
            .ok()
            .flatten()
            .is_some()
            || result
                .as_ref()
                .err()
                .is_some_and(|e| e.starts_with("传输组件")))
    {
        backend.take();
    }
    result
}

async fn call(
    app: tauri::AppHandle,
    action: &'static str,
    settings: Option<Settings>,
) -> Result<Value, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let service = app.state::<Service>();
        send(
            &app,
            &service,
            json!({"action": action, "settings": settings}),
        )
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn service_status(app: tauri::AppHandle) -> Result<Value, String> {
    call(app, "status", None).await
}
#[tauri::command]
async fn start_service(app: tauri::AppHandle) -> Result<Value, String> {
    call(app, "start", None).await
}
#[tauri::command]
async fn stop_service(app: tauri::AppHandle) -> Result<Value, String> {
    call(app, "stop", None).await
}
#[tauri::command]
async fn save_settings(app: tauri::AppHandle, settings: Settings) -> Result<Value, String> {
    call(app, "configure", Some(settings)).await
}
#[tauri::command]
async fn choose_directory(language: Option<String>) -> Result<Option<String>, String> {
    Ok(rfd::AsyncFileDialog::new()
        .set_title(if language.as_deref() == Some("en") {
            "Choose shared folder"
        } else {
            "选择共享目录"
        })
        .pick_folder()
        .await
        .map(|folder| folder.path().to_string_lossy().into_owned()))
}
#[tauri::command]
async fn open_file_browser(app: tauri::AppHandle) -> Result<(), String> {
    let status = call(app.clone(), "status", None).await?;
    let url = status
        .get("localUrl")
        .and_then(Value::as_str)
        .filter(|url| !url.is_empty())
        .ok_or("请先启动传输服务")?;
    app.opener()
        .open_url(url, None::<&str>)
        .map_err(|e| e.to_string())
}
#[tauri::command]
async fn open_shared_directory(app: tauri::AppHandle) -> Result<(), String> {
    let status = call(app.clone(), "status", None).await?;
    let path = status["settings"]["directory"]
        .as_str()
        .ok_or("共享目录不可用")?;
    app.opener()
        .open_path(path, None::<&str>)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn open_product_feedback(app: tauri::AppHandle) -> Result<(), String> {
    app.opener()
        .open_url("https://api.ip21.cn/products/10/feedback", None::<&str>)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn open_github(app: tauri::AppHandle) -> Result<(), String> {
    app.opener()
        .open_url(
            "https://github.com/goldenwind/fangxu-file-transfer",
            None::<&str>,
        )
        .map_err(|e| e.to_string())
}

pub fn run() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _, _| {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.unminimize();
                let _ = window.set_focus();
            }
        }))
        .plugin(tauri_plugin_opener::init())
        .manage(Service::default())
        .setup(|app| {
            // Launch the worker during native startup, before the WebView asks
            // for status. Go immediately starts sharing when autoStart is on.
            match Backend::spawn(app.handle()) {
                Ok(backend) => *app.state::<Service>().0.lock().unwrap() = Some(backend),
                // Keep the settings UI available; status polling can retry and
                // surface a startup failure instead of closing the client.
                Err(error) => eprintln!("{error}"),
            }
            Ok(())
        })
        .on_window_event(|window, event| {
            if window.label() == "main" {
                if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                    // Closing the main window also quits on macOS, where an
                    // app can otherwise stay running without any windows.
                    api.prevent_close();
                    window.app_handle().exit(0);
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            service_status,
            start_service,
            stop_service,
            save_settings,
            choose_directory,
            open_file_browser,
            open_github,
            open_product_feedback,
            open_shared_directory
        ])
        .build(tauri::generate_context!())
        .expect("无法启动方序传文件客户端");
    app.run(|app, event| {
        if matches!(event, tauri::RunEvent::Exit) {
            if let Ok(mut backend) = app.state::<Service>().0.lock() {
                backend.take();
            }
        }
    });
}
