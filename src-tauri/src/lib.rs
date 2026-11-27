use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    io::{BufRead, BufReader, Write},
    path::PathBuf,
    process::{Child, ChildStdin, Command, Stdio},
    sync::{mpsc, Mutex},
    time::Duration,
};
use tauri::{
    menu::{Menu, MenuItem, MenuItemBuilder},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager,
};
use tauri_plugin_opener::OpenerExt;

mod cloud_drive;
mod easychuan;
mod feedback;
mod relay;
mod updates;

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
    startup_pending: bool,
}

fn sidecar_command() -> Result<Command, String> {
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
    let command = Command::new(binary);
    #[cfg(windows)]
    let command = {
        use std::os::windows::process::CommandExt;
        let mut command = command;
        command.creation_flags(0x08000000); // CREATE_NO_WINDOW
        command
    };
    Ok(command)
}

impl Backend {
    fn spawn(app: &tauri::AppHandle) -> Result<Self, String> {
        let config = app
            .path()
            .app_config_dir()
            .map_err(|e| e.to_string())?
            .join("settings.json");
        let mut command = sidecar_command()?;
        command
            .arg("--desktop")
            .arg("--config")
            .arg(config)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit());
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
        let mut backend = Self {
            child,
            input,
            responses,
            startup_pending: true,
        };
        backend.write_request(&json!({"action": "status"}))?;
        Ok(backend)
    }

    fn write_request(&mut self, request: &Value) -> Result<(), String> {
        let input = self.input.as_mut().ok_or("传输组件已关闭")?;
        writeln!(input, "{request}")
            .and_then(|_| input.flush())
            .map_err(|e| format!("传输组件已退出: {e}"))
    }

    fn response_status(response: Value) -> Result<Value, String> {
        if let Some(error) = response.get("error").and_then(Value::as_str) {
            return Err(error.to_owned());
        }
        response
            .get("status")
            .cloned()
            .ok_or_else(|| "传输组件返回无效状态".to_owned())
    }

    fn request(&mut self, request: Value) -> Result<Value, String> {
        if self.startup_pending {
            // A system folder-access prompt can wait for the user indefinitely.
            // Leave the worker running and let status polling collect its reply.
            let startup = match self.responses.try_recv() {
                Ok(response) => response,
                Err(mpsc::TryRecvError::Empty) if request["action"] == "status" => {
                    return Ok(Value::Null);
                }
                Err(mpsc::TryRecvError::Empty) => return Err("正在检查共享目录，请稍候".to_owned()),
                Err(mpsc::TryRecvError::Disconnected) => return Err("传输组件已退出".to_owned()),
            };
            self.startup_pending = false;
            let status = Self::response_status(startup?)?;
            if request["action"] == "status" {
                return Ok(status);
            }
        }
        self.write_request(&request)?;
        let response = self
            .responses
            .recv_timeout(Duration::from_secs(15))
            .map_err(|e| format!("传输组件没有响应: {e}"))??;
        Self::response_status(response)
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
fn open_github(app: tauri::AppHandle) -> Result<(), String> {
    app.opener()
        .open_url(
            "https://github.com/goldenwind/fangxu-file-transfer",
            None::<&str>,
        )
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn open_product_catalog(app: tauri::AppHandle) -> Result<(), String> {
    app.opener()
        .open_url("https://www.ip21.cn/works/", None::<&str>)
        .map_err(|_| "无法打开产品目录".to_owned())
}

#[cfg(target_os = "macos")]
#[tauri::command]
async fn set_native_window_controls_hidden(
    window: tauri::Window,
    hidden: bool,
) -> Result<(), String> {
    use objc2_app_kit::{NSWindow, NSWindowButton};

    let (sender, mut receiver) = tauri::async_runtime::channel(1);
    let native_window = window.clone();
    window
        .run_on_main_thread(move || {
            let result = native_window.ns_window().map(|pointer| {
                // SAFETY: Tauri owns the NSWindow, and AppKit is accessed on the main thread.
                let native = unsafe { &*pointer.cast::<NSWindow>() };
                for kind in [
                    NSWindowButton::CloseButton,
                    NSWindowButton::MiniaturizeButton,
                    NSWindowButton::ZoomButton,
                ] {
                    if let Some(button) = native.standardWindowButton(kind) {
                        button.setHidden(hidden);
                    }
                }
            });
            let _ = sender.try_send(result);
        })
        .map_err(|error| error.to_string())?;
    receiver
        .recv()
        .await
        .ok_or_else(|| "无法更新窗口按钮".to_owned())?
        .map_err(|error| error.to_string())
}

fn show_main_window(app: &tauri::AppHandle) {
    // After WeChat is attached, the main window contains multiple webviews.
    #[cfg(target_os = "macos")]
    let window = app.get_window("main");
    #[cfg(not(target_os = "macos"))]
    let window = app.get_webview_window("main");
    if let Some(window) = window {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}

struct TrayMenu {
    open: MenuItem<tauri::Wry>,
    quit: MenuItem<tauri::Wry>,
}

#[tauri::command]
fn set_tray_language(language: &str, menu: tauri::State<'_, TrayMenu>) -> Result<(), String> {
    let (open, quit) = if language == "en" {
        ("Open Main Window", "Quit")
    } else {
        ("打开主窗口", "退出")
    };
    menu.open
        .set_text(open)
        .map_err(|error| error.to_string())?;
    menu.quit.set_text(quit).map_err(|error| error.to_string())
}

fn setup_tray(app: &tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let open = MenuItemBuilder::with_id("tray-open", "打开主窗口").build(app)?;
    let quit = MenuItemBuilder::with_id("tray-quit", "退出").build(app)?;
    let menu = Menu::with_items(app, &[&open, &quit])?;
    let icon = app.default_window_icon().ok_or("找不到应用图标")?.clone();
    TrayIconBuilder::with_id("main-tray")
        .icon(icon)
        .tooltip("方序快传")
        .menu(&menu)
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id().as_ref() {
            "tray-open" => show_main_window(app),
            "tray-quit" => app.exit(0),
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if matches!(
                event,
                TrayIconEvent::Click {
                    button: MouseButton::Left,
                    button_state: MouseButtonState::Up,
                    ..
                }
            ) {
                show_main_window(tray.app_handle());
            }
        })
        .build(app)?;
    app.manage(TrayMenu { open, quit });
    Ok(())
}

#[cfg(target_os = "macos")]
#[derive(Deserialize)]
struct WebviewBounds {
    x: f64,
    y: f64,
    width: f64,
    height: f64,
}

#[cfg(target_os = "macos")]
impl WebviewBounds {
    fn logical_parts(self) -> (tauri::LogicalPosition<f64>, tauri::LogicalSize<f64>) {
        (
            tauri::LogicalPosition::new(self.x, self.y),
            tauri::LogicalSize::new(self.width, self.height),
        )
    }
}

pub fn run() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _, _| {
            show_main_window(app);
        }))
        .plugin(tauri_plugin_opener::init())
        .manage(Service::default())
        .manage(updates::UpdateState::default())
        .manage(relay::RelayState::default())
        .setup(|app| {
            setup_tray(app)?;
            // Launch the worker to check folder access during native startup.
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
                    api.prevent_close();
                    let _ = window.hide();
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
            open_product_catalog,
            #[cfg(target_os = "macos")]
            set_native_window_controls_hidden,
            set_tray_language,
            easychuan::register_easychuan_files,
            easychuan::register_easychuan_text,
            easychuan::register_easychuan_media,
            feedback::open_product_feedback,
            relay::choose_relay_files,
            relay::remove_relay_file,
            relay::start_relay_upload,
            relay::cancel_relay_upload,
            relay::open_relay_link,
            cloud_drive::open_cloud_drive,
            #[cfg(target_os = "macos")]
            cloud_drive::set_cloud_drive_view,
            relay::open_wechat_transfer,
            #[cfg(target_os = "macos")]
            relay::set_wechat_view,
            updates::check_product_update,
            updates::open_product_update,
            open_shared_directory
        ])
        .build(tauri::generate_context!())
        .expect("无法启动方序快传客户端");
    app.run(|app, event| match event {
        // The tray menu requests an explicit exit. Other quit requests, such as Cmd+Q, keep the service running.
        tauri::RunEvent::ExitRequested {
            code: None, api, ..
        } => api.prevent_exit(),
        tauri::RunEvent::Exit => {
            if let Ok(mut backend) = app.state::<Service>().0.lock() {
                backend.take();
            }
        }
        #[cfg(target_os = "macos")]
        tauri::RunEvent::Reopen {
            has_visible_windows: false,
            ..
        } => show_main_window(app),
        _ => {}
    });
}
