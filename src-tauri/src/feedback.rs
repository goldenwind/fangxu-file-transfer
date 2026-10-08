use tauri::{Manager, WebviewUrl, WebviewWindowBuilder};

const FEEDBACK_WINDOW: &str = "product-feedback";
const FEEDBACK_URL: &str = "https://api.ip21.cn/products/10/feedback";

pub(crate) struct Environment {
    app_version: String,
    os: String,
    os_version: String,
    arch: String,
}

impl Environment {
    pub(crate) fn current(app_version: &str) -> Self {
        let info = os_info::get();
        let os_version = if info.version() == &os_info::Version::Unknown {
            String::new()
        } else if cfg!(target_os = "macos") {
            info.version().to_string()
        } else {
            // Include the Linux distribution or Windows edition for diagnosis.
            info.to_string()
        };
        let arch = match info.architecture().unwrap_or(std::env::consts::ARCH) {
            "arm64" | "ARM64" => "aarch64",
            "amd64" | "AMD64" => "x86_64",
            arch => arch,
        };
        Self {
            app_version: app_version.to_owned(),
            os: std::env::consts::OS.to_owned(),
            os_version,
            arch: arch.to_owned(),
        }
    }

    pub(crate) fn url(&self) -> String {
        // These limits match gf_api's product feedback contract. Fragments
        // reach the feedback page without appearing in HTTP request URLs.
        let fields = [
            ("app_version", &self.app_version, 64),
            ("os", &self.os, 32),
            ("os_version", &self.os_version, 128),
            ("arch", &self.arch, 32),
        ];
        let mut fragment = url::form_urlencoded::Serializer::new(String::new());
        for (name, value, limit) in fields {
            let value: String = value.trim().chars().take(limit).collect();
            if !value.is_empty() {
                fragment.append_pair(name, &value);
            }
        }
        format!("{FEEDBACK_URL}#{}", fragment.finish())
    }
}

#[tauri::command]
pub async fn open_product_feedback(
    app: tauri::AppHandle,
    language: Option<String>,
) -> Result<(), String> {
    // Native window creation must run outside the command/UI thread on Windows.
    // Keep system detection independent of the transfer service as well.
    tauri::async_runtime::spawn_blocking(move || {
        let title = if language.as_deref() == Some("en") {
            "Product feedback · Fangxu File Transfer"
        } else {
            "产品反馈 · 方序传文件"
        };
        if let Some(window) = app.get_webview_window(FEEDBACK_WINDOW) {
            // Reuse the page without navigating, preserving the user's draft/login.
            window.set_title(title)?;
            window.unminimize()?;
            window.show()?;
            return window.set_focus();
        }
        let environment = Environment::current(&app.package_info().version.to_string());
        let url = tauri::Url::parse(&environment.url()).expect("valid product feedback URL");
        // Load the remote page as its own webview, so its login, storage and file
        // input work normally. Only the main window has native capabilities.
        WebviewWindowBuilder::new(&app, FEEDBACK_WINDOW, WebviewUrl::External(url))
            .title(title)
            .inner_size(560.0, 835.0)
            .min_inner_size(560.0, 580.0)
            .center()
            .disable_drag_drop_handler()
            .build()
            .map(|_| ())
    })
    .await
    .map_err(|error| format!("无法打开产品反馈: {error}"))?
    .map_err(|error| format!("无法打开产品反馈: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    fn fields(url: &str) -> HashMap<String, String> {
        let url = url::Url::parse(url).unwrap();
        assert_eq!(url.scheme(), "https");
        assert_eq!(url.host_str(), Some("api.ip21.cn"));
        assert_eq!(url.path(), "/products/10/feedback");
        assert!(url.query().is_none());
        url::form_urlencoded::parse(url.fragment().unwrap().as_bytes())
            .into_owned()
            .collect()
    }

    #[test]
    fn environment_survives_fragment_encoding() {
        for (os, os_version, arch) in [
            ("macos", "15.6.1", "aarch64"),
            (
                "windows",
                "Windows 11 企业版 24H2 & build=26100+#",
                "x86_64",
            ),
            ("linux", "Ubuntu 24.04.1 LTS", "aarch64"),
        ] {
            let environment = Environment {
                app_version: "1.1.0+feedback".into(),
                os: os.into(),
                os_version: os_version.into(),
                arch: arch.into(),
            };
            assert_eq!(
                fields(&environment.url()),
                HashMap::from([
                    ("app_version".into(), "1.1.0+feedback".into()),
                    ("os".into(), os.into()),
                    ("os_version".into(), os_version.into()),
                    ("arch".into(), arch.into()),
                ])
            );
        }
    }

    #[test]
    fn environment_obeys_server_character_limits_and_omits_missing_values() {
        let environment = Environment {
            app_version: "a".repeat(65),
            os: "o".repeat(33),
            os_version: format!("  {}  ", "系".repeat(129)),
            arch: "r".repeat(33),
        };
        let parsed = fields(&environment.url());
        for (name, limit) in [
            ("app_version", 64),
            ("os", 32),
            ("os_version", 128),
            ("arch", 32),
        ] {
            assert_eq!(parsed[name].chars().count(), limit);
        }
        let environment = Environment {
            os_version: " \n ".into(),
            ..environment
        };
        assert!(!fields(&environment.url()).contains_key("os_version"));
    }

    #[test]
    fn current_environment_uses_native_system_information() {
        let environment = Environment::current("1.1.0");
        let parsed = fields(&environment.url());
        assert_eq!(parsed["app_version"], "1.1.0");
        assert_eq!(parsed["os"], std::env::consts::OS);
        assert!(!parsed["arch"].is_empty());
        if os_info::get().version() != &os_info::Version::Unknown {
            assert!(!parsed["os_version"].is_empty());
        }
    }
}
