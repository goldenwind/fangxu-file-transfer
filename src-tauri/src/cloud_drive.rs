use serde::Deserialize;
use tauri::{webview::NewWindowResponse, Manager, WebviewUrl, WebviewWindowBuilder};
use tauri_plugin_opener::OpenerExt;
use url::Url;

const OPEN_ERROR: &str = "无法打开网盘，请尝试在浏览器中打开。";

#[derive(Clone, Copy, Deserialize, PartialEq, Eq)]
pub(super) enum Provider {
    #[serde(rename = "baidu")]
    Baidu,
    #[serde(rename = "quark")]
    Quark,
    #[serde(rename = "alipan")]
    Alipan,
    #[serde(rename = "weiyun")]
    Weiyun,
    #[serde(rename = "mobile")]
    Mobile,
    #[serde(rename = "123pan")]
    Pan123,
    #[serde(rename = "tianyi")]
    Tianyi,
    #[serde(rename = "xunlei")]
    Xunlei,
    #[serde(rename = "115pan")]
    Pan115,
}

impl Provider {
    #[cfg(target_os = "macos")]
    const ALL: [Self; 9] = [
        Self::Baidu,
        Self::Quark,
        Self::Alipan,
        Self::Weiyun,
        Self::Xunlei,
        Self::Tianyi,
        Self::Mobile,
        Self::Pan123,
        Self::Pan115,
    ];

    fn id(self) -> &'static str {
        match self {
            Self::Baidu => "baidu",
            Self::Quark => "quark",
            Self::Alipan => "alipan",
            Self::Weiyun => "weiyun",
            Self::Mobile => "mobile",
            Self::Pan123 => "123pan",
            Self::Tianyi => "tianyi",
            Self::Xunlei => "xunlei",
            Self::Pan115 => "115pan",
        }
    }

    fn url(self) -> Result<Url, String> {
        Url::parse(match self {
            Self::Baidu => "https://pan.baidu.com/disk/main",
            Self::Quark => "https://pan.quark.cn/",
            Self::Alipan => "https://www.alipan.com/drive",
            Self::Weiyun => "https://www.weiyun.com/",
            Self::Mobile => "https://yun.139.com/w/",
            Self::Pan123 => "https://yun.123pan.cn/?notoken=1",
            Self::Tianyi => "https://cloud.189.cn/web/main",
            Self::Xunlei => "https://pan.xunlei.com/",
            Self::Pan115 => "https://f.115.com/",
        })
        .map_err(|_| OPEN_ERROR.to_owned())
    }

    fn allows_navigation(self, url: &Url) -> bool {
        if url.scheme() != "https"
            || url.port().is_some()
            || !url.username().is_empty()
            || url.password().is_some()
        {
            return false;
        }
        match self {
            Self::Baidu => matches!(
                url.host_str(),
                Some("pan.baidu.com" | "passport.baidu.com" | "wappass.baidu.com")
            ),
            Self::Quark => matches!(url.host_str(), Some("pan.quark.cn" | "uop.quark.cn")),
            Self::Alipan => matches!(
                url.host_str(),
                Some("www.alipan.com" | "auth.alipan.com" | "passport.alipan.com")
            ),
            Self::Weiyun => matches!(
                url.host_str(),
                Some("www.weiyun.com" | "open.weixin.qq.com" | "xui.ptlogin2.qq.com")
            ),
            Self::Mobile => matches!(
                url.host_str(),
                Some("yun.139.com" | "caiyun.feixin.10086.cn")
            ),
            Self::Pan123 => matches!(url.host_str(), Some("yun.123pan.cn" | "user.123pan.cn")),
            Self::Tianyi => matches!(url.host_str(), Some("cloud.189.cn" | "open.e.189.cn")),
            Self::Xunlei => matches!(url.host_str(), Some("pan.xunlei.com")),
            Self::Pan115 => matches!(
                url.host_str(),
                Some("f.115.com" | "115.com" | "passport.115.com")
            ),
        }
    }

    #[cfg(target_os = "macos")]
    fn pane_label(self) -> String {
        format!("cloud-drive-pane-{}", self.id())
    }
}

#[tauri::command]
pub(super) async fn open_cloud_drive(
    app: tauri::AppHandle,
    provider: Provider,
    language: String,
    external: bool,
) -> Result<(), String> {
    if external {
        return app
            .opener()
            .open_url(provider.url()?.to_string(), None::<&str>)
            .map_err(|_| OPEN_ERROR.to_owned());
    }

    let label = format!("cloud-drive-{}", provider.id());
    if let Some(window) = app.get_webview_window(&label) {
        window.unminimize().map_err(|_| OPEN_ERROR)?;
        window.show().map_err(|_| OPEN_ERROR)?;
        return window.set_focus().map_err(|_| OPEN_ERROR.to_owned());
    }

    WebviewWindowBuilder::new(&app, label, WebviewUrl::External(provider.url()?))
        .title(if language == "en" {
            "Cloud Drive"
        } else {
            "网盘发送"
        })
        .inner_size(1080.0, 740.0)
        .min_inner_size(600.0, 460.0)
        .center()
        .disable_drag_drop_handler()
        .on_navigation(move |url| provider.allows_navigation(url))
        .on_new_window(|_, _| NewWindowResponse::Deny)
        .build()
        .map_err(|_| OPEN_ERROR)?;
    Ok(())
}

#[cfg(target_os = "macos")]
#[tauri::command]
pub(super) async fn set_cloud_drive_view(
    webview: tauri::Webview,
    provider: Option<Provider>,
    bounds: Option<super::WebviewBounds>,
) -> Result<(), String> {
    use tauri::{webview::WebviewBuilder, Rect};

    let window = webview.window();
    if webview.label() != "main" || window.label() != "main" {
        return Err(OPEN_ERROR.to_owned());
    }

    // Retain mounted sites when changing tabs so that uploads and logins can continue.
    let selected = provider.zip(bounds);
    let app = webview.app_handle();
    for site in Provider::ALL {
        if selected
            .as_ref()
            .is_some_and(|(provider, _)| site == *provider)
        {
            continue;
        }
        if let Some(pane) = app.get_webview(&site.pane_label()) {
            pane.hide().map_err(|_| OPEN_ERROR)?;
        }
    }
    let Some((provider, bounds)) = selected else {
        return Ok(());
    };

    let label = provider.pane_label();
    let (position, size) = bounds.logical_parts();
    if let Some(pane) = app.get_webview(&label) {
        pane.set_bounds(Rect {
            position: position.into(),
            size: size.into(),
        })
        .map_err(|_| OPEN_ERROR)?;
        return pane.show().map_err(|_| OPEN_ERROR.to_owned());
    }

    let builder = WebviewBuilder::new(label, WebviewUrl::External(provider.url()?))
        .focused(false)
        .disable_drag_drop_handler()
        .on_navigation(move |url| provider.allows_navigation(url))
        .on_new_window(|_, _| NewWindowResponse::Deny);
    window
        .add_child(builder, position, size)
        .map_err(|_| OPEN_ERROR)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn provider_ids_open_only_the_expected_https_sites() {
        for (id, host) in [
            ("baidu", "pan.baidu.com"),
            ("quark", "pan.quark.cn"),
            ("alipan", "www.alipan.com"),
            ("weiyun", "www.weiyun.com"),
            ("xunlei", "pan.xunlei.com"),
            ("tianyi", "cloud.189.cn"),
            ("mobile", "yun.139.com"),
            ("123pan", "yun.123pan.cn"),
            ("115pan", "f.115.com"),
        ] {
            let provider: Provider = serde_json::from_str(&format!("\"{id}\"")).unwrap();
            let home = provider.url().unwrap();
            assert_eq!(provider.id(), id);
            assert_eq!(home.host_str(), Some(host));
            assert!(provider.allows_navigation(&home));
            for disallowed in [
                format!("http://{host}"),
                format!("https://{host}:8443"),
                format!("https://user@{host}"),
                format!("https://{host}.example.com"),
            ] {
                assert!(!provider.allows_navigation(&Url::parse(&disallowed).unwrap()));
            }
        }
    }
}
